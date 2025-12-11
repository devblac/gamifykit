package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"gamifykit/core"

	"github.com/redis/go-redis/v9"
)

// Config holds Redis connection configuration
type Config struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DefaultConfig returns sensible defaults for Redis configuration
func DefaultConfig() Config {
	return Config{
		Addr:         "localhost:6379",
		Password:     "",
		DB:           0,
		PoolSize:     10,
		MinIdleConns: 2,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// Store implements the engine.Storage interface using Redis as the backend.
// Data structure:
// - user:{user_id}:points:{metric} -> int64 (points total)
// - user:{user_id}:badges -> set of badge strings
// - user:{user_id}:levels:{metric} -> int64 (level)
// - user:{user_id}:state -> JSON blob of UserState for quick retrieval
type Store struct {
	client *redis.Client
}

// New creates a new Redis-backed storage with the provided configuration
func New(config Config) (*Store, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         config.Addr,
		Password:     config.Password,
		DB:           config.DB,
		PoolSize:     config.PoolSize,
		MinIdleConns: config.MinIdleConns,
		DialTimeout:  config.DialTimeout,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &Store{client: client}, nil
}

// NewWithClient creates a Store using an existing Redis client (useful for testing)
func NewWithClient(client *redis.Client) *Store {
	return &Store{client: client}
}

// Close closes the Redis connection
func (s *Store) Close() error {
	return s.client.Close()
}

// userPointsKey generates the Redis key for user points
func userPointsKey(userID core.UserID, metric core.Metric) string {
	return fmt.Sprintf("user:%s:points:%s", userID, metric)
}

// userBadgesKey generates the Redis key for user badges
func userBadgesKey(userID core.UserID) string {
	return fmt.Sprintf("user:%s:badges", userID)
}

// userLevelsKey generates the Redis key for user levels
func userLevelsKey(userID core.UserID, metric core.Metric) string {
	return fmt.Sprintf("user:%s:levels:%s", userID, metric)
}

// userStateKey generates the Redis key for cached user state
func userStateKey(userID core.UserID) string {
	return fmt.Sprintf("user:%s:state", userID)
}

// Lua script for atomic point addition with overflow protection
var addPointsScript = redis.NewScript(`
	local key = KEYS[1]
	local delta = tonumber(ARGV[1])
	local current = tonumber(redis.call('GET', key) or '0')
	local next_val = current + delta

	-- Check for overflow (simplified check for large numbers)
	if next_val > 9223372036854775807 or next_val < -9223372036854775808 then
		return redis.error_reply('integer overflow')
	end

	redis.call('SET', key, next_val)
	return next_val
`)

// AddPoints atomically adds points to a user's metric with overflow protection
func (s *Store) AddPoints(ctx context.Context, userID core.UserID, metric core.Metric, delta int64) (int64, error) {
	if delta == 0 {
		return 0, errors.New("delta cannot be zero")
	}

	key := userPointsKey(userID, metric)
	result, err := addPointsScript.Run(ctx, s.client, []string{key}, delta).Result()
	if err != nil {
		return 0, fmt.Errorf("failed to add points: %w", err)
	}

	total, ok := result.(int64)
	if !ok {
		return 0, errors.New("unexpected result type from Redis script")
	}

	// Invalidate cached state since it changed
	s.invalidateStateCache(ctx, userID)

	return total, nil
}

// AwardBadge adds a badge to the user's badge set
func (s *Store) AwardBadge(ctx context.Context, userID core.UserID, badge core.Badge) error {
	key := userBadgesKey(userID)
	err := s.client.SAdd(ctx, key, string(badge)).Err()
	if err != nil {
		return fmt.Errorf("failed to award badge: %w", err)
	}

	// Invalidate cached state since it changed
	s.invalidateStateCache(ctx, userID)

	return nil
}

// GetState retrieves the complete user state, using cache when possible
func (s *Store) GetState(ctx context.Context, userID core.UserID) (core.UserState, error) {
	// Try to get from cache first
	cached, err := s.getCachedState(ctx, userID)
	if err == nil {
		return cached, nil
	}

	// Cache miss or error, rebuild from individual keys
	state, err := s.buildStateFromKeys(ctx, userID)
	if err != nil {
		return core.UserState{}, err
	}

	// Update cache (best-effort); keep it synchronous for determinism.
	ctxCache, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	_ = s.updateStateCache(ctxCache, userID, state)

	return state, nil
}

// SetLevel sets the user's level for a specific metric
func (s *Store) SetLevel(ctx context.Context, userID core.UserID, metric core.Metric, level int64) error {
	key := userLevelsKey(userID, metric)
	err := s.client.Set(ctx, key, level, 0).Err()
	if err != nil {
		return fmt.Errorf("failed to set level: %w", err)
	}

	// Invalidate cached state since it changed
	s.invalidateStateCache(ctx, userID)

	return nil
}

// getCachedState attempts to retrieve the cached user state
func (s *Store) getCachedState(ctx context.Context, userID core.UserID) (core.UserState, error) {
	key := userStateKey(userID)
	data, err := s.client.Get(ctx, key).Bytes()
	if err != nil {
		return core.UserState{}, err
	}

	var state core.UserState
	if err := json.Unmarshal(data, &state); err != nil {
		return core.UserState{}, err
	}

	return state, nil
}

// updateStateCache stores the user state in cache with a TTL
func (s *Store) updateStateCache(ctx context.Context, userID core.UserID, state core.UserState) error {
	key := userStateKey(userID)
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	// Cache for 5 minutes
	return s.client.Set(ctx, key, data, 5*time.Minute).Err()
}

// invalidateStateCache removes the cached state
func (s *Store) invalidateStateCache(ctx context.Context, userID core.UserID) {
	s.client.Del(ctx, userStateKey(userID))
}

// buildStateFromKeys reconstructs the user state from individual Redis keys
func (s *Store) buildStateFromKeys(ctx context.Context, userID core.UserID) (core.UserState, error) {
	state := core.UserState{
		UserID:  userID,
		Points:  make(map[core.Metric]int64),
		Badges:  make(map[core.Badge]struct{}),
		Levels:  make(map[core.Metric]int64),
		Updated: time.Now().UTC(),
	}

	// Get all points
	pattern := fmt.Sprintf("user:%s:points:*", userID)
	keys, err := s.client.Keys(ctx, pattern).Result()
	if err != nil {
		return core.UserState{}, fmt.Errorf("failed to get points keys: %w", err)
	}

	for _, key := range keys {
		// Extract metric from key: user:{user_id}:points:{metric}
		parts := redisKeyParts(key)
		if len(parts) >= 4 && parts[2] == "points" {
			metric := core.Metric(parts[3])
			val, err := s.client.Get(ctx, key).Int64()
			if err != nil {
				continue // Skip invalid entries
			}
			state.Points[metric] = val
		}
	}

	// Get all badges
	badgesKey := userBadgesKey(userID)
	badges, err := s.client.SMembers(ctx, badgesKey).Result()
	if err == nil {
		for _, badge := range badges {
			state.Badges[core.Badge(badge)] = struct{}{}
		}
	}

	// Get all levels
	levelPattern := fmt.Sprintf("user:%s:levels:*", userID)
	levelKeys, err := s.client.Keys(ctx, levelPattern).Result()
	if err == nil {
		for _, key := range levelKeys {
			parts := redisKeyParts(key)
			if len(parts) >= 4 && parts[2] == "levels" {
				metric := core.Metric(parts[3])
				val, err := s.client.Get(ctx, key).Int64()
				if err != nil {
					continue
				}
				state.Levels[metric] = val
			}
		}
	}

	return state, nil
}

// redisKeyParts splits a Redis key by colon separator
func redisKeyParts(key string) []string {
	var parts []string
	current := ""
	for _, r := range key {
		if r == ':' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(r)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
