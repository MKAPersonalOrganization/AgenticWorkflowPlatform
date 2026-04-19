package landing

import (
	"aicompass/frontend/components/icon"

	"github.com/a-h/templ"
)

// Icon helpers — pre-built icon components to avoid struct-literal syntax
// in templ expressions (templ parser confuses { with children blocks).

func iconSized(fn func(...icon.Props) templ.Component) templ.Component {
	return fn(icon.Props{Class: "size-5"})
}

var (
	iconGitBranch   = iconSized(icon.GitBranch)
	iconDatabase    = iconSized(icon.Database)
	iconBrain       = iconSized(icon.Brain)
	iconShieldCheck = iconSized(icon.ShieldCheck)
	iconMessageSq   = iconSized(icon.MessageSquare)
	iconA2ALogo     = icon.A2ALogo("size-5")
)
