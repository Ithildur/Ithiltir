// Package noderpc adapts the node RPC protocol to the existing ingest boundary.
package noderpc

import (
	"context"
	"encoding/json/v2"
	"errors"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/Ithildur/EiluneKit/http/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"dash/internal/config"
	"dash/internal/http/request"
	"dash/internal/metrics"
	"dash/internal/model"
	"dash/internal/nodeingest"
	"dash/internal/nodesession"
	"dash/internal/nodewire"
	"dash/internal/virt"
)

type receipt struct {
	at     time.Time
	secret string
	ip     netip.Addr
	server model.Server
	stream *queryStream
}
type receiptKey struct{}

type Server struct {
	nodewire.UnimplementedNodeServer
	rpc        *grpc.Server
	ingest     *nodeingest.Receiver
	sessions   *nodesession.Hub
	failedAuth http.Handler
}

func New(ingest *nodeingest.Receiver, sessions *nodesession.Hub, trustedProxies []netip.Prefix) *Server {
	s := &Server{ingest: ingest, sessions: sessions}
	s.rpc = grpc.NewServer(grpc.MaxRecvMsgSize(virt.MaxBytes+1024), grpc.MaxSendMsgSize(virt.HistoryMaxBytes+1024))
	s.failedAuth = middleware.RateLimit(middleware.RateLimitOptions{
		Requests: config.NodeRateLimitRequests, Window: config.NodeRateLimitWindow,
		KeyFunc: middleware.RateLimitKeyByIP(24, 40, middleware.RateLimitKeyOptions{TrustedProxies: trustedProxies}),
		OnLimit: func(w http.ResponseWriter, _ *http.Request) { writeStatus(w, codes.ResourceExhausted, "rate_limited") },
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { writeStatus(w, codes.Unauthenticated, "unauthorized") }))
	nodewire.RegisterNodeServer(s.rpc, s)
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ip, _ := request.NodeIP(r)
	meta := receipt{at: time.Now().UTC(), secret: r.Header.Get(request.NodeSecretHeader), ip: ip}
	authCtx, cancel := context.WithTimeout(r.Context(), config.PGWriteTimeout)
	server, err := s.ingest.Authenticate(authCtx, meta.secret)
	cancel()
	if err != nil {
		failure := rpcError(err)
		if status.Code(failure) == codes.Unauthenticated {
			s.failedAuth.ServeHTTP(w, r)
		} else {
			writeStatus(w, status.Code(failure), status.Convert(failure).Message())
		}
		return
	}
	meta.server = server
	if r.URL.Path == nodewire.Node_Connect_FullMethodName {
		flow, err := newQueryStream(w)
		if err != nil {
			writeStatus(w, codes.Internal, "stream deadlines unavailable")
			return
		}
		defer flow.close()
		meta.stream = flow
		w = flow
	}
	s.rpc.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), receiptKey{}, meta)))
}
func (s *Server) Close() { s.sessions.Close(); s.rpc.Stop() }
func IsRequest(r *http.Request) bool {
	return r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc")
}
func (s *Server) authenticate(ctx context.Context) (receipt, model.Server, error) {
	meta, ok := ctx.Value(receiptKey{}).(receipt)
	if !ok {
		return meta, model.Server{}, status.Error(codes.Unauthenticated, "unauthorized")
	}
	return meta, meta.server, nil
}

func writeStatus(w http.ResponseWriter, code codes.Code, message string) {
	w.Header().Set("Content-Type", "application/grpc")
	w.Header().Set("Grpc-Status", strconv.Itoa(int(code)))
	w.Header().Set("Grpc-Message", message)
	w.WriteHeader(http.StatusOK)
}
func reportContext(ctx context.Context) (context.Context, context.CancelFunc) {
	meta, _ := ctx.Value(receiptKey{}).(receipt)
	return context.WithDeadline(ctx, meta.at.Add(config.PGWriteTimeout))
}
func (s *Server) Identify(ctx context.Context, _ *nodewire.Empty) (*nodewire.Identity, error) {
	if _, _, err := s.authenticate(ctx); err != nil {
		return nil, err
	}
	identity, err := s.ingest.Identity(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	return &nodewire.Identity{InstallId: identity.InstallID, Created: identity.Created, ProtocolVersion: 1}, nil
}
func (s *Server) Metrics(ctx context.Context, in *nodewire.Report) (*nodewire.Reply, error) {
	ctx, cancel := reportContext(ctx)
	defer cancel()
	meta, server, err := s.authenticate(ctx)
	if err != nil {
		return nil, err
	}
	var report metrics.NodeReport
	if err := decode(in.Json, config.NodeMaxMetricsBodySize, &report); err != nil {
		return nil, err
	}
	result, err := s.ingest.Metrics(ctx, meta.secret, server, report, meta.at, meta.ip)
	if err != nil {
		return nil, rpcError(err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		return nil, status.Error(codes.Internal, "encode response")
	}
	return &nodewire.Reply{Json: raw}, nil
}
func (s *Server) Static(ctx context.Context, in *nodewire.Report) (*nodewire.Empty, error) {
	meta, server, err := s.authenticate(ctx)
	if err != nil {
		return nil, err
	}
	var snapshot metrics.StaticMetrics
	if err := decode(in.Json, config.NodeMaxMetricsBodySize, &snapshot); err != nil {
		return nil, err
	}
	if err := s.ingest.Static(ctx, meta.secret, server.ID, snapshot, meta.ip); err != nil {
		return nil, rpcError(err)
	}
	return &nodewire.Empty{}, nil
}
func (s *Server) Virt(ctx context.Context, in *nodewire.Report) (*nodewire.Empty, error) {
	ctx, cancel := reportContext(ctx)
	defer cancel()
	meta, server, err := s.authenticate(ctx)
	if err != nil {
		return nil, err
	}
	var snapshot virt.Snapshot
	if err := decode(in.Json, virt.MaxBytes, &snapshot); err != nil {
		return nil, err
	}
	if err := s.ingest.Virt(ctx, server.ID, meta.secret, snapshot, meta.at); err != nil {
		return nil, rpcError(err)
	}
	return &nodewire.Empty{}, nil
}
func decode(raw []byte, limit int64, v any) error {
	if int64(len(raw)) > limit {
		return status.Error(codes.ResourceExhausted, "body_too_large")
	}
	if err := json.Unmarshal(raw, v); err != nil {
		return status.Error(codes.InvalidArgument, "invalid_request")
	}
	return nil
}
func rpcError(err error) error {
	if err == nil {
		return nil
	}
	if failure, ok := errors.AsType[*nodeingest.Error](err); ok {
		code := codes.Internal
		switch failure.Code {
		case "unauthorized":
			code = codes.Unauthenticated
		case "invalid_metrics", "invalid_static_payload", "invalid_virt":
			code = codes.InvalidArgument
		case "service_unavailable", "virt_unavailable":
			code = codes.Unavailable
		}
		return status.Error(code, failure.Code)
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, "deadline_exceeded")
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, "canceled")
	}
	return status.Error(codes.Internal, "internal_error")
}
