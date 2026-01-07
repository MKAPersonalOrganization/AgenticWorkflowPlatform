package middleware

import (
	"context"
	"encoding/json"
	"fmt"

	"school-with-ai/sdk/agentsdk/core"
)

// WhiteboardAction represents the type of whiteboard update
type WhiteboardAction string

const (
	WhiteboardActionWrite  WhiteboardAction = "write"  // Replace entire content
	WhiteboardActionAppend WhiteboardAction = "append" // Append to existing content
	WhiteboardActionClear  WhiteboardAction = "clear"  // Clear all content
)

const WriteWhiteboardToolDescription = `Write information to the shared whiteboard that all agents can see. Use this to share key formulas, diagrams, or important points. ALWAYS use markdown format.

## Actions
- 'write': REPLACE entire whiteboard content (check current content in system prompt to preserve existing sections)
- 'append': Add new content to the end
- 'clear': Erase all content

## Best Practice Workflow
The current whiteboard content is always shown in your system prompt. Review it first, then modify the content you need, and write back the entire updated content using action='write'.

Example workflow for updating a section:
1. Review current whiteboard content in your system prompt
2. Modify the specific section you need (e.g., update TODOS)
3. Keep other sections unchanged (e.g., formulas from specialists)
4. write_whiteboard(updated_content, action='write')

## Adding Images (Charts, Plots, Diagrams)

⚠️ CRITICAL: You CANNOT add images by hallucinating URLs. You MUST use the plot tool first!

Workflow:
1. Call the plot tool with gnuplot_script and title parameters
2. The tool returns JSON with image_url field
3. COPY THE EXACT image_url VALUE from the JSON response (it starts with /assets/)
4. Use markdown syntax: ![Description](PASTE_EXACT_IMAGE_URL_HERE)

EXACT COPY-PASTE REQUIRED:
When plot tool returns: {"image_url": "/assets/charts/sine-wave_1234567890.png", "success": true}
You write to whiteboard: ![Sine Wave Graph](/assets/charts/sine-wave_1234567890.png)
                                          ↑ EXACT COPY of image_url value ↑

CRITICAL RULES:
1. The path MUST start with /assets/ (with leading slash, no https://)
2. NEVER add "https://" or any protocol prefix
3. COPY the image_url value EXACTLY as returned by the plot tool
4. Do NOT modify, shorten, or change the path in any way

WRONG ❌: ![Plot](https://assets/charts/file.png)  ← Don't add https://
WRONG ❌: ![Plot](assets/charts/file.png)  ← Missing leading slash /
WRONG ❌: ![Plot](/charts/file.png)  ← Don't skip the /assets/ part
CORRECT ✓: ![Plot](/assets/charts/sine-wave_1234567890.png)  ← Exact copy from image_url!

## LaTeX Math Rendering

### Inline Math
- Use single dollar signs: $x^2 + y^2 = z^2$
- Or backslash parentheses: \\(E = mc^2\\)
- For inline formulas in explanations: The force $F = ma$ acts on the object

### Display Math (Centered Equations)
Use double dollar signs on separate lines:

$$
x = \\frac{-b \\pm \\sqrt{b^2 - 4ac}}{2a}
$$

Or backslash brackets:

\\[
E_k = \\frac{1}{2}mv^2
\\]

### Numbered Equations
Use equation environment for reference:

\\begin{equation}
E = mc^2
\\label{eq:einstein}
\\end{equation}

Reference with: See equation \\ref{eq:einstein} or \\eqref{eq:einstein}

For unnumbered:

\\begin{equation*}
a^2 + b^2 = c^2
\\end{equation*}

### Multiple Aligned Equations
Use align for multi-line equations with alignment:

\\begin{align}
E_{kinetic} &= \\frac{1}{2}mv^2 \\\\
E_{potential} &= mgh \\\\
E_{total} &= E_{kinetic} + E_{potential}
\\end{align}

Use align* for unnumbered:

\\begin{align*}
F_x &= ma_x \\\\
F_y &= ma_y
\\end{align*}

### Split Long Equations

\\begin{split}
a &= b + c - d \\\\
  &\\quad + e - f \\\\
  &= g + h
\\end{split}

### Multiple Equations (Ungrouped)

\\begin{gather}
a_1 = b_1 + c_1 \\\\
a_2 = b_2 + c_2
\\end{gather}

### Custom Tags

$$
\\frac{x(x^{2n} - x^{-2n})}{x^{2n} + x^{-2n}}
\\tag{A.1}
$$

### Matrices
Use matrix environments:

\\begin{pmatrix}
a & b \\\\
c & d
\\end{pmatrix}

Other types:
- bmatrix: brackets [a b; c d]
- vmatrix: determinant |a b; c d|
- Vmatrix: double bars ||a b; c d||

### Piecewise Functions

$$
f(x) = \\begin{cases}
x^2 & \\text{if } x \\geq 0 \\\\
-x & \\text{if } x < 0
\\end{cases}
$$

### Tables

\\begin{tabular}{l c r}
Left & Center & Right \\\\
1 & 2 & 3 \\\\
4 & 5 & 6
\\end{tabular}

### Common Symbols
- Subscripts: $v_x$, $F_g$, $E_k$
- Superscripts: $r^2$, $t^3$, $x^{2n}$
- Greek letters: $\\alpha$, $\\beta$, $\\theta$, $\\omega$, $\\lambda$, $\\rho$, $\\nabla$
- Derivatives: $\\frac{dx}{dt}$, $\\frac{d^2x}{dt^2}$
- Partial derivatives: $\\frac{\\partial f}{\\partial x}$
- Vectors: $\\vec{F}$, $\\vec{v}$ or bold $\\mathbf{F}$, $\\mathbf{v}$
- Units in equations: $E = 50$ \\text{ J}, $v = 10$ \\text{ m/s}

## Chemistry-Specific Formatting

### Chemical Equations
For balanced chemical equations:

$$
2H_2 + O_2 \\rightarrow 2H_2O
$$

With reaction conditions:

$$
CH_4 + 2O_2 \\xrightarrow{\\text{combustion}} CO_2 + 2H_2O
$$

Equilibrium:

$$
N_2 + 3H_2 \\rightleftharpoons 2NH_3
$$

### Chemical Formulas
- Inline: $H_2O$, $CO_2$, $H_2SO_4$, $Ca(OH)_2$
- Charges: $Na^+$, $SO_4^{2-}$, $Fe^{3+}$
- Equilibrium constant: $K_{eq} = \\frac{[C]^c[D]^d}{[A]^a[B]^b}$
- pH: $pH = -\\log[H^+]$

### SMILES Notation (Molecular Structures)
Use <smiles>SMILES_STRING</smiles> tags for inline structures:

- Water: <smiles>O</smiles>
- Methane: <smiles>C</smiles>
- Ethanol: <smiles>CCO</smiles>
- Benzene: <smiles>c1ccccc1</smiles>
- Glucose: <smiles>OC[CH]1OC(O)[CH](O)[CH](O)[CH]1O</smiles>

For larger structures, use code blocks with smiles language (three backticks with smiles).

## Markdown Formatting
- Headers: # Title, ## Section, ### Subsection
- Bold: **text** or \\textbf{text}
- Italic: *text* or \\textit{text}
- Underline: \\underline{text}
- Strikethrough: \\sout{text}
- Lists: use -, *, or numbered
- Code: backtick for inline code or triple backticks for blocks
- Footnotes: Text\\footnote{Explanation here}`

