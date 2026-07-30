package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"dash/internal/config"
	"dash/internal/infra"
)

const (
	redisCheckLimit     = 12 * time.Second
	redisOperationLimit = 8 * time.Second
	redisPasswordLimit  = 4 << 10
)

func runRedisCheck(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("check-redis", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var addr string
	var passwordFile string
	fs.StringVar(&addr, "addr", "", "Redis address")
	fs.StringVar(&passwordFile, "password-file", "", "file containing the Redis password")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "check-redis does not accept positional arguments")
		return 2
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		fmt.Fprintln(stderr, "Redis address is required")
		return 2
	}

	password, err := readRedisPassword(passwordFile)
	if err != nil {
		fmt.Fprintf(stderr, "read Redis password file: %v\n", err)
		return 2
	}
	client, err := infra.NewRedisClient(config.RedisConfig{Addr: addr, Password: password})
	if err != nil {
		fmt.Fprintf(stderr, "create Redis client: %v\n", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), redisCheckLimit)
	version, checkErr := infra.CheckRedis(ctx, client, redisOperationLimit)
	cancel()
	closeErr := client.Close()
	if err := errors.Join(checkErr, closeErr); err != nil {
		fmt.Fprintf(stderr, "check Redis %s: %v\n", addr, err)
		return 1
	}
	fmt.Fprintf(stdout, "Redis %s\n", version)
	return 0
}

func readRedisPassword(file string) (string, error) {
	file = strings.TrimSpace(file)
	if file == "" {
		return "", nil
	}
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	info, statErr := f.Stat()
	if statErr != nil {
		return "", errors.Join(statErr, f.Close())
	}
	if !info.Mode().IsRegular() {
		return "", errors.Join(fmt.Errorf("%s is not a regular file", file), f.Close())
	}
	if info.Size() > redisPasswordLimit {
		return "", errors.Join(fmt.Errorf("%s exceeds %d bytes", file, redisPasswordLimit), f.Close())
	}
	raw, readErr := io.ReadAll(io.LimitReader(f, redisPasswordLimit+1))
	closeErr := f.Close()
	if err := errors.Join(readErr, closeErr); err != nil {
		return "", err
	}
	if len(raw) > redisPasswordLimit {
		return "", fmt.Errorf("%s exceeds %d bytes", file, redisPasswordLimit)
	}
	password := string(raw)
	if strings.ContainsAny(password, "\x00\r\n") {
		return "", errors.New("redis password file must contain exactly one line without a trailing newline")
	}
	return password, nil
}
