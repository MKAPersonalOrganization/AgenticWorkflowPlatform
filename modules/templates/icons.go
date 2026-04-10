package templates

import "github.com/google/uuid"

// svg wraps Lucide SVG inner paths in a complete theme-compatible <svg> element.
func svg(inner string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + inner + `</svg>`
}

// ── Lucide icon inner paths ───────────────────────────────────────────────────

var (
	iconBot             = svg(`<path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/>`)
	iconTerminal        = svg(`<path d="M12 19h8"/><path d="m4 17 6-6-6-6"/>`)
	iconSearchCode      = svg(`<path d="m13 13.5 2-2.5-2-2.5"/><path d="m21 21-4.3-4.3"/><path d="M9 8.5 7 11l2 2.5"/><circle cx="11" cy="11" r="8"/>`)
	iconCode            = svg(`<path d="m16 18 6-6-6-6"/><path d="m8 6-6 6 6 6"/>`)
	iconGitBranch       = svg(`<line x1="6" x2="6" y1="3" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/>`)
	iconWorkflow        = svg(`<circle cx="12" cy="12" r="3"/><path d="M3 7V5a2 2 0 0 1 2-2h2"/><path d="M17 3h2a2 2 0 0 1 2 2v2"/><path d="M21 17v2a2 2 0 0 1-2 2h-2"/><path d="M7 21H5a2 2 0 0 1-2-2v-2"/><path d="m15 9-3 3-3-3"/>`)
	iconPipeline        = svg(`<path d="M12 3v18"/><path d="M18 9H6"/><path d="M18 15H6"/>`)
	iconRefresh         = svg(`<path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/>`)
	iconGraduationCap   = svg(`<path d="M21.42 10.922a1 1 0 0 0-.019-1.838L12.83 5.18a2 2 0 0 0-1.66 0L2.6 9.08a1 1 0 0 0 0 1.832l8.57 3.908a2 2 0 0 0 1.66 0z"/><path d="M22 10v6"/><path d="M6 12.5V16a6 3 0 0 0 12 0v-3.5"/>`)
	iconBookOpen        = svg(`<path d="M12 7v14"/><path d="M3 18a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1h5a4 4 0 0 1 4 4 4 4 0 0 1 4-4h5a1 1 0 0 1 1 1v13a1 1 0 0 1-1 1h-6a3 3 0 0 0-3 3 3 3 0 0 0-3-3z"/>`)
	iconPencilLine      = svg(`<path d="M12 20h9"/><path d="M16.376 3.622a1 1 0 0 1 3.002 3.002L7.368 18.635a2 2 0 0 1-.855.506l-2.872.838a.5.5 0 0 1-.62-.62l.838-2.872a2 2 0 0 1 .506-.854z"/>`)
	iconClipboardCheck  = svg(`<rect width="8" height="4" x="8" y="2" rx="1" ry="1"/><path d="M16 4h2a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2H6a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h2"/><path d="m9 14 2 2 4-4"/>`)
	iconGamepad         = svg(`<line x1="6" x2="10" y1="11" y2="11"/><line x1="8" x2="8" y1="9" y2="13"/><line x1="15" x2="15.01" y1="12" y2="12"/><line x1="18" x2="18.01" y1="10" y2="10"/><path d="M17.32 5H6.68a4 4 0 0 0-3.978 3.59c-.006.052-.01.101-.017.152C2.604 9.416 2 14.456 2 16a3 3 0 0 0 3 3c1.09 0 2.067-.58 2.6-1.5l1.5-2.5h5.8l1.5 2.5c.532.92 1.51 1.5 2.6 1.5a3 3 0 0 0 3-3c0-1.544-.604-6.584-.685-7.258-.007-.05-.011-.1-.017-.151A4 4 0 0 0 17.32 5z"/>`)
	iconPlane           = svg(`<path d="M17.8 19.2 16 11l3.5-3.5C21 6 21.5 4 21 3c-1-.5-3 0-4.5 1.5L13 8 4.8 6.2c-.5-.1-.9.1-1.1.5l-.3.5c-.2.5-.1 1 .3 1.3L9 12l-2 3H4l-1 1 3 2 2 3 1-1v-3l3-2 3.5 5.3c.3.4.8.5 1.3.3l.5-.2c.4-.3.6-.7.5-1.2z"/>`)
	iconBriefcase       = svg(`<path d="M16 20V4a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"/><rect width="20" height="14" x="2" y="6" rx="2"/>`)
	iconCamera          = svg(`<path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0-2-2h-3l-2.5-3z"/><circle cx="12" cy="13" r="3"/>`)
	iconTarget          = svg(`<circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="6"/><circle cx="12" cy="12" r="2"/>`)
	iconUsers           = svg(`<path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M22 21v-2a4 4 0 0 0-3-3.87"/><path d="M16 3.13a4 4 0 0 1 0 7.75"/>`)
	iconClapperboard    = svg(`<path d="M20.2 6 3 11l-.9-2.4c-.3-1.1.3-2.2 1.3-2.5l13.5-4c1.1-.3 2.2.3 2.5 1.3Z"/><path d="m6.2 5.3 3.1 3.9"/><path d="m12.4 3.4 3.1 4"/><path d="M3 11h18v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2Z"/>`)
	iconTrendingUp      = svg(`<polyline points="22 7 13.5 15.5 8.5 10.5 2 17"/><polyline points="16 7 22 7 22 13"/>`)
	iconCalendarClock   = svg(`<path d="M21 7.5V6a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h3.5"/><path d="M16 2v4"/><path d="M8 2v4"/><path d="M3 10h5"/><path d="M17.5 17.5 16 16.3V14"/><circle cx="16" cy="16" r="6"/>`)
	iconMapPin          = svg(`<path d="M20 10c0 4.993-5.539 10.193-7.399 11.799a1 1 0 0 1-1.202 0C9.539 20.193 4 14.993 4 10a8 8 0 0 1 16 0"/><circle cx="12" cy="10" r="3"/>`)
	iconBookOpenCheck   = svg(`<path d="M12 21V7"/><path d="m16 12 2 2 4-4"/><path d="M12 7H4a1 1 0 0 0-1 1v11c0 .6.4 1 1 1h5a4 4 0 0 1 4-4 4 4 0 0 1 4 4h5a1 1 0 0 0 1-1V8a1 1 0 0 0-1-1h-1"/>`)
	iconSparkles        = svg(`<path d="M9.937 15.5A2 2 0 0 0 8.5 14.063l-6.135-1.582a.5.5 0 0 1 0-.962L8.5 9.936A2 2 0 0 0 9.937 8.5l1.582-6.135a.5.5 0 0 1 .963 0L14.063 8.5A2 2 0 0 0 15.5 9.937l6.135 1.581a.5.5 0 0 1 0 .964L15.5 14.063a2 2 0 0 0-1.437 1.437l-1.582 6.135a.5.5 0 0 1-.963 0z"/><path d="M20 3v4"/><path d="M22 5h-4"/><path d="M4 17v2"/><path d="M5 18H3"/>`)
)

