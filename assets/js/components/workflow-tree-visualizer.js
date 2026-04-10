/**
 * Workflow Tree Visualizer — Horizontal Pipeline View
 *
 * Renders the recursive composition tree as a polished left-to-right pipeline
 * using native HTML/CSS with the app's theme CSS variables.  Features:
 *
 *   • Glass-morphism node cards with theme-aware colours
 *   • Animated flow particles on horizontal connectors
 *   • Vertical fork / join visual for parallel branches
 *   • Loop-back indicator with iteration badge
 *   • Horizontal scroll – ideal for bottom-docked debugger panels
 *   • Fully reactive – updates as the user edits the tree
 */
export function workflowTreeVisualizer() {
    // ── Type metadata ───────────────────────────────────────────────
    const META = {
        sequential: { label: 'Sequential' },
        parallel: { label: 'Parallel' },
        loop: { label: 'Loop' },
        agent: { label: 'Agent' },
        conditional: { label: 'Conditional' },
    };

    function esc(s) {
        return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    }

    // ── CSS (injected once) ─────────────────────────────────────────
    function injectStyles() {
        if (document.getElementById('wf-viz-styles')) return;
        const style = document.createElement('style');
        style.id = 'wf-viz-styles';
        style.textContent = `
            /* ── Type badge ── */
            .wf-badge {
                display: inline-flex;
                align-items: center;
                gap: 4px;
                padding: 2px 8px;
                border-radius: 9999px;
                font-size: 10px;
                font-weight: 600;
                letter-spacing: 0.02em;
                line-height: 1;
                white-space: nowrap;
                background: var(--muted);
                color: var(--muted-foreground);
                border: 1px solid var(--border);
                flex-shrink: 0;
            }
            .wf-badge svg { flex-shrink: 0; }

            /* ── Node card ── */
            .wf-node {
                position: relative;
                border-radius: 0.5rem;
                border: 1px solid var(--border);
                background: var(--card);
                padding: 10px 14px;
                min-width: 140px;
                max-width: 220px;
                flex-shrink: 0;
                transition: all 0.15s ease;
            }
            .wf-node:hover {
                border-color: var(--primary);
                box-shadow: 0 4px 12px var(--shadow);
            }

        // Active, completed, and failed node CSS animations are globally provided via input.css (.node-active, etc.)

            /* ── Connector (horizontal line between nodes) ── */
            .wf-connector {
                position: relative;
                display: flex;
                align-items: center;
                justify-content: center;
                width: 28px;
                height: 2px;
                background: var(--border);
                flex-shrink: 0;
            }
            .wf-connector-flow {
                position: absolute;
                width: 6px;
                height: 2px;
                border-radius: 1px;
                background: var(--primary);
                animation: wfFlowPulse 2s ease-in-out infinite;
            }
            @keyframes wfFlowPulse {
                0%   { left: -6px; opacity: 0; }
                20%  { opacity: 1; }
                80%  { opacity: 1; }
                100% { left: 28px; opacity: 0; }
            }

            /* ── Parallel branches (stacked vertically) ── */
            .wf-branches {
                display: flex;
                flex-direction: column;
            }
            .wf-branch {
                display: flex;
                flex-direction: row;
                align-items: center;
            }
            .wf-branch-connector {
                width: 16px;
                height: 2px;
                background: var(--border);
                flex-shrink: 0;
            }

            /* Fork line (vertical line beside parallel branches) */
            .wf-fork-line {
                width: 2px;
                background: var(--border);
                align-self: stretch;
                flex-shrink: 0;
            }

            /* ── Loop wrapper ── */
            .wf-loop-wrap {
                position: relative;
                border-radius: 0.5rem;
                border: 1.5px dashed var(--border);
                padding: 12px 14px 12px 12px;
                background: oklch(from var(--primary) l c h / 5%);
                flex-shrink: 0;
            }
            .wf-loop-iteration {
                position: absolute;
                top: -10px;
                right: 12px;
                font-size: 9px;
                font-weight: 700;
                padding: 1px 6px;
                border-radius: 9999px;
                background: var(--background);
                color: var(--foreground);
                border: 1px solid var(--border);
            }

            /* ── Conditional wrapper ── */
            .wf-conditional-wrap {
                position: relative;
                border-radius: 0.5rem;
                border: 1.5px solid var(--border);
                padding: 12px 14px 12px 12px;
                background: oklch(from var(--destructive) l c h / 5%);
                flex-shrink: 0;
            }
            .wf-conditional-label {
                position: absolute;
                top: -9px;
                left: 12px;
                font-size: 9px;
                font-weight: 700;
                padding: 1px 6px;
                border-radius: 4px;
                background: var(--background);
                color: var(--destructive);
                border: 1px solid var(--border);
                text-transform: uppercase;
                letter-spacing: 0.05em;
            }

            /* ── Agent name & description ── */
            .wf-agent-name {
                font-size: 13px;
                font-weight: 600;
                color: var(--foreground);
                line-height: 1.3;
            }
            .wf-agent-desc {
                font-size: 11px;
                color: var(--muted-foreground);
                margin-top: 2px;
            }

            /* ── Empty node placeholder ── */
            .wf-empty {
                font-size: 11px;
                color: var(--muted-foreground);
                font-style: italic;
                padding: 8px 16px;
                background: var(--muted);
                border-radius: 0.5rem;
                border: 1px dashed var(--border);
                white-space: nowrap;
                flex-shrink: 0;
            }

            /* ── Tree container ── */
            .wf-tree {
                display: flex;
                flex-direction: row;
                align-items: center;
                gap: 0;
                padding: 16px;
                min-height: 60px;
            }

            /* ── Group container (boundary for each group type) ── */
            .wf-group {
                position: relative;
                display: flex;
                flex-direction: column;
                border-radius: 8px;
                padding: 16px 20px 20px 20px;
                background: var(--card);
                border: 1px solid var(--border);
                flex-shrink: 0;
            }

            /* Group label positioned inside the box (top-left) */
            .wf-group > .wf-group-label {
                position: absolute;
                top: -10px;
                left: 12px;
                display: inline-flex;
                align-items: center;
                gap: 4px;
                padding: 2px 10px;
                border-radius: 4px;
                font-size: 10px;
                font-weight: 700;
                text-transform: uppercase;
                letter-spacing: 0.05em;
                background: var(--background);
                color: var(--foreground);
                border: 1px solid var(--border);
                white-space: nowrap;
                z-index: 1;
            }

            /* Sequential group - solid border */
            .wf-group.wf-group-sequential {
                border-style: solid;
                border-color: var(--border);
            }

            /* Parallel group - double border style */
            .wf-group.wf-group-parallel {
                border-width: 2px;
                border-style: double;
                border-color: oklch(from var(--primary) l c h / 40%);
            }

            /* Loop group - dashed border */
            .wf-group.wf-group-loop {
                border-width: 1.5px;
                border-style: dashed;
                border-color: oklch(from var(--primary) l c h / 50%);
                background: oklch(from var(--primary) l c h / 3%);
            }

            /* Conditional group - dotted border */
            .wf-group.wf-group-conditional {
                border-width: 1.5px;
                border-style: dotted;
                border-color: oklch(from var(--destructive) l c h / 40%);
                background: oklch(from var(--destructive) l c h / 3%);
            }

            /* Content inside group */
            .wf-group-content {
                display: flex;
                flex-direction: row;
                align-items: center;
                gap: 8px;
            }
        `;
        document.head.appendChild(style);
    }

    // ── Node renderers ───────────────────────────────────────────────

    function renderPipelineNode(node, agentMap, depth, activeAgentNames, completedNodes, failedNodes) {
        if (!node) return '';
        const m = META[node.type] || META.agent;

        switch (node.type) {
            case 'agent': return renderAgentNode(node, agentMap, m, activeAgentNames, completedNodes, failedNodes);
            case 'sequential': return renderSequentialNode(node, agentMap, m, depth, activeAgentNames, completedNodes, failedNodes);
            case 'parallel': return renderParallelNode(node, agentMap, m, depth, activeAgentNames, completedNodes, failedNodes);
            case 'loop': return renderLoopNode(node, agentMap, m, depth, activeAgentNames, completedNodes, failedNodes);
            case 'conditional': return renderConditionalNode(node, agentMap, m, depth, activeAgentNames, completedNodes, failedNodes);
            default: return renderAgentNode(node, agentMap, m, activeAgentNames, completedNodes, failedNodes);
        }
    }

    function renderAgentNode(node, agentMap, m, activeAgentNames, completedNodes, failedNodes) {
        const name = node.agent_id ? (agentMap[node.agent_id] || 'Unknown Agent') : 'No agent selected';
        const completedInfo = completedNodes?.[name];
        const isFailed    = !!failedNodes?.[name];
        const isCompleted = !!completedInfo && !isFailed;
        const isActive    = !isCompleted && !isFailed &&
                            activeAgentNames && (activeAgentNames[name] || activeAgentNames[node.label]);
        // Dim nodes that haven't started yet when other agents are actively running
        const hasAnyActive = activeAgentNames && Object.keys(activeAgentNames).length > 0;
        const isInactive   = !isActive && !isCompleted && !isFailed && hasAnyActive;

        let nodeClass = 'wf-node';
        if (isActive)         nodeClass += ' node-active';
        else if (isCompleted) nodeClass += ' node-completed';
        else if (isFailed)    nodeClass += ' node-failed';
        else if (isInactive)  nodeClass += ' node-inactive';

        // Duration badge for completed nodes, error badge for failed
        let statusBadge = '';
        if (isCompleted && completedInfo?.durationMs) {
            const secs = (completedInfo.durationMs / 1000).toFixed(1) + 's';
            statusBadge = `<span class="node-completed-badge" style="display:inline-flex;align-items:center;gap:3px;font-size:10px;font-weight:700;padding:2px 7px;border-radius:9999px;border-width:1px;white-space:nowrap;">${secs}</span>`;
        } else if (isFailed) {
            statusBadge = `<span class="node-failed-badge" style="display:inline-flex;align-items:center;gap:3px;font-size:10px;font-weight:700;padding:2px 7px;border-radius:9999px;border-width:1px;white-space:nowrap;">error</span>`;
        }

        const outputKey = node.output_key
            ? `<span class="wf-agent-desc">\u2192 <code style="font-size:10px;padding:0 3px;border-radius:3px;background:var(--muted);font-family:var(--font-mono);">${esc(node.output_key)}</code></span>`
            : '';

        // Build orchestration badges
        let badges = '';
        if (node.disallow_transfer_to_parent) {
            badges += `<span style="font-size:9px;padding:0 4px;border-radius:4px;background:var(--muted);color:var(--foreground);font-weight:600;white-space:nowrap;" title="Transfer to parent blocked">no parent</span>`;
        }
        if (node.disallow_transfer_to_peers) {
            badges += `<span style="font-size:9px;padding:0 4px;border-radius:4px;background:var(--muted);color:var(--foreground);font-weight:600;white-space:nowrap;" title="Transfer to peers blocked">no peers</span>`;
        }
        if (node.include_contents === 'none') {
            badges += `<span style="font-size:9px;padding:0 4px;border-radius:4px;background:var(--muted);color:var(--foreground);font-weight:600;white-space:nowrap;" title="Fresh context (no history)">fresh</span>`;
        }
        const badgesHtml = badges ? `<div style="display:flex;gap:3px;flex-wrap:wrap;margin-top:2px;">${badges}</div>` : '';

        const nameFontStyle = isActive ? 'font-weight:700;' : '';

        return `
            <div class="${nodeClass}">
                <div style="display:flex;align-items:center;gap:8px;">
                    <div style="flex:1;min-width:0;">
                        <div class="wf-agent-name" style="${nameFontStyle}">${esc(name)}</div>
                        ${outputKey}
                        ${badgesHtml}
                    </div>
                    ${statusBadge}
                </div>
            </div>`;
    }

    function connector() {
        return `<div class="wf-connector"><div class="wf-connector-flow"></div></div>`;
    }

    function renderSequentialNode(node, agentMap, m, depth, activeAgentNames, completedNodes, failedNodes) {
        const kids = node.children || [];
        if (kids.length === 0) return renderEmptyNode(m, 'Empty sequence');

        let body = '';
        for (let i = 0; i < kids.length; i++) {
            body += renderPipelineNode(kids[i], agentMap, depth + 1, activeAgentNames, completedNodes, failedNodes);
            if (i < kids.length - 1) {
                body += connector();
            }
        }

        const groupLabel = depth === 0 ? '' : `<div class="wf-group-label">${esc(node.label || m.label)}</div>`;

        return `
            <div class="wf-group wf-group-sequential">
                ${groupLabel}
                <div class="wf-group-content">${body}</div>
            </div>`;
    }

    function renderParallelNode(node, agentMap, m, depth, activeAgentNames, completedNodes, failedNodes) {
        const kids = node.children || [];
        if (kids.length === 0) return renderEmptyNode(m, 'Empty parallel');

        let branches = '';
        for (const child of kids) {
            branches += `
                <div class="wf-branch">
                    <div class="wf-branch-connector"></div>
                    ${renderPipelineNode(child, agentMap, depth + 1, activeAgentNames, completedNodes, failedNodes)}
                    <div class="wf-branch-connector"></div>
                </div>`;
        }

        return `
            <div class="wf-group wf-group-parallel">
                <div class="wf-group-label">${esc(node.label || m.label)}</div>
                <div class="wf-group-content" style="display:flex;flex-direction:column;gap:12px;align-items:stretch;">
                    <div style="display:flex;flex-direction:row;align-items:stretch;">
                        <div class="wf-fork-line"></div>
                        <div class="wf-branches">${branches}</div>
                        <div class="wf-fork-line"></div>
                    </div>
                </div>
            </div>`;
    }

    function renderLoopNode(node, agentMap, m, depth, activeAgentNames, completedNodes, failedNodes) {
        const kids = node.children || [];
        const maxIter = node.max_iterations || 3;

        let body = '';
        if (kids.length === 0) {
            body = renderEmptyNode(m, 'Empty loop');
        } else {
            for (let i = 0; i < kids.length; i++) {
                body += renderPipelineNode(kids[i], agentMap, depth + 1, activeAgentNames, completedNodes, failedNodes);
                if (i < kids.length - 1) {
                    body += connector();
                }
            }
        }

        return `
            <div class="wf-group wf-group-loop">
                <div class="wf-group-label">${esc(node.label || m.label)} (${maxIter}x)</div>
                <div class="wf-group-content">${body}</div>
            </div>`;
    }

    function renderConditionalNode(node, agentMap, m, depth, activeAgentNames, completedNodes, failedNodes) {
        const kids = node.children || [];

        let evaluatorHtml = '';
        let branchesHtml = '';

        if (kids.length > 0) {
            evaluatorHtml = `
                <div style="display:flex;flex-direction:column;align-items:center;flex-shrink:0;">
                    <div style="font-size:9px;color:var(--foreground);font-weight:600;margin-bottom:4px;opacity:0.8;">Evaluator</div>
                    ${renderPipelineNode(kids[0], agentMap, depth + 1, activeAgentNames, completedNodes, failedNodes)}
                </div>`;
        }

        if (kids.length > 1) {
            let branchItems = '';
            for (let i = 1; i < kids.length; i++) {
                const branch = kids[i];
                const condLabel = branch.condition || '???';
                const isFallback = condLabel === '*';
                branchItems += `
                    <div class="wf-branch">
                        <div class="wf-branch-connector"></div>
                        <div style="display:flex;flex-direction:column;align-items:center;">
                            <div style="font-size:9px;color:${isFallback ? 'var(--muted-foreground)' : 'var(--foreground)'};font-weight:600;margin-bottom:3px;">
                                ${isFallback ? 'fallback' : esc(condLabel)}
                            </div>
                            ${renderPipelineNode(branch, agentMap, depth + 1, activeAgentNames, completedNodes, failedNodes)}
                        </div>
                        <div class="wf-branch-connector"></div>
                    </div>`;
            }
            branchesHtml = `
                <div class="wf-fork-line"></div>
                <div class="wf-branches">${branchItems}</div>
                <div class="wf-fork-line"></div>`;
        }

        if (kids.length === 0) {
            return renderEmptyNode(m, 'Empty conditional');
        }

        return `
            <div class="wf-group wf-group-conditional">
                <div class="wf-group-label">${esc(node.label || m.label)}</div>
                <div class="wf-group-content" style="display:flex;flex-direction:row;align-items:stretch;">
                    ${evaluatorHtml}
                    ${evaluatorHtml && branchesHtml ? connector() : ''}
                    ${branchesHtml}
                </div>
            </div>`;
    }

    function renderEmptyNode(m, msg) {
        return `
            <div class="wf-node" style="opacity:0.5;max-width:200px;">
                <div style="font-size:10px;color:var(--muted-foreground);font-style:italic;text-align:center;padding:2px 0;">${msg}</div>
            </div>`;
    }

    // ── Alpine component ────────────────────────────────────────────
    return {
        init() {
            injectStyles();
        },

        renderTree(graph, agentsList, activeAgentNames, completedNodes, failedNodes) {
            if (!graph) {
                return '<div class="wf-empty">Add nodes to see the pipeline preview</div>';
            }

            const agentMap = {};
            if (agentsList) {
                for (const a of agentsList) {
                    agentMap[a.id] = a.name || a.id;
                }
            }

            return `
                <div class="wf-viz">
                    <div class="wf-pipeline-canvas" style="padding:14px 20px;overflow-x:auto;">
                        <div style="display:flex;flex-direction:row;align-items:center;justify-content:center;">
                            ${renderPipelineNode(graph, agentMap, 0, activeAgentNames, completedNodes, failedNodes)}
                        </div>
                    </div>
                </div>`;
        },
    };
}
