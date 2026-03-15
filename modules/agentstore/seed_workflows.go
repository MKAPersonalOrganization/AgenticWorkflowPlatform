package agentstore

import (
	"encoding/json"

	"github.com/google/uuid"
)

// workflowCatalog returns workflow-based catalog entries. Each entry includes
// a workflow_graph template (with placeholder agent IDs) and the sub_agents
// definitions. When a user forks one of these, the system creates the
// sub-agents as real user_agents and a workflow_definition with remapped IDs.
func workflowCatalog() []AgentCatalogEntry {
	return []AgentCatalogEntry{
		// ── Research & Analysis Workflows ──
		researchPipelineEntry(),
		deepDiveAnalysisEntry(),

		// ── Content Creation Workflows ──
		contentCreationPipelineEntry(),
		blogPostFactoryEntry(),

		// ── Professional Workflows ──
		interviewBootcampEntry(),
		codeReviewPipelineEntry(),
	}
}

// ─── Research Pipeline (Sequential) ─────────────────────────────────────────

func researchPipelineEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:           "sub-researcher",
			Name:         "Research Gatherer",
			Description:  "Collects and organizes raw information on a topic",
			SystemPrompt: "You are a thorough research specialist. When given a topic, gather comprehensive information covering key facts, different perspectives, recent developments, and relevant statistics. Organize findings into clear sections. Always note your sources when possible. Present raw research material for further analysis.",
			Icon:         iconMicroscope,
		},
		{
			ID:           "sub-analyst",
			Name:         "Research Analyst",
			Description:  "Analyzes research findings and extracts insights",
			SystemPrompt: "You are an expert analyst. You receive raw research data and extract meaningful insights. Identify patterns, contradictions, and gaps. Evaluate source reliability. Highlight the most significant findings. Draw connections between different pieces of information. Present your analysis in a structured format with clear conclusions.",
			Icon:         iconChartBar,
		},
		{
			ID:           "sub-writer",
			Name:         "Report Writer",
			Description:  "Produces polished reports from analyzed research",
			SystemPrompt: "You are a professional report writer. Transform analyzed research into a well-structured, readable report. Include an executive summary, key findings, detailed analysis sections, and actionable recommendations. Use clear headings, bullet points, and transitions. Maintain a professional but accessible tone. Cite sources where appropriate.",
			Icon:         iconFileText,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-researcher", "output_key": "raw_research"},
			{"id": "n2", "type": "agent", "agent_id": "sub-analyst", "output_key": "analysis"},
			{"id": "n3", "type": "agent", "agent_id": "sub-writer"},
		},
	}

	return catalogWorkflowEntry(
		"Research Pipeline",
		"A 3-agent sequential pipeline: a Researcher gathers data, an Analyst extracts insights, and a Writer produces a polished report.",
		"Research → Analysis → Report in one click.",
		"Workflows",
		[]string{"research", "analysis", "report", "pipeline", "sequential"},
		iconPipeline,
		true, // featured
		subs,
		graph,
	)
}

// ─── Deep Dive Analysis (Parallel) ──────────────────────────────────────────

func deepDiveAnalysisEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:           "sub-technical",
			Name:         "Technical Analyst",
			Description:  "Analyzes technical and scientific aspects",
			SystemPrompt: "You are a technical analyst. Examine the given topic from a technical and scientific perspective. Evaluate feasibility, technical challenges, implementation details, and scientific evidence. Use precise terminology. Present technical findings clearly with supporting data.",
			Icon:         iconAtom,
		},
		{
			ID:           "sub-business",
			Name:         "Business Analyst",
			Description:  "Analyzes market and business implications",
			SystemPrompt: "You are a business and market analyst. Examine the given topic from a business perspective. Evaluate market opportunity, competitive landscape, financial implications, risks, and strategic value. Use frameworks like SWOT, Porter's Five Forces, or cost-benefit analysis when appropriate.",
			Icon:         iconBriefcase,
		},
		{
			ID:           "sub-creative",
			Name:         "Creative Strategist",
			Description:  "Explores unconventional angles and opportunities",
			SystemPrompt: "You are a creative strategist. Look at the given topic from unconventional angles. Identify hidden opportunities, novel applications, potential pivots, and creative solutions others might miss. Think laterally. Challenge assumptions. Propose bold but defensible ideas.",
			Icon:         iconLightbulb,
		},
		{
			ID:           "sub-synthesizer",
			Name:         "Synthesis Editor",
			Description:  "Combines parallel analyses into a unified report",
			SystemPrompt: "You are a synthesis editor. You receive multiple analyses of the same topic from different perspectives (technical, business, creative). Combine them into one cohesive, well-structured report. Highlight areas of agreement, tension, and synergy. Produce a balanced conclusion with prioritized recommendations.",
			Icon:         iconFileText,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{
				"id":   "parallel-analysis",
				"type": "parallel",
				"children": []map[string]interface{}{
					{"id": "n1", "type": "agent", "agent_id": "sub-technical", "output_key": "technical_analysis"},
					{"id": "n2", "type": "agent", "agent_id": "sub-business", "output_key": "business_analysis"},
					{"id": "n3", "type": "agent", "agent_id": "sub-creative", "output_key": "creative_analysis"},
				},
			},
			{"id": "n4", "type": "agent", "agent_id": "sub-synthesizer"},
		},
	}

	return catalogWorkflowEntry(
		"Deep Dive Analysis",
		"Three specialist agents analyze a topic in parallel from technical, business, and creative angles, then a synthesizer merges their findings into a unified report.",
		"Multi-perspective analysis with parallel execution.",
		"Workflows",
		[]string{"analysis", "parallel", "multi-perspective", "strategy"},
		iconWorkflow,
		true, // featured
		subs,
		graph,
	)
}

// ─── Content Creation Pipeline (Sequential) ─────────────────────────────────

func contentCreationPipelineEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:           "sub-outliner",
			Name:         "Content Outliner",
			Description:  "Creates structured outlines from topic briefs",
			SystemPrompt: "You are a content strategist specializing in outlines. Given a topic or brief, create a detailed, well-structured outline with main sections, subsections, and key points to cover. Consider audience, tone, and purpose. Include suggested hooks, transitions, and calls to action. Format clearly with numbered sections.",
			Icon:         iconNotebookPen,
		},
		{
			ID:           "sub-drafter",
			Name:         "Content Drafter",
			Description:  "Writes full first drafts from outlines",
			SystemPrompt: "You are a skilled content writer. Transform outlines into complete, engaging first drafts. Write naturally with a clear voice. Use compelling openings, smooth transitions, and strong conclusions. Include examples and anecdotes where appropriate. Match the tone and style specified in the outline.",
			Icon:         iconFeather,
		},
		{
			ID:           "sub-editor",
			Name:         "Content Editor",
			Description:  "Polishes drafts for clarity, flow, and impact",
			SystemPrompt: "You are a meticulous editor. Review content for grammar, spelling, punctuation, clarity, flow, and impact. Tighten prose, eliminate redundancy, strengthen weak passages, and improve transitions. Preserve the author's voice while elevating quality. Flag any factual claims that need verification. Return the polished final version.",
			Icon:         iconSpellCheck,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-outliner", "output_key": "outline"},
			{"id": "n2", "type": "agent", "agent_id": "sub-drafter", "output_key": "draft"},
			{"id": "n3", "type": "agent", "agent_id": "sub-editor"},
		},
	}

	return catalogWorkflowEntry(
		"Content Creation Pipeline",
		"A 3-stage writing pipeline: an Outliner structures the content, a Drafter writes the full piece, and an Editor polishes it to publication quality.",
		"From topic to polished content in three steps.",
		"Workflows",
		[]string{"writing", "content", "pipeline", "editing", "sequential"},
		iconPipeline,
		false,
		subs,
		graph,
	)
}

