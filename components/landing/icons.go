package landing

import (
	"school-with-ai/components/icon"

	"github.com/a-h/templ"
)

// Icon helpers — pre-built icon components to avoid struct-literal syntax
// in templ expressions (templ parser confuses { with children blocks).

func iconSized(fn func(...icon.Props) templ.Component) templ.Component {
	return fn(icon.Props{Class: "size-5"})
}

var (
	iconCompass     = iconSized(icon.Compass)
	iconRocket      = iconSized(icon.Rocket)
	iconStore       = iconSized(icon.Store)
	iconPuzzle      = iconSized(icon.Puzzle)
	iconDatabase    = iconSized(icon.Database)
	iconBrain       = iconSized(icon.Brain)
	iconLayers      = iconSized(icon.Layers)
	iconShieldCheck = iconSized(icon.ShieldCheck)
	iconMessageSq   = iconSized(icon.MessageSquare)
	iconPlug        = iconSized(icon.Plug)
	iconStar        = iconSized(icon.Star)
	iconUser        = iconSized(icon.User)
	iconHeadphones  = iconSized(icon.Headphones)
	iconActivity    = iconSized(icon.Activity)
)
