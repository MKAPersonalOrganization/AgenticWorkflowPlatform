package docs

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
	iconCompass           = iconSized(icon.Compass)
	iconRocket            = iconSized(icon.Rocket)
	iconStore             = iconSized(icon.Store)
	iconWand              = iconSized(icon.Wand)
	iconNetwork           = iconSized(icon.Network)
	iconPuzzle            = iconSized(icon.Puzzle)
	iconDatabase          = iconSized(icon.Database)
	iconBrain             = iconSized(icon.Brain)
	iconLayers            = iconSized(icon.Layers)
	iconShieldCheck       = iconSized(icon.ShieldCheck)
	iconLanguages         = iconSized(icon.Languages)
	iconLock              = iconSized(icon.Lock)
	iconCode              = iconSized(icon.Code)
	iconHeadphones        = iconSized(icon.Headphones)
	iconActivity          = iconSized(icon.Activity)
	iconGraduationCap     = iconSized(icon.GraduationCap)
	iconCpu               = iconSized(icon.Cpu)
	iconPaperclip         = iconSized(icon.Paperclip)
	iconFolderOpen        = iconSized(icon.FolderOpen)
	iconSearch            = iconSized(icon.Search)
	iconChartBar          = iconSized(icon.ChartBar)
	iconSlidersHorizontal = iconSized(icon.SlidersHorizontal)
)