// ─── Blog Post Factory (Loop + Sequential) ──────────────────────────────────

func blogPostFactoryEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:           "sub-blog-writer",
			Name:         "Blog Writer",
			Description:  "Writes engaging blog posts with SEO awareness",
			SystemPrompt: "You are a blog writer skilled at creating engaging, SEO-friendly content. Write blog posts with compelling headlines, clear structure, relevant keywords, and a conversational yet authoritative tone. Include an introduction hook, well-organized body sections with subheadings, and a strong conclusion with a call to action.",
			Icon:         iconFeather,
		},
		{
			ID:           "sub-blog-reviewer",
			Name:         "Blog Reviewer",
			Description:  "Reviews blog posts and suggests improvements",
			SystemPrompt: "You are a critical blog content reviewer. Evaluate blog posts for: headline effectiveness, opening hook strength, logical flow, readability (aim for grade 8 reading level), SEO optimization, factual accuracy, and engagement factor. Provide specific, actionable feedback. Rate each aspect 1-5 and explain how to improve weak areas. If the post is excellent, say 'APPROVED' at the start.",
			Icon:         iconSearchCode,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "loop",
		// Writer writes, reviewer reviews. Loop continues until reviewer approves
		// or max iterations reached.
		"max_iterations": 3,
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-blog-writer", "output_key": "blog_draft"},
			{"id": "n2", "type": "agent", "agent_id": "sub-blog-reviewer"},
		},
	}

	return catalogWorkflowEntry(
		"Blog Post Factory",
		"An iterative write-review loop: a Writer creates a blog post, a Reviewer evaluates it and provides feedback. The loop repeats until the post is approved or 3 iterations pass.",
		"Self-improving blog posts through iterative review.",
		"Workflows",
		[]string{"blog", "writing", "review", "loop", "iterative"},
		iconWorkflow,
		false,
		subs,
		graph,
	)
}

// ─── Interview Bootcamp (Sequential + Parallel) ─────────────────────────────

func interviewBootcampEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:           "sub-behavioral",
			Name:         "Behavioral Interviewer",
			Description:  "Conducts behavioral interview questions using STAR method",
			SystemPrompt: "You are a behavioral interviewer. Ask one behavioral question at a time using common frameworks (STAR method). Focus on: leadership, teamwork, conflict resolution, problem-solving, and failure/growth stories. After the user responds, provide detailed feedback on their answer structure, specificity, and impact. Suggest how to improve weak responses.",
			Icon:         iconBriefcase,
		},
		{
			ID:           "sub-technical-int",
			Name:         "Technical Interviewer",
			Description:  "Conducts technical interview assessments",
			SystemPrompt: "You are a technical interviewer. Ask progressively harder technical questions relevant to the user's target role. Cover system design, coding concepts, problem-solving approaches, and domain-specific knowledge. Evaluate responses for correctness, depth, and communication clarity. Provide hints when the user is stuck and detailed explanations after each question.",
			Icon:         iconCode,
		},
		{
			ID:           "sub-coach",
			Name:         "Interview Coach",
			Description:  "Synthesizes performance and builds an improvement plan",
			SystemPrompt: "You are a career coach specializing in interview preparation. You receive notes from behavioral and technical interview sessions. Synthesize the feedback into: overall readiness assessment, top 3 strengths demonstrated, top 3 areas for improvement, specific practice exercises for each weakness, and a prioritized 1-week preparation plan. Be encouraging but honest.",
			Icon:         iconTarget,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{
				"id":   "interviews",
				"type": "parallel",
				"children": []map[string]interface{}{
					{"id": "n1", "type": "agent", "agent_id": "sub-behavioral", "output_key": "behavioral_feedback"},
					{"id": "n2", "type": "agent", "agent_id": "sub-technical-int", "output_key": "technical_feedback"},
				},
			},
			{"id": "n3", "type": "agent", "agent_id": "sub-coach"},
		},
	}

	return catalogWorkflowEntry(
		"Interview Bootcamp",
		"Two interviewers (behavioral + technical) run in parallel, then a Coach synthesizes their feedback into a personalized improvement plan.",
		"Parallel mock interviews with coaching synthesis.",
		"Workflows",
		[]string{"interview", "career", "practice", "parallel", "coaching"},
		iconWorkflow,
		true, // featured
		subs,
		graph,
	)
}

