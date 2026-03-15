package agentstore

// crewAICatalog returns workflow entries translated from the crewAI-examples
// repository (github.com/crewAIInc/crewAI-examples).
//
// Each entry maps CrewAI concepts onto ADK-Go equivalents behind our unified
// agent interface:
//
//	CrewAI concept                →  ADK-Go / composition model
//	─────────────────────────────────────────────────────────────
//	Crew(process=sequential)     →  CompositionNode type "sequential"
//	Agent (role, goal, backstory)→  CatalogSubAgent (system prompt)
//	Task context dependencies    →  OutputKey + {variable} templates
//	async_execution=True         →  CompositionNode type "parallel"
//	kickoff_for_each / retries   →  CompositionNode type "loop"
//	Flow @router + @listen       →  nested composition (loop → sequential)
func crewAICatalog() []AgentCatalogEntry {
	return []AgentCatalogEntry{
		// ── Sequential pipelines ─────────────────────────
		gameBuilderEntry(),
		tripPlannerEntry(),
		jobPostingEntry(),
		instagramPostEntry(),
		marketingStrategyEntry(),
		recruitmentEntry(),
		screenplayWriterEntry(),

		// ── Parallel fan-out → fan-in ────────────────────
		stockAnalysisEntry(),
		prepForMeetingEntry(),
		surpriseTripEntry(),

		// ── Map-reduce (sequential → parallel → sequential)
		writeABookEntry(),

		// ── Self-evaluation loop ─────────────────────────
		selfEvaluationLoopEntry(),
	}
}

// ─── Game Builder Crew ───────────────────────────────────────────────────────
// Source: crewAI-examples/crews/game-builder-crew
//
// Three-agent sequential pipeline that writes a Python game, reviews the code
// for bugs, then validates it does the right thing.
//
//	SeniorEngineer → output_key: "game_code"
//	QAEngineer     → reads {game_code}, output_key: "temp:qa_review"
//	ChiefQA        → reads {game_code} + {temp:qa_review}, output_key: "final_game"

func gameBuilderEntry() AgentCatalogEntry {
	fence := "```"

	subs := []CatalogSubAgent{
		{
			ID:          "sub-game-coder",
			Name:        "SeniorEngineer",
			Description: "Writes Python game code from the user's specification.",
			Icon:        iconTerminal,
			SystemPrompt: `You are a Senior Software Engineer specialising in game development with Python.

Based on the user's game instructions, write a complete, working Python game.

**Requirements:**
- Use only the Python standard library (no external dependencies).
- Include clear comments and docstrings.
- The game must be runnable from a single file via "python game.py".
- Handle user input gracefully and include a game loop.

Output the full Python source code in a fenced code block. Nothing else.`,
		},
		{
			ID:          "sub-game-qa",
			Name:        "QAEngineer",
			Description: "Reviews the game code for bugs and quality issues. Reads {game_code}.",
			Icon:        iconSearchCode,
			SystemPrompt: `You are a Software Quality Control Engineer.

**Code to Review:**
` + fence + `python
{game_code}
` + fence + `

Review the game code above for:
1. **Bugs** — runtime errors, logic mistakes, off-by-one errors.
2. **Edge cases** — unexpected input, boundary conditions.
3. **Code quality** — readability, naming, structure.

Output a concise bulleted list of issues found with suggested fixes.
If the code is solid, state: "No issues found — code is clean."`,
		},
		{
			ID:          "sub-game-chief-qa",
			Name:        "ChiefQAEngineer",
			Description: "Validates the game meets the specification. Reads {game_code} and {temp:qa_review}.",
			Icon:        iconClipboardCheck,
			SystemPrompt: `You are the Chief Software Quality Control Engineer.

**Game Code:**
` + fence + `python
{game_code}
` + fence + `

**QA Review:**
{temp:qa_review}

Your job:
1. Apply all fixes identified in the QA review.
2. Verify the final code fulfils the original game specification.
3. Add any missing error handling or polish.

Output the final, production-ready Python game code in a fenced code block.`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-game-coder", "output_key": "game_code"},
			{"id": "n2", "type": "agent", "agent_id": "sub-game-qa", "output_key": "temp:qa_review"},
			{"id": "n3", "type": "agent", "agent_id": "sub-game-chief-qa", "output_key": "final_game"},
		},
	}

	return catalogWorkflowEntry(
		"Game Builder Crew",
		"Three-agent code pipeline: a Senior Engineer writes a Python game, a QA Engineer reviews for bugs, "+
			"and a Chief QA validates against the spec. Translated from crewAI-examples/crews/game-builder-crew.",
		"Write a Python game, review it, and validate the final code.",
		"Examples",
		[]string{"sequential", "state-passing", "output-key", "coding", "game", "crew-ai"},
		iconGamepad,
		false,
		subs,
		graph,
	)
}

// ─── Trip Planner ────────────────────────────────────────────────────────────
// Source: crewAI-examples/crews/trip_planner
//
// Three agents plan a detailed trip: select the best city, gather local
// expertise, then compile a 7-day itinerary.
//
//	CitySelector    → output_key: "selected_city"
//	LocalExpert     → reads {selected_city}, output_key: "city_guide"
//	TravelConcierge → reads {selected_city} + {city_guide}, output_key: "travel_plan"

func tripPlannerEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-city-selector",
			Name:        "CitySelector",
			Description: "Analyses weather, season, prices, and events to pick the best destination city.",
			Icon:        iconMapPin,
			SystemPrompt: `You are a City Selection Expert with deep knowledge of worldwide travel destinations.

Given the user's travel preferences (dates, interests, budget), analyse and select the **single best city** from 3–5 candidates.

**For each candidate, evaluate:**
- Current weather and seasonal conditions
- Flight and accommodation price ranges
- Notable events or festivals during the travel window
- Overall suitability for the stated interests

**Output:**
1. A short-list comparison table.
2. Your recommended city with a clear justification.

Output only the analysis and recommendation.`,
		},
		{
			ID:          "sub-local-expert",
			Name:        "LocalExpert",
			Description: "Provides insider knowledge about the selected city. Reads {selected_city}.",
			Icon:        iconMapPin,
			SystemPrompt: `You are a Local Expert with deep insider knowledge of {selected_city}.

Provide a comprehensive city guide covering:
1. **Must-see attractions** — top 5 with brief descriptions.
2. **Hidden gems** — 3 off-the-beaten-path recommendations.
3. **Cultural tips** — customs, etiquette, local phrases.
4. **Transportation** — how to get around efficiently.
5. **Safety** — areas to avoid, common scams.

Write as a knowledgeable local sharing tips with a friend.`,
		},
		{
			ID:          "sub-travel-concierge",
			Name:        "TravelConcierge",
			Description: "Compiles a detailed day-by-day itinerary. Reads {selected_city} and {city_guide}.",
			Icon:        iconPlane,
			SystemPrompt: `You are an Amazing Travel Concierge creating a perfect trip itinerary.

**Destination:** {selected_city}

**Local Guide:**
{city_guide}

Create a **detailed 7-day itinerary** including:
- Day-by-day schedule with morning, afternoon, and evening activities.
- Restaurant recommendations for each meal.
- Estimated daily budget breakdown.
- Packing suggestions for the destination and season.
- Transportation between activities.

Format as a clear, day-by-day Markdown document the traveller can follow.`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-city-selector", "output_key": "selected_city"},
			{"id": "n2", "type": "agent", "agent_id": "sub-local-expert", "output_key": "city_guide"},
			{"id": "n3", "type": "agent", "agent_id": "sub-travel-concierge", "output_key": "travel_plan"},
		},
	}

	return catalogWorkflowEntry(
		"Trip Planner",
		"Three-agent travel pipeline: select the best city, gather local expertise, then compile a 7-day itinerary with budget. "+
			"Translated from crewAI-examples/crews/trip_planner.",
		"Plan a detailed trip — select a city, get local tips, and build a day-by-day itinerary.",
		"Examples",
		[]string{"sequential", "state-passing", "travel", "planning", "crew-ai"},
		iconPlane,
		false,
		subs,
		graph,
	)
}

// ─── Job Posting ─────────────────────────────────────────────────────────────
// Source: crewAI-examples/crews/job-posting
//
// Three agents create a polished job posting: research the company, write the
// posting, then review and edit.
//
//	CompanyResearcher → output_key: "company_research"
//	JobWriter         → reads {company_research}, output_key: "job_posting_draft"
//	PostingReviewer   → reads {company_research} + {job_posting_draft}, output_key: "job_posting"

func jobPostingEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-company-researcher",
			Name:        "CompanyResearcher",
			Description: "Researches a company's culture, values, and role requirements.",
			Icon:        iconSearchCode,
			SystemPrompt: `You are a Research Analyst specialising in company analysis for hiring.

Given a company name and a job role, produce a research brief covering:
1. **Company overview** — mission, size, industry, recent news.
2. **Culture & values** — work style, team structure, DEI initiatives.
3. **Role context** — why this role exists, team it belongs to, growth trajectory.
4. **Compensation benchmark** — typical salary range for this role at similar companies.
5. **Key selling points** — what makes this company attractive to candidates.

Output only the research brief in Markdown.`,
		},
		{
			ID:          "sub-job-writer",
			Name:        "JobWriter",
			Description: "Writes a compelling job posting based on company research. Reads {company_research}.",
			Icon:        iconPencilLine,
			SystemPrompt: `You are a Job Description Writer who creates compelling, inclusive job postings.

**Company Research:**
{company_research}

Write a complete job posting that includes:
1. **Job title** — clear and searchable.
2. **About the company** — 2-3 engaging sentences.
3. **Role summary** — what the person will do and why it matters.
4. **Responsibilities** — 5-8 bullet points.
5. **Requirements** — must-have vs. nice-to-have (clearly separated).
6. **Benefits & perks** — compensation range, benefits, growth opportunities.
7. **How to apply** — next steps.

Use inclusive language. Avoid jargon and unnecessary requirements.`,
		},
		{
			ID:          "sub-posting-reviewer",
			Name:        "PostingReviewer",
			Description: "Reviews the job posting for clarity, accuracy, and inclusivity. Reads {company_research} and {job_posting_draft}.",
			Icon:        iconClipboardCheck,
			SystemPrompt: `You are a Review and Editing Specialist for job postings.

**Company Research:**
{company_research}

**Draft Job Posting:**
{job_posting_draft}

Review the draft for:
1. **Accuracy** — does it match the research? Any factual errors?
2. **Clarity** — is every section clear and concise?
3. **Inclusivity** — any gendered language, unnecessary requirements, or bias?
4. **Completeness** — anything missing that a strong candidate would want to know?
5. **Tone** — professional yet inviting?

Output the final, polished job posting with all improvements applied.`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-company-researcher", "output_key": "company_research"},
			{"id": "n2", "type": "agent", "agent_id": "sub-job-writer", "output_key": "job_posting_draft"},
			{"id": "n3", "type": "agent", "agent_id": "sub-posting-reviewer", "output_key": "job_posting"},
		},
	}

	return catalogWorkflowEntry(
		"Job Posting Creator",
		"Three-agent HR pipeline: research a company, write a compelling job posting, then review for clarity and inclusivity. "+
			"Translated from crewAI-examples/crews/job-posting.",
		"Research a company and create a polished, inclusive job posting.",
		"Examples",
		[]string{"sequential", "state-passing", "hr", "job-posting", "crew-ai"},
		iconBriefcase,
		false,
		subs,
		graph,
	)
}

// ─── Instagram Post Creator ─────────────────────────────────────────────────
// Source: crewAI-examples/crews/instagram_post
//
// Five agents in a two-phase sequential pipeline: Phase 1 (copy) analyses
// the product, develops strategy, and writes ad copy; Phase 2 (visual)
// describes photographs and reviews the creative.
//
// Demonstrates a longer sequential chain (5 agents) with state cascading.

func instagramPostEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-ig-market-analyst",
			Name:        "MarketAnalyst",
			Description: "Analyses the product and competitive landscape.",
			Icon:        iconTarget,
			SystemPrompt: `You are a Lead Market Analyst specialising in product positioning for social media.

Given a product description, produce:
1. **Product analysis** — key features, unique selling points, target audience.
2. **Competitor scan** — 3 main competitors and their social media approach.
3. **Market positioning** — where this product sits and what gap it fills.
4. **Audience insights** — demographics, pain points, desires.

Be concise and data-driven. Output only the analysis.`,
		},
		{
			ID:          "sub-ig-strategist",
			Name:        "MarketingStrategist",
			Description: "Develops an Instagram campaign strategy. Reads {market_analysis}.",
			Icon:        iconTarget,
			SystemPrompt: `You are a Chief Marketing Strategist for Instagram campaigns.

**Market Analysis:**
{market_analysis}

Develop a focused Instagram campaign strategy:
1. **Campaign concept** — one compelling theme/angle.
2. **Key messages** — 3 core messages to communicate.
3. **Tone & voice** — how the brand should sound.
4. **Hashtag strategy** — 10-15 relevant hashtags.
5. **Posting cadence** — frequency and best times.

Keep it actionable and specific to Instagram.`,
		},
		{
			ID:          "sub-ig-copywriter",
			Name:        "AdCopywriter",
			Description: "Writes Instagram ad copy. Reads {market_analysis} and {campaign_strategy}.",
			Icon:        iconPencilLine,
			SystemPrompt: `You are a Creative Content Creator for Instagram ads.

**Market Analysis:**
{market_analysis}

**Campaign Strategy:**
{campaign_strategy}

Write **3 Instagram ad copy variations**, each with:
- A scroll-stopping opening hook (first line).
- Compelling body text (3-5 lines).
- A clear call-to-action.
- Relevant emoji usage (tasteful, not excessive).
- Suggested hashtags from the strategy.

Label them "Variation A", "Variation B", "Variation C".`,
		},
		{
			ID:          "sub-ig-photographer",
			Name:        "Photographer",
			Description: "Describes ideal photographs to pair with the ad copy. Reads {ad_copy}.",
			Icon:        iconCamera,
			SystemPrompt: `You are a Senior Photographer directing an Instagram photo shoot.

**Ad Copy Variations:**
{ad_copy}

For each ad copy variation, describe the **ideal photograph** in detail:
1. **Scene composition** — what's in the frame, arrangement, focal point.
2. **Lighting** — natural/studio, mood, time of day.
3. **Color palette** — dominant colors, brand alignment.
4. **Props & styling** — objects, wardrobe, set design.
5. **Mood & emotion** — what the viewer should feel.

Write each description so a photographer or AI image generator could produce it.`,
		},
		{
			ID:          "sub-ig-creative-director",
			Name:        "CreativeDirector",
			Description: "Reviews the complete creative package. Reads {ad_copy} and {temp:photo_descriptions}.",
			Icon:        iconClipboardCheck,
			SystemPrompt: `You are a Chief Creative Director reviewing an Instagram campaign.

**Ad Copy:**
{ad_copy}

**Photo Descriptions:**
{temp:photo_descriptions}

Review the complete creative package:
1. **Brand consistency** — do copy and visuals align?
2. **Scroll-stop factor** — will this stop thumbs mid-scroll?
3. **Message clarity** — is the value proposition instantly clear?
4. **Call-to-action strength** — will users take action?
5. **Platform fit** — does this feel native to Instagram?

Select the **best variation** and output the final approved copy + photo brief with any refinements.`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-ig-market-analyst", "output_key": "market_analysis"},
			{"id": "n2", "type": "agent", "agent_id": "sub-ig-strategist", "output_key": "campaign_strategy"},
			{"id": "n3", "type": "agent", "agent_id": "sub-ig-copywriter", "output_key": "ad_copy"},
			{"id": "n4", "type": "agent", "agent_id": "sub-ig-photographer", "output_key": "temp:photo_descriptions"},
			{"id": "n5", "type": "agent", "agent_id": "sub-ig-creative-director", "output_key": "creative_review"},
		},
	}

	return catalogWorkflowEntry(
		"Instagram Post Creator",
		"Five-agent creative pipeline: analyse the product, develop campaign strategy, write ad copy, "+
			"describe photographs, and review the creative package. Translated from crewAI-examples/crews/instagram_post.",
		"Create a complete Instagram ad campaign — analysis, strategy, copy, photo direction, and creative review.",
		"Examples",
		[]string{"sequential", "state-passing", "marketing", "instagram", "creative", "crew-ai"},
		iconCamera,
		true,
		subs,
		graph,
	)
}

// ─── Marketing Strategy ─────────────────────────────────────────────────────
// Source: crewAI-examples/crews/marketing_strategy
//
// Three agents with explicit context fan-in: the final content task merges
// outputs from both strategy and campaign tasks.
//
//	MarketResearcher → output_key: "market_research"
//	Strategist       → reads {market_research}, output_key: "campaign_strategy"
//	ContentCreator   → reads {market_research} + {campaign_strategy}, output_key: "marketing_content"

func marketingStrategyEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-mkt-researcher",
			Name:        "MarketResearcher",
			Description: "Conducts in-depth product and competitor analysis.",
			Icon:        iconSearchCode,
			SystemPrompt: `You are a Lead Market Analyst with expertise in competitive research.

Given a product or service description, produce a comprehensive market research report:
1. **Product strengths** — core differentiators and unique value.
2. **Target audience** — customer segments with demographics and psychographics.
3. **Competitor landscape** — top 3 competitors, their positioning, and weaknesses.
4. **Market trends** — relevant industry trends and opportunities.
5. **SWOT summary** — strengths, weaknesses, opportunities, threats.

Be analytical and data-oriented. Output only the research report.`,
		},
		{
			ID:          "sub-mkt-strategist",
			Name:        "MarketingStrategist",
			Description: "Synthesises market research into a strategy and campaign ideas. Reads {market_research}.",
			Icon:        iconTarget,
			SystemPrompt: `You are a Chief Marketing Strategist.

**Market Research:**
{market_research}

Based on the research, develop:
1. **Marketing strategy** — positioning statement, key messages, channel mix.
2. **Campaign ideas** — 3 creative campaign concepts with names, descriptions, and target channels.
3. **KPIs** — measurable success metrics for each campaign.
4. **Timeline** — suggested rollout phases.

Be strategic and actionable.`,
		},
		{
			ID:          "sub-mkt-content-creator",
			Name:        "ContentCreator",
			Description: "Creates marketing content from the research and strategy. Reads {market_research} and {campaign_strategy}.",
			Icon:        iconPencilLine,
			SystemPrompt: `You are a Creative Content Creator for digital marketing.

**Market Research:**
{market_research}

**Campaign Strategy:**
{campaign_strategy}

Create ready-to-publish content for the top campaign idea:
1. **Social media posts** — 3 posts (one each for LinkedIn, Twitter/X, Instagram).
2. **Email copy** — a launch announcement email (subject + body).
3. **Landing page headline + subheadline** — for the campaign landing page.
4. **Ad copy** — 2 short-form ad variants (Google Ads style).

Match the tone and messaging from the strategy. Output in Markdown.`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-mkt-researcher", "output_key": "market_research"},
			{"id": "n2", "type": "agent", "agent_id": "sub-mkt-strategist", "output_key": "campaign_strategy"},
			{"id": "n3", "type": "agent", "agent_id": "sub-mkt-content-creator", "output_key": "marketing_content"},
		},
	}

	return catalogWorkflowEntry(
		"Marketing Strategy",
		"Three-agent marketing pipeline: research the market, develop campaign strategy, then create ready-to-publish content. "+
			"The final agent merges outputs from both prior tasks. Translated from crewAI-examples/crews/marketing_strategy.",
		"Research, strategise, and produce marketing content for a product launch.",
		"Examples",
		[]string{"sequential", "state-passing", "marketing", "strategy", "content", "crew-ai"},
		iconTarget,
		false,
		subs,
		graph,
	)
}

// ─── Recruitment Pipeline ───────────────────────────────────────────────────
// Source: crewAI-examples/crews/recruitment
//
// Four agents: research candidates, score/match them, create outreach, then
// compile a comprehensive report that aggregates all prior outputs.

func recruitmentEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-recruiter-researcher",
			Name:        "CandidateResearcher",
			Description: "Finds potential candidates matching the job requirements.",
			Icon:        iconSearchCode,
			SystemPrompt: `You are a Job Candidate Researcher.

Given a job role and requirements, identify and profile 5 potential candidates:
1. **Candidate name** (fictional for this exercise).
2. **Background summary** — experience, skills, education.
3. **Strengths** — what makes them a strong fit.
4. **Potential concerns** — gaps or areas to probe.
5. **Source** — where this candidate might be found (LinkedIn, GitHub, etc.).

Output a structured list of all 5 candidates.`,
		},
		{
			ID:          "sub-candidate-matcher",
			Name:        "CandidateMatcher",
			Description: "Scores and ranks candidates against the job requirements. Reads {candidate_research}.",
			Icon:        iconClipboardCheck,
			SystemPrompt: `You are a Candidate Matcher and Scorer.

**Candidate Research:**
{candidate_research}

Score each candidate on a 1-10 scale across:
1. **Technical fit** — skills alignment.
2. **Experience level** — years and relevance.
3. **Culture fit** — based on stated values and work style.
4. **Growth potential** — ability to grow in the role.

Output a ranked table with overall scores and a 1-line justification per candidate.`,
		},
		{
			ID:          "sub-outreach-strategist",
			Name:        "OutreachStrategist",
			Description: "Develops personalised outreach strategies. Reads {candidate_research} and {match_scores}.",
			Icon:        iconPencilLine,
			SystemPrompt: `You are a Candidate Outreach Strategist.

**Candidate Research:**
{candidate_research}

**Match Scores:**
{match_scores}

For the top 3 candidates, create personalised outreach:
1. **Channel** — best way to reach them (email, LinkedIn, referral).
2. **Message** — a personalised outreach message (3-5 sentences).
3. **Hook** — what about this role would specifically appeal to them.
4. **Follow-up plan** — timing and second-touch approach.

Be personal, not generic.`,
		},
		{
			ID:          "sub-recruitment-reporter",
			Name:        "RecruitmentReporter",
			Description: "Compiles a comprehensive recruitment report. Reads {candidate_research}, {match_scores}, and {temp:outreach_plans}.",
			Icon:        iconBriefcase,
			SystemPrompt: `You are a Recruitment Reporting Specialist.

**Candidate Research:**
{candidate_research}

**Match Scores:**
{match_scores}

**Outreach Plans:**
{temp:outreach_plans}

Compile a comprehensive recruitment report for the hiring manager:
1. **Executive summary** — role, search scope, key findings.
2. **Candidate rankings** — final ranked list with scores.
3. **Top candidate deep-dives** — detailed profiles of top 3.
4. **Outreach status** — planned outreach for each.
5. **Recommendations** — next steps, timeline, hiring strategy.

Format as a professional Markdown document.`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-recruiter-researcher", "output_key": "candidate_research"},
			{"id": "n2", "type": "agent", "agent_id": "sub-candidate-matcher", "output_key": "match_scores"},
			{"id": "n3", "type": "agent", "agent_id": "sub-outreach-strategist", "output_key": "temp:outreach_plans"},
			{"id": "n4", "type": "agent", "agent_id": "sub-recruitment-reporter", "output_key": "recruitment_report"},
		},
	}

	return catalogWorkflowEntry(
		"Recruitment Pipeline",
		"Four-agent HR pipeline: research candidates, score/match them, develop personalised outreach, "+
			"then compile a comprehensive report. Translated from crewAI-examples/crews/recruitment.",
		"Find, evaluate, and plan outreach for job candidates.",
		"Examples",
		[]string{"sequential", "state-passing", "hr", "recruitment", "crew-ai"},
		iconUsers,
		false,
		subs,
		graph,
	)
}

// ─── Screenplay Writer ──────────────────────────────────────────────────────
// Source: crewAI-examples/crews/screenplay_writer
//
// Three agents turn a discussion or topic into a formatted screenplay:
// analyse the material, write dialogue, then format it.

func screenplayWriterEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-discussion-analyst",
			Name:        "DiscussionAnalyst",
			Description: "Distils key arguments and character perspectives from source material.",
			Icon:        iconSearchCode,
			SystemPrompt: `You are a Discussion Analyst for screenplay development.

Given a topic, debate, or source text from the user, produce:
1. **Key perspectives** — 2-4 distinct viewpoints/characters with their positions.
2. **Central conflict** — the core tension driving the discussion.
3. **Emotional beats** — the emotional arc of the discussion.
4. **Memorable moments** — potential dramatic highlights.
5. **Character voices** — brief description of how each character speaks.

Output only the analysis. This will feed the scriptwriter.`,
		},
		{
			ID:          "sub-scriptwriter",
			Name:        "Scriptwriter",
			Description: "Writes screenplay dialogue from the analysis. Reads {discussion_analysis}.",
			Icon:        iconPencilLine,
			SystemPrompt: `You are a professional Scriptwriter.

**Discussion Analysis:**
{discussion_analysis}

Write a short screenplay scene (2-4 pages) that dramatises the discussion:
1. Use proper screenplay format (scene headings, action lines, dialogue).
2. Give each character a distinct voice matching the analysis.
3. Build tension through the central conflict.
4. Include stage directions and emotional cues.
5. End with a compelling beat — resolution or cliffhanger.

Output only the screenplay.`,
		},
		{
			ID:          "sub-script-formatter",
			Name:        "ScriptFormatter",
			Description: "Polishes and formats the screenplay to industry standards. Reads {screenplay_draft}.",
			Icon:        iconClapperboard,
			SystemPrompt: `You are a Script Formatter and Editor.

**Draft Screenplay:**
{screenplay_draft}

Polish the screenplay to industry-standard format:
1. **Formatting** — proper scene headings (INT./EXT.), character cues, parentheticals.
2. **Pacing** — tighten dialogue, remove redundancies.
3. **Consistency** — character names, locations, continuity.
4. **Stage directions** — clear, visual, present tense.
5. **Final touches** — add FADE IN / FADE OUT, page breaks where appropriate.

Output the final formatted screenplay.`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-discussion-analyst", "output_key": "discussion_analysis"},
			{"id": "n2", "type": "agent", "agent_id": "sub-scriptwriter", "output_key": "screenplay_draft"},
			{"id": "n3", "type": "agent", "agent_id": "sub-script-formatter", "output_key": "formatted_screenplay"},
		},
	}

	return catalogWorkflowEntry(
		"Screenplay Writer",
		"Three-agent creative pipeline: analyse a discussion into characters and conflict, write screenplay dialogue, "+
			"then format to industry standards. Translated from crewAI-examples/crews/screenplay_writer.",
		"Turn a topic or discussion into a formatted screenplay scene.",
		"Examples",
		[]string{"sequential", "state-passing", "creative", "screenplay", "writing", "crew-ai"},
		iconClapperboard,
		false,
		subs,
		graph,
	)
}

// ─── Stock Analysis ──────────────────────────────────────────────────────────
// Source: crewAI-examples/crews/stock_analysis
//
// Two analysts run in parallel (financial + research), then an investment
// advisor synthesises both into a recommendation.
//
// CrewAI uses sequential with implicit ordering; here we model the first two
// tasks as parallel since they have no context dependencies on each other.

func stockAnalysisEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-financial-analyst",
			Name:        "FinancialAnalyst",
			Description: "Analyses financial data and market trends for a stock.",
			Icon:        iconTrendingUp,
			SystemPrompt: `You are a top Financial Analyst on Wall Street.

Given a stock ticker, produce a financial analysis:
1. **Price overview** — current price, 52-week range, market cap.
2. **Key metrics** — P/E ratio, EPS, revenue growth, profit margins.
3. **Technical indicators** — moving averages, RSI, support/resistance levels.
4. **Financial health** — debt-to-equity, free cash flow, balance sheet strength.
5. **Valuation** — is the stock overvalued, undervalued, or fairly valued?

Be quantitative and cite specific numbers. Output only the analysis.`,
		},
		{
			ID:          "sub-research-analyst",
			Name:        "ResearchAnalyst",
			Description: "Researches news, sentiment, and industry developments for a stock.",
			Icon:        iconSearchCode,
			SystemPrompt: `You are a Staff Research Analyst specialising in market intelligence.

Given a stock ticker, produce a research brief:
1. **Recent news** — top 3-5 recent headlines and their implications.
2. **Industry trends** — sector performance and tailwinds/headwinds.
3. **Analyst consensus** — summary of analyst ratings and price targets.
4. **Sentiment** — social media and news sentiment (bullish/bearish/neutral).
5. **Risk factors** — regulatory, competitive, and macro risks.

Be thorough and evidence-based. Output only the research brief.`,
		},
		{
			ID:          "sub-investment-advisor",
			Name:        "InvestmentAdvisor",
			Description: "Synthesises financial and research analysis into a recommendation. Reads {financial_analysis} and {research_brief}.",
			Icon:        iconTrendingUp,
			SystemPrompt: `You are a Private Investment Advisor.

**Financial Analysis:**
{financial_analysis}

**Research Brief:**
{research_brief}

Synthesise both inputs into an investment recommendation:
1. **Summary** — one-paragraph executive summary.
2. **Bull case** — best-case scenario and upside potential.
3. **Bear case** — worst-case scenario and downside risk.
4. **Recommendation** — Buy / Hold / Sell with conviction level (High/Medium/Low).
5. **Price target** — your 12-month price target with justification.
6. **Risk management** — suggested position size and stop-loss level.

This is for educational purposes only — not financial advice.`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{
				"id":   "n1",
				"type": "parallel",
				"children": []map[string]interface{}{
					{"id": "n1a", "type": "agent", "agent_id": "sub-financial-analyst", "output_key": "financial_analysis"},
					{"id": "n1b", "type": "agent", "agent_id": "sub-research-analyst", "output_key": "research_brief"},
				},
			},
			{"id": "n2", "type": "agent", "agent_id": "sub-investment-advisor", "output_key": "investment_recommendation"},
		},
	}

	return catalogWorkflowEntry(
		"Stock Analysis",
		"Two analysts run in parallel (financial data + market research), then an investment advisor synthesises both into a Buy/Hold/Sell recommendation. "+
			"Demonstrates parallel fan-in. Translated from crewAI-examples/crews/stock_analysis.",
		"Analyse a stock from financial and research angles, then get an investment recommendation.",
		"Examples",
		[]string{"parallel", "fan-in", "finance", "stock", "analysis", "crew-ai"},
		iconTrendingUp,
		true,
		subs,
		graph,
	)
}

