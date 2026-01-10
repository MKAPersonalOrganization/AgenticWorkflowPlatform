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
eventStore, err := kurrentdb.NewKurrentDBEventStore("esdb://localhost:2113?tls=false")
if err != nil {
log.Fatalf("Failed to connect: %v", err)
}
defer eventStore.Close()

// Additional corrupted thread
threadID := "thread_1768074493250_y38rmbhzj"

ctx := context.Background()
fmt.Printf("Deleting corrupted thread: %s\n", threadID)

event := &domain.ThreadDeleted{
ThreadID:  threadID,
Timestamp: time.Now(),
}

streamName := fmt.Sprintf("Thread-%s", threadID)
_, err = eventStore.AppendWithVersion(ctx, streamName, -1, event)
if err != nil {
log.Fatalf("Failed to delete: %v", err)
}

fmt.Printf("✓ Successfully deleted thread: %s\n", threadID)
}
