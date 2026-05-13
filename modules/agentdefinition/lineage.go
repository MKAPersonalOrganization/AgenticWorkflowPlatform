package agentdefinition

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// State-lineage analyzer
//
// Walks a composition graph and computes, for every leaf agent in the tree,
// which session-state keys are guaranteed producible by the time that leaf
// runs (`ProducibleBefore`) and which keys the leaf's instruction templates
// reference (`References`). The set difference produces `Issues` —
// design-time warnings the UI surfaces beside the agent designer's
// existing structural validation.
//
// The same placeholder regex ADK-Go uses at runtime is mirrored here so
// what the analyzer flags is exactly what the runtime would try to inject:
// `{+[^{}]*}+`, see
// vendor/google.golang.org/adk/internal/llminternal/instruction_processor.go:70.
// References to scoped state (`app:`, `user:`, `temp:`) and artifacts
// (`artifact.X`) bypass graph-flow lineage and are skipped — those are
// resolved by separate ADK subsystems and have no producer in the graph.
//
// Composition semantics modelled (mirroring ADK):
//   - Sequential: child[i] sees outputs of child[0..i-1]
//   - Parallel:   children run state-isolated; their outputs become visible
//     only AFTER the parallel completes (merged delta).
//   - Loop:       within a single iteration, sequential semantics apply.
//     After the first iteration every child observes the loop body's full
//     output set — modelled as "every child can see every loop-body output"
//     since runs > 1 in steady state.
//   - Conditional: the FIRST child is the evaluator (its OutputKey drives
//     routing); subsequent children are branches matched against the
//     evaluator output by their `Condition` field. The conditional as a
//     whole produces evaluator-output ∪ ⋂(branch outputs) — strict
//     intersection so downstream agents only "see" keys every branch
//     guarantees.
// ─────────────────────────────────────────────────────────────────────────────

// placeholderRegex mirrors ADK-Go's instruction-template placeholder regex
// at vendor/google.golang.org/adk/internal/llminternal/instruction_processor.go:70.
// What ADK-Go injects at runtime is exactly what this analyzer parses, so
// the two cannot drift on `{{key}}` vs `{key}` quirks.
var placeholderRegex = regexp.MustCompile(`{+[^{}]*}+`)

// validKeyRegex matches a bare ADK state-key identifier — letters, digits,
// underscore, leading non-digit. Matches the runtime accept set in
// vendor/google.golang.org/adk/.../instruction_processor.go.
var validKeyRegex = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// IssueKind enumerates the categories the analyzer reports. UI surfaces
// each kind differently (orange-warning vs red-error).
type IssueKind string

const (
	// IssueMissingRef — instruction references {key} but nothing in the
	// reachable lineage produces it. Surfaces as a red ring on the prompt
	// editor and a Validation-tab error.
	IssueMissingRef IssueKind = "missing_ref"

	// IssueUnusedOutput — agent declares output_key but no downstream peer
	// reads it. Often legitimate (workflow's final answer), so the
	// analyzer suppresses this for the last sequential child. Surfaces as
	// a soft warning.
	IssueUnusedOutput IssueKind = "unused_output"

	// IssueConditionalNoEvaluatorOutput — a conditional node's evaluator
	// (first child) has no output_key, so branch routing has nothing to
	// match against. This is always a hard error.
	IssueConditionalNoEvaluatorOutput IssueKind = "conditional_no_evaluator_output"

	// IssueBranchOnlyKey — a key is produced in one branch but referenced
	// downstream of the conditional. Soft warning because the user may
	// know they always take that branch in practice.
	IssueBranchOnlyKey IssueKind = "branch_only_key"
)

// Reference captures a single instruction-template placeholder. The
// frontend uses these to highlight references inline in the prompt
// editor.
type Reference struct {
	Key      string `json:"key"`
	Optional bool   `json:"optional"`           // true for `{key?}`
	Source   string `json:"source"`             // "system_prompt" or "global_instruction"
	NodeID   string `json:"node_id,omitempty"`  // composition node that owns this reference
	AgentID  string `json:"agent_id,omitempty"` // underlying agent
}

