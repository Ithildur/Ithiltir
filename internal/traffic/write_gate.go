package traffic

import "context"

type writeGate struct {
	token chan struct{}
}

func newWriteGate() *writeGate {
	g := &writeGate{token: make(chan struct{}, 1)}
	g.token <- struct{}{}
	return g
}

func writeGateOrNew(g *writeGate) *writeGate {
	if g != nil {
		return g
	}
	return newWriteGate()
}

func (g *writeGate) with(ctx context.Context, fn func(context.Context) error) error {
	if g == nil {
		return fn(ctx)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-g.token:
	}
	defer func() {
		g.token <- struct{}{}
	}()
	return fn(ctx)
}
