package engine

// GenerateThreadTitle produces a thread title from the content of the first message.
func GenerateThreadTitle(content string) string {
	if len(content) > 30 {
		return content[:27] + "..."
	}
	return content
}
