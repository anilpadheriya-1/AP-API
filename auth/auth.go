package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type AuthEngine struct {
	client *redis.Client
	ctx    context.Context
}

// In-memory mock for valid keys structure (Task 1 requirement, combined with Task 2)
var validKeys = map[string]bool{
	"key_mock123": true,
	"key_test456": true,
	"key_prod789": true,
}

func NewAuthEngine(redisAddr string) (*AuthEngine, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // no password set
		DB:       0,  // use default DB
		PoolSize: 100, // optimize connection pool
	})

	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}

	// Seed Redis with the hardcoded mock keys for Task 1/2 fusion
	for key := range validKeys {
		// Set key to exist with value "1"
		client.Set(ctx, "apikey:"+key, "1", 0)
	}

	return &AuthEngine{
		client: client,
		ctx:    ctx,
	}, nil
}

type AuthResult int

const (
	AuthOK AuthResult = iota
	AuthInvalidKey
	AuthRateLimited
	AuthError
)

// ValidateAndLimit checks if key exists and enforces rate limit (60 req / min)
func (e *AuthEngine) ValidateAndLimit(apiKey string) (AuthResult, error) {
	redisKey := "apikey:" + apiKey

	// 1. Check if key is valid
	exists, err := e.client.Exists(e.ctx, redisKey).Result()
	if err != nil {
		return AuthError, err
	}
	if exists == 0 {
		return AuthInvalidKey, nil
	}

	// 2. Sliding window / Token bucket rate limiter using simple INCR + EXPIRE
	// Simple counter for the current minute: "ratelimit:<apikey>:<minute>"
	currentMinute := time.Now().Unix() / 60
	rateLimitKey := fmt.Sprintf("ratelimit:%s:%d", apiKey, currentMinute)

	// Pipeline the INCR and EXPIRE to save roundtrips
	pipe := e.client.Pipeline()
	incrCmd := pipe.Incr(e.ctx, rateLimitKey)
	pipe.Expire(e.ctx, rateLimitKey, time.Minute*2) // keep around for a bit
	_, err = pipe.Exec(e.ctx)
	if err != nil {
		return AuthError, err
	}

	count := incrCmd.Val()
	if count > 60 {
		return AuthRateLimited, nil
	}

	return AuthOK, nil
}
