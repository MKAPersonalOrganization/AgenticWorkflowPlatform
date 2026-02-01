package engine

// GenerateThreadTitle produces a thread title from the content of the first message.
func GenerateThreadTitle(content string) string {
	runes := []rune(content)
	if len(runes) > 30 {
		return string(runes[:27]) + "..."
	}
	return string(runes)
}
