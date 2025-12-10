package main

import (
	"context"
	"fmt"
	"log"
	"time"

	sdk "gamifykit/sdk/go"
)

func main() {
	client, err := sdk.NewClient("http://localhost:8080/api")
	if err != nil {
		log.Fatalf("create client: %v", err)
	}

	ctx := context.Background()
	userID := "alice"

	// Add XP
	total, err := client.AddPoints(ctx, userID, 50, "xp")
	if err != nil {
		log.Fatalf("add points: %v", err)
	}
	fmt.Printf("User %s now has %d xp\n", userID, total)

	// Award badge
	if err := client.AwardBadge(ctx, userID, "onboarded"); err != nil {
		log.Fatalf("award badge: %v", err)
	}

	// Fetch state
	state, err := client.GetUser(ctx, userID)
	if err != nil {
		log.Fatalf("get user: %v", err)
	}
	fmt.Printf("State: points=%v badges=%v levels=%v updated=%s\n", state.Points, state.Badges, state.Levels, state.Updated.Format(time.RFC3339))

	// Listen for realtime events for a short period
	eventsCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	events, err := client.SubscribeEvents(eventsCtx)
	if err != nil {
		log.Fatalf("subscribe: %v", err)
	}

	for evt := range events {
		fmt.Printf("event: %s user=%s delta=%d total=%d badge=%s level=%d\n",
			evt.Type, evt.UserID, evt.Delta, evt.Total, evt.Badge, evt.Level)
	}
}