const WhiteboardSystemPrompt = `## Shared Whiteboard

The whiteboard is a shared markdown document visible to all agents. The current whiteboard content is always included below in your system prompt.

You can update the whiteboard using the write_whiteboard tool to:
- Share routing plans and task progress
- Display formulas, equations, and calculations
- Show diagrams and visualizations
- Communicate important context between agents

Best practices:
- Check the current whiteboard content in your system prompt before making updates
- Use read-modify-write pattern: review current content → update specific sections → write entire content
- Structure content with markdown headers (# Section Name)
- Preserve content from other agents when updating
- Use action='write' (not 'append') when updating existing sections to avoid duplication`

// WriteWhiteboardTool is the tool for writing to the whiteboard
type WriteWhiteboardTool struct {
	reqCtx *core.RequestContext
}

// NewWriteWhiteboardTool creates a new WriteWhiteboardTool
func NewWriteWhiteboardTool(reqCtx *core.RequestContext) *WriteWhiteboardTool {
	return &WriteWhiteboardTool{
		reqCtx: reqCtx,
	}
}

func (t *WriteWhiteboardTool) Name() string {
	return "write_whiteboard"
}

func (t *WriteWhiteboardTool) Description() string {
	return WriteWhiteboardToolDescription
}

func (t *WriteWhiteboardTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{
				"type":        "string",
				"description": "The content to write to the whiteboard (use markdown format)",
			},
			"action": map[string]interface{}{
				"type":        "string",
				"description": "How to update: 'write' (replace entire content - use read_whiteboard first to preserve sections), 'append' (add to end), 'clear' (erase all)",
				"enum":        []string{"write", "append", "clear"},
			},
		},
		"required": []string{"content", "action"},
	}
}

