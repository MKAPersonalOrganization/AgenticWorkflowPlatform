package agentorchestrator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTitleGenerator_FromFirstMessage(t *testing.T) {
	t.Parallel()
	tg := NewTitleGenerator()

	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "Short content stays same",
			content:  "Hello world",
			expected: "Hello world",
		},
		{
			name:     "Exactly 50 chars stays same",
			content:  "This is a message that is exactly fifty chars lo!!",
			expected: "This is a message that is exactly fifty chars lo!!",
		},
		{
			name:     "Long content is truncated",
			content:  "This is a message that is longer than fifty characters and should be truncated with an ellipsis.",
			expected: "This is a message that is longer than fifty charac...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, tg.FromFirstMessage(tt.content))
		})
	}
}
