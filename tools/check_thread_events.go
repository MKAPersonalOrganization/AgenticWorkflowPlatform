package main

import (
	"context"
	"fmt"
	"log"

	"school-with-ai/infra/eventstore/kurrentdb"
)

func main() {
	// Connect to KurrentDB
	eventStore, err := kurrentdb.NewKurrentDBEventStore("esdb://localhost:2113?tls=false")
	if err != nil {
		log.Fatalf("Failed to connect to event store: %v", err)
	}
	defer eventStore.Close()

	// Check corrupted thread IDs
	threadIDs := []string{
		"thread_1768076006191_aqnncngc8",
		"thread_1768075803764_9q6v0zn83",
		"thread_1768075731444_rwhno3x46",
	}

	ctx := context.Background()

	for _, threadID := range threadIDs {
		streamName := fmt.Sprintf("Thread-%s", threadID)
		events, err := eventStore.ReadStream(ctx, streamName, 0)
		if err != nil {
			log.Printf("Error reading stream %s: %v", streamName, err)
			continue
		}

		fmt.Printf("\n=== Thread: %s ===\n", threadID)
		fmt.Printf("Total events: %d\n", len(events))

		if len(events) > 0 {
			lastEvent := events[len(events)-1]
			fmt.Printf("Last event type: %s\n", lastEvent.EventType())
		}
	}
}
