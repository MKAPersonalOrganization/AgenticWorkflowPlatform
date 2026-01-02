package sdk

import (
	"encoding/json"
	"fmt"
)

// EventStreamDebugger provides utilities for debugging and analyzing event streams
type EventStreamDebugger struct {
	log EventLog
}

// NewEventStreamDebugger creates a new event stream debugger
func NewEventStreamDebugger(log EventLog) *EventStreamDebugger {
	return &EventStreamDebugger{
		log: log,
	}
}

// PrintEventStream prints all events for a thread in a human-readable format
func (d *EventStreamDebugger) PrintEventStream(threadID string) error {
	events, err := d.log.History(threadID)
	if err != nil {
		return err
	}

	fmt.Printf("\n=== Event Stream for Thread: %s ===\n", threadID)
	fmt.Printf("Total Events: %d\n\n", len(events))

	for i, evt := range events {
		fmt.Printf("[%d] %s at %s\n",
			i+1,
			evt.EventType(),
			evt.EventTimestamp().Format("2006-01-02 15:04:05.000"))

		// Print event details based on type
		switch e := evt.(type) {
		case *ThreadCreatedEvent:
			fmt.Printf("    Title: %s\n", e.Title)
		case *UserMessageAddedEvent:
			fmt.Printf("    MessageID: %s\n", e.MessageID)
			fmt.Printf("    Content: %s\n", truncate(e.Content, 100))
		case *AssistantMessageCompletedEvent:
			fmt.Printf("    MessageID: %s\n", e.MessageID)
			fmt.Printf("    Content: %s\n", truncate(e.Content, 100))
			if len(e.ToolCalls) > 0 {
				fmt.Printf("    ToolCalls: %d\n", len(e.ToolCalls))
			}
		case *ToolCallExecutedEvent:
			fmt.Printf("    ToolCallID: %s\n", e.ToolCallID)
			fmt.Printf("    ToolName: %s\n", e.ToolName)
			fmt.Printf("    Arguments: %s\n", truncate(e.ToolArguments, 100))
			fmt.Printf("    Result: %s\n", truncate(e.ToolResult, 100))
		case *ThreadTitleUpdatedEvent:
			fmt.Printf("    NewTitle: %s\n", e.NewTitle)
		case *ThreadDeletedEvent:
			fmt.Printf("    [Thread marked as deleted]\n")
		}
		fmt.Println()
	}

	return nil
}

// ExportEventsAsJSON exports all events for a thread as JSON
func (d *EventStreamDebugger) ExportEventsAsJSON(threadID string) (string, error) {
	events, err := d.log.History(threadID)
	if err != nil {
		return "", err
	}

	jsonBytes, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return "", err
	}

	return string(jsonBytes), nil
}

// GetEventStatistics returns statistics about the event stream
func (d *EventStreamDebugger) GetEventStatistics(threadID string) (map[string]int, error) {
	events, err := d.log.History(threadID)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]int)
	stats["TotalEvents"] = len(events)

	for _, evt := range events {
		eventType := evt.EventType()
		stats[eventType]++
	}

	return stats, nil
}

// truncate truncates a string to maxLen characters
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