// LineageIssue is one analyzer finding.
type LineageIssue struct {
	Kind    IssueKind `json:"kind"`
	NodeID  string    `json:"node_id,omitempty"`
	AgentID string    `json:"agent_id,omitempty"`
	Key     string    `json:"key,omitempty"`
	Message string    `json:"message"`
}

// StateLineage is the analyzer output. Sets are returned sorted (slices,
// not maps) so the JSON wire-format is deterministic and the client can
// diff results stably across calls.
type StateLineage struct {
	// ProducibleBefore[nodeID] lists the state keys guaranteed available
	// when this leaf starts. Empty for composite nodes.
	ProducibleBefore map[string][]string `json:"producible_before"`

	// Produces[nodeID] lists state keys this node (and its subtree) is
	// guaranteed to write before completing.
	Produces map[string][]string `json:"produces"`

	// References[nodeID] lists every {key} placeholder the underlying
	// agent's instructions reference. Surfacing the full list lets the
	// UI render inline "this key comes from X" tooltips on the prompt.
	References map[string][]Reference `json:"references"`

	// Issues are the analyzer's findings, sorted by node ID for stable
	// output. UI groups them by NodeID.
	Issues []LineageIssue `json:"issues"`
}

// AnalyzeLineage runs the static analyzer. `root` is the composition
// graph (typically `agent.Graph` from the in-flight designer state);
// `agents` is a lookup from AgentID to the underlying AgentDefinition
// so the analyzer can read SystemPrompt / OutputKey / GlobalInstruction.
//
// When `agents` is missing an entry referenced by the graph the analyzer
// records that leaf with an empty References set (the structural
// validator already catches the missing-agent case, so it isn't
// re-reported here).
func AnalyzeLineage(root *CompositionNode, agents map[string]*AgentDefinition) StateLineage {
	out := StateLineage{
		ProducibleBefore: map[string][]string{},
		Produces:         map[string][]string{},
		References:       map[string][]Reference{},
	}
	if root == nil {
		return out
	}

	ctx := &lineageCtx{
		agents:           agents,
		producibleBefore: map[string]map[string]bool{},
		produces:         map[string]map[string]bool{},
		references:       map[string][]Reference{},
		referencedAfter:  map[string]bool{},
		issues:           nil,
	}

	// First pass: produce + references + producibleBefore.
	// The empty set is what's available before the root runs — workflows
	// don't have ambient state keys in the design-time model.
	ctx.walk(root, map[string]bool{})

	// Second pass: collect every key referenced anywhere in the tree, so
	// we can flag unused outputs (a producer's key that no downstream
	// node mentions).
	ctx.collectReferencedKeys(root)

	// Third pass: emit unused-output issues. Skip the final child of the
	// top-level sequential — that's by convention the workflow's user-
	// facing answer and writes a key the chat surface displays.
	ctx.emitUnusedOutputs(root)

	// Materialise maps into the deterministic-output struct.
	for nodeID, set := range ctx.producibleBefore {
		out.ProducibleBefore[nodeID] = sortedKeys(set)
	}
	for nodeID, set := range ctx.produces {
		out.Produces[nodeID] = sortedKeys(set)
	}
	for nodeID, refs := range ctx.references {
		out.References[nodeID] = refs
	}
	out.Issues = ctx.issues
	sort.SliceStable(out.Issues, func(i, j int) bool {
		if out.Issues[i].NodeID != out.Issues[j].NodeID {
			return out.Issues[i].NodeID < out.Issues[j].NodeID
		}
		if out.Issues[i].Kind != out.Issues[j].Kind {
			return string(out.Issues[i].Kind) < string(out.Issues[j].Kind)
		}
		return out.Issues[i].Key < out.Issues[j].Key
	})

	return out
}

