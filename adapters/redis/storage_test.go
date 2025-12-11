package redis

import (
	"context"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gamifykit/core"
)

// newTestClient spins up a miniredis server and returns a client plus cleanup.
func newTestClient(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cleanup := func() {
		_ = client.Close()
		mr.Close()
	}
	return client, cleanup
}

func cleanupTestData(t *testing.T, client *redis.Client, userID core.UserID) {
	t.Helper()
	ctx := context.Background()
	pattern := "user:" + string(userID) + ":*"
	keys, err := client.Keys(ctx, pattern).Result()
	if err == nil && len(keys) > 0 {
		_, _ = client.Del(ctx, keys...).Result()
	}
}

func TestStore_AddPoints(t *testing.T) {
	client, cleanup := newTestClient(t)
	defer cleanup()

	store := NewWithClient(client)
	ctx := context.Background()

	userID := core.UserID("test-user")
	metric := core.MetricXP

	// Clean up
	defer cleanupTestData(t, client, userID)

	// Test adding points
	total, err := store.AddPoints(ctx, userID, metric, 50)
	require.NoError(t, err)
	assert.Equal(t, int64(50), total)

	// Test adding more points
	total, err = store.AddPoints(ctx, userID, metric, 25)
	require.NoError(t, err)
	assert.Equal(t, int64(75), total)

	// Test subtracting points
	total, err = store.AddPoints(ctx, userID, metric, -30)
	require.NoError(t, err)
	assert.Equal(t, int64(45), total)
}

func TestStore_AddPoints_ZeroDelta(t *testing.T) {
	// This test doesn't need Redis connection
	store := &Store{}
	ctx := context.Background()

	userID := core.UserID("test-user")
	metric := core.MetricXP

	_, err := store.AddPoints(ctx, userID, metric, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delta cannot be zero")
}

func TestStore_AwardBadge(t *testing.T) {
	client, cleanup := newTestClient(t)
	defer cleanup()

	store := NewWithClient(client)
	ctx := context.Background()

	userID := core.UserID("test-user")
	badge := core.Badge("first-win")

	// Clean up
	defer cleanupTestData(t, client, userID)

	// Test awarding badge
	err := store.AwardBadge(ctx, userID, badge)
	require.NoError(t, err)

	// Verify badge was added
	badges, err := client.SMembers(ctx, userBadgesKey(userID)).Result()
	require.NoError(t, err)
	assert.Contains(t, badges, string(badge))

	// Test awarding same badge again (should be idempotent)
	err = store.AwardBadge(ctx, userID, badge)
	require.NoError(t, err)

	// Should still only have one instance
	badges, err = client.SMembers(ctx, userBadgesKey(userID)).Result()
	require.NoError(t, err)
	assert.Len(t, badges, 1)
}

func TestStore_GetState(t *testing.T) {
	client, cleanup := newTestClient(t)
	defer cleanup()

	store := NewWithClient(client)
	ctx := context.Background()

	userID := core.UserID("test-user")

	// Clean up
	defer cleanupTestData(t, client, userID)

	// Set up some state
	_, err := store.AddPoints(ctx, userID, core.MetricXP, 100)
	require.NoError(t, err)
	_, err = store.AddPoints(ctx, userID, core.MetricPoints, 50)
	require.NoError(t, err)

	err = store.AwardBadge(ctx, userID, core.Badge("winner"))
	require.NoError(t, err)

	err = store.SetLevel(ctx, userID, core.MetricXP, 5)
	require.NoError(t, err)

	// Get state
	state, err := store.GetState(ctx, userID)
	require.NoError(t, err)

	assert.Equal(t, userID, state.UserID)
	assert.Equal(t, int64(100), state.Points[core.MetricXP])
	assert.Equal(t, int64(50), state.Points[core.MetricPoints])
	assert.Contains(t, state.Badges, core.Badge("winner"))
	assert.Equal(t, int64(5), state.Levels[core.MetricXP])
	assert.True(t, time.Since(state.Updated) < time.Second)
}

func TestStore_GetState_Cache(t *testing.T) {
	client, cleanup := newTestClient(t)
	defer cleanup()

	store := NewWithClient(client)
	ctx := context.Background()

	userID := core.UserID("test-user-cache")

	// Clean up
	defer cleanupTestData(t, client, userID)

	// Set up some state
	_, err := store.AddPoints(ctx, userID, core.MetricXP, 200)
	require.NoError(t, err)

	// First get should build from keys and cache
	state1, err := store.GetState(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(200), state1.Points[core.MetricXP])

	// Check cache was created
	cacheKey := userStateKey(userID)
	exists, err := client.Exists(ctx, cacheKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), exists)

	// Modify underlying data directly (simulating external change)
	pointsKey := userPointsKey(userID, core.MetricXP)
	err = client.Set(ctx, pointsKey, 300, 0).Err()
	require.NoError(t, err)

	// Second get should return cached data (old value)
	state2, err := store.GetState(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(200), state2.Points[core.MetricXP]) // Should be cached value

	// Add more points (this should invalidate cache)
	_, err = store.AddPoints(ctx, userID, core.MetricXP, 50)
	require.NoError(t, err)

	// Next get should rebuild from keys (new value)
	state3, err := store.GetState(ctx, userID)
	require.NoError(t, err)
	assert.Equal(t, int64(350), state3.Points[core.MetricXP])
}

