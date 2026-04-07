/**
 * Workflow Tree Visualizer — Premium Pipeline View
 *
 * Renders the recursive composition tree as a polished top-down pipeline
 * using native HTML/CSS with the app's theme CSS variables.  Features:
 *
 *   • Glass-morphism node cards with theme-aware colours
 *   • Animated flow particles on connectors
 *   • Fork / join visual for parallel branches
 *   • Loop-back indicator with iteration badge
 *   • Responsive – works inside the dialog scroll area
 *   • Fully reactive – updates as the user edits the tree
 */
export function workflowTreeVisualizer() {
    // ── Type metadata ───────────────────────────────────────────────
    const META = {
        sequential: {
            icon: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14"/><path d="m19 12-7 7-7-7"/></svg>`,
            label: 'Sequential',
            accent: 'var(--wf-blue)',
            accentMuted: 'var(--wf-blue-muted)',
            accentFg: 'var(--wf-blue-fg)',
        },
        parallel: {
            icon: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 3v18"/><path d="M16 3v18"/></svg>`,
            label: 'Parallel',
            accent: 'var(--wf-amber)',
            accentMuted: 'var(--wf-amber-muted)',
            accentFg: 'var(--wf-amber-fg)',
        },
        loop: {
            icon: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/></svg>`,
            label: 'Loop',
            accent: 'var(--wf-purple)',
            accentMuted: 'var(--wf-purple-muted)',
            accentFg: 'var(--wf-purple-fg)',
        },
        agent: {
            icon: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></svg>`,
            label: 'Agent',
            accent: 'var(--wf-emerald)',
            accentMuted: 'var(--wf-emerald-muted)',
            accentFg: 'var(--wf-emerald-fg)',
        },
        conditional: {
            icon: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M16 3h5v5"/><path d="M8 3H3v5"/><path d="M12 22v-8.3a4 4 0 0 0-1.172-2.872L3 3"/><path d="m15 9 6-6"/></svg>`,
            label: 'Conditional',
            accent: 'var(--wf-red, oklch(0.62 0.2 25))',
            accentMuted: 'var(--wf-red-muted, oklch(0.62 0.2 25 / 12%))',
            accentFg: 'var(--wf-red-fg, oklch(0.55 0.22 25))',
        },
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
            /* ── Workflow visualizer CSS variables ── */
            .wf-viz {
                --wf-blue: oklch(0.62 0.16 250);
                --wf-blue-muted: oklch(0.62 0.16 250 / 12%);
                --wf-blue-fg: oklch(0.55 0.18 250);
                --wf-amber: oklch(0.75 0.16 75);
                --wf-amber-muted: oklch(0.75 0.16 75 / 12%);
                --wf-amber-fg: oklch(0.65 0.18 75);
                --wf-purple: oklch(0.62 0.2 300);
                --wf-purple-muted: oklch(0.62 0.2 300 / 12%);
                --wf-purple-fg: oklch(0.55 0.22 300);
                --wf-emerald: oklch(0.7 0.17 165);
                --wf-emerald-muted: oklch(0.7 0.17 165 / 12%);
                --wf-emerald-fg: oklch(0.6 0.19 165);
                --wf-red: oklch(0.62 0.2 25);
                --wf-red-muted: oklch(0.62 0.2 25 / 12%);
                --wf-red-fg: oklch(0.55 0.22 25);
                /* Done / success green */
                --wf-done:        oklch(0.52 0.17 160);
                --wf-done-muted:  oklch(0.52 0.17 160 / 14%);
                --wf-done-fg:     oklch(0.48 0.19 160);
                /* Running blue */
                --wf-live:        oklch(0.55 0.20 260);
                --wf-live-muted:  oklch(0.55 0.20 260 / 14%);
            }
            .dark .wf-viz {
                --wf-blue: oklch(0.7 0.14 250);
                --wf-blue-muted: oklch(0.7 0.14 250 / 14%);
                --wf-blue-fg: oklch(0.78 0.12 250);
                --wf-amber: oklch(0.8 0.14 80);
                --wf-amber-muted: oklch(0.8 0.14 80 / 14%);
                --wf-amber-fg: oklch(0.85 0.12 80);
                --wf-purple: oklch(0.72 0.16 300);
                --wf-purple-muted: oklch(0.72 0.16 300 / 14%);
                --wf-purple-fg: oklch(0.8 0.14 300);
                --wf-emerald: oklch(0.78 0.14 165);
                --wf-emerald-muted: oklch(0.78 0.14 165 / 14%);
                --wf-emerald-fg: oklch(0.84 0.12 165);
                --wf-red: oklch(0.68 0.2 25);
                --wf-red-muted: oklch(0.68 0.2 25 / 14%);
                --wf-red-fg: oklch(0.78 0.15 25);
                --wf-done:        oklch(0.72 0.15 160);
                --wf-done-muted:  oklch(0.72 0.15 160 / 16%);
                --wf-done-fg:     oklch(0.78 0.13 160);
                --wf-live:        oklch(0.72 0.17 260);
                --wf-live-muted:  oklch(0.72 0.17 260 / 16%);
            }

            /* ── Pipeline connector ── */
            .wf-connector {
                position: relative;
                display: flex;
                align-items: center;
                justify-content: center;
                height: 28px;
                width: 100%;
            }
            .wf-connector::before {
                content: '';
                position: absolute;
                left: 50%;
                top: 0;
                bottom: 0;
                width: 2px;
                background: linear-gradient(to bottom, var(--connector-color, var(--border)), oklch(from var(--connector-color, var(--border)) l c h / 40%));
                transform: translateX(-50%);
            }
            .wf-connector-dot {
                position: relative;
                z-index: 1;
                width: 7px;
                height: 7px;
                border-radius: 50%;
                background: var(--connector-color, var(--border));
                box-shadow: 0 0 0 3px var(--background), 0 0 0 4px oklch(from var(--connector-color, var(--border)) l c h / 40%);
            }

            /* ── Animated flow pulse ── */
            .wf-connector-flow {
                position: absolute;
                left: 50%;
                top: 0;
                width: 4px;
                height: 4px;
                margin-left: -2px;
                border-radius: 50%;
                background: var(--connector-color, var(--border));
                opacity: 0;
                animation: wfFlowPulse 2.4s ease-in-out infinite;
            }
            @keyframes wfFlowPulse {
                0%   { top: -2px; opacity: 0; transform: scale(0.6); }
                15%  { opacity: 0.9; transform: scale(1); }
                85%  { opacity: 0.9; transform: scale(1); }
                100% { top: calc(100% + 2px); opacity: 0; transform: scale(0.6); }
            }

            /* ── Node card ── */
            .wf-node {
                position: relative;
                border-radius: var(--radius, 0.75rem);
                border: 1px solid var(--node-border, var(--border));
                background: var(--node-bg, var(--card));
                padding: 10px 14px;
                transition: box-shadow 0.2s ease, border-color 0.2s ease, transform 0.15s ease;
                width: 100%;
                min-width: 200px;
            }
            .wf-node:hover {
                border-color: oklch(from var(--node-accent, var(--border)) l c h / 50%);
                box-shadow: 0 0 0 1px oklch(from var(--node-accent, var(--border)) l c h / 20%),
                            0 2px 8px -2px oklch(from var(--node-accent, var(--border)) l c h / 12%),
                            0 8px 24px -4px oklch(0 0 0 / 6%);
                transform: translateY(-1px);
            }
            .dark .wf-node:hover {
                box-shadow: 0 0 0 1px oklch(from var(--node-accent, var(--border)) l c h / 25%),
                            0 2px 8px -2px oklch(from var(--node-accent, var(--border)) l c h / 15%),
                            0 8px 24px -4px oklch(0 0 0 / 25%);
            }

            /* ── Active Node Highlight ── */
            .wf-node.is-active {
                --node-accent: var(--wf-live);
                --node-border: var(--wf-live-muted);
                --node-bg: var(--wf-live-muted);
                border-color: var(--wf-live);
                box-shadow: 0 0 0 2px var(--wf-live), 0 0 20px oklch(from var(--wf-live) l c h / 35%);
                transform: translateY(-2px);
                animation: wfPulseActive 2.5s ease-in-out infinite;
            }
            @keyframes wfPulseActive {
                0%   { box-shadow: 0 0 0 2px var(--wf-live), 0 0 12px oklch(from var(--wf-live) l c h / 20%); }
                50%  { box-shadow: 0 0 0 2px var(--wf-live), 0 0 28px oklch(from var(--wf-live) l c h / 55%); }
                100% { box-shadow: 0 0 0 2px var(--wf-live), 0 0 12px oklch(from var(--wf-live) l c h / 20%); }
            }
            /* Spinning indicator inside icon badge when active */
            .wf-node.is-active .wf-badge {
                position: relative;
                overflow: hidden;
            }
            .wf-node.is-active .wf-badge::after {
                content: '';
                position: absolute;
                inset: -1px;
                border-radius: inherit;
                border: 2px solid transparent;
                border-top-color: var(--wf-live);
                animation: wfSpinBadge 1s linear infinite;
            }
            @keyframes wfSpinBadge {
                to { transform: rotate(360deg); }
            }
            /* Shimmer sweep on active node */
            .wf-node.is-active::after {
                content: '';
                position: absolute;
                inset: 0;
                border-radius: inherit;
                background: linear-gradient(105deg, transparent 30%, oklch(from var(--wf-live) l c h / 10%) 50%, transparent 70%);
                background-size: 200% 100%;
                animation: wfActiveShimmer 2s ease-in-out infinite;
                pointer-events: none;
            }
            @keyframes wfActiveShimmer {
                0%   { background-position: 200% 0; }
                100% { background-position: -200% 0; }
            }

            /* ── Completed Node — green success state ── */
            .wf-node.is-completed {
                --node-accent: var(--wf-done);
                --node-border: var(--wf-done-muted);
                --node-bg: var(--wf-done-muted);
                border-color: var(--wf-done-muted);
            }
            .wf-node.is-completed::before {
                background: var(--wf-done);
                opacity: 1;
            }
            .wf-node-done-badge {
                display: inline-flex;
                align-items: center;
                gap: 3px;
                font-size: 10px;
                font-weight: 700;
                padding: 2px 7px;
                border-radius: 999px;
                background: var(--wf-done-muted);
                color: var(--wf-done-fg, var(--wf-done));
                border: 1px solid oklch(from var(--wf-done) l c h / 30%);
                flex-shrink: 0;
                white-space: nowrap;
            }

            /* ── Failed Node ── */
            .wf-node.is-failed {
                --node-accent: var(--destructive);
                --node-border: oklch(from var(--destructive) l c h / 30%);
                --node-bg: oklch(from var(--destructive) l c h / 10%);
                border-color: oklch(from var(--destructive) l c h / 45%);
            }
            .wf-node.is-failed::before {
                background: var(--destructive);
                opacity: 1;
            }
            .wf-node-failed-badge {
                display: inline-flex;
                align-items: center;
                gap: 3px;
                font-size: 10px;
                font-weight: 700;
                padding: 2px 7px;
                border-radius: 999px;
                background: oklch(from var(--destructive) l c h / 12%);
                color: var(--destructive);
                border: 1px solid oklch(from var(--destructive) l c h / 30%);
                flex-shrink: 0;
                white-space: nowrap;
            }

            /* ── Accent stripe on the left edge ── */
            .wf-node::before {
                content: '';
                position: absolute;
                left: 0;
                top: 8px;
                bottom: 8px;
                width: 3px;
                border-radius: 0 3px 3px 0;
                background: var(--node-accent, var(--primary));
                opacity: 0.8;
                transition: opacity 0.2s;
            }
            .wf-node:hover::before { opacity: 1; }

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
                background: var(--badge-bg, var(--muted));
                color: var(--badge-fg, var(--muted-foreground));
                border: 1px solid oklch(from var(--badge-fg, var(--muted-foreground)) l c h / 15%);
            }
            .wf-badge svg { flex-shrink: 0; }

            /* ── Parallel fork/join ── */
            .wf-fork-join {
                position: relative;
                height: 20px;
                width: 100%;
            }
            .wf-fork-join::before {
                content: '';
                position: absolute;
                left: 8%;
                right: 8%;
                top: 50%;
                height: 2px;
                background: var(--connector-color, var(--border));
                transform: translateY(-50%);
                border-radius: 1px;
            }
            .wf-fork-join-center {
                position: absolute;
                left: 50%;
                top: 50%;
                transform: translate(-50%, -50%);
                width: 2px;
                height: 100%;
                background: var(--connector-color, var(--border));
            }

            /* ── Loop wrapper ── */
            .wf-loop-wrap {
                position: relative;
                border-radius: var(--radius, 0.75rem);
                border: 1.5px dashed oklch(from var(--node-accent, var(--border)) l c h / 40%);
                padding: 12px 10px 16px;
                background: oklch(from var(--node-accent, var(--border)) l c h / 4%);
                width: 100%;
                max-width: 300px;
                transition: border-color 0.2s, background 0.2s;
            }
            .wf-loop-wrap:hover {
                border-color: oklch(from var(--node-accent, var(--border)) l c h / 60%);
                background: oklch(from var(--node-accent, var(--border)) l c h / 6%);
            }
            .wf-loop-badge {
                position: absolute;
                top: -9px;
                right: 12px;
                display: inline-flex;
                align-items: center;
                gap: 3px;
                padding: 1px 7px 1px 5px;
                border-radius: 9999px;
                font-size: 9px;
                font-weight: 700;
                letter-spacing: 0.03em;
                background: var(--background);
                color: var(--badge-fg, var(--muted-foreground));
                border: 1.5px dashed oklch(from var(--node-accent, var(--border)) l c h / 40%);
            }
            .wf-loop-badge svg { width: 10px; height: 10px; }
            .wf-loop-return {
                display: flex;
                align-items: center;
                justify-content: center;
                margin-top: 6px;
            }

            /* ── Agent name ── */
            .wf-agent-name {
                font-size: 12px;
                font-weight: 600;
                color: var(--foreground);
                line-height: 1.3;
                max-width: 180px;
                overflow: hidden;
                text-overflow: ellipsis;
                white-space: nowrap;
            }
            .wf-agent-desc {
                font-size: 10px;
                color: var(--muted-foreground);
                line-height: 1.3;
                margin-top: 1px;
            }

            /* ── Empty state ── */
            .wf-empty {
                text-align: center;
                padding: 24px 16px;
                color: var(--muted-foreground);
                font-size: 12px;
                font-style: italic;
            }

            /* ── Parallel branch columns ── */
            .wf-branches {
                display: flex;
                gap: 8px;
                justify-content: center;
                align-items: flex-start;
                width: 100%;
            }
            .wf-branch {
                flex: 1;
                min-width: 0;
                max-width: 280px;
                display: flex;
                flex-direction: column;
                align-items: center;
            }
            .wf-branch-connector {
                position: relative;
                height: 14px;
                width: 2px;
                background: var(--connector-color, var(--border));
            }

            /* ── Pipeline wrapper (dot-grid background) ── */
            .wf-pipeline-canvas {
                position: relative;
                background-image: radial-gradient(circle, oklch(from var(--border) l c h / 30%) 1px, transparent 1px);
                background-size: 14px 14px;
                border-radius: var(--radius, 0.75rem);
                min-height: 80px;
            }
        `;
        document.head.appendChild(style);
    }

    // ── Node renderers ──────────────────────────────────────────────

    function renderPipelineNode(node, agentMap, depth, activeAgentName, completedNodes, failedNodes) {
        if (!node) return '';
        const m = META[node.type] || META.agent;

        switch (node.type) {
            case 'agent': return renderAgentNode(node, agentMap, m, activeAgentName, completedNodes, failedNodes);
            case 'sequential': return renderSequentialNode(node, agentMap, m, depth, activeAgentName, completedNodes, failedNodes);
            case 'parallel': return renderParallelNode(node, agentMap, m, depth, activeAgentName, completedNodes, failedNodes);
            case 'loop': return renderLoopNode(node, agentMap, m, depth, activeAgentName, completedNodes, failedNodes);
            case 'conditional': return renderConditionalNode(node, agentMap, m, depth, activeAgentName, completedNodes, failedNodes);
            default: return renderAgentNode(node, agentMap, m, activeAgentName, completedNodes, failedNodes);
        }
    }

    function renderAgentNode(node, agentMap, m, activeAgentName, completedNodes, failedNodes) {
        const name = node.agent_id ? (agentMap[node.agent_id] || 'Unknown Agent') : 'No agent selected';
        const completedInfo = completedNodes?.[name];
        const isFailed    = !!failedNodes?.[name];
        const isCompleted = !!completedInfo && !isFailed;
        const isActive    = !isCompleted && !isFailed &&
                            activeAgentName && ((name === activeAgentName) || (node.label === activeAgentName));

        let nodeClass = 'wf-node';
        if (isActive)    nodeClass += ' is-active';
        else if (isCompleted) nodeClass += ' is-completed';
        else if (isFailed)    nodeClass += ' is-failed';

        // Duration badge for completed nodes, error badge for failed
        const checkSvg = `<svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"></polyline></svg>`;
        const xSvg     = `<svg width="9" height="9" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>`;
        let statusBadge = '';
        if (isCompleted) {
            const secs = completedInfo.durationMs != null
                ? (completedInfo.durationMs / 1000).toFixed(1) + 's'
                : 'done';
            statusBadge = `<span class="wf-node-done-badge">${checkSvg} ${secs}</span>`;
        } else if (isFailed) {
            statusBadge = `<span class="wf-node-failed-badge">${xSvg} error</span>`;
        }

        const outputKey = node.output_key
            ? `<span class="wf-agent-desc">→ <code style="font-size:10px;padding:0 3px;border-radius:3px;background:var(--muted);font-family:var(--font-mono);">${esc(node.output_key)}</code></span>`
            : '';

        // Build orchestration badges
        let badges = '';
        if (node.disallow_transfer_to_parent) {
            badges += `<span style="font-size:9px;padding:0 4px;border-radius:4px;background:oklch(0.65 0.15 25/12%);color:oklch(0.55 0.18 25);font-weight:600;white-space:nowrap;" title="Transfer to parent blocked">⛔ ↑parent</span>`;
        }
        if (node.disallow_transfer_to_peers) {
            badges += `<span style="font-size:9px;padding:0 4px;border-radius:4px;background:oklch(0.65 0.15 25/12%);color:oklch(0.55 0.18 25);font-weight:600;white-space:nowrap;" title="Transfer to peers blocked">⛔ ↔peers</span>`;
        }
        if (node.include_contents === 'none') {
            badges += `<span style="font-size:9px;padding:0 4px;border-radius:4px;background:oklch(0.7 0.14 250/12%);color:oklch(0.55 0.16 250);font-weight:600;white-space:nowrap;" title="Fresh context (no history)">🧹 fresh</span>`;
        }
        const badgesHtml = badges ? `<div style="display:flex;gap:3px;flex-wrap:wrap;margin-top:2px;">${badges}</div>` : '';

        const nameFontStyle = isActive ? 'font-weight:700;' : '';
        const accentOverride = isCompleted ? '' : isFailed ? '' : `--node-accent:${m.accent};--node-border:${m.accentMuted};--node-bg:${m.accentMuted};`;

        return `
            <div class="${nodeClass}" style="${accentOverride}">
                <div style="display:flex;align-items:center;gap:8px;">
                    <div class="wf-badge" style="--badge-bg:${m.accentMuted};--badge-fg:${m.accentFg};">
                        ${m.icon}
                    </div>
                    <div style="flex:1;min-width:0;">
                        <div class="wf-agent-name" style="${nameFontStyle}">${esc(name)}</div>
                        ${outputKey}
                        ${badgesHtml}
                    </div>
                    ${statusBadge}
                </div>
            </div>`;
    }

    function connector(accentColor, delay) {
        const d = delay !== undefined ? delay : (Math.random() * 1.5);
        return `
            <div class="wf-connector" style="--connector-color:${accentColor};">
                <div class="wf-connector-flow" style="animation-delay:${d.toFixed(2)}s;"></div>
                <div class="wf-connector-dot"></div>
            </div>`;
    }

    function renderSequentialNode(node, agentMap, m, depth, activeAgentName, completedNodes, failedNodes) {
        const kids = node.children || [];
        if (kids.length === 0) return renderEmptyNode(m, 'Empty sequence');
        
        const isActive = activeAgentName && (node.label === activeAgentName);
        const nodeClass = isActive ? 'wf-node is-active' : 'wf-node';

        let body = '';
        for (let i = 0; i < kids.length; i++) {
            body += renderPipelineNode(kids[i], agentMap, depth + 1, activeAgentName, completedNodes, failedNodes);
            if (i < kids.length - 1) {
                body += connector(m.accent, i * 0.4);
            }
        }

        if (depth === 0) {
            return `
                <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                    <div class="wf-badge" style="--badge-bg:${m.accentMuted};--badge-fg:${m.accentFg};margin-bottom:6px;">
                        ${m.icon}&nbsp;${m.label}
                    </div>
                    <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                        ${body}
                    </div>
                </div>`;
        }

        return `
            <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                <div class="${nodeClass}" style="--node-accent:${m.accent};--node-border:${m.accentMuted};--node-bg:${m.accentMuted};max-width:unset;">
                    <div class="wf-badge" style="--badge-bg:${m.accentMuted};--badge-fg:${m.accentFg};">
                        ${m.icon}&nbsp;${esc(node.label || m.label)}
                    </div>
                    <div style="display:flex;flex-direction:column;align-items:center;width:100%;padding-top:8px;">
                        ${body}
                    </div>
                </div>
            </div>`;
    }

    function renderParallelNode(node, agentMap, m, depth, activeAgentName, completedNodes, failedNodes) {
        const kids = node.children || [];
        if (kids.length === 0) return renderEmptyNode(m, 'Empty parallel');

        let branches = '';
        for (const child of kids) {
            branches += `
                <div class="wf-branch">
                    <div class="wf-branch-connector" style="--connector-color:${m.accent};"></div>
                    ${renderPipelineNode(child, agentMap, depth + 1, activeAgentName, completedNodes, failedNodes)}
                    <div class="wf-branch-connector" style="--connector-color:${m.accent};"></div>
                </div>`;
        }

        return `
            <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                <div class="wf-badge" style="--badge-bg:${m.accentMuted};--badge-fg:${m.accentFg};">
                    ${m.icon}&nbsp;${esc(node.label || m.label)}
                </div>
                <div class="wf-fork-join" style="--connector-color:${m.accent};">
                    <div class="wf-fork-join-center" style="--connector-color:${m.accent};"></div>
                </div>
                <div class="wf-branches">
                    ${branches}
                </div>
                <div class="wf-fork-join" style="--connector-color:${m.accent};">
                    <div class="wf-fork-join-center" style="--connector-color:${m.accent};"></div>
                </div>
            </div>`;
    }

    function renderLoopNode(node, agentMap, m, depth, activeAgentName, completedNodes, failedNodes) {
        const kids = node.children || [];
        const maxIter = node.max_iterations || 3;

        let body = '';
        if (kids.length === 0) {
            body = renderEmptyNode(m, 'Empty loop');
        } else {
            for (let i = 0; i < kids.length; i++) {
                body += renderPipelineNode(kids[i], agentMap, depth + 1, activeAgentName, completedNodes, failedNodes);
                if (i < kids.length - 1) {
                    body += connector(m.accent, i * 0.3);
                }
            }
        }

        // Loop-back arrow SVG
        const loopArrow = `
            <div class="wf-loop-return">
                <svg width="80" height="18" viewBox="0 0 80 18" fill="none">
                    <path d="M10 2 C10 14, 70 14, 70 2" stroke="${m.accent}" stroke-width="1.5" stroke-dasharray="4 3" fill="none" opacity="0.45"/>
                    <path d="M67 5 L70 1 L73 5" stroke="${m.accent}" stroke-width="1.5" fill="none" opacity="0.45" stroke-linecap="round"/>
                </svg>
            </div>`;

        return `
            <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                <div class="wf-loop-wrap" style="--node-accent:${m.accent};">
                    <div class="wf-loop-badge" style="--node-accent:${m.accent};--badge-fg:${m.accentFg};">
                        ${m.icon}&nbsp;${esc(node.label || m.label)} (×${maxIter})
                    </div>
                    <div style="display:flex;flex-direction:column;align-items:center;width:100%;padding-top:2px;">
                        ${body}
                    </div>
                    ${loopArrow}
                </div>
            </div>`;
    }

    function renderConditionalNode(node, agentMap, m, depth, activeAgentName, completedNodes, failedNodes) {
        const kids = node.children || [];

        // First child is the evaluator; remaining are branches
        let evaluatorHtml = '';
        let branchesHtml = '';

        if (kids.length > 0) {
            evaluatorHtml = `
                <div style="display:flex;flex-direction:column;align-items:center;width:100%;max-width:280px;margin:0 auto;">
                    <div style="font-size:9px;color:var(--wf-red-fg, #fca5a5);font-weight:600;margin-bottom:4px;opacity:0.8;">Evaluator</div>
                    ${renderPipelineNode(kids[0], agentMap, depth + 1, activeAgentName, completedNodes, failedNodes)}
                </div>`;
        }

        if (kids.length > 1) {
            branchesHtml = '<div class="wf-branches">';
            for (let i = 1; i < kids.length; i++) {
                const branch = kids[i];
                const condLabel = branch.condition || '???';
                const isFallback = condLabel === '*';
                branchesHtml += `
                    <div class="wf-branch">
                        <div class="wf-branch-connector" style="--connector-color:${m.accent};"></div>
                        <div style="font-size:9px;color:${isFallback ? 'var(--muted-foreground)' : 'var(--wf-red-fg, #fca5a5)'};font-weight:600;margin-bottom:3px;">
                            ${isFallback ? 'fallback' : esc(condLabel)}
                        </div>
                        ${renderPipelineNode(branch, agentMap, depth + 1, activeAgentName, completedNodes, failedNodes)}
                        <div class="wf-branch-connector" style="--connector-color:${m.accent};"></div>
                    </div>`;
            }
            branchesHtml += '</div>';
        }

        if (kids.length === 0) {
            return renderEmptyNode(m, 'Empty conditional');
        }

        const forkJoin = kids.length > 1 ? `
            <div class="wf-fork-join" style="--connector-color:${m.accent};">
                <div class="wf-fork-join-center" style="--connector-color:${m.accent};"></div>
            </div>` : '';

        return `
            <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                <div class="wf-badge" style="--badge-bg:${m.accentMuted};--badge-fg:${m.accentFg};">
                    ${m.icon}&nbsp;${esc(node.label || m.label)}
                </div>
                <div style="display:flex;flex-direction:column;align-items:center;width:100%;padding-top:6px;">
                    ${evaluatorHtml}
                    ${forkJoin}
                    ${branchesHtml}
                    ${forkJoin}
                </div>
            </div>`;
    }

    function renderEmptyNode(m, msg) {
        return `
            <div class="wf-node" style="--node-accent:${m.accent};--node-border:${m.accentMuted};--node-bg:${m.accentMuted};opacity:0.5;max-width:200px;">
                <div style="font-size:10px;color:var(--muted-foreground);font-style:italic;text-align:center;padding:2px 0;">${msg}</div>
            </div>`;
    }

    // ── Alpine component ────────────────────────────────────────────
    return {
        init() {
            injectStyles();
        },

        renderTree(graph, agentsList, activeAgentName, completedNodes, failedNodes) {
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
                    <div class="wf-pipeline-canvas" style="padding:20px 14px;">
                        <div style="display:flex;flex-direction:column;align-items:center;">
                            ${renderPipelineNode(graph, agentMap, 0, activeAgentName, completedNodes, failedNodes)}
                        </div>
                    </div>
                </div>`;
        },
    };
}
