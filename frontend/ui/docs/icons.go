package docs

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
	iconCompass           = icon.CompassLogo("size-5")
	iconRocket            = iconSized(icon.Rocket)
	iconStore             = iconSized(icon.Store)
	iconWand              = iconSized(icon.Wand)
	iconGitBranch         = iconSized(icon.GitBranch)
	iconPuzzle            = iconSized(icon.Puzzle)
	iconDatabase          = iconSized(icon.Database)
	iconBrain             = iconSized(icon.Brain)
	iconLayers            = iconSized(icon.Layers)
	iconShieldCheck       = iconSized(icon.ShieldCheck)
	iconLanguages         = iconSized(icon.Languages)
	iconLock              = iconSized(icon.Lock)
	iconUser              = iconSized(icon.User)
	iconHeadphones        = iconSized(icon.Headphones)
	iconCpu               = iconSized(icon.Cpu)
	iconPaperclip         = iconSized(icon.Paperclip)
	iconFolderOpen        = iconSized(icon.FolderOpen)
	iconSearch            = iconSized(icon.Search)
	iconChartBar          = iconSized(icon.ChartBar)
	iconSlidersHorizontal = iconSized(icon.SlidersHorizontal)

	// ── Workflow examples section icons ──
	iconBlocks      = iconSized(icon.Blocks)
	iconPipelineDoc = iconSized(icon.GitBranch)
	iconNetworkDoc  = iconSized(icon.Network)
	iconRepeatDoc   = iconSized(icon.Repeat)
	iconWandDoc     = iconSized(icon.WandSparkles)
	iconGitForkDoc  = iconSized(icon.GitFork)

	// ── MCP section icon ──
	iconMCPLogo = icon.MCPLogo("size-5")

	// ── A2A section icon ──
	iconA2ALogo = icon.A2ALogo("size-5")
)
