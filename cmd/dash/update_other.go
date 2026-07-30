//go:build !linux

package main

import (
	"fmt"
	"os"
)

func runUpdate(_ []string) int {
	fmt.Fprintln(os.Stderr, "error: Dash self-update is supported only on Linux")
	return 1
}
