package cache

import (
	"context"
	"testing"

	"github.com/go-redis/redis/v8"
)

func TestSets(t *testing.T) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	client := &client[string, string]{
		redis:       redisClient,
		cfg:         KeyspaceConfig{},
		keyMapper:   func(k string) string { return k },
		valueMapper: func(val string) (string, error) { return val, nil },
	}

	ctx := context.Background()
	ks := &SetKeyspace[string, string]{c: client}

	added, err := ks.Add(ctx, "foo", "one", "two")
	if err != nil {
		t.Errorf("Add() unexpected error: %v", err)
	} else if added != 2 {
		t.Errorf("Add() = %d, want %d", added, 2)
	}
}
