package main

import (
	"bufio"
	"fmt"
	"io"
	"os"

	"dash/internal/version"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}

	switch args[0] {
	case "validate":
		if len(args) != 2 {
			usage(stderr)
			return 2
		}
		if err := version.Validate(args[1]); err != nil {
			fmt.Fprintf(stderr, "invalid version: %v\n", err)
			return 1
		}
		return 0
	case "channel":
		if len(args) != 2 {
			usage(stderr)
			return 2
		}
		channel, err := version.ChannelFor(args[1])
		if err != nil {
			fmt.Fprintf(stderr, "invalid version: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, channel)
		return 0
	case "compare":
		if len(args) != 3 {
			usage(stderr)
			return 2
		}
		cmp, err := version.Compare(args[1], args[2])
		if err != nil {
			fmt.Fprintf(stderr, "invalid version: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, cmp)
		return 0
	case "latest":
		if len(args) != 2 {
			usage(stderr)
			return 2
		}
		channel, err := version.ParseChannel(args[1])
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		versions, err := readLines(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "read versions: %v\n", err)
			return 1
		}
		latest, ok := version.Latest(versions, channel)
		if !ok {
			fmt.Fprintf(stderr, "no %s versions found\n", channel)
			return 1
		}
		fmt.Fprintln(stdout, latest)
		return 0
	default:
		usage(stderr)
		return 2
	}
}

func readLines(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: go run ./cmd/releasever validate VERSION | channel VERSION | compare A B | latest CHANNEL")
}