// resolveIcon maps a short icon name to its Lucide SVG string.
// If the name is not recognised (e.g. it is already raw SVG), it is returned as-is.
func resolveIcon(name string) string {
	m := map[string]string{
		"bot":             iconBot,
		"terminal":        iconTerminal,
		"search-code":     iconSearchCode,
		"code":            iconCode,
		"git-branch":      iconGitBranch,
		"workflow":        iconWorkflow,
		"pipeline":        iconPipeline,
		"refresh":         iconRefresh,
		"graduation-cap":  iconGraduationCap,
		"book-open":       iconBookOpen,
		"pencil-line":     iconPencilLine,
		"clipboard-check": iconClipboardCheck,
		"gamepad":         iconGamepad,
		"plane":           iconPlane,
		"briefcase":       iconBriefcase,
		"camera":          iconCamera,
		"target":          iconTarget,
		"users":           iconUsers,
		"clapperboard":    iconClapperboard,
		"trending-up":     iconTrendingUp,
		"calendar-clock":  iconCalendarClock,
		"map-pin":         iconMapPin,
		"book-open-check": iconBookOpenCheck,
		"sparkles":        iconSparkles,
	}
	if s, ok := m[name]; ok {
		return s
	}
	return name
}

// deterministicID returns a stable UUID derived from a seed string.
func deterministicID(seed string) string {
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte(seed)).String()
}
