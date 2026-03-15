/**
 * Workflow Tree Builder
 *
 * An Alpine.js component that renders a recursive composition tree for
 * multi-agent workflows. Each node can be:
 *   - agent: a leaf node referencing an existing AgentDefinition
 *   - sequential: runs children one after another
 *   - parallel: runs all children concurrently
 *   - loop: repeats children until max iterations or exit
 *
 * Nodes can be arbitrarily nested, e.g.:
 *   sequential → [agent, parallel → [agent, loop → [agent, agent]]]
 */
export function workflowTreeBuilder() {
    return {
        init() {
            // The parent agentsManager owns the data; we just render it.
        },

        /**
         * Recursively render a node and its children as HTML.
         * Uses onclick handlers that call methods on the parent agentsManager.
         */
        renderNode(node, path, depth, canRemove) {
            if (!node) return '';

            const isComposite = ['sequential', 'parallel', 'loop'].includes(node.type);
            const indent = depth > 0 ? 'ml-4 ' : '';
            const borderColor = {
                sequential: 'border-blue-500/30',
                parallel: 'border-amber-500/30',
                loop: 'border-purple-500/30',
                agent: 'border-emerald-500/30',
            }[node.type] || 'border-border';

            const bgColor = {
                sequential: 'bg-blue-500/5',
                parallel: 'bg-amber-500/5',
                loop: 'bg-purple-500/5',
                agent: 'bg-emerald-500/5',
            }[node.type] || 'bg-muted/20';

            const badgeClass = {
                sequential: 'bg-blue-500/15 text-blue-600 dark:text-blue-400',
                parallel: 'bg-amber-500/15 text-amber-600 dark:text-amber-400',
                loop: 'bg-purple-500/15 text-purple-600 dark:text-purple-400',
                agent: 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400',
            }[node.type] || '';

            const typeLabel = {
                sequential: '↓ Sequential',
                parallel: '∥ Parallel',
                loop: '↻ Loop',
                agent: '● Agent',
            }[node.type] || node.type;

            let html = `<div class="${indent}rounded-lg border ${borderColor} ${bgColor} p-3 space-y-2">`;

            // ── Header: type badge + type selector + remove button ──
            html += `<div class="flex items-center gap-2">`;
            html += `<span class="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-bold select-none ${badgeClass}">${typeLabel}</span>`;

            // Type selector
            html += `<select class="h-7 rounded-md border border-border bg-background px-2 text-[11px] text-foreground focus:outline-none focus:ring-1 focus:ring-ring" `;
            html += `onchange="`;
            html += `  const mgr = Alpine.closestDataStack(this).find(d => d.changeNodeType);`;
            html += `  if (mgr) mgr.changeNodeType('${path}', this.value);`;
            html += `">`;
            for (const t of ['agent', 'sequential', 'parallel', 'loop']) {
                const selected = t === node.type ? ' selected' : '';
                const label = { agent: 'Agent', sequential: 'Sequential', parallel: 'Parallel', loop: 'Loop' }[t];
                html += `<option value="${t}"${selected}>${label}</option>`;
            }
            html += `</select>`;

            // Spacer
            html += `<div class="flex-1"></div>`;

            // Remove button (not for root)
            if (canRemove) {
                const parentPath = path.substring(0, path.lastIndexOf('.'));
                const childIdx = path.substring(path.lastIndexOf('.') + 1);
                html += `<button type="button" class="size-6 rounded-md flex items-center justify-center text-destructive hover:bg-destructive/10 transition-colors" `;
                html += `onclick="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d.removeChildFromNode);`;
                html += `  if (mgr) mgr.removeChildFromNode('${parentPath}', ${childIdx});`;
                html += `">`;
                html += `<svg xmlns="http://www.w3.org/2000/svg" class="size-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>`;
                html += `</button>`;
            }

            html += `</div>`;

            // ── Agent-specific: agent selector + output key + orchestration overrides ──
            if (node.type === 'agent') {
                html += `<div class="space-y-2">`;
                // Agent dropdown — we read agents from the parent scope
                html += `<select class="w-full h-8 rounded-md border border-border bg-background px-3 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring" `;
                html += `onchange="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d._getNodeByPath);`;
                html += `  if (mgr) { const n = mgr._getNodeByPath('${path}'); if (n) n.agent_id = this.value; }`;
                html += `">`;
                html += `<option value="">Select an agent...</option>`;
                // Populate from parent agentsManager's agents array
                const mgr = Alpine.closestDataStack(this.$el).find(d => d.agents);
                const agentsList = mgr ? (mgr.agents || []) : [];
                for (const ag of agentsList) {
                    const selected = ag.id === node.agent_id ? ' selected' : '';
                    const name = this._escapeAttr(ag.name || ag.id);
                    html += `<option value="${this._escapeAttr(ag.id)}"${selected}>${name}</option>`;
                }
                html += `</select>`;

                // Output key
                html += `<input type="text" placeholder="Output key (optional, for state sharing)" maxlength="50" class="w-full h-7 rounded-md border border-border bg-background px-3 text-[11px] text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring" `;
                html += `value="${this._escapeAttr(node.output_key || '')}" `;
                html += `oninput="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d._getNodeByPath);`;
                html += `  if (mgr) { const n = mgr._getNodeByPath('${path}'); if (n) n.output_key = this.value; }`;
                html += `">`;

                // ── Orchestration overrides (collapsible) ──
                const hasOverrides = node.disallow_transfer_to_parent || node.disallow_transfer_to_peers || node.include_contents;
                html += `<details class="group" ${hasOverrides ? 'open' : ''}>`;
                html += `<summary class="cursor-pointer text-[10px] font-medium text-muted-foreground hover:text-foreground transition-colors select-none flex items-center gap-1">`;
                html += `<svg xmlns="http://www.w3.org/2000/svg" class="size-3 transition-transform group-open:rotate-90" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>`;
                html += `Transfer &amp; Context Overrides`;
                html += `</summary>`;
                html += `<div class="mt-1.5 space-y-1.5 pl-4 border-l border-border/50">`;

                // Transfer to parent toggle
                const parentChecked = node.disallow_transfer_to_parent ? ' checked' : '';
                html += `<label class="flex items-center gap-2 cursor-pointer">`;
                html += `<input type="checkbox" class="size-3.5 rounded border-border accent-primary" ${parentChecked} `;
                html += `onchange="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d._getNodeByPath);`;
                html += `  if (mgr) { const n = mgr._getNodeByPath('${path}'); if (n) n.disallow_transfer_to_parent = this.checked || null; }`;
                html += `">`;
                html += `<span class="text-[10px] text-muted-foreground">Block transfer to parent</span>`;
                html += `</label>`;

                // Transfer to peers toggle
                const peersChecked = node.disallow_transfer_to_peers ? ' checked' : '';
                html += `<label class="flex items-center gap-2 cursor-pointer">`;
                html += `<input type="checkbox" class="size-3.5 rounded border-border accent-primary" ${peersChecked} `;
                html += `onchange="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d._getNodeByPath);`;
                html += `  if (mgr) { const n = mgr._getNodeByPath('${path}'); if (n) n.disallow_transfer_to_peers = this.checked || null; }`;
                html += `">`;
                html += `<span class="text-[10px] text-muted-foreground">Block transfer to peers</span>`;
                html += `</label>`;

                // Include contents selector
                html += `<div class="flex items-center gap-2">`;
                html += `<span class="text-[10px] text-muted-foreground whitespace-nowrap">History:</span>`;
                html += `<select class="h-6 rounded border border-border bg-background px-1.5 text-[10px] text-foreground focus:outline-none" `;
                html += `onchange="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d._getNodeByPath);`;
                html += `  if (mgr) { const n = mgr._getNodeByPath('${path}'); if (n) n.include_contents = this.value; }`;
                html += `">`;
                html += `<option value=""${!node.include_contents ? ' selected' : ''}>Default (include)</option>`;
                html += `<option value="none"${node.include_contents === 'none' ? ' selected' : ''}>Fresh context</option>`;
                html += `</select>`;
                html += `</div>`;

                html += `</div></details>`;
                html += `</div>`;
            }

            // ── Loop-specific: max iterations ──
            if (node.type === 'loop') {
                html += `<div class="flex items-center gap-2">`;
                html += `<span class="text-[11px] text-muted-foreground">Max iterations:</span>`;
                html += `<input type="number" min="1" max="100" class="w-20 h-7 rounded-md border border-border bg-background px-2 text-[11px] text-foreground focus:outline-none focus:ring-1 focus:ring-ring" `;
                html += `value="${node.max_iterations || 3}" `;
                html += `oninput="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d._getNodeByPath);`;
                html += `  if (mgr) { const n = mgr._getNodeByPath('${path}'); if (n) n.max_iterations = parseInt(this.value) || 3; }`;
                html += `">`;
                html += `</div>`;
            }

            // ── Composite: render children + add child buttons ──
            if (isComposite) {
                if (node.children && node.children.length > 0) {
                    html += `<div class="space-y-2 mt-1">`;
                    for (let i = 0; i < node.children.length; i++) {
                        html += this.renderNode(node.children[i], `${path}.${i}`, depth + 1, node.children.length > 1);
                    }
                    html += `</div>`;
                }

                // Add child buttons
                html += `<div class="flex items-center gap-1.5 mt-1">`;
                for (const childType of ['agent', 'sequential', 'parallel', 'loop']) {
                    const icon = { agent: '+●', sequential: '+↓', parallel: '+∥', loop: '+↻' }[childType];
                    const btnColor = {
                        agent: 'hover:bg-emerald-500/10 hover:text-emerald-600',
                        sequential: 'hover:bg-blue-500/10 hover:text-blue-600',
                        parallel: 'hover:bg-amber-500/10 hover:text-amber-600',
                        loop: 'hover:bg-purple-500/10 hover:text-purple-600',
                    }[childType];
                    html += `<button type="button" class="h-6 px-2 rounded text-[10px] font-medium border border-dashed border-border text-muted-foreground ${btnColor} transition-colors" `;
                    html += `onclick="`;
                    html += `  const mgr = Alpine.closestDataStack(this).find(d => d.addChildToNode);`;
                    html += `  if (mgr) mgr.addChildToNode('${path}', '${childType}');`;
                    html += `">${icon} ${childType[0].toUpperCase() + childType.slice(1)}</button>`;
                }
                html += `</div>`;
            }

            html += `</div>`;
            return html;
        },

        /**
         * Render a pipeline node for the Agent Designer dialog.
         * Same rendering logic as renderNode, but reads agents from the
         * agentDesigner component and calls its methods for mutations.
         */
        renderPipelineNode(node, path, depth, canRemove) {
            if (!node) return '';

            const isComposite = ['sequential', 'parallel', 'loop'].includes(node.type);
            const indent = depth > 0 ? 'ml-4 ' : '';
            const borderColor = {
                sequential: 'border-blue-500/30',
                parallel: 'border-amber-500/30',
                loop: 'border-purple-500/30',
                agent: 'border-emerald-500/30',
            }[node.type] || 'border-border';

            const bgColor = {
                sequential: 'bg-blue-500/5',
                parallel: 'bg-amber-500/5',
                loop: 'bg-purple-500/5',
                agent: 'bg-emerald-500/5',
            }[node.type] || 'bg-muted/20';

            const badgeClass = {
                sequential: 'bg-blue-500/15 text-blue-600 dark:text-blue-400',
                parallel: 'bg-amber-500/15 text-amber-600 dark:text-amber-400',
                loop: 'bg-purple-500/15 text-purple-600 dark:text-purple-400',
                agent: 'bg-emerald-500/15 text-emerald-600 dark:text-emerald-400',
            }[node.type] || '';

            const typeLabel = {
                sequential: '↓ Sequential',
                parallel: '∥ Parallel',
                loop: '↻ Loop',
                agent: '● Agent',
            }[node.type] || node.type;

            let html = `<div class="${indent}rounded-lg border ${borderColor} ${bgColor} p-3 space-y-2">`;

            // Header
            html += `<div class="flex items-center gap-2">`;
            html += `<span class="inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-bold select-none ${badgeClass}">${typeLabel}</span>`;

            // Type selector
            html += `<select class="h-7 rounded-md border border-border bg-background px-2 text-[11px] text-foreground focus:outline-none focus:ring-1 focus:ring-ring" `;
            html += `onchange="`;
            html += `  const mgr = Alpine.closestDataStack(this).find(d => d.changeNodeType);`;
            html += `  if (mgr) mgr.changeNodeType('${path}', this.value);`;
            html += `">`;
            for (const t of ['agent', 'sequential', 'parallel', 'loop']) {
                const selected = t === node.type ? ' selected' : '';
                const label = { agent: 'Agent', sequential: 'Sequential', parallel: 'Parallel', loop: 'Loop' }[t];
                html += `<option value="${t}"${selected}>${label}</option>`;
            }
            html += `</select>`;

            html += `<div class="flex-1"></div>`;

            // Remove button (not for root)
            if (canRemove) {
                const parentPath = path.substring(0, path.lastIndexOf('.'));
                const childIdx = path.substring(path.lastIndexOf('.') + 1);
                html += `<button type="button" class="size-6 rounded-md flex items-center justify-center text-destructive hover:bg-destructive/10 transition-colors" `;
                html += `onclick="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d.removeChildFromNode);`;
                html += `  if (mgr) mgr.removeChildFromNode('${parentPath}', ${childIdx});`;
                html += `">`;
                html += `<svg xmlns="http://www.w3.org/2000/svg" class="size-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6"/><path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2"/></svg>`;
                html += `</button>`;
            }

            html += `</div>`;

            // Agent-specific
            if (node.type === 'agent') {
                html += `<div class="space-y-2">`;
                html += `<select class="w-full h-8 rounded-md border border-border bg-background px-3 text-xs text-foreground focus:outline-none focus:ring-1 focus:ring-ring" `;
                html += `onchange="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d._getNodeByPath);`;
                html += `  if (mgr) { const n = mgr._getNodeByPath('${path}'); if (n) n.agent_id = this.value; }`;
                html += `">`;
                html += `<option value="">Select an agent node...</option>`;
                // Read agents from the agentDesigner component (simple agents only)
                const designer = Alpine.closestDataStack(this.$el).find(d => d.agents && d.pipelineGraph !== undefined);
                const agentsList = designer ? (designer.agents || []) : [];
                for (const ag of agentsList) {
                    const selected = ag.id === node.agent_id ? ' selected' : '';
                    const name = this._escapeAttr(ag.name || ag.id);
                    html += `<option value="${this._escapeAttr(ag.id)}"${selected}>${name}</option>`;
                }
                html += `</select>`;

                // Output key
                html += `<input type="text" placeholder="Output key (maps to ADK-Go OutputKey)" maxlength="50" class="w-full h-7 rounded-md border border-border bg-background px-3 text-[11px] text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring" `;
                html += `value="${this._escapeAttr(node.output_key || '')}" `;
                html += `oninput="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d._getNodeByPath);`;
                html += `  if (mgr) { const n = mgr._getNodeByPath('${path}'); if (n) n.output_key = this.value; }`;
                html += `">`;

                // Orchestration overrides
                const hasOverrides = node.disallow_transfer_to_parent || node.disallow_transfer_to_peers || node.include_contents;
                html += `<details class="group" ${hasOverrides ? 'open' : ''}>`;
                html += `<summary class="cursor-pointer text-[10px] font-medium text-muted-foreground hover:text-foreground transition-colors select-none flex items-center gap-1">`;
                html += `<svg xmlns="http://www.w3.org/2000/svg" class="size-3 transition-transform group-open:rotate-90" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="m9 18 6-6-6-6"/></svg>`;
                html += `Transfer &amp; Context Overrides`;
                html += `</summary>`;
                html += `<div class="mt-1.5 space-y-1.5 pl-4 border-l border-border/50">`;

                const parentChecked = node.disallow_transfer_to_parent ? ' checked' : '';
                html += `<label class="flex items-center gap-2 cursor-pointer">`;
                html += `<input type="checkbox" class="size-3.5 rounded border-border accent-primary" ${parentChecked} `;
                html += `onchange="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d._getNodeByPath);`;
                html += `  if (mgr) { const n = mgr._getNodeByPath('${path}'); if (n) n.disallow_transfer_to_parent = this.checked || null; }`;
                html += `">`;
                html += `<span class="text-[10px] text-muted-foreground">Block transfer to parent</span>`;
                html += `</label>`;

                const peersChecked = node.disallow_transfer_to_peers ? ' checked' : '';
                html += `<label class="flex items-center gap-2 cursor-pointer">`;
                html += `<input type="checkbox" class="size-3.5 rounded border-border accent-primary" ${peersChecked} `;
                html += `onchange="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d._getNodeByPath);`;
                html += `  if (mgr) { const n = mgr._getNodeByPath('${path}'); if (n) n.disallow_transfer_to_peers = this.checked || null; }`;
                html += `">`;
                html += `<span class="text-[10px] text-muted-foreground">Block transfer to peers</span>`;
                html += `</label>`;

                html += `<div class="flex items-center gap-2">`;
                html += `<span class="text-[10px] text-muted-foreground whitespace-nowrap">History:</span>`;
                html += `<select class="h-6 rounded border border-border bg-background px-1.5 text-[10px] text-foreground focus:outline-none" `;
                html += `onchange="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d._getNodeByPath);`;
                html += `  if (mgr) { const n = mgr._getNodeByPath('${path}'); if (n) n.include_contents = this.value; }`;
                html += `">`;
                html += `<option value=""${!node.include_contents ? ' selected' : ''}>Default (include)</option>`;
                html += `<option value="none"${node.include_contents === 'none' ? ' selected' : ''}>Fresh context</option>`;
                html += `</select>`;
                html += `</div>`;

                html += `</div></details>`;
                html += `</div>`;
            }

            // Loop-specific
            if (node.type === 'loop') {
                html += `<div class="flex items-center gap-2">`;
                html += `<span class="text-[11px] text-muted-foreground">Max iterations:</span>`;
                html += `<input type="number" min="1" max="100" class="w-20 h-7 rounded-md border border-border bg-background px-2 text-[11px] text-foreground focus:outline-none focus:ring-1 focus:ring-ring" `;
                html += `value="${node.max_iterations || 3}" `;
                html += `oninput="`;
                html += `  const mgr = Alpine.closestDataStack(this).find(d => d._getNodeByPath);`;
                html += `  if (mgr) { const n = mgr._getNodeByPath('${path}'); if (n) n.max_iterations = parseInt(this.value) || 3; }`;
                html += `">`;
                html += `</div>`;
            }

            // Composite: children + add buttons
            if (isComposite) {
                if (node.children && node.children.length > 0) {
                    html += `<div class="space-y-2 mt-1">`;
                    for (let i = 0; i < node.children.length; i++) {
                        html += this.renderPipelineNode(node.children[i], `${path}.${i}`, depth + 1, node.children.length > 1);
                    }
                    html += `</div>`;
                }

                html += `<div class="flex items-center gap-1.5 mt-1">`;
                for (const childType of ['agent', 'sequential', 'parallel', 'loop']) {
                    const icon = { agent: '+●', sequential: '+↓', parallel: '+∥', loop: '+↻' }[childType];
                    const btnColor = {
                        agent: 'hover:bg-emerald-500/10 hover:text-emerald-600',
                        sequential: 'hover:bg-blue-500/10 hover:text-blue-600',
                        parallel: 'hover:bg-amber-500/10 hover:text-amber-600',
                        loop: 'hover:bg-purple-500/10 hover:text-purple-600',
                    }[childType];
                    html += `<button type="button" class="h-6 px-2 rounded text-[10px] font-medium border border-dashed border-border text-muted-foreground ${btnColor} transition-colors" `;
                    html += `onclick="`;
                    html += `  const mgr = Alpine.closestDataStack(this).find(d => d.addChildToNode);`;
                    html += `  if (mgr) mgr.addChildToNode('${path}', '${childType}');`;
                    html += `">${icon} ${childType[0].toUpperCase() + childType.slice(1)}</button>`;
                }
                html += `</div>`;
            }

            html += `</div>`;
            return html;
        },

        _escapeAttr(str) {
            return str.replace(/"/g, '&quot;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
        },
    };
}