func TestStore_SetLevel(t *testing.T) {
	client, cleanup := newTestClient(t)
	defer cleanup()

	store := NewWithClient(client)
	ctx := context.Background()

	userID := core.UserID("test-user")
	metric := core.MetricXP

	// Clean up
	defer cleanupTestData(t, client, userID)

	// Test setting level
	err := store.SetLevel(ctx, userID, metric, 10)
	require.NoError(t, err)

	// Verify level was set
	level, err := client.Get(ctx, userLevelsKey(userID, metric)).Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(10), level)

	// Test updating level
	err = store.SetLevel(ctx, userID, metric, 15)
	require.NoError(t, err)

	level, err = client.Get(ctx, userLevelsKey(userID, metric)).Int64()
	require.NoError(t, err)
	assert.Equal(t, int64(15), level)
}

func TestStore_EmptyUser(t *testing.T) {
	client, cleanup := newTestClient(t)
	defer cleanup()

	store := NewWithClient(client)
	ctx := context.Background()

	userID := core.UserID("nonexistent-user")

	// Clean up
	defer cleanupTestData(t, client, userID)

	// Get state for user that doesn't exist
	state, err := store.GetState(ctx, userID)
	require.NoError(t, err)

	assert.Equal(t, userID, state.UserID)
	assert.Empty(t, state.Points)
	assert.Empty(t, state.Badges)
	assert.Empty(t, state.Levels)
	assert.True(t, time.Since(state.Updated) < time.Second)
}

func TestRedisKeyParts(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"user:alice:points:xp", []string{"user", "alice", "points", "xp"}},
		{"user:bob:badges", []string{"user", "bob", "badges"}},
		{"simple", []string{"simple"}},
		{"a:b:c:d:e", []string{"a", "b", "c", "d", "e"}},
		{"trailing:", []string{"trailing"}},
		{":leading", []string{"leading"}},
		{"empty::parts", []string{"empty", "parts"}},
	}

	for _, test := range tests {
		result := redisKeyParts(test.input)
		assert.Equal(t, test.expected, result, "Failed for input: %s", test.input)
	}
}

func TestConfig_DefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "localhost:6379", config.Addr)
	assert.Equal(t, "", config.Password)
	assert.Equal(t, 0, config.DB)
	assert.Equal(t, 10, config.PoolSize)
	assert.Equal(t, 2, config.MinIdleConns)
	assert.Equal(t, 5*time.Second, config.DialTimeout)
	assert.Equal(t, 3*time.Second, config.ReadTimeout)
	assert.Equal(t, 3*time.Second, config.WriteTimeout)
}
