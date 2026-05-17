package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	themefs "dash/internal/theme"
)

func runPackTheme(args []string) {
	fs := flag.NewFlagSet("pack-theme", flag.ExitOnError)
	var src string
	var out string
	fs.StringVar(&src, "src", "", "theme source directory (required)")
	fs.StringVar(&out, "out", "", "output zip path (optional, default: <theme-id>.zip)")
	_ = fs.Parse(args)

	src = strings.TrimSpace(src)
	if src == "" {
		fmt.Fprintln(os.Stderr, "pack-theme: -src is required")
		fs.Usage()
		os.Exit(2)
	}

	manifest, archive, err := themefs.PackDir(src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pack-theme failed: %v\n", err)
		os.Exit(1)
	}

	out = strings.TrimSpace(out)
	if out == "" {
		out = manifest.ID + ".zip"
	}
	if !strings.EqualFold(filepath.Ext(out), ".zip") {
		out += ".zip"
	}
	out = filepath.Clean(out)

	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "pack-theme failed: create output directory: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(out, archive, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "pack-theme failed: write archive: %v\n", err)
		os.Exit(1)
	}

	absOut, err := filepath.Abs(out)
	if err != nil {
		absOut = out
	}
	fmt.Printf(
		"pack-theme: id=%s version=%s output=%s bytes=%d\n",
		manifest.ID,
		manifest.Version,
		absOut,
		len(archive),
	)
}
