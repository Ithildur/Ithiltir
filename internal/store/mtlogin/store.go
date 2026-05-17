package mtlogin

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	backend mtprotoLoginBackend
}

type mtprotoLoginBackend interface {
	setMTProtoLogin(ctx context.Context, id string, raw []byte, ttl time.Duration) error
	getMTProtoLogin(ctx context.Context, id string) ([]byte, error)
	deleteMTProtoLogin(ctx context.Context, id string) error
}

func New(redisClient *redis.Client) *Store {
	mem := newMemory()
	return &Store{backend: newMTProtoLoginBackend(redisClient, mem)}
}