// lineageCtx threads mutable analyzer state through the recursive walk.
type lineageCtx struct {
	agents           map[string]*AgentDefinition
	producibleBefore map[string]map[string]bool
	produces         map[string]map[string]bool
	references       map[string][]Reference
	referencedAfter  map[string]bool
	issues           []LineageIssue
}

// walk computes Produces[node] and ProducibleBefore[child] for every
// descendant of `node`, given `incoming` — the set of keys producible
// at this node's entry. Returns the set produced by `node`'s subtree.
func (c *lineageCtx) walk(node *CompositionNode, incoming map[string]bool) map[string]bool {
	if node == nil {
		return map[string]bool{}
	}

	// Record entry context for every node (composite or leaf) — the
	// frontend uses this for "where does X come from" tooltips on nested
	// composites too.
	c.producibleBefore[node.ID] = copySet(incoming)

	switch node.Type {
	case NodeAgent:
		out := c.walkLeaf(node, incoming)
		c.produces[node.ID] = out
		return out

	case NodeRemote:
		// Remote agents are opaque — we don't know what they write to
		// state. Treat them as producing nothing for downstream peers.
		c.produces[node.ID] = map[string]bool{}
		return map[string]bool{}

	case NodeSequential:
		return c.walkSequential(node, incoming)

	case NodeParallel:
		return c.walkParallel(node, incoming)

	case NodeLoop:
		return c.walkLoop(node, incoming)

	case NodeConditional:
		return c.walkConditional(node, incoming)
	}

	// Unknown node type — the structural validator already errors on
	// these. Return empty to avoid masking the structural failure with
	// spurious lineage noise.
	c.produces[node.ID] = map[string]bool{}
	return map[string]bool{}
}

// walkLeaf records the leaf's instruction references, emits missing-ref
// issues, and returns the leaf's OutputKey (if any) as its output set.
func (c *lineageCtx) walkLeaf(node *CompositionNode, incoming map[string]bool) map[string]bool {
	out := map[string]bool{}
	agent := c.agents[node.AgentID]
	if agent == nil {
		// Missing agent — structural validator owns this error.
		return out
	}

	refs := extractReferences(agent.SystemPrompt, "system_prompt")
	refs = append(refs, extractReferences(agent.GlobalInstruction, "global_instruction")...)
	for i := range refs {
		refs[i].NodeID = node.ID
		refs[i].AgentID = agent.ID
	}
	c.references[node.ID] = refs

	for _, ref := range refs {
		if ref.Optional {
			continue
		}
		if !incoming[ref.Key] {
			c.issues = append(c.issues, LineageIssue{
				Kind:    IssueMissingRef,
				NodeID:  node.ID,
				AgentID: agent.ID,
				Key:     ref.Key,
				Message: fmt.Sprintf("%s references {%s} but no upstream agent produces it", agentLabel(agent), ref.Key),
			})
		}
	}

	if agent.OutputKey != "" {
		out[agent.OutputKey] = true
	}
	return out
}

// walkSequential propagates each child's output set into the next
// child's incoming set, so child[i] sees keys from child[0..i-1].
func (c *lineageCtx) walkSequential(node *CompositionNode, incoming map[string]bool) map[string]bool {
	available := copySet(incoming)
	out := map[string]bool{}
	for i := range node.Children {
		child := &node.Children[i]
		produced := c.walk(child, available)
		mergeInto(out, produced)
		mergeInto(available, produced)
	}
	c.produces[node.ID] = out
	return out
}

// walkParallel runs children with the same incoming set (no sibling
// visibility) and merges all of their outputs into the parallel's
// produces set — that union is what AN OUTER sequential sees after the
// parallel completes.
func (c *lineageCtx) walkParallel(node *CompositionNode, incoming map[string]bool) map[string]bool {
	out := map[string]bool{}
	for i := range node.Children {
		child := &node.Children[i]
		produced := c.walk(child, incoming)
		mergeInto(out, produced)
	}
	c.produces[node.ID] = out
	return out
}

