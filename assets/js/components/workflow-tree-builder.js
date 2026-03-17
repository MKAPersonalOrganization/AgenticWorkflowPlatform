/**
 * Workflow Tree Builder — ADK-Go Aligned Composition Tree Renderer
 *
 * Renders a recursive composition tree where each node maps 1:1 to an ADK-Go
 * agent constructor:
 *
 *   ● Agent (leaf)      → llmagent.New(llmagent.Config{Name, Instruction, Tools, ...})
 *   ↓ Sequential         → sequentialagent.New(sequentialagent.Config{AgentConfig: agent.Config{Name, SubAgents}})
 *   ∥ Parallel           → parallelagent.New(parallelagent.Config{AgentConfig: agent.Config{Name, SubAgents}})
 *   ↻ Loop               → loopagent.New(loopagent.Config{AgentConfig: agent.Config{Name, SubAgents}, MaxIterations})
 *   ⇝ Remote             → agent.New wrapping A2A connector (delegates full turn via A2A protocol)
 *
 * Key ADK-Go behaviors surfaced:
 *   - Every agent has a Name (agent.Config.Name) — composite nodes expose a Label input
 *   - Loop's last direct child agent automatically receives the exit_loop tool (backend)
 *   - Agent nodes can override TransferPolicy & IncludeContents per-workflow
 *   - OutputKey saves agent output to session state for {key} template references
 *
 * This single component works in any Alpine.js context that provides:
 *   agents, _getNodeByPath, changeNodeType, addChildToNode, removeChildFromNode
 */
