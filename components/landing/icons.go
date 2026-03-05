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
	iconWand        = iconSized(icon.Wand)
	iconNetwork     = iconSized(icon.Network)
	iconPuzzle      = iconSized(icon.Puzzle)
	iconDatabase    = iconSized(icon.Database)
	iconBrain       = iconSized(icon.Brain)
	iconLayers      = iconSized(icon.Layers)
	iconShieldCheck = iconSized(icon.ShieldCheck)
	iconLanguages   = iconSized(icon.Languages)
	iconLock        = iconSized(icon.Lock)
	iconCode        = iconSized(icon.Code)
	iconMessageSq   = iconSized(icon.MessageSquare)
	iconPlug        = iconSized(icon.Plug)
	iconBookOpen    = iconSized(icon.BookOpen)
	iconSparkles    = iconSized(icon.Sparkles)
	iconZap         = iconSized(icon.Zap)
	iconBot         = iconSized(icon.Bot)
	iconServer      = iconSized(icon.Server)
	iconCheck       = iconSized(icon.Check)
	iconArrowRight  = iconSized(icon.ArrowRight)
)