// walkLoop models steady-state visibility: after iteration 1 every child
// can see every other child's output, so we feed each child the union of
// (incoming, loop-body produces). We do a two-pass fixed point so a
// child's output is visible to siblings declared before it. Issues
// emitted during the discovery pass are discarded — they correspond to
// "iteration 1 hasn't run yet" states that don't reflect runtime
// behaviour. The second pass re-emits any issues that remain at steady
// state.
func (c *lineageCtx) walkLoop(node *CompositionNode, incoming map[string]bool) map[string]bool {
	// First pass: discover what the loop body writes. Suppress issue
	// emission since we haven't established steady state yet.
	issuesBefore := len(c.issues)
	bodyOut := map[string]bool{}
	avail := copySet(incoming)
	for i := range node.Children {
		mergeInto(bodyOut, c.walk(&node.Children[i], avail))
		mergeInto(avail, bodyOut)
	}
	c.issues = c.issues[:issuesBefore]

	// Second pass: re-walk children with full body output visible.
	steady := copySet(incoming)
	mergeInto(steady, bodyOut)
	for i := range node.Children {
		c.walk(&node.Children[i], steady)
	}
	c.produces[node.ID] = bodyOut
	return bodyOut
}

// walkConditional treats the first child as the evaluator (its OutputKey
// is what branch.Condition matches against). Subsequent children are
// branches that each see `evaluator's output ∪ incoming`. The conditional's
// own produces set is `evaluator.output ∪ ⋂(branch outputs)` — strict
// intersection so downstream peers only see keys EVERY branch guarantees.
func (c *lineageCtx) walkConditional(node *CompositionNode, incoming map[string]bool) map[string]bool {
	if len(node.Children) == 0 {
		c.produces[node.ID] = map[string]bool{}
		return map[string]bool{}
	}

	evaluator := &node.Children[0]
	evalProduces := c.walk(evaluator, incoming)
	if len(evalProduces) == 0 {
		c.issues = append(c.issues, LineageIssue{
			Kind:    IssueConditionalNoEvaluatorOutput,
			NodeID:  evaluator.ID,
			AgentID: evaluator.AgentID,
			Message: "conditional evaluator must set output_key — without one, branch routing has nothing to match against",
		})
	}

	branchIncoming := copySet(incoming)
	mergeInto(branchIncoming, evalProduces)

	// Intersect branch outputs. nil = "not yet initialised"; once any
	// branch reports its set we switch to intersecting against it.
	var intersected map[string]bool
	for i := 1; i < len(node.Children); i++ {
		branch := &node.Children[i]
		bProduces := c.walk(branch, branchIncoming)
		if intersected == nil {
			intersected = copySet(bProduces)
		} else {
			intersected = intersect(intersected, bProduces)
		}
	}

	out := copySet(evalProduces)
	mergeInto(out, intersected)
	c.produces[node.ID] = out
	return out
}

// collectReferencedKeys gathers every key any node in the tree reads, so
// emitUnusedOutputs can spot producers whose key is never read.
func (c *lineageCtx) collectReferencedKeys(node *CompositionNode) {
	if node == nil {
		return
	}
	for _, ref := range c.references[node.ID] {
		c.referencedAfter[ref.Key] = true
	}
	for i := range node.Children {
		c.collectReferencedKeys(&node.Children[i])
	}
}

// emitUnusedOutputs walks the tree and flags any leaf whose OutputKey
// no other agent reads. The last child of the root sequential is exempt
// — it's the workflow's user-facing answer by convention.
func (c *lineageCtx) emitUnusedOutputs(root *CompositionNode) {
	// Identify the "terminal" node IDs we should not flag.
	terminal := map[string]bool{}
	c.markTerminal(root, terminal)

	root.Walk(func(n *CompositionNode) {
		if n.Type != NodeAgent {
			return
		}
		agent := c.agents[n.AgentID]
		if agent == nil || agent.OutputKey == "" {
			return
		}
		if terminal[n.ID] {
			return
		}
		if c.referencedAfter[agent.OutputKey] {
			return
		}
		c.issues = append(c.issues, LineageIssue{
			Kind:    IssueUnusedOutput,
			NodeID:  n.ID,
			AgentID: agent.ID,
			Key:     agent.OutputKey,
			Message: fmt.Sprintf("%s writes {%s} but no downstream agent reads it", agentLabel(agent), agent.OutputKey),
		})
	})
}

