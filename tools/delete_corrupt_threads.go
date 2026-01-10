package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"school-with-ai/domain"
	"school-with-ai/infra/eventstore/kurrentdb"
)

func main() {
	// Connect to KurrentDB
	eventStore, err := kurrentdb.NewKurrentDBEventStore("esdb://localhost:2113?tls=false")
	if err != nil {
		log.Fatalf("Failed to connect to event store: %v", err)
	}
	defer eventStore.Close()

	// List of corrupted thread IDs
	corruptedThreads := []string{
		"thread_1768076006191_aqnncngc8",
		"thread_1768075803764_9q6v0zn83",
		"thread_1768075731444_rwhno3x46",
	}

	ctx := context.Background()

	for _, threadID := range corruptedThreads {
		fmt.Printf("Deleting corrupted thread: %s\n", threadID)

		// Create ThreadDeleted event
		event := &domain.ThreadDeleted{
			ThreadID:  threadID,
			Timestamp: time.Now(),
		}

		// Append directly to stream without loading (use -1 for Any version)
		streamName := fmt.Sprintf("Thread-%s", threadID)
		_, err := eventStore.AppendWithVersion(ctx, streamName, -1, event)
		if err != nil {
			log.Printf("Failed to delete thread %s: %v", threadID, err)
			continue
		}

		fmt.Printf("✓ Successfully deleted thread: %s\n", threadID)
	}

	fmt.Println("\nDone! Corrupted threads have been marked as deleted.")
}
