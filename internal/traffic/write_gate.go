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

func (g *writeGate) with(ctx context.Context, fn func(context.Context) error) error {
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
