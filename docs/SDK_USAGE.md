# GamifyKit SDK (Go) & Deployment Quickstart

## Install
Add to your module:
```bash
go get gamifykit/sdk/go
```

## Initialize the client
```go
import sdk "gamifykit/sdk/go"

client, err := sdk.NewClient("http://localhost:8080/api",
    sdk.WithAuthToken("your-token"), // optional
)
```

## Core calls
- Add points: `client.AddPoints(ctx, "alice", 50, "xp")`
- Award badge: `client.AwardBadge(ctx, "alice", "onboarded")`
- Get state: `client.GetUser(ctx, "alice")`
- Health: `client.Health(ctx)`
- Realtime: `events, _ := client.SubscribeEvents(ctx); range events { ... }`

See `examples/sdk-go` for a runnable sample.

## Running the API via container
Build or pull the image (published by the release workflow):
```bash
docker run -p 8080:8080 ghcr.io/OWNER/REPO/gamifykit:latest
```

Environment hints (via config package):
- `GAMIFYKIT_ENVIRONMENT`: development|production
- `GAMIFYKIT_SERVER_ADDRESS`: default `:8080`
- `GAMIFYKIT_SERVER_PATHPREFIX`: e.g., `/api`
- `GAMIFYKIT_SERVER_CORSORIGIN`: e.g., `*`
- `GAMIFYKIT_STORAGE_ADAPTER`: memory|redis|sql

For Redis/SQL adapters provide their respective env vars as defined in `config/README.md`.

