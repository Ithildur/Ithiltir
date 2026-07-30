//go:build linux

package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"dash/internal/dashupdate"
)

func runUpdate(args []string) int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return dashupdate.RunCommand(ctx, args, os.Stdin, os.Stdout, os.Stderr)
}
