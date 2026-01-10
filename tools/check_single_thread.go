package main

import (
"context"
"fmt"
"log"

"school-with-ai/infra/eventstore/kurrentdb"
)

func main() {
eventStore, err := kurrentdb.NewKurrentDBEventStore("esdb://localhost:2113?tls=false")
if err != nil {
log.Fatalf("Failed to connect: %v", err)
}
defer eventStore.Close()

threadID := "thread_1768074493250_y38rmbhzj"
streamName := fmt.Sprintf("Thread-%s", threadID)

ctx := context.Background()
events, err := eventStore.ReadStream(ctx, streamName, 0)
if err != nil {
log.Fatalf("Error reading: %v", err)
}

fmt.Printf("Thread: %s\n", threadID)
fmt.Printf("Total events: %d\n\n", len(events))

for i, event := range events {
fmt.Printf("Event %d: %s - AggregateID: %q\n", i, event.EventType(), event.AggregateID())
}
}