// markTerminal flags the rightmost descendant leaf at each sequential
// level — those are the conventional "final answer" producers and
// shouldn't be reported as unused.
func (c *lineageCtx) markTerminal(node *CompositionNode, out map[string]bool) {
	if node == nil {
		return
	}
	switch node.Type {
	case NodeAgent, NodeRemote:
		out[node.ID] = true
	case NodeSequential, NodeLoop:
		if len(node.Children) > 0 {
			c.markTerminal(&node.Children[len(node.Children)-1], out)
		}
	case NodeParallel:
		// Every parallel child counts as terminal — they all run in
		// parallel and contribute to the merged output, so a "final"
		// designation isn't meaningful.
		for i := range node.Children {
			c.markTerminal(&node.Children[i], out)
		}
	case NodeConditional:
		// Branches (children 1..n) are all terminal candidates; the
		// evaluator (child 0) is not.
		for i := 1; i < len(node.Children); i++ {
			c.markTerminal(&node.Children[i], out)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// extractReferences pulls every `{key}` / `{key?}` placeholder out of a
// template string, skipping scoped (`app:`, `user:`, `temp:`) and
// artifact (`artifact.X`) references — those resolve outside the graph
// flow and have no design-time producer. Also skips placeholders whose
// inner text doesn't match the ADK identifier shape, mirroring runtime
// behaviour where malformed placeholders are left as literal text.
func extractReferences(template, source string) []Reference {
	if template == "" {
		return nil
	}
	matches := placeholderRegex.FindAllString(template, -1)
	if len(matches) == 0 {
		return nil
	}
	seen := map[string]bool{}
	out := make([]Reference, 0, len(matches))
	for _, m := range matches {
		// Trim outer braces (the regex allows {+ ... }+ for ADK
		// quirks like `{{key}}`).
		inner := strings.TrimFunc(m, func(r rune) bool { return r == '{' || r == '}' })
		inner = strings.TrimSpace(inner)
		if inner == "" {
			continue
		}
		optional := false
		if strings.HasSuffix(inner, "?") {
			optional = true
			inner = strings.TrimSpace(strings.TrimSuffix(inner, "?"))
		}
		// Skip scoped references — these read from session-state
		// scopes that aren't produced by graph agents.
		if strings.HasPrefix(inner, "app:") ||
			strings.HasPrefix(inner, "user:") ||
			strings.HasPrefix(inner, "temp:") {
			continue
		}
		// Skip artifact references — those resolve via the artifact
		// service, not session state.
		if strings.Contains(inner, ".") {
			continue
		}
		if !validKeyRegex.MatchString(inner) {
			continue
		}
		dedupKey := inner
		if optional {
			dedupKey = inner + "?"
		}
		if seen[dedupKey] {
			continue
		}
		seen[dedupKey] = true
		out = append(out, Reference{
			Key:      inner,
			Optional: optional,
			Source:   source,
		})
	}
	return out
}

func copySet(in map[string]bool) map[string]bool {
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mergeInto(dst, src map[string]bool) {
	for k, v := range src {
		if v {
			dst[k] = true
		}
	}
}

func intersect(a, b map[string]bool) map[string]bool {
	out := map[string]bool{}
	for k := range a {
		if b[k] {
			out[k] = true
		}
	}
	return out
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// agentLabel returns the human-readable name the analyzer uses in issue
// messages — falls back to a short ID if name is missing.
func agentLabel(agent *AgentDefinition) string {
	if agent == nil {
		return "agent"
	}
	if agent.Name != "" {
		return agent.Name
	}
	if len(agent.ID) > 8 {
		return "agent " + agent.ID[:8]
	}
	return "agent " + agent.ID
}