// ─── Prep for a Meeting ─────────────────────────────────────────────────────
// Source: crewAI-examples/crews/prep-for-a-meeting
//
// CrewAI's async_execution pattern: two research tasks run in parallel, then
// a strategy task fans in both results, and a final briefing aggregates
// everything. Classic parallel → sequential DAG.
//
// CompositionNode mapping:
//   parallel(Researcher ∥ IndustryAnalyst) → StrategyAdvisor → BriefingCoordinator

func prepForMeetingEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-participant-researcher",
			Name:        "ParticipantResearcher",
			Description: "Researches meeting participants — roles, backgrounds, recent activity.",
			Icon:        iconUsers,
			SystemPrompt: `You are a Research Specialist preparing intelligence for an important meeting.

Given a list of meeting participants (names and/or companies), produce a dossier:
1. **For each participant:**
   - Current role and company.
   - Professional background highlights.
   - Recent news, publications, or social media activity.
   - Known interests, priorities, or pain points.
2. **Relationship map** — any known connections between participants.
3. **Conversation starters** — 2-3 personalised talking points per person.

Be thorough but concise. Output only the research dossier.`,
		},
		{
			ID:          "sub-industry-analyst",
			Name:        "IndustryAnalyst",
			Description: "Analyses industry trends relevant to the meeting topic. Runs in parallel with ParticipantResearcher.",
			Icon:        iconTrendingUp,
			SystemPrompt: `You are an Industry Analyst preparing context for a business meeting.

Given a meeting topic or industry context, produce an analysis:
1. **Industry overview** — current state and key players.
2. **Recent trends** — 3-5 significant developments.
3. **Challenges** — top industry challenges relevant to the meeting.
4. **Opportunities** — emerging opportunities to discuss.
5. **Data points** — 3-5 statistics that could strengthen the conversation.

Be analytical and forward-looking. Output only the industry analysis.`,
		},
		{
			ID:          "sub-meeting-strategist",
			Name:        "MeetingStrategist",
			Description: "Develops talking points and strategy from both research streams. Reads {participant_research} and {industry_analysis}.",
			Icon:        iconTarget,
			SystemPrompt: `You are a Meeting Strategy Advisor.

**Participant Research:**
{participant_research}

**Industry Analysis:**
{industry_analysis}

Develop a meeting strategy:
1. **Objectives** — 3 clear goals for this meeting.
2. **Key talking points** — 5-7 points that leverage both the participant insights and industry context.
3. **Questions to ask** — strategic questions that demonstrate knowledge and build rapport.
4. **Potential objections** — likely pushback and prepared responses.
5. **Desired outcomes** — what success looks like.

Be strategic and actionable.`,
		},
		{
			ID:          "sub-briefing-coordinator",
			Name:        "BriefingCoordinator",
			Description: "Compiles everything into a single-page briefing document. Reads all prior outputs.",
			Icon:        iconCalendarClock,
			SystemPrompt: `You are a Briefing Coordinator.

**Participant Research:**
{participant_research}

**Industry Analysis:**
{industry_analysis}

**Meeting Strategy:**
{temp:meeting_strategy}

Compile a **one-page executive briefing** for the meeting:
1. **Meeting header** — date placeholder, attendees, topic.
2. **At a glance** — 3-bullet summary of what you need to know.
3. **Key participants** — one-liner per person.
4. **Talking points** — prioritised list.
5. **Watch-outs** — 2-3 things to be careful about.
6. **Success criteria** — how to know the meeting went well.

Keep it scannable — the reader has 5 minutes before walking in.`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{
				"id":   "n1",
				"type": "parallel",
				"children": []map[string]interface{}{
					{"id": "n1a", "type": "agent", "agent_id": "sub-participant-researcher", "output_key": "participant_research"},
					{"id": "n1b", "type": "agent", "agent_id": "sub-industry-analyst", "output_key": "industry_analysis"},
				},
			},
			{"id": "n2", "type": "agent", "agent_id": "sub-meeting-strategist", "output_key": "temp:meeting_strategy"},
			{"id": "n3", "type": "agent", "agent_id": "sub-briefing-coordinator", "output_key": "meeting_briefing"},
		},
	}

	return catalogWorkflowEntry(
		"Meeting Prep",
		"Parallel research fan-in: a Participant Researcher and Industry Analyst run concurrently, "+
			"then a Strategist merges both outputs, and a Coordinator compiles a one-page briefing. "+
			"Demonstrates async_execution → context fan-in. Translated from crewAI-examples/crews/prep-for-a-meeting.",
		"Prepare a comprehensive meeting briefing with parallel research streams.",
		"Examples",
		[]string{"parallel", "fan-in", "meeting", "strategy", "research", "crew-ai"},
		iconCalendarClock,
		true,
		subs,
		graph,
	)
}

// ─── Surprise Trip ──────────────────────────────────────────────────────────
// Source: crewAI-examples/crews/surprise_trip
//
// Two scouts run in parallel (activities + restaurants), then a compiler
// merges both into a day-by-day itinerary.

func surpriseTripEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-activity-planner",
			Name:        "ActivityPlanner",
			Description: "Finds personalised activities based on traveller interests. Runs in parallel with RestaurantScout.",
			Icon:        iconMapPin,
			SystemPrompt: `You are a Personalised Activity Planner.

Given a destination city, travel dates, and the traveller's interests, find:
1. **Must-do activities** — 5 experiences tailored to their interests.
2. **Off-the-beaten-path** — 3 unique, lesser-known activities.
3. **Weather-dependent options** — indoor alternatives for bad weather.
4. **Booking tips** — what to reserve in advance vs. walk-up.
5. **Time estimates** — how long each activity takes, including travel time.

Prioritise activities that match the traveller's stated interests. Output as a structured list.`,
		},
		{
			ID:          "sub-restaurant-scout",
			Name:        "RestaurantScout",
			Description: "Finds restaurants, scenic spots, and local experiences. Runs in parallel with ActivityPlanner.",
			Icon:        iconMapPin,
			SystemPrompt: `You are a Restaurant and Dining Scout.

Given a destination city and traveller preferences (dietary restrictions, cuisine preferences, budget), find:
1. **Top restaurants** — 5 standout dining experiences with cuisine type and price range.
2. **Local specialties** — must-try dishes and where to get them.
3. **Scenic dining** — 2-3 restaurants with exceptional views or ambiance.
4. **Budget options** — 3 excellent affordable spots.
5. **Food experiences** — markets, food tours, or cooking classes.

Include a mix of price ranges and cuisine styles. Output as a structured list.`,
		},
		{
			ID:          "sub-itinerary-compiler",
			Name:        "ItineraryCompiler",
			Description: "Compiles activities and dining into a day-by-day itinerary. Reads {planned_activities} and {restaurant_picks}.",
			Icon:        iconPlane,
			SystemPrompt: `You are an Itinerary Compiler creating the perfect surprise trip.

**Planned Activities:**
{planned_activities}

**Restaurant Picks:**
{restaurant_picks}

Create a **day-by-day trip itinerary** that weaves activities and dining together:
1. **For each day:**
   - Morning activity + lunch recommendation.
   - Afternoon activity + snack/café suggestion.
   - Evening activity + dinner recommendation.
2. **Logistics** — estimated travel times between venues.
3. **Budget summary** — estimated daily spend.
4. **Packing essentials** — items needed for planned activities.

Make the flow natural — don't zigzag across the city.`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{
				"id":   "n1",
				"type": "parallel",
				"children": []map[string]interface{}{
					{"id": "n1a", "type": "agent", "agent_id": "sub-activity-planner", "output_key": "planned_activities"},
					{"id": "n1b", "type": "agent", "agent_id": "sub-restaurant-scout", "output_key": "restaurant_picks"},
				},
			},
			{"id": "n2", "type": "agent", "agent_id": "sub-itinerary-compiler", "output_key": "trip_itinerary"},
		},
	}

	return catalogWorkflowEntry(
		"Surprise Trip Planner",
		"Two scouts run in parallel — activities and restaurants — then a compiler weaves them into a day-by-day itinerary. "+
			"Demonstrates parallel fan-in for travel planning. Translated from crewAI-examples/crews/surprise_trip.",
		"Plan a surprise trip: find activities and restaurants in parallel, then compile an itinerary.",
		"Examples",
		[]string{"parallel", "fan-in", "travel", "trip", "planning", "crew-ai"},
		iconMapPin,
		false,
		subs,
		graph,
	)
}

// ─── Write a Book ───────────────────────────────────────────────────────────
// Source: crewAI-examples/flows/write_a_book_with_flows
//
// Map-reduce pattern: generate an outline, then write 3 chapters in parallel,
// then join into a final book. In the original flow, the number of chapters is
// dynamic; here we use 3 fixed parallel chapter writers as a representative.
//
// CompositionNode mapping:
//   sequential(
//     Outliner,
//     parallel(ChapterWriter1 ∥ ChapterWriter2 ∥ ChapterWriter3),
//     Editor
//   )

func writeABookEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-book-outliner",
			Name:        "BookOutliner",
			Description: "Generates a structured book outline with chapter summaries.",
			Icon:        iconBookOpen,
			SystemPrompt: `You are a Book Outline Architect.

Given a book topic and target audience from the user, create a detailed outline:
1. **Title** — a compelling book title.
2. **Premise** — one-paragraph book description.
3. **Target audience** — who this book is for.
4. **Chapter outline** — exactly 3 chapters, each with:
   - Chapter title
   - Key points to cover (3-5 bullets)
   - Opening hook
   - Estimated word count (aim for ~1,500 words per chapter)

Output only the outline. The chapter writers will use it to write the full text.`,
		},
		{
			ID:          "sub-chapter-writer-1",
			Name:        "ChapterWriter1",
			Description: "Writes Chapter 1 based on the outline. Reads {book_outline}. Runs in parallel with other chapter writers.",
			Icon:        iconPencilLine,
			SystemPrompt: `You are a Book Chapter Writer.

**Book Outline:**
{book_outline}

Write **Chapter 1** in full based on the outline above.

**Requirements:**
- Follow the chapter's key points from the outline.
- Open with the specified hook.
- Aim for ~1,500 words.
- Use clear, engaging prose appropriate for the target audience.
- Include relevant examples, anecdotes, or data.
- End with a transition that leads into Chapter 2.

Output only the chapter text with a "# Chapter 1: [Title]" heading.`,
		},
		{
			ID:          "sub-chapter-writer-2",
			Name:        "ChapterWriter2",
			Description: "Writes Chapter 2 based on the outline. Reads {book_outline}. Runs in parallel with other chapter writers.",
			Icon:        iconPencilLine,
			SystemPrompt: `You are a Book Chapter Writer.

**Book Outline:**
{book_outline}

Write **Chapter 2** in full based on the outline above.

**Requirements:**
- Follow the chapter's key points from the outline.
- Open with the specified hook.
- Aim for ~1,500 words.
- Use clear, engaging prose appropriate for the target audience.
- Include relevant examples, anecdotes, or data.
- End with a transition that leads into Chapter 3.

Output only the chapter text with a "# Chapter 2: [Title]" heading.`,
		},
		{
			ID:          "sub-chapter-writer-3",
			Name:        "ChapterWriter3",
			Description: "Writes Chapter 3 based on the outline. Reads {book_outline}. Runs in parallel with other chapter writers.",
			Icon:        iconPencilLine,
			SystemPrompt: `You are a Book Chapter Writer.

**Book Outline:**
{book_outline}

Write **Chapter 3** — the final chapter — in full based on the outline above.

**Requirements:**
- Follow the chapter's key points from the outline.
- Open with the specified hook.
- Aim for ~1,500 words.
- Use clear, engaging prose appropriate for the target audience.
- Include relevant examples, anecdotes, or data.
- End with a strong conclusion that wraps up the entire book.

Output only the chapter text with a "# Chapter 3: [Title]" heading.`,
		},
		{
			ID:          "sub-book-editor",
			Name:        "BookEditor",
			Description: "Joins all chapters and edits for consistency. Reads {book_outline}, {chapter_1}, {chapter_2}, {chapter_3}.",
			Icon:        iconBookOpenCheck,
			SystemPrompt: `You are a Book Editor and Publisher.

**Book Outline:**
{book_outline}

**Chapter 1:**
{chapter_1}

**Chapter 2:**
{chapter_2}

**Chapter 3:**
{chapter_3}

Compile and edit the complete book:
1. **Title page** — title, subtitle, author placeholder.
2. **Table of contents** — with chapter titles.
3. **All chapters** — in order, with consistent formatting.
4. **Edit for:**
   - Consistency of tone and style across chapters.
   - Smooth transitions between chapters.
   - Factual accuracy and internal consistency.
   - Grammar, spelling, and punctuation.
5. **Afterword** — a brief concluding note.

Output the complete, publication-ready book in Markdown.`,
		},
	}

	graph := map[string]interface{}{
		"id":   "root",
		"type": "sequential",
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-book-outliner", "output_key": "book_outline"},
			{
				"id":   "n2",
				"type": "parallel",
				"children": []map[string]interface{}{
					{"id": "n2a", "type": "agent", "agent_id": "sub-chapter-writer-1", "output_key": "chapter_1"},
					{"id": "n2b", "type": "agent", "agent_id": "sub-chapter-writer-2", "output_key": "chapter_2"},
					{"id": "n2c", "type": "agent", "agent_id": "sub-chapter-writer-3", "output_key": "chapter_3"},
				},
			},
			{"id": "n3", "type": "agent", "agent_id": "sub-book-editor", "output_key": "final_book"},
		},
	}

	return catalogWorkflowEntry(
		"Write a Book",
		"Map-reduce pattern: an outliner generates a 3-chapter plan, three chapter writers work in parallel, "+
			"then an editor joins and polishes the complete book. Demonstrates sequential → parallel → sequential. "+
			"Translated from crewAI-examples/flows/write_a_book_with_flows.",
		"Generate a book outline, write chapters in parallel, then edit into a complete book.",
		"Examples",
		[]string{"parallel", "map-reduce", "sequential", "book", "writing", "crew-ai"},
		iconBookOpenCheck,
		true,
		subs,
		graph,
	)
}

// ─── Self-Evaluation Loop ───────────────────────────────────────────────────
// Source: crewAI-examples/flows/self_evaluation_loop_flow
//
// A generator and evaluator run inside a loop (max 3 iterations). The
// evaluator exits the loop early (via exit_loop) when quality is sufficient.
//
// CrewAI concept mapping:
//   Flow @router → "retry"/"complete"  →  loopagent with exit_loop tool
//   Flow State retry_count             →  max_iterations: 3
//   ShakespeareanXPostCrew             →  PostGenerator sub-agent
//   XPostReviewCrew                    →  PostEvaluator sub-agent

func selfEvaluationLoopEntry() AgentCatalogEntry {
	subs := []CatalogSubAgent{
		{
			ID:          "sub-post-generator",
			Name:        "ContentGenerator",
			Description: "Generates or refines content based on the user's request and any prior feedback.",
			Icon:        iconPencilLine,
			SystemPrompt: `You are a creative Content Generator.

On each iteration of this loop, you either:
- **First pass:** Generate content based on the user's request.
- **Subsequent passes:** Refine your previous output based on the evaluator's feedback (visible in conversation history).

**Requirements:**
- Be creative and original.
- Keep the content concise (200-300 words).
- Incorporate all feedback from previous evaluations.
- Label your output with the current pass number: "Pass N:".

Output only the content.`,
		},
		{
			ID:          "sub-post-evaluator",
			Name:        "ContentEvaluator",
			Description: "Evaluates content quality and calls exit_loop when satisfied.",
			Icon:        iconSparkles,
			SystemPrompt: `You are a strict Content Evaluator. You run after the Content Generator in a loop.

**Evaluate the generated content on:**
1. **Quality** (1-10) — is it well-written, engaging, and original?
2. **Completeness** (1-10) — does it address the user's request fully?
3. **Conciseness** (1-10) — is it appropriately sized (200-300 words)?
4. **Creativity** (1-10) — is it fresh and interesting?

**Rules:**
- If ALL scores are ≥ 8: the content is **approved**. Call the **exit_loop** tool to end the loop.
- If ANY score is < 8: provide specific, actionable feedback for the next iteration. Do NOT call exit_loop.

**Output format:**
- Scores table.
- Verdict: "Approved — exiting loop" or "Needs revision — feedback below".
- If revision needed: numbered list of specific improvements.`,
		},
	}

	graph := map[string]interface{}{
		"id":             "root",
		"type":           "loop",
		"max_iterations": 3,
		"children": []map[string]interface{}{
			{"id": "n1", "type": "agent", "agent_id": "sub-post-generator"},
			{"id": "n2", "type": "agent", "agent_id": "sub-post-evaluator"},
		},
	}

	return catalogWorkflowEntry(
		"Self-Evaluation Loop",
		"A generator and evaluator run inside a loop (max 3 iterations). The evaluator exits early via exit_loop "+
			"when quality is sufficient, or provides feedback for the next pass. Demonstrates the generate→evaluate "+
			"refinement pattern. Translated from crewAI-examples/flows/self_evaluation_loop_flow.",
		"Generate content, evaluate it, and refine in a loop until quality criteria are met.",
		"Examples",
		[]string{"loop", "self-evaluation", "refinement", "quality", "crew-ai"},
		iconSparkles,
		true,
		subs,
		graph,
	)
}
