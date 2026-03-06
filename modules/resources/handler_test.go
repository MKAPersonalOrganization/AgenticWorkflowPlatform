package resources

import (
	"mime"
	"path/filepath"
	"strings"
	"testing"
)

// sanitizeFilename mirrors the logic applied in the handler to prevent
// Content-Disposition header injection.
func sanitizeFilename(title string) string {
	safe := filepath.Base(title)
	safe = strings.ReplaceAll(safe, "\"", "_")
	safe = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return '_'
		}
		return r
	}, safe)
	return safe
}

func TestContentDisposition_NormalFilename(t *testing.T) {
	t.Parallel()
	name := sanitizeFilename("report.pdf")
	disp := mime.FormatMediaType("attachment", map[string]string{"filename": name})
	if !strings.Contains(disp, "report.pdf") {
		t.Errorf("expected report.pdf in disposition, got %q", disp)
	}
}

func TestContentDisposition_QuoteInjection(t *testing.T) {
	t.Parallel()
	malicious := "foo\"; malicious-header: value"
	name := sanitizeFilename(malicious)
	disp := mime.FormatMediaType("attachment", map[string]string{"filename": name})
	if strings.Contains(disp, "\"; malicious-header") {
		t.Errorf("Content-Disposition header injection possible: %q", disp)
	}
}

func TestContentDisposition_NewlineInjection(t *testing.T) {
	t.Parallel()
	malicious := "file\r\nX-Injected: true"
	name := sanitizeFilename(malicious)
	if strings.ContainsAny(name, "\r\n") {
		t.Errorf("sanitized filename still contains newlines: %q", name)
	}
}

func TestContentDisposition_PathTraversal(t *testing.T) {
	t.Parallel()
	malicious := "../../../etc/passwd"
	name := sanitizeFilename(malicious)
	if strings.Contains(name, "..") {
		t.Errorf("sanitized filename still contains path traversal: %q", name)
	}
	if name != "passwd" {
		t.Errorf("expected base filename 'passwd', got %q", name)
	}
}

func TestContentDisposition_EmptyFilename(t *testing.T) {
	t.Parallel()
	name := sanitizeFilename("")
	disp := mime.FormatMediaType("attachment", map[string]string{"filename": name})
	if disp == "" {
		t.Error("expected non-empty disposition for empty filename")
	}
}
