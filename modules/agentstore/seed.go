package agentstore

// svg wraps Lucide SVG inner paths in a complete theme-compatible <svg> element.
// Width/height are omitted — size via CSS on the container.
func svg(inner string) string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">` + inner + `</svg>`
}

// ── Lucide icon inner paths ───────────────────────────────────────────────────

var (
	iconBot       = svg(`<path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/>`)
	iconTerminal  = svg(`<path d="M12 19h8"/><path d="m4 17 6-6-6-6"/>`)
	iconSearchCode = svg(`<path d="m13 13.5 2-2.5-2-2.5"/><path d="m21 21-4.3-4.3"/><path d="M9 8.5 7 11l2 2.5"/><circle cx="11" cy="11" r="8"/>`)
	iconCode      = svg(`<path d="m16 18 6-6-6-6"/><path d="m8 6-6 6 6 6"/>`)
	iconGitBranch = svg(`<line x1="6" x2="6" y1="3" y2="15"/><circle cx="18" cy="6" r="3"/><circle cx="6" cy="18" r="3"/><path d="M18 9a9 9 0 0 1-9 9"/>`)
	iconWorkflow  = svg(`<circle cx="12" cy="12" r="3"/><path d="M3 7V5a2 2 0 0 1 2-2h2"/><path d="M17 3h2a2 2 0 0 1 2 2v2"/><path d="M21 17v2a2 2 0 0 1-2 2h-2"/><path d="M7 21H5a2 2 0 0 1-2-2v-2"/><path d="m15 9-3 3-3-3"/>`)
	iconPipeline  = svg(`<path d="M12 3v18"/><path d="M18 9H6"/><path d="M18 15H6"/>`)
	iconRefresh   = svg(`<path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/>`)
)

// SeedCatalog returns the catalog entries — one per ADK-Go example.
func seedCatalog() []AgentCatalogEntry {
	single := singleAgentCatalog()
	workflows := workflowCatalog()
	for i := range single {
		if single[i].ID == "" {
			single[i].ID = deterministicID("catalog-single:" + single[i].Name)
		}
	}
	return append(single, workflows...)
}

// singleAgentCatalog mirrors the ADK-Go quickstart example:
// examples/quickstart/main.go
//
// The quickstart shows the simplest possible LLM agent: a focused Instruction
// and an optional tool. No sub-agents, no composition.
func singleAgentCatalog() []AgentCatalogEntry {
	return []AgentCatalogEntry{
		// ── ADK-Go: examples/quickstart/main.go ─────────────────────────────────
		// Original: weather_time_agent — focused LLM agent with a single purpose
		// and a GoogleSearch tool. Here adapted as a general research assistant
		// that demonstrates the same pattern: tight Instruction + tool access.
		{
			Name:        "Quickstart Agent",
			Description: "The simplest ADK-Go LLM agent — a focused Instruction defines its sole purpose. Fork this to see the entry point of the framework.",
			Category:    "Examples",
			Tags:        []string{"quickstart", "llm-agent", "instruction"},
			AuthorName:  "ADK-Go",
			Icon:        iconBot,
			Featured:    true,
			SystemPrompt: `You are a focused research assistant. Your SOLE purpose is to answer questions clearly and accurately on the topic the user brings to you.

For any question outside your scope, politely decline and redirect: "I'm scoped to [topic]. Please ask me something related to that."

Keep answers concise, factual, and well-structured. Cite sources when possible.`,
		},
	}
}