export function workflowTreeBuilder() {
    return {
        init() { },

        // ══════════════════════════════════════════════════════════════════
        // Main render — single unified method (replaces the former
        // duplicated renderNode / renderPipelineNode pair)
        // ══════════════════════════════════════════════════════════════════

        /**
         * Recursively render a composition node and its children.
         *
         * @param {Object}  node         - CompositionNode to render
         * @param {string}  path         - Dot-notation path for mutations (e.g. "graph.0.1")
         * @param {number}  depth        - Nesting depth (controls indentation)
         * @param {boolean} canRemove    - Whether this node can be removed
         * @param {boolean} isLastInLoop - Whether this agent node is the last direct child of a loop
         */
        renderCompositionNode(node, path, depth, canRemove, isLastInLoop) {
            if (!node) return '';

            const isComposite = node.type === 'sequential' || node.type === 'parallel' || node.type === 'loop';
            const indent = depth > 0 ? 'ml-4 ' : '';
            const s = this._style(node.type);

            let html = `<div class="${indent}rounded-lg border ${s.border} ${s.bg} p-3 space-y-2">`;

            // ── Header: badge + type selector + name (composites) + remove ──
            html += `<div class="flex items-center gap-2 flex-wrap">`;
            html += `<span class="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-bold select-none ${s.badge}">${s.label}</span>`;

            // Type selector — shows ADK-Go type for clarity
            html += `<select class="h-7 rounded-md border border-border bg-background px-2 text-[11px] text-foreground focus:outline-none focus:ring-1 focus:ring-ring" `;
            html += `onchange="Alpine.closestDataStack(this).find(d=>d.changeNodeType)?.changeNodeType('${path}',this.value)">`;
            for (const [val, lbl] of [
                ['agent', 'LLM Agent → llmagent'],
                ['sequential', 'Sequential → sequentialagent'],
                ['parallel', 'Parallel → parallelagent'],
                ['loop', 'Loop → loopagent'],
                ['remote', 'Remote Agent → A2A'],
            ]) {
                html += `<option value="${val}"${val === node.type ? ' selected' : ''}>${lbl}</option>`;
            }
            html += `</select>`;

            // Name/Label for composite nodes → maps to agent.Config.Name in ADK-Go
            if (isComposite) {
                html += `<input type="text" placeholder="Name → agent.Config.Name" maxlength="60" `;
                html += `class="flex-1 min-w-[140px] h-7 rounded-md border border-border bg-background px-2 text-[11px] text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring" `;
                html += `value="${this._esc(node.label || '')}" `;
                html += `oninput="const n=Alpine.closestDataStack(this).find(d=>d._getNodeByPath)?._getNodeByPath('${path}');if(n)n.label=this.value">`;
            }

            html += `<div class="flex-1"></div>`;

            // Remove button (not for root)
            if (canRemove) {
                const parentPath = path.substring(0, path.lastIndexOf('.'));
                const childIdx = path.substring(path.lastIndexOf('.') + 1);
                html += `<button type="button" class="size-6 rounded-md flex items-center justify-center text-destructive hover:bg-destructive/10 transition-colors" `;
                html += `title="Remove this node" `;
                html += `onclick="Alpine.closestDataStack(this).find(d=>d.removeChildFromNode)?.removeChildFromNode('${parentPath}',${childIdx})">`;
                html += `<svg xmlns="http://www.w3.org/2000/svg" class="size-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>`;
                html += `</button>`;
            }
            html += `</div>`;

            // ── Agent leaf node fields ──
            if (node.type === 'agent') {
                html += this._renderAgentFields(node, path, isLastInLoop);
            }

            // ── Remote leaf node fields ──
            if (node.type === 'remote') {
                html += this._renderRemoteFields(node, path);
            }

            // ── Loop: max iterations ──
            if (node.type === 'loop') {
                html += this._renderLoopFields(node, path);
            }

            // ── Composite: children ──
            if (isComposite && node.children && node.children.length > 0) {
                html += `<div class="space-y-2 mt-1">`;
                for (let i = 0; i < node.children.length; i++) {
                    // Last direct child of a loop gets exit_loop indicator
                    const lastInLoop = node.type === 'loop' && i === node.children.length - 1;
                    html += this.renderCompositionNode(
                        node.children[i], `${path}.${i}`, depth + 1,
                        node.children.length > 1, lastInLoop,
                    );
                }
                html += `</div>`;
            }

            // ── Composite: add child buttons ──
            if (isComposite) {
                html += this._renderAddButtons(path);
            }

            html += `</div>`;
            return html;
        },

        // Backward-compatible aliases
        renderNode(node, path, depth, canRemove) {
            return this.renderCompositionNode(node, path, depth, canRemove, false);
        },
        renderPipelineNode(node, path, depth, canRemove) {
            return this.renderCompositionNode(node, path, depth, canRemove, false);
        },

        // ══════════════════════════════════════════════════════════════════
        // Agent leaf node fields
        // ══════════════════════════════════════════════════════════════════

        _renderAgentFields(node, path, isLastInLoop) {
            // Resolve agents from nearest Alpine parent scope
            const src = Alpine.closestDataStack(this.$el).find(d => d.agents);
            const agents = src ? (src.agents || []) : [];

            let html = `<div class="space-y-2">`;

            // Agent selector → resolves to an AgentDefinition → llmagent.New(...)
            html += `<select class="w-full h-8 rounded-md border border-border bg-background px-3 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring" `;
            html += `onchange="const n=Alpine.closestDataStack(this).find(d=>d._getNodeByPath)?._getNodeByPath('${path}');if(n)n.agent_id=this.value">`;
            html += `<option value="">Select an LLM Agent (→ resolves to llmagent.New)...</option>`;
            for (const ag of agents) {
                html += `<option value="${this._esc(ag.id)}"${ag.id === node.agent_id ? ' selected' : ''}>${this._esc(ag.name || ag.id)}</option>`;
            }
            html += `</select>`;

            // OutputKey → llmagent.Config.OutputKey
            html += `<input type="text" placeholder="OutputKey → saves output to session state (reference as {key} downstream)" maxlength="50" `;
            html += `class="w-full h-7 rounded-md border border-border bg-background px-3 text-[11px] text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring" `;
            html += `value="${this._esc(node.output_key || '')}" `;
            html += `oninput="const n=Alpine.closestDataStack(this).find(d=>d._getNodeByPath)?._getNodeByPath('${path}');if(n)n.output_key=this.value">`;

            // exit_loop indicator — ADK-Go auto-injects exit_loop tool on last child of loop
            if (isLastInLoop) {
                html += `<div class="flex items-center gap-1.5 px-2 py-1.5 rounded-md bg-purple-500/10 border border-purple-500/20">`;
                html += `<svg xmlns="http://www.w3.org/2000/svg" class="size-3.5 text-purple-600 dark:text-purple-400 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="m15 9-6 6"/><path d="m9 9 6 6"/></svg>`;
                html += `<span class="text-[10px] text-purple-600 dark:text-purple-400">ADK-Go auto-injects the <strong>exit_loop</strong> tool into this agent (last child of loop). The LLM can call it to break out early.</span>`;
                html += `</div>`;
            }

            // Workflow overrides (collapsible)
            const hasOverrides = node.disallow_transfer_to_parent || node.disallow_transfer_to_peers || node.include_contents;
            html += `<details class="group" ${hasOverrides ? 'open' : ''}>`;
            html += `<summary class="cursor-pointer text-[10px] font-medium text-muted-foreground hover:text-foreground transition-colors select-none flex items-center gap-1">`;
            html += `<svg xmlns="http://www.w3.org/2000/svg" class="size-3 transition-transform group-open:rotate-90" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>`;
            html += `Workflow Overrides (TransferPolicy &amp; IncludeContents)`;
            html += `</summary>`;
            html += `<div class="mt-1.5 space-y-1.5 pl-4 border-l border-border/50">`;

            // DisallowTransferToParent
            html += `<label class="flex items-center gap-2 cursor-pointer">`;
            html += `<input type="checkbox" class="size-3.5 rounded border-border accent-primary" ${node.disallow_transfer_to_parent ? 'checked' : ''} `;
            html += `onchange="const n=Alpine.closestDataStack(this).find(d=>d._getNodeByPath)?._getNodeByPath('${path}');if(n)n.disallow_transfer_to_parent=this.checked||null">`;
            html += `<span class="text-[10px] text-muted-foreground">DisallowTransferToParent — block escalation to parent agent</span>`;
            html += `</label>`;

            // DisallowTransferToPeers
            html += `<label class="flex items-center gap-2 cursor-pointer">`;
            html += `<input type="checkbox" class="size-3.5 rounded border-border accent-primary" ${node.disallow_transfer_to_peers ? 'checked' : ''} `;
            html += `onchange="const n=Alpine.closestDataStack(this).find(d=>d._getNodeByPath)?._getNodeByPath('${path}');if(n)n.disallow_transfer_to_peers=this.checked||null">`;
            html += `<span class="text-[10px] text-muted-foreground">DisallowTransferToPeers — block delegation to sibling agents</span>`;
            html += `</label>`;

            // IncludeContents
            html += `<div class="flex items-center gap-2">`;
            html += `<span class="text-[10px] text-muted-foreground whitespace-nowrap">IncludeContents:</span>`;
            html += `<select class="h-6 rounded border border-border bg-background px-1.5 text-[10px] text-foreground focus:outline-none" `;
            html += `onchange="const n=Alpine.closestDataStack(this).find(d=>d._getNodeByPath)?._getNodeByPath('${path}');if(n)n.include_contents=this.value">`;
            html += `<option value=""${!node.include_contents ? ' selected' : ''}>default — receives conversation history</option>`;
            html += `<option value="none"${node.include_contents === 'none' ? ' selected' : ''}>none — starts with fresh context</option>`;
            html += `</select>`;
            html += `</div>`;

            html += `</div></details>`;
            html += `</div>`;
            return html;
        },

        // ══════════════════════════════════════════════════════════════════
        // Remote leaf node fields (Type = "remote")
        // ══════════════════════════════════════════════════════════════════

        _renderRemoteFields(node, path) {
            // Resolve connectors from nearest Alpine parent scope
            const src = Alpine.closestDataStack(this.$el).find(d => d.connectors);
            const connectors = src ? (src.connectors || []) : [];

            let html = `<div class="space-y-2">`;

            // Connector selector — delegates an entire turn to a registered A2A endpoint
            html += `<select class="w-full h-8 rounded-md border border-border bg-background px-3 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring" `;
            html += `onchange="const n=Alpine.closestDataStack(this).find(d=>d._getNodeByPath)?._getNodeByPath('${path}');if(n)n.a2a_connector_id=this.value">`;
            html += `<option value="">Select an A2A Connector (→ remote agent.New via A2A protocol)...</option>`;
            for (const conn of connectors) {
                html += `<option value="${this._esc(conn.id)}"${conn.id === node.a2a_connector_id ? ' selected' : ''}>${this._esc(conn.title || conn.agent_name || conn.id)}</option>`;
            }
            html += `</select>`;

            // OutputKey — saves remote response to session state
            html += `<input type="text" placeholder="OutputKey → saves remote response to session state (reference as {key} downstream)" maxlength="50" `;
            html += `class="w-full h-7 rounded-md border border-border bg-background px-3 text-[11px] text-foreground placeholder:text-muted-foreground/60 focus:outline-none focus:ring-1 focus:ring-ring" `;
            html += `value="${this._esc(node.output_key || '')}" `;
            html += `oninput="const n=Alpine.closestDataStack(this).find(d=>d._getNodeByPath)?._getNodeByPath('${path}');if(n)n.output_key=this.value">`;

            html += `<div class="flex items-center gap-1.5 px-2 py-1.5 rounded-md bg-rose-500/10 border border-rose-500/20">`;
            html += `<svg xmlns="http://www.w3.org/2000/svg" class="size-3.5 text-rose-600 dark:text-rose-400 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M5 12h14"/><path d="m12 5 7 7-7 7"/></svg>`;
            html += `<span class="text-[10px] text-rose-600 dark:text-rose-400">Delegates the full turn to a registered A2A remote agent. The response is returned as plain text and optionally stored via OutputKey.</span>`;
            html += `</div>`;

            html += `</div>`;
            return html;
        },

        // ══════════════════════════════════════════════════════════════════
        // Loop fields
        // ══════════════════════════════════════════════════════════════════

        _renderLoopFields(node, path) {
            let html = `<div class="flex items-center gap-2">`;
            html += `<span class="text-[11px] text-muted-foreground whitespace-nowrap">MaxIterations:</span>`;
            html += `<input type="number" min="1" max="100" `;
            html += `class="w-20 h-7 rounded-md border border-border bg-background px-2 text-[11px] text-foreground focus:outline-none focus:ring-1 focus:ring-ring" `;
            html += `value="${node.max_iterations || 3}" `;
            html += `oninput="const n=Alpine.closestDataStack(this).find(d=>d._getNodeByPath)?._getNodeByPath('${path}');if(n)n.max_iterations=parseInt(this.value)||3">`;
            html += `<span class="text-[10px] text-muted-foreground">→ loopagent.Config.MaxIterations (0 = run until exit_loop)</span>`;
            html += `</div>`;
            return html;
        },

        // ══════════════════════════════════════════════════════════════════
        // Add child buttons for composite nodes
        // ══════════════════════════════════════════════════════════════════

        _renderAddButtons(path) {
            const types = [
                { type: 'agent', icon: '+●', color: 'hover:bg-emerald-500/10 hover:text-emerald-600 dark:hover:text-emerald-400', label: 'LLM Agent' },
                { type: 'sequential', icon: '+↓', color: 'hover:bg-blue-500/10 hover:text-blue-600 dark:hover:text-blue-400', label: 'Sequential' },
                { type: 'parallel', icon: '+∥', color: 'hover:bg-amber-500/10 hover:text-amber-600 dark:hover:text-amber-400', label: 'Parallel' },
                { type: 'loop', icon: '+↻', color: 'hover:bg-purple-500/10 hover:text-purple-600 dark:hover:text-purple-400', label: 'Loop' },
                { type: 'remote', icon: '+⇝', color: 'hover:bg-rose-500/10 hover:text-rose-600 dark:hover:text-rose-400', label: 'Remote A2A' },
            ];
            let html = `<div class="flex items-center gap-1.5 mt-1">`;
            for (const { type, icon, color, label } of types) {
                html += `<button type="button" class="h-6 px-2 rounded text-[10px] font-medium border border-dashed border-border text-muted-foreground ${color} transition-colors" `;
                const titleSuffix = type === 'agent' ? 'llmagent.New' : type === 'remote' ? 'agent.New via A2A' : type + 'agent.New';
                html += `title="Add ${label} child (→ ${titleSuffix})" `;
                html += `onclick="Alpine.closestDataStack(this).find(d=>d.addChildToNode)?.addChildToNode('${path}','${type}')"`;
                html += `>${icon} ${label}</button>`;
            }
            html += `</div>`;
            return html;
        },

        // ══════════════════════════════════════════════════════════════════
        // Helpers
        // ══════════════════════════════════════════════════════════════════

        _style(type) {
            const styles = {
                sequential: { border: 'border-blue-500/30', bg: 'bg-blue-500/5', badge: 'bg-blue-500/15 text-blue-600 dark:text-blue-400', label: '↓ Sequential' },
                parallel: { border: 'border-amber-500/30', bg: 'bg-amber-500/5', badge: 'bg-amber-500/15 text-amber-600 dark:text-amber-400', label: '∥ Parallel' },
                loop: { border: 'border-purple-500/30', bg: 'bg-purple-500/5', badge: 'bg-purple-500/15 text-purple-600 dark:text-purple-400', label: '↻ Loop' },
                agent: { border: 'border-emerald-500/30', bg: 'bg-emerald-500/5', badge: 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400', label: '● LLM Agent' },
                remote: { border: 'border-rose-500/30', bg: 'bg-rose-500/5', badge: 'bg-rose-500/15 text-rose-600 dark:text-rose-400', label: '⇝ Remote A2A' },
            };
            return styles[type] || styles.agent;
        },

        _esc(str) {
            return String(str).replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
        },
    };
}
