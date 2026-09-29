package noderpc

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"dash/internal/nodesession"
	"dash/internal/nodewire"
	"dash/internal/virt"
)

func (s *Server) Connect(stream grpc.BidiStreamingServer[nodewire.NodeMessage, nodewire.DashMessage]) error {
	ctx := stream.Context()
	meta, server, err := s.authenticate(ctx)
	if err != nil {
		return err
	}
	if meta.stream == nil || !meta.stream.enter() {
		return status.Error(codes.Canceled, "stream closed")
	}
	defer meta.stream.leave()
	// The HTTP request owns these tasks and joins them after gRPC releases I/O.
	incoming := make(chan *nodewire.NodeMessage, 1)
	failures := make(chan error, 2)
	done := make(chan struct{})
	defer close(done)
	meta.stream.tasks.Go(func() {
		for {
			message, err := stream.Recv()
			if err != nil {
				failures <- err
				return
			}
			select {
			case incoming <- message:
			case <-done:
				return
			}
		}
	})
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	var first *nodewire.NodeMessage
	select {
	case first = <-incoming:
	case err := <-failures:
		return err
	case <-timer.C:
		return status.Error(codes.DeadlineExceeded, "capabilities timeout")
	case <-ctx.Done():
		return ctx.Err()
	}
	caps := first.GetCapabilities()
	if caps == nil || len(caps.Version) > 128 || len(caps.HelperVersion) > 128 {
		return status.Error(codes.InvalidArgument, "capabilities required")
	}
	session := s.sessions.Open(ctx, server.ID, meta.secret, nodesession.Capabilities{PVEHistory: caps.PveHistory, Version: caps.Version, HelperVersion: caps.HelperVersion})
	defer s.sessions.Remove(session)
	meta.stream.tasks.Go(func() {
		heartbeat := time.NewTicker(20 * time.Second)
		defer heartbeat.Stop()
		for {
			var message *nodewire.DashMessage
			select {
			case command := <-session.Commands():
				message = &nodewire.DashMessage{Id: command.ID}
				if command.Cancel {
					message.Body = &nodewire.DashMessage_Cancel{Cancel: true}
				} else {
					remaining := time.Until(command.Deadline)
					if remaining <= 0 {
						session.Complete(command.ID, nodesession.Result{Err: context.DeadlineExceeded})
						continue
					}
					message.Body = &nodewire.DashMessage_History{History: &nodewire.HistoryQuery{VmId: uint32(command.Query.VMID), Timeframe: command.Query.Timeframe, Consolidation: command.Query.Consolidation, TimeoutMs: uint32(max(1, remaining.Milliseconds()))}}
				}
			case <-heartbeat.C:
				message = &nodewire.DashMessage{Body: &nodewire.DashMessage_Heartbeat{Heartbeat: true}}
			case <-done:
				return
			}
			if err := stream.Send(message); err != nil {
				failures <- err
				return
			}
		}
	})
	check := time.NewTicker(time.Second)
	defer check.Stop()
	lastSeen := time.Now()
	for {
		select {
		case <-session.Context().Done():
			return status.Error(codes.Canceled, "session closed")
		case err := <-failures:
			return err
		case message := <-incoming:
			lastSeen = time.Now()
			result := message.GetResult()
			if result == nil {
				continue
			}
			if len(message.Id) > 64 || len(result.Json) > virt.HistoryMaxBytes {
				return status.Error(codes.ResourceExhausted, "query result exceeds limit")
			}
			var resultErr error
			switch result.Error {
			case "":
			case "busy":
				resultErr = nodesession.ErrBusy
			case "unsupported":
				resultErr = nodesession.ErrUnsupported
			case "vm_not_found":
				resultErr = nodesession.ErrVMNotFound
			case "deadline_exceeded":
				resultErr = context.DeadlineExceeded
			default:
				resultErr = nodesession.ErrSource
			}
			session.Complete(message.Id, nodesession.Result{JSON: result.Json, Err: resultErr})
		case <-check.C:
			authCtx, cancel := context.WithTimeout(ctx, time.Second)
			current, err := s.ingest.Authenticate(authCtx, meta.secret)
			cancel()
			if err != nil {
				return rpcError(err)
			}
			if current.ID != server.ID {
				return status.Error(codes.Unauthenticated, "credentials revoked")
			}
			if time.Since(lastSeen) > 60*time.Second {
				return status.Error(codes.DeadlineExceeded, "heartbeat timeout")
			}
		}
	}
}
