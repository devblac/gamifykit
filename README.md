GamifyKit

[![CI](https://github.com/devblac/gamifykit/actions/workflows/ci.yml/badge.svg)](https://github.com/devblac/gamifykit/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/devblac/gamifykit/branch/main/graph/badge.svg)](https://codecov.io/gh/devblac/gamifykit)
[![Go Report Card](https://goreportcard.com/badge/github.com/devblac/gamifykit)](https://goreportcard.com/report/github.com/devblac/gamifykit)
[![Go Reference](https://pkg.go.dev/badge/github.com/devblac/gamifykit.svg)](https://pkg.go.dev/github.com/devblac/gamifykit)
[![Latest Release](https://img.shields.io/github/v/release/devblac/gamifykit)](https://github.com/devblac/gamifykit/releases)

High-performance, modular gamification for Go.

### Overview
GamifyKit is a fast, composable gamification engine for Go 1.22+. It provides ultra-low-latency, horizontally scalable building blocks to add points, XP, levels, badges, achievements, challenges, leaderboards, realtime events, and analytics to any app with minimal code.

Key goals:
- Simplicity of API with strong domain types
- Safe concurrency and immutability of state snapshots
- Pluggable storage and rules
- Realtime-first event model

### Features
- Points and XP with overflow-safe arithmetic
- Levels with configurable rule engine (default XP→level progression)
- Badges and achievements via events and rules
- Event bus with sync/async dispatch modes
- Realtime hub and WebSocket adapter for streaming domain events
- In-memory storage adapter (production adapters sketched for Redis/SQLx)
- Leaderboard interfaces (Redis backing sketched)
- Analytics hooks (e.g., DAU aggregator)

### Install
Until this module is published under a VCS path, use local replace or set your module path. Once hosted:

```bash
go get github.com/devblac/gamifykit
```

### Quick Start
```go
package main

import (
    "context"

    "gamifykit/core"
    "gamifykit/gamify"
    "gamifykit/realtime"
)

func main() {
    ctx := context.Background()
    hub := realtime.NewHub()
    svc := gamify.New(
        gamify.WithRealtime(hub), // optional: stream events to subscribers/WebSocket
    )

    user := core.UserID("alice")
    _, _ = svc.AddPoints(ctx, user, core.MetricXP, 50)

    // Listen for when users level up
    unsub := svc.Subscribe(core.EventLevelUp, func(ctx context.Context, e core.Event) {
        // do something when someone levels up
        _ = e
    })
    defer unsub()
}
```

More examples in `docs/QuickStart.md` and `cmd/demo-server`.

### Architecture
- `core`: domain types, events, rules, and safe math utilities
- `engine`: orchestrates storage, rule evaluation, and event dispatch
- `gamify`: ergonomic builder for the engine with sensible defaults
- `adapters`: storage layers and transports (in-memory, Redis/SQLx placeholders, WebSocket, gRPC placeholder)
- `realtime`: lightweight pub/sub for broadcasting events
- `leaderboard`: interface and scaffolding for scoreboards
- `analytics`: hooks to aggregate KPIs (e.g., DAU)

### Storage adapters
- **In-memory**: production-grade for demos/tests, thread-safe
- **Redis**: complete implementation with connection pooling, atomic operations via Lua scripts, caching, and overflow protection
- **SQLx**: full implementation for PostgreSQL and MySQL with migrations, transactions, and concurrent access support

### Realtime
Use the `realtime.Hub` directly or the WebSocket adapter:

```go
http.Handle("/ws", ws.Handler(hub)) // stream events to clients
```

### Leaderboards
Efficient score tracking with Redis sorted sets:

```go
import "gamifykit/leaderboard"

// Create a Redis leaderboard
client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
board := leaderboard.NewRedisBoard(client, "game:scores")

// Update scores
board.Update("alice", 1500)
board.Update("bob", 1200)

// Get top players
topPlayers := board.TopN(10)

// Get specific player
if entry, exists := board.Get("alice"); exists {
    fmt.Printf("%s has %d points\n", entry.User, entry.Score)
}
```

### Demo server
Run a tiny HTTP server exposing points/badges and a WebSocket stream:

```bash
go run ./cmd/demo-server
```

Routes:
- POST `/users/{id}/points?metric=xp&delta=50`
- POST `/users/{id}/badges/{badge}`
- GET `/users/{id}`
- WS `/ws`

### One-command API server
Spin up a ready-to-use GamifyKit API with CORS and a `/healthz` endpoint:

```bash
go run ./cmd/gamifykit-server -addr :8080 -prefix /api -cors "*"
```

Endpoints:
- GET `/api/healthz`
- POST `/api/users/{id}/points?metric=xp&delta=50`
- POST `/api/users/{id}/badges/{badge}`
- GET `/api/users/{id}`
- WS `/api/ws`

Use this from a React app by calling the HTTP endpoints and subscribing to the WebSocket for realtime updates.

### Roadmap
- Production-ready Redis adapter for storage and leaderboard
- SQLx adapter
- Pluggable rule sets (achievements/challenges)
- OpenTelemetry spans/metrics
- gRPC/HTTP APIs with OpenAPI spec

### Versioning
Semantic Versioning once published. Current API may evolve.

### License
Apache-2.0. See `LICENSE`.

