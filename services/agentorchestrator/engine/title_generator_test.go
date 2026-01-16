package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateThreadTitle(t *testing.T) {
	t.Parallel()

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
			name:     "Exactly 30 chars stays same",
			content:  "This is a message exactly 30..",
			expected: "This is a message exactly 30..",
		},
		{
			name:     "Long content is truncated",
			content:  "This is a message that is longer than thirty characters and should be truncated with an ellipsis.",
			expected: "This is a message that is l...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, GenerateThreadTitle(tt.content))
		})
	}
}
