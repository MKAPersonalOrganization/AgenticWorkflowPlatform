package icon

import (
	"context"
	"fmt"
	"io"

	"github.com/a-h/templ"
)

// CompassLogo renders the custom SVG compass logo (not the Lucide icon).
// It uses the branded compass with outer ring, needle, and center dot.
// The class parameter controls sizing (e.g., "size-4", "size-5", "size-12").
func CompassLogo(class string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		svg := fmt.Sprintf(`<svg class="%s" viewBox="0 0 32 32" fill="none" xmlns="http://www.w3.org/2000/svg">`+
			`<circle cx="16" cy="16" r="15" stroke="currentColor" stroke-width="1.5" fill="none"></circle>`+
			`<path d="M16 6 L19 14 L16 12.5 L13 14 Z" fill="currentColor"></path>`+
			`<circle cx="16" cy="16" r="2" fill="currentColor"></circle>`+
			`</svg>`, class)
		_, err := w.Write([]byte(svg))
		return err
	})
}