// ─── Code Review Pipeline (Sequential) ──────────────────────────────────────

func codeReviewPipelineEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:           "sub-security",
			Name:         "Security Auditor",
			Description:  "Reviews code for security vulnerabilities",
			SystemPrompt: "You are a security-focused code reviewer. Analyze code for: injection vulnerabilities (SQL, XSS, command), authentication/authorization flaws, insecure data handling, secrets in code, OWASP Top 10 issues, and dependency vulnerabilities. Classify findings by severity (Critical/High/Medium/Low). Provide specific fix suggestions for each finding.",
			Icon:         iconSearchCode,
		},
		{
			ID:           "sub-quality",
			Name:         "Quality Engineer",
			Description:  "Reviews code for quality, patterns, and maintainability",
			SystemPrompt: "You are a software quality engineer. Review code for: design pattern adherence, SOLID principles, code duplication, naming conventions, error handling, test coverage gaps, performance bottlenecks, and documentation quality. Suggest specific refactoring improvements. Prioritize findings by impact on maintainability.",
			Icon:         iconTerminal,
		},
		{
			ID:           "sub-summarizer",
			Name:         "Review Summarizer",
			Description:  "Creates actionable review summary from multiple reviews",
			SystemPrompt: "You are a technical project lead. You receive security and quality review findings. Create a unified, prioritized action plan: group related findings, eliminate duplicates, rank by impact and effort, and produce a clear checklist the developer can follow. Include a risk assessment summary and suggested PR review comments.",
			Icon:         iconFileText,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{
				"id":   "reviews",
				"type": "parallel",
				"children": []map[string]interface{}{
					{"id": "n1", "type": "agent", "agent_id": "sub-security", "output_key": "security_findings"},
					{"id": "n2", "type": "agent", "agent_id": "sub-quality", "output_key": "quality_findings"},
				},
			},
			{"id": "n3", "type": "agent", "agent_id": "sub-summarizer"},
		},
	}

	return catalogWorkflowEntry(
		"Code Review Pipeline",
		"A Security Auditor and Quality Engineer review code in parallel, then a Summarizer merges their findings into a prioritized action plan.",
		"Comprehensive automated code review from two angles.",
		"Workflows",
		[]string{"code-review", "security", "quality", "parallel", "pipeline"},
		iconPipeline,
		false,
		subs,
		graph,
	)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// catalogWorkflowEntry is a convenience constructor for workflow catalog entries.
// Uses a deterministic ID based on the name so ON CONFLICT (id) DO NOTHING
// works correctly across restarts.
func catalogWorkflowEntry(
	name, description, systemPrompt, category string,
	tags []string,
	icon string,
	featured bool,
	subs []CatalogSubAgent,
	graph map[string]interface{},
) AgentCatalogEntry {
	graphJSON, _ := json.Marshal(graph)
	subsJSON, _ := json.Marshal(subs)

	// Deterministic UUID v5 from a fixed namespace + name.
	id := deterministicID("catalog-workflow:" + name)

	return AgentCatalogEntry{
		ID:                id,
		Name:              name,
		Description:       description,
		SystemPrompt:      systemPrompt,
		Category:          category,
		Tags:              tags,
		AuthorName:        "AI Platform",
		Icon:              icon,
		Featured:          featured,
		WorkflowGraphJSON: string(graphJSON),
		SubAgentsJSON:     string(subsJSON),
	}
}

// deterministicID generates a stable UUID v5 from a seed string.
func deterministicID(seed string) string {
	return uuid.NewSHA1(uuid.NameSpaceDNS, []byte(seed)).String()
}
