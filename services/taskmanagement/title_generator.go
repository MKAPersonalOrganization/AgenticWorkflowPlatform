package taskmanagement

// TitleGenerator handles the logic for generating thread titles.
type TitleGenerator struct{}

// NewTitleGenerator creates a new TitleGenerator.
func NewTitleGenerator() *TitleGenerator {
	return &TitleGenerator{}
}

// FromFirstMessage generates a thread title from the content of the first message.
// It truncates the content to 50 characters and adds an ellipsis if needed.
func (tg *TitleGenerator) FromFirstMessage(content string) string {
	title := content
	if len(title) > 50 {
		title = title[:50] + "..."
	}

	return title
}