func (t *WriteWhiteboardTool) Invoke(ctx context.Context, arguments string) (string, error) {
	var args struct {
		Content string           `json:"content"`
		Action  WhiteboardAction `json:"action"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("failed to parse whiteboard arguments: %w", err)
	}

	// Validate action
	validActions := map[WhiteboardAction]bool{
		WhiteboardActionWrite:  true,
		WhiteboardActionAppend: true,
		WhiteboardActionClear:  true,
	}

	if !validActions[args.Action] {
		return "", fmt.Errorf("invalid action: %s (must be write, append, or clear)", args.Action)
	}

	var finalContent string

	// Update whiteboard based on action
	switch args.Action {
	case WhiteboardActionWrite:
		finalContent = args.Content
		SetWhiteboardContent(t.reqCtx, finalContent)
	case WhiteboardActionAppend:
		current := GetWhiteboardContent(t.reqCtx)
		if current == "" {
			finalContent = args.Content
		} else {
			finalContent = current + "\n" + args.Content
		}
		SetWhiteboardContent(t.reqCtx, finalContent)
	case WhiteboardActionClear:
		finalContent = ""
		SetWhiteboardContent(t.reqCtx, finalContent)
	}

	// No need to emit custom event - ToolCallExecutedEvent is emitted automatically
	// by the service layer and contains all the tool execution details

	return fmt.Sprintf("Whiteboard updated (%s)", args.Action), nil
}

// WhiteboardProjection provides read access to whiteboard content
type WhiteboardProjection interface {
	Get(threadID string) string
}

// WhiteboardMiddleware provides whiteboard management capabilities to agents
type WhiteboardMiddleware struct {
	systemPrompt string
	projection   WhiteboardProjection
}

// NewWhiteboardMiddleware creates a new WhiteboardMiddleware with default prompts
func NewWhiteboardMiddleware(projection WhiteboardProjection) *WhiteboardMiddleware {
	return &WhiteboardMiddleware{
		systemPrompt: WhiteboardSystemPrompt,
		projection:   projection,
	}
}

// NewWhiteboardMiddlewareWithPrompt creates a new WhiteboardMiddleware with custom prompt
func NewWhiteboardMiddlewareWithPrompt(projection WhiteboardProjection, systemPrompt string) *WhiteboardMiddleware {
	return &WhiteboardMiddleware{
		systemPrompt: systemPrompt,
		projection:   projection,
	}
}

// Process implements the Middleware interface
func (m *WhiteboardMiddleware) Process(ctx context.Context, reqCtx *core.RequestContext, next core.NextHandler) error {
	// Add write_whiteboard tool to the request context
	reqCtx.AddTool(NewWriteWhiteboardTool(reqCtx))

	// Add whiteboard instructions to system prompt
	reqCtx.AddSystemPrompt(m.systemPrompt)

	// Inject current whiteboard content from projection into system prompt
	var whiteboardContent string
	if m.projection != nil && reqCtx.ThreadID != "" {
		whiteboardContent = m.projection.Get(reqCtx.ThreadID)
	}

	if whiteboardContent != "" {
		whiteboardSection := fmt.Sprintf("\n\n---\n\n## Current Whiteboard Content\n\n%s\n\n---\n", whiteboardContent)
		reqCtx.AddSystemPrompt(whiteboardSection)
	} else {
		reqCtx.AddSystemPrompt("\n\n---\n\n## Current Whiteboard Content\n\nThe whiteboard is currently empty.\n\n---\n")
	}

	return next(ctx, reqCtx)
}

// GetWhiteboardContent retrieves the current whiteboard content from the request context
func GetWhiteboardContent(reqCtx *core.RequestContext) string {
	if content, ok := reqCtx.State["whiteboard_content"].(string); ok {
		return content
	}
	return ""
}

// SetWhiteboardContent updates the whiteboard content in the request context
func SetWhiteboardContent(reqCtx *core.RequestContext, content string) {
	reqCtx.State["whiteboard_content"] = content
}
