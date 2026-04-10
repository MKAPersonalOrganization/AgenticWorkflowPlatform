/**
 * Editable Workflow Tree Component
 *
 * A tree-based workflow editor with the visual style of the workflow viewer
 * but with full editing capabilities:
 *
 *   • Inline node editing (label, agent selection, properties)
 *   • Drag-and-drop to add nodes from palette
 *   • Node reordering (move up/down within parent)
 *   • Node deletion with confirmation
 *   • Add child nodes to composite containers
 *   • Visual distinction for sequential, parallel, loop, and agent nodes
 *   • Fork/join visuals for parallel branches
 *   • Loop-back indicator with iteration badge
 */

export const editableWorkflowTree = () => {
    // ── Type metadata ───────────────────────────────────────────────
    const META = {
        sequential: {
            icon: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 5v14"/><path d="m19 12-7 7-7-7"/></svg>`,
            label: 'Sequential',
            color: '#7c3aed',
            colorMuted: 'rgba(124, 58, 237, 0.12)',
            colorFg: '#8b5cf6',
        },
        parallel: {
            icon: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M8 3v18"/><path d="M16 3v18"/></svg>`,
            label: 'Parallel',
            color: '#0284c7',
            colorMuted: 'rgba(2, 132, 199, 0.12)',
            colorFg: '#38bdf8',
        },
        loop: {
            icon: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21 12a9 9 0 1 1-9-9c2.52 0 4.93 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/></svg>`,
            label: 'Loop',
            color: '#d97706',
            colorMuted: 'rgba(217, 119, 6, 0.12)',
            colorFg: '#fbbf24',
        },
        agent: {
            icon: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></svg>`,
            label: 'Agent',
            color: '#059669',
            colorMuted: 'rgba(5, 150, 105, 0.12)',
            colorFg: '#34d399',
        },
        remote: {
            icon: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/><path d="M2 12h20"/></svg>`,
            label: 'Remote',
            color: '#be123c',
            colorMuted: 'rgba(190, 18, 60, 0.12)',
            colorFg: '#fb7185',
        },
        conditional: {
            icon: `<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M16 3h5v5"/><path d="M8 3H3v5"/><path d="M12 22v-8.3a4 4 0 0 0-1.172-2.872L3 3"/><path d="m15 9 6-6"/></svg>`,
            label: 'Conditional',
            color: '#dc2626',
            colorMuted: 'rgba(220, 38, 38, 0.12)',
            colorFg: '#f87171',
        },
    };

    function esc(s) {
        if (!s) return '';
        return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
    }

    // ── CSS (injected once) ─────────────────────────────────────────
    function injectStyles() {
        if (document.getElementById('ewt-styles')) return;
        const style = document.createElement('style');
        style.id = 'ewt-styles';
        style.textContent = `
            /* ══════════════════════════════════════════════
               Editable Workflow Tree — Theme-Aware Style
               ══════════════════════════════════════════════ */

            .ewt-container { font-family: inherit; }

            .ewt-pipeline-canvas {
                position: relative;
                background-color: var(--background);
                background-image:
                    radial-gradient(circle, var(--border) 1px, transparent 1px);
                background-size: 20px 20px;
                min-height: 100%;
                padding: 28px 24px 40px;
                width: 100%;
                box-sizing: border-box;
            }

            /* ── Connector ── */
            .ewt-connector {
                position: relative;
                display: flex;
                align-items: center;
                justify-content: center;
                height: 24px;
                width: 100%;
            }
            .ewt-connector::before {
                content: '';
                position: absolute;
                left: 50%;
                top: 0;
                bottom: 0;
                width: 2px;
                background: var(--ewt-color, var(--border));
                transform: translateX(-50%);
                opacity: 0.5;
            }
            .ewt-connector-dot {
                position: relative;
                z-index: 1;
                width: 6px;
                height: 6px;
                border-radius: 50%;
                background: var(--ewt-color, var(--primary));
                box-shadow: 
                    0 0 6px var(--ewt-color, var(--primary)),
                    0 0 12px var(--ewt-color, var(--primary));
                animation: ewt-pulse 2s ease-in-out infinite;
            }
            @keyframes ewt-pulse {
                0%, 100% { opacity: 1; transform: scale(1); }
                50% { opacity: 0.7; transform: scale(0.85); }
            }

            /* ── Node card ── */
            .ewt-node {
                position: relative;
                border-radius: 8px;
                border: 1px solid var(--border);
                background: var(--card);
                background-image: linear-gradient(180deg, oklch(from var(--card) l c h / 0.5) 0%, var(--card) 100%);
                padding: 10px 12px 10px 15px;
                transition: border-color 0.12s, box-shadow 0.12s, transform 0.15s ease;
                width: 100%;
                overflow: hidden;
            }
            .ewt-node:hover {
                border-color: var(--muted-foreground);
                box-shadow: 0 2px 12px oklch(0 0 0 / 18%);
                transform: translateY(-1px);
            }
            .ewt-node.selected {
                border-color: var(--ewt-color, var(--primary));
                box-shadow:
                    0 0 0 1px var(--ewt-color, var(--primary)),
                    0 4px 16px oklch(from var(--primary) l c h / 15%);
            }
            .ewt-node.drop-target {
                border-color: #22c55e;
                box-shadow: 0 0 0 1px #22c55e, 0 0 8px rgba(34, 197, 94, 0.15);
            }

            /* ── Left accent bar ── */
            .ewt-node::after {
                content: '';
                position: absolute;
                left: 0;
                top: 4px;
                bottom: 4px;
                width: 3px;
                background: var(--ewt-color, var(--muted-foreground));
                border-radius: 0 2px 2px 0;
            }

            /* ── Type badge ── */
            .ewt-badge {
                display: inline-flex;
                align-items: center;
                gap: 4px;
                padding: 3px 8px;
                border-radius: 6px;
                font-size: 10px;
                font-weight: 600;
                letter-spacing: 0.02em;
                white-space: nowrap;
                background: var(--ewt-color-muted, var(--muted));
                color: var(--ewt-color-fg, var(--muted-foreground));
                box-shadow: 0 1px 2px oklch(0 0 0 / 0.1);
                transition: all 0.15s ease;
            }
            .ewt-badge:hover {
                box-shadow: 0 2px 8px var(--ewt-color-muted, var(--muted));
            }
            .ewt-badge svg { flex-shrink: 0; }

            /* ── Agent name ── */
            .ewt-agent-name {
                font-size: 12px;
                font-weight: 600;
                color: var(--foreground);
                line-height: 1.3;
                overflow: hidden;
                text-overflow: ellipsis;
                white-space: nowrap;
            }
            .ewt-agent-desc {
                font-size: 10px;
                color: var(--muted-foreground);
                font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
                line-height: 1.3;
                margin-top: 3px;
            }

            /* ── Parallel fork/join ── */
            .ewt-fork-join {
                position: relative;
                height: 14px;
                width: 100%;
            }
            .ewt-fork-join::before {
                content: '';
                position: absolute;
                left: 10%;
                right: 10%;
                top: 50%;
                height: 1px;
                background: var(--ewt-color, var(--border));
                transform: translateY(-50%);
                opacity: 0.6;
            }
            .ewt-fork-join-center {
                position: absolute;
                left: 50%;
                top: 50%;
                transform: translate(-50%, -50%);
                width: 1px;
                height: 100%;
                background: var(--ewt-color, var(--border));
                opacity: 0.6;
            }

            /* ── Parallel branches ── */
            .ewt-branches {
                display: flex;
                gap: 12px;
                justify-content: center;
                align-items: flex-start;
                width: 100%;
                flex-wrap: wrap;
            }
            .ewt-branch {
                flex: 1;
                min-width: 180px;
                display: flex;
                flex-direction: column;
                align-items: center;
            }
            .ewt-branch-connector {
                height: 10px;
                width: 1px;
                background: var(--ewt-color, var(--border));
                opacity: 0.6;
            }

            /* ── Loop wrapper ── */
            .ewt-loop-wrap {
                position: relative;
                border-radius: 10px;
                border: 1px dashed oklch(0.45 0.15 45 / 0.3);
                padding: 16px 14px 18px;
                background: oklch(0.45 0.15 45 / 0.02);
                width: 100%;
                transition: all 0.2s ease;
            }
            .ewt-loop-wrap:hover {
                border-color: oklch(0.45 0.15 45 / 0.5);
                background: oklch(0.45 0.15 45 / 0.04);
                box-shadow: 0 0 20px oklch(0.45 0.15 45 / 0.08);
            }
            .ewt-loop-badge {
                position: absolute;
                top: -10px;
                right: 12px;
                display: inline-flex;
                align-items: center;
                gap: 3px;
                padding: 2px 10px;
                border-radius: 6px;
                font-size: 9px;
                font-weight: 700;
                font-family: ui-monospace, SFMono-Regular, 'SF Mono', Menlo, monospace;
                background: var(--background);
                color: oklch(0.6 0.18 45);
                border: 1px dashed oklch(0.45 0.15 45 / 0.4);
            }
            .ewt-loop-return {
                display: flex;
                align-items: center;
                justify-content: center;
                margin-top: 6px;
            }

            /* ── Action buttons ── */
            .ewt-actions {
                display: flex;
                gap: 1px;
                opacity: 0;
                transition: opacity 0.1s;
            }
            .ewt-node:hover .ewt-actions,
            .ewt-node.selected .ewt-actions {
                opacity: 1;
            }
            .ewt-action-btn {
                display: flex;
                align-items: center;
                justify-content: center;
                width: 22px;
                height: 22px;
                border-radius: 4px;
                border: 1px solid var(--border);
                background: var(--card);
                color: var(--muted-foreground);
                cursor: pointer;
                font-size: 11px;
                transition: all 0.15s ease;
            }
            .ewt-action-btn:hover {
                background: var(--primary);
                border-color: var(--primary);
                color: var(--primary-foreground);
                box-shadow: 0 2px 8px oklch(from var(--primary) l c h / 25%);
            }
            .ewt-action-btn.danger:hover {
                background: #dc2626;
                border-color: #dc2626;
                color: white;
            }

            /* ── Add child buttons ── */
            .ewt-add-children {
                display: flex;
                gap: 3px;
                flex-wrap: wrap;
                padding: 6px 8px;
                border-top: 1px solid var(--border);
                background: var(--muted);
                border-radius: 0 0 8px 8px;
            }
            .ewt-add-btn {
                font-size: 9px;
                padding: 2px 7px;
                border-radius: 3px;
                border: 1px solid transparent;
                cursor: pointer;
                font-weight: 600;
                letter-spacing: 0.02em;
                transition: all 0.1s;
                background: transparent;
            }
            .ewt-add-btn.agent {
                color: #059669;
                border-color: rgba(5, 150, 105, 0.25);
            }
            .ewt-add-btn.agent:hover {
                background: rgba(5, 150, 105, 0.1);
                border-color: rgba(5, 150, 105, 0.4);
            }
            .ewt-add-btn.sequential {
                color: #7c3aed;
                border-color: rgba(124, 58, 237, 0.25);
            }
            .ewt-add-btn.sequential:hover {
                background: rgba(124, 58, 237, 0.1);
                border-color: rgba(124, 58, 237, 0.4);
            }
            .ewt-add-btn.parallel {
                color: #0284c7;
                border-color: rgba(2, 132, 199, 0.25);
            }
            .ewt-add-btn.parallel:hover {
                background: rgba(2, 132, 199, 0.1);
                border-color: rgba(2, 132, 199, 0.4);
            }
            .ewt-add-btn.loop {
                color: #d97706;
                border-color: rgba(217, 119, 6, 0.25);
            }
            .ewt-add-btn.loop:hover {
                background: rgba(217, 119, 6, 0.1);
                border-color: rgba(217, 119, 6, 0.4);
            }
            .ewt-add-btn.conditional {
                color: #dc2626;
                border-color: rgba(220, 38, 38, 0.25);
            }
            .ewt-add-btn.conditional:hover {
                background: rgba(220, 38, 38, 0.1);
                border-color: rgba(220, 38, 38, 0.4);
            }

            /* ── Inline edit ── */
            .ewt-inline-edit {
                background: transparent;
                border: 1px solid transparent;
                border-radius: 3px;
                padding: 1px 4px;
                font-size: 12px;
                font-weight: 600;
                color: var(--foreground);
                width: 100%;
                outline: none;
                transition: all 0.1s;
            }
            .ewt-inline-edit:hover {
                border-color: var(--border);
            }
            .ewt-inline-edit:focus {
                border-color: var(--ewt-color, var(--ring));
                background: var(--muted);
            }

            /* ── Agent select dropdown ── */
            .ewt-agent-select {
                background: var(--muted);
                border: 1px solid var(--border);
                border-radius: 3px;
                padding: 3px 6px;
                font-size: 10px;
                color: var(--foreground);
                width: 100%;
                outline: none;
                cursor: pointer;
                transition: border-color 0.1s;
            }
            .ewt-agent-select:focus {
                border-color: var(--ewt-color, var(--ring));
            }

            /* ── Empty state ── */
            .ewt-empty {
                text-align: center;
                padding: 20px 16px;
                color: var(--muted-foreground);
                font-size: 11px;
                border: 1px dashed var(--border);
                border-radius: 6px;
                margin: 4px 0;
            }

            /* ── Drop zone indicator ── */
            .ewt-drop-zone {
                border: 1px dashed rgba(34, 197, 94, 0.35);
                border-radius: 6px;
                padding: 16px;
                text-align: center;
                color: #22c55e;
                font-size: 11px;
                margin: 6px 0;
                transition: all 0.12s;
            }
            .ewt-drop-zone.active {
                border-color: #22c55e;
                background: rgba(34, 197, 94, 0.06);
            }

            /* ── Move buttons ── */
            .ewt-move-btn {
                display: flex;
                align-items: center;
                justify-content: center;
                width: 18px;
                height: 18px;
                border-radius: 3px;
                border: none;
                background: transparent;
                color: var(--muted-foreground);
                cursor: pointer;
                font-size: 10px;
                transition: all 0.1s;
            }
            .ewt-move-btn:hover:not(:disabled) {
                background: var(--accent);
                color: var(--foreground);
            }
            .ewt-move-btn:disabled {
                opacity: 0.2;
                cursor: not-allowed;
            }

            /* ── Settings panel ── */
            .ewt-settings-panel {
                background: oklch(from var(--card) l c h / 0.5);
                border: 1px solid var(--border);
                border-radius: 8px;
                padding: 12px;
                margin-top: 8px;
                font-size: 10px;
                box-shadow: inset 0 1px 3px oklch(0 0 0 / 0.05);
            }
            .ewt-settings-row {
                display: flex;
                align-items: center;
                gap: 8px;
                margin-bottom: 8px;
                padding-bottom: 8px;
                border-bottom: 1px solid var(--border);
            }
            .ewt-settings-row:last-child { margin-bottom: 0; padding-bottom: 0; border-bottom: none; }
            .ewt-settings-label {
                color: var(--muted-foreground);
                font-size: 9px;
                font-weight: 600;
                text-transform: uppercase;
                letter-spacing: 0.05em;
                width: 72px;
                flex-shrink: 0;
            }
            .ewt-settings-input {
                flex: 1;
                background: var(--background);
                border: 1px solid var(--border);
                border-radius: 3px;
                padding: 3px 6px;
                font-size: 10px;
                color: var(--foreground);
                outline: none;
                transition: border-color 0.1s;
            }
            .ewt-settings-input:focus {
                border-color: var(--ewt-color, var(--ring));
            }

            /* ── Collapse/expand ── */
            .ewt-collapse-btn {
                display: flex;
                align-items: center;
                justify-content: center;
                width: 16px;
                height: 16px;
                border-radius: 3px;
                border: none;
                background: transparent;
                color: var(--muted-foreground);
                cursor: pointer;
                font-size: 8px;
                transition: all 0.1s;
                flex-shrink: 0;
            }
            .ewt-collapse-btn:hover { background: var(--accent); color: var(--foreground); }
            .ewt-collapse-btn svg { transition: transform 0.15s ease; }
            .ewt-collapsed-summary {
                font-size: 10px;
                color: var(--muted-foreground);
                padding: 6px 0 2px;
                text-align: center;
                font-style: italic;
            }

            .ewt-node, .ewt-loop-wrap { cursor: grab; }
            .ewt-node:active, .ewt-loop-wrap:active { cursor: grabbing; }
        `;
        document.head.appendChild(style);
    }

    // ── Node ID counter ─────────────────────────────────────────────
    let _nodeCounter = 1000;
    function nextId() {
        _nodeCounter++;
        return `ewt-node-${_nodeCounter}`;
    }

    // ── Create new node ─────────────────────────────────────────────
    function createNode(type) {
        const id = nextId();
        if (type === 'agent') {
            return { id, type: 'agent', label: '', agent_id: '', output_key: '' };
        }
        if (type === 'remote') {
            return { id, type: 'remote', label: '', a2a_connector_id: '', output_key: '' };
        }
        const node = { id, type, label: '', children: [] };
        if (type === 'loop') node.max_iterations = 3;
        return node;
    }

    // ── Collapse helpers ────────────────────────────────────────────

    function isCollapsed(nodeId) {
        const store = typeof Alpine !== 'undefined' ? Alpine.store('flow') : null;
        return store ? store.isCollapsed(nodeId) : false;
    }

    function collapseChevron(nodeId) {
        const collapsed = isCollapsed(nodeId);
        const rot = collapsed ? 'rotate(-90deg)' : 'rotate(0deg)';
        return `<button class="ewt-collapse-btn" onclick="event.stopPropagation(); Alpine.store('flow').toggleCollapse('${nodeId}')" title="${collapsed ? 'Expand' : 'Collapse'}">
            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" style="transform:${rot}"><path d="m6 9 6 6 6-6"/></svg>
        </button>`;
    }

    function collapsedSummary(node) {
        const count = node.children ? node.children.length : 0;
        return `<div class="ewt-collapsed-summary">${count} node${count !== 1 ? 's' : ''} collapsed</div>`;
    }

    // ── Render functions ────────────────────────────────────────────

    function renderConnector(color) {
        return `
            <div class="ewt-connector" style="--ewt-color:${color};">
                <div class="ewt-connector-dot"></div>
            </div>`;
    }

    function renderAgentNode(node, agents, isSelected, nodeId) {
        const m = META.agent;
        const selectedAgent = agents.find(a => a.id === node.agent_id);
        const name = node.agent_id ? (selectedAgent?.name || 'Unknown') : 'No agent selected';
        const isComposed = selectedAgent?.isComposed || false;
        const agentOptions = agents.map(a => 
            `<option value="${a.id}" ${node.agent_id === a.id ? 'selected' : ''}>${esc(a.name)}${a.isComposed ? ' (Workflow)' : ''}</option>`
        ).join('');

        return `
            <div class="ewt-node ${isSelected ? 'selected' : ''}" 
                 style="--ewt-color:${m.color};--ewt-color-muted:${m.colorMuted};--ewt-color-fg:${m.colorFg};"
                 data-node-id="${nodeId}"
                 data-node-type="agent"
                 data-agent-id="${node.agent_id}"
                 data-agent-name="${esc(name)}"
                 data-node-label="${esc(node.label || '')}"
                 draggable="true">
                <div style="display:flex;align-items:flex-start;gap:8px;">
                    <div class="ewt-badge" style="background:${m.colorMuted};color:${m.colorFg};">
                        ${m.icon}
                    </div>
                    <div style="flex:1;min-width:0;">
                        <input type="text" 
                               class="ewt-inline-edit" 
                               value="${esc(node.label || name)}" 
                               placeholder="Node label..."
                               data-field="label"
                               data-node-id="${nodeId}" />
                        <select class="ewt-agent-select" data-field="agent_id" data-node-id="${nodeId}">
                            <option value="">— Select agent —</option>
                            ${agentOptions}
                        </select>
                        ${node.condition !== undefined ? `
                            <div style="display:flex;align-items:center;gap:4px;margin-top:4px;">
                                <span style="font-size:9px;color:#f87171;font-weight:600;">condition:</span>
                                <input type="text" 
                                       class="ewt-inline-edit" 
                                       value="${esc(node.condition)}" 
                                       placeholder="value or * for fallback"
                                       style="font-size:10px;color:#fca5a5;"
                                       data-field="condition"
                                       data-node-id="${nodeId}" />
                            </div>` : ''}
                        ${node.output_key ? `<div class="ewt-agent-desc">→ ${esc(node.output_key)}</div>` : ''}
                    </div>
                    <div class="ewt-actions">
                        ${isComposed ? `<button class="ewt-action-btn" onclick="window.ewtEditAgent('${node.agent_id}')" title="Edit Workflow">✎</button>` : ''}
                        <button class="ewt-action-btn" onclick="window.ewtMoveUp('${nodeId}')" title="Move up">↑</button>
                        <button class="ewt-action-btn" onclick="window.ewtMoveDown('${nodeId}')" title="Move down">↓</button>
                        <button class="ewt-action-btn" onclick="window.ewtDuplicate('${nodeId}')" title="Duplicate (Ctrl+D)">⧉</button>
                        <button class="ewt-action-btn danger" onclick="window.ewtDeleteNode('${nodeId}')" title="Delete">✕</button>
                    </div>
                </div>
            </div>`;
    }

    function renderRemoteNode(node, connectors, isSelected, nodeId) {
        const m = META.remote;
        const name = node.a2a_connector_id ? (connectors.find(c => c.id === node.a2a_connector_id)?.name || 'Unknown') : 'No connector selected';
        const connOptions = connectors.map(c => 
            `<option value="${c.id}" ${node.a2a_connector_id === c.id ? 'selected' : ''}>${esc(c.name || c.id)}</option>`
        ).join('');

        return `
            <div class="ewt-node ${isSelected ? 'selected' : ''}" 
                 style="--ewt-color:${m.color};--ewt-color-muted:${m.colorMuted};--ewt-color-fg:${m.colorFg};"
                 data-node-id="${nodeId}"
                 data-node-type="remote"
                 data-agent-id="${node.a2a_connector_id}"
                 data-agent-name="${esc(name)}"
                 data-node-label="${esc(node.label || '')}"
                 draggable="true">
                <div style="display:flex;align-items:flex-start;gap:8px;">
                    <div class="ewt-badge" style="background:${m.colorMuted};color:${m.colorFg};">
                        ${m.icon}
                    </div>
                    <div style="flex:1;min-width:0;">
                        <input type="text" 
                               class="ewt-inline-edit" 
                               value="${esc(node.label || name)}" 
                               placeholder="Node label..."
                               data-field="label"
                               data-node-id="${nodeId}" />
                        <select class="ewt-agent-select" data-field="a2a_connector_id" data-node-id="${nodeId}">
                            <option value="">— Select connector —</option>
                            ${connOptions}
                        </select>
                    </div>
                    <div class="ewt-actions">
                        <button class="ewt-action-btn" onclick="window.ewtMoveUp('${nodeId}')" title="Move up">↑</button>
                        <button class="ewt-action-btn" onclick="window.ewtMoveDown('${nodeId}')" title="Move down">↓</button>
                        <button class="ewt-action-btn" onclick="window.ewtDuplicate('${nodeId}')" title="Duplicate (Ctrl+D)">⧉</button>
                        <button class="ewt-action-btn danger" onclick="window.ewtDeleteNode('${nodeId}')" title="Delete">✕</button>
                    </div>
                </div>
            </div>`;
    }

    function renderSequentialNode(node, agents, connectors, depth, isSelected, nodeId, graph) {
        const m = META.sequential;
        const kids = node.children || [];
        const collapsed = isCollapsed(nodeId);

        let body = '';
        if (!collapsed) {
            for (let i = 0; i < kids.length; i++) {
                body += renderPipelineNode(kids[i], agents, connectors, depth + 1, graph);
                if (i < kids.length - 1) body += renderConnector(m.color);
            }
            if (kids.length === 0) body = `<div class="ewt-empty">Drop agents here or click + to add</div>`;
        } else {
            body = collapsedSummary(node);
        }

        const addButtons = collapsed ? '' : `
            <div class="ewt-add-children">
                <button class="ewt-add-btn agent" onclick="window.ewtAddChild('${nodeId}', 'agent')">+ Agent</button>
                <button class="ewt-add-btn sequential" onclick="window.ewtAddChild('${nodeId}', 'sequential')">+ Seq</button>
                <button class="ewt-add-btn parallel" onclick="window.ewtAddChild('${nodeId}', 'parallel')">+ Par</button>
                <button class="ewt-add-btn loop" onclick="window.ewtAddChild('${nodeId}', 'loop')">+ Loop</button>
                <button class="ewt-add-btn conditional" onclick="window.ewtAddChild('${nodeId}', 'conditional')">+ Cond</button>
            </div>`;

        if (depth === 0) {
            return `
                <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                    <div style="display:flex;align-items:center;gap:4px;margin-bottom:8px;">
                        ${collapseChevron(nodeId)}
                        <div class="ewt-badge" style="background:${m.colorMuted};color:${m.colorFg};">
                            ${m.icon}&nbsp;Sequential
                        </div>
                        ${collapsed ? `<span style="font-size:9px;color:#6b7280;">(${kids.length})</span>` : ''}
                    </div>
                    <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                        ${body}
                    </div>
                    ${addButtons}
                </div>`;
        }

        return `
            <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                <div class="ewt-node ${isSelected ? 'selected' : ''}"
                     style="--ewt-color:${m.color};--ewt-color-muted:${m.colorMuted};--ewt-color-fg:${m.colorFg};max-width:unset;"
                     data-node-id="${nodeId}"
                     data-node-type="sequential"
                     draggable="true">
                    <div style="display:flex;align-items:center;gap:8px;">
                        ${collapseChevron(nodeId)}
                        <div class="ewt-badge" style="background:${m.colorMuted};color:${m.colorFg};">
                            ${m.icon}&nbsp;Sequential
                        </div>
                        <input type="text" class="ewt-inline-edit" value="${esc(node.label)}" placeholder="Sequence label..." style="flex:1;" data-field="label" data-node-id="${nodeId}" />
                        <div class="ewt-actions">
                            <button class="ewt-action-btn" onclick="window.ewtMoveUp('${nodeId}')" title="Move up">↑</button>
                            <button class="ewt-action-btn" onclick="window.ewtMoveDown('${nodeId}')" title="Move down">↓</button>
                            <button class="ewt-action-btn danger" onclick="window.ewtDeleteNode('${nodeId}')" title="Delete">✕</button>
                        </div>
                    </div>
                    <div style="display:flex;flex-direction:column;align-items:center;width:100%;padding-top:8px;">
                        ${body}
                    </div>
                    ${addButtons}
                </div>
            </div>`;
    }

    function renderParallelNode(node, agents, connectors, depth, isSelected, nodeId, graph) {
        const m = META.parallel;
        const kids = node.children || [];
        const collapsed = isCollapsed(nodeId);

        const addButtons = collapsed ? '' : `
            <div class="ewt-add-children">
                <button class="ewt-add-btn agent" onclick="window.ewtAddChild('${nodeId}', 'agent')">+ Agent</button>
                <button class="ewt-add-btn sequential" onclick="window.ewtAddChild('${nodeId}', 'sequential')">+ Seq</button>
                <button class="ewt-add-btn parallel" onclick="window.ewtAddChild('${nodeId}', 'parallel')">+ Par</button>
                <button class="ewt-add-btn loop" onclick="window.ewtAddChild('${nodeId}', 'loop')">+ Loop</button>
                <button class="ewt-add-btn conditional" onclick="window.ewtAddChild('${nodeId}', 'conditional')">+ Cond</button>
            </div>`;

        let innerContent = '';
        if (collapsed) {
            innerContent = collapsedSummary(node);
        } else {
            let branches = '';
            for (let i = 0; i < kids.length; i++) {
                branches += `
                    <div class="ewt-branch">
                        <div class="ewt-branch-connector" style="background:${m.color};"></div>
                        ${renderPipelineNode(kids[i], agents, connectors, depth + 1, graph)}
                        <div class="ewt-branch-connector" style="background:${m.color};"></div>
                    </div>`;
            }
            if (kids.length === 0) branches = `<div class="ewt-empty">Drop agents here or click + to add</div>`;
            innerContent = `
                <div class="ewt-fork-join" style="--ewt-color:${m.color};"><div class="ewt-fork-join-center" style="background:${m.color};"></div></div>
                <div class="ewt-branches">${branches}</div>
                <div class="ewt-fork-join" style="--ewt-color:${m.color};"><div class="ewt-fork-join-center" style="background:${m.color};"></div></div>`;
        }

        return `
            <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                <div class="ewt-node ${isSelected ? 'selected' : ''}"
                     style="--ewt-color:${m.color};--ewt-color-muted:${m.colorMuted};--ewt-color-fg:${m.colorFg};max-width:unset;"
                     data-node-id="${nodeId}"
                     data-node-type="parallel"
                     draggable="true">
                    <div style="display:flex;align-items:center;gap:8px;">
                        ${collapseChevron(nodeId)}
                        <div class="ewt-badge" style="background:${m.colorMuted};color:${m.colorFg};">
                            ${m.icon}&nbsp;Parallel
                        </div>
                        <input type="text" class="ewt-inline-edit" value="${esc(node.label)}" placeholder="Parallel label..." style="flex:1;" data-field="label" data-node-id="${nodeId}" />
                        <div class="ewt-actions">
                            <button class="ewt-action-btn" onclick="window.ewtMoveUp('${nodeId}')" title="Move up">↑</button>
                            <button class="ewt-action-btn" onclick="window.ewtMoveDown('${nodeId}')" title="Move down">↓</button>
                            <button class="ewt-action-btn danger" onclick="window.ewtDeleteNode('${nodeId}')" title="Delete">✕</button>
                        </div>
                    </div>
                </div>
                ${innerContent}
                ${addButtons}
            </div>`;
    }

    function renderLoopNode(node, agents, connectors, depth, isSelected, nodeId, graph) {
        const m = META.loop;
        const kids = node.children || [];
        const maxIter = node.max_iterations || 3;
        const collapsed = isCollapsed(nodeId);

        let body = '';
        if (!collapsed) {
            for (let i = 0; i < kids.length; i++) {
                body += renderPipelineNode(kids[i], agents, connectors, depth + 1, graph);
                if (i < kids.length - 1) body += renderConnector(m.color);
            }
            if (kids.length === 0) body = `<div class="ewt-empty">Drop agents here or click + to add</div>`;
        } else {
            body = collapsedSummary(node);
        }

        const loopArrow = collapsed ? '' : `
            <div class="ewt-loop-return">
                <svg width="80" height="18" viewBox="0 0 80 18" fill="none">
                    <path d="M10 2 C10 14, 70 14, 70 2" stroke="${m.color}" stroke-width="1.5" stroke-dasharray="4 3" fill="none" opacity="0.45"/>
                    <path d="M67 5 L70 1 L73 5" stroke="${m.color}" stroke-width="1.5" fill="none" opacity="0.45" stroke-linecap="round"/>
                </svg>
            </div>`;

        const addButtons = collapsed ? '' : `
            <div class="ewt-add-children">
                <button class="ewt-add-btn agent" onclick="window.ewtAddChild('${nodeId}', 'agent')">+ Agent</button>
                <button class="ewt-add-btn sequential" onclick="window.ewtAddChild('${nodeId}', 'sequential')">+ Seq</button>
                <button class="ewt-add-btn parallel" onclick="window.ewtAddChild('${nodeId}', 'parallel')">+ Par</button>
                <button class="ewt-add-btn loop" onclick="window.ewtAddChild('${nodeId}', 'loop')">+ Loop</button>
                <button class="ewt-add-btn conditional" onclick="window.ewtAddChild('${nodeId}', 'conditional')">+ Cond</button>
            </div>`;

        return `
            <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                <div class="ewt-loop-wrap" style="--ewt-color:${m.color};--ewt-color-muted:${m.colorMuted};--ewt-color-fg:${m.colorFg};" draggable="true">
                    <div class="ewt-loop-badge" style="background:var(--background);color:${m.colorFg};border-color:${m.colorMuted};">
                        ${collapseChevron(nodeId)}
                        ${m.icon}&nbsp;Loop (×<input type="number" value="${maxIter}" min="1" max="100" style="width:24px;background:transparent;border:none;color:${m.colorFg};font-size:9px;font-weight:700;text-align:center;outline:none;" data-field="max_iterations" data-node-id="${nodeId}" />)
                    </div>
                    <div style="display:flex;align-items:center;gap:8px;margin-bottom:4px;">
                        <input type="text" class="ewt-inline-edit" value="${esc(node.label)}" placeholder="Loop label..." style="flex:1;" data-field="label" data-node-id="${nodeId}" />
                        <div class="ewt-actions" style="opacity:1;">
                            <button class="ewt-action-btn" onclick="window.ewtMoveUp('${nodeId}')" title="Move up">↑</button>
                            <button class="ewt-action-btn" onclick="window.ewtMoveDown('${nodeId}')" title="Move down">↓</button>
                            <button class="ewt-action-btn danger" onclick="window.ewtDeleteNode('${nodeId}')" title="Delete">✕</button>
                        </div>
                    </div>
                    <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                        ${body}
                    </div>
                    ${loopArrow}
                </div>
                ${addButtons}
            </div>`;
    }

    function renderConditionalNode(node, agents, connectors, depth, isSelected, nodeId, graph) {
        const m = META.conditional;
        const kids = node.children || [];

        // First child is the evaluator; remaining are branches with conditions
        let evaluatorHtml = '';
        let branchesHtml = '';

        if (kids.length > 0) {
            const evaluator = kids[0];
            evaluatorHtml = `
                <div style="display:flex;flex-direction:column;align-items:center;width:100%;margin:0 auto;">
                    <div class="ewt-badge" style="background:rgba(220,38,38,0.08);color:#fca5a5;font-size:9px;margin-bottom:4px;">
                        <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg>
                        &nbsp;Evaluator (first child must have output_key)
                    </div>
                    ${renderPipelineNode(evaluator, agents, connectors, depth + 1, graph)}
                </div>`;
        }

        if (kids.length > 1) {
            branchesHtml = '<div class="ewt-branches">';
            for (let i = 1; i < kids.length; i++) {
                const branch = kids[i];
                const condLabel = branch.condition || '???';
                const isFallback = condLabel === '*';
                branchesHtml += `
                    <div class="ewt-branch">
                        <div class="ewt-branch-connector" style="background:${m.color};"></div>
                        <div class="ewt-badge" style="background:rgba(220,38,38,0.08);color:${isFallback ? '#9ca3af' : '#fca5a5'};font-size:9px;margin-bottom:4px;">
                            ${isFallback ? 'fallback' : 'condition: ' + esc(condLabel)}
                        </div>
                        ${renderPipelineNode(branch, agents, connectors, depth + 1, graph)}
                        <div class="ewt-branch-connector" style="background:${m.color};"></div>
                    </div>`;
            }
            branchesHtml += '</div>';
        }

        if (kids.length === 0) {
            evaluatorHtml = '<div class="ewt-empty">Add an evaluator agent and branches</div>';
        }

        const forkJoin = kids.length > 1 ? `
            <div class="ewt-fork-join" style="--ewt-color:${m.color};">
                <div class="ewt-fork-join-center" style="background:${m.color};"></div>
            </div>` : '';

        const addButtons = `
            <div class="ewt-add-children">
                <button class="ewt-add-btn agent" onclick="window.ewtAddChild('${nodeId}', 'agent')">+ Branch</button>
                <button class="ewt-add-btn conditional" onclick="window.ewtAddConditionalBranch('${nodeId}')">+ Conditional Branch</button>
            </div>`;

        return `
            <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                <div class="ewt-node ${isSelected ? 'selected' : ''}" 
                     style="--ewt-color:${m.color};--ewt-color-muted:${m.colorMuted};--ewt-color-fg:${m.colorFg};max-width:unset;"
                     data-node-id="${nodeId}"
                     data-node-type="conditional"
                     draggable="true">
                    <div style="display:flex;align-items:center;gap:8px;">
                        <div class="ewt-badge" style="background:${m.colorMuted};color:${m.colorFg};">
                            ${m.icon}&nbsp;Conditional
                        </div>
                        <input type="text" 
                               class="ewt-inline-edit" 
                               value="${esc(node.label)}" 
                               placeholder="Conditional label..."
                               style="flex:1;"
                               data-field="label"
                               data-node-id="${nodeId}" />
                        <div class="ewt-actions">
                            <button class="ewt-action-btn" onclick="window.ewtMoveUp('${nodeId}')" title="Move up">↑</button>
                            <button class="ewt-action-btn" onclick="window.ewtMoveDown('${nodeId}')" title="Move down">↓</button>
                            <button class="ewt-action-btn danger" onclick="window.ewtDeleteNode('${nodeId}')" title="Delete">✕</button>
                        </div>
                    </div>
                    <div style="display:flex;flex-direction:column;align-items:center;width:100%;padding-top:8px;">
                        ${evaluatorHtml}
                        ${forkJoin}
                        ${branchesHtml}
                        ${forkJoin}
                    </div>
                    ${addButtons}
                </div>
            </div>`;
    }

    function renderPipelineNode(node, agents, connectors, depth, graph) {
        if (!node) return '';
        const m = META[node.type] || META.agent;
        const isSelected = graph._selectedId === node.id;

        switch (node.type) {
            case 'agent': return renderAgentNode(node, agents, isSelected, node.id);
            case 'remote': return renderRemoteNode(node, connectors, isSelected, node.id);
            case 'sequential': return renderSequentialNode(node, agents, connectors, depth, isSelected, node.id, graph);
            case 'parallel': return renderParallelNode(node, agents, connectors, depth, isSelected, node.id, graph);
            case 'loop': return renderLoopNode(node, agents, connectors, depth, isSelected, node.id, graph);
            case 'conditional': return renderConditionalNode(node, agents, connectors, depth, isSelected, node.id, graph);
            default: return renderAgentNode(node, agents, isSelected, node.id);
        }
    }

    // ── Tree helpers ────────────────────────────────────────────────

    function findNodeById(node, id) {
        if (!node) return null;
        if (node.id === id) return node;
        for (const child of (node.children || [])) {
            const found = findNodeById(child, id);
            if (found) return found;
        }
        return null;
    }

    function findParentAndIndex(node, id, parent = null) {
        if (!node) return null;
        if (node.id === id) return { parent, index: -1 };
        for (let i = 0; i < (node.children || []).length; i++) {
            if (node.children[i].id === id) return { parent: node, index: i };
            const found = findParentAndIndex(node.children[i], id, node);
            if (found) return found;
        }
        return null;
    }

    function deleteNodeFromTree(graph, id) {
        if (!graph || id === graph.id) return false;
        const info = findParentAndIndex(graph, id);
        if (!info || !info.parent) return false;
        info.parent.children.splice(info.index, 1);
        return true;
    }

    function addChildToNode(graph, parentId, childType) {
        const parent = findNodeById(graph, parentId);
        if (!parent) return false;
        if (!parent.children) parent.children = [];
        parent.children.push(createNode(childType));
        return true;
    }

    function moveNodeUp(graph, id) {
        const info = findParentAndIndex(graph, id);
        if (!info || !info.parent || info.index <= 0) return false;
        const children = info.parent.children;
        [children[info.index - 1], children[info.index]] = [children[info.index], children[info.index - 1]];
        return true;
    }

    function moveNodeDown(graph, id) {
        const info = findParentAndIndex(graph, id);
        if (!info || !info.parent) return false;
        const children = info.parent.children;
        if (info.index >= children.length - 1) return false;
        [children[info.index], children[info.index + 1]] = [children[info.index + 1], children[info.index]];
        return true;
    }

    function moveNode(graph, sourceId, targetParentId, targetIndex) {
        // Remove source node from its parent
        const sourceInfo = findParentAndIndex(graph, sourceId);
        if (!sourceInfo || !sourceInfo.parent) return false;
        const sourceParent = sourceInfo.parent;
        const sourceIndex = sourceInfo.index;
        const node = sourceParent.children[sourceIndex];
        // Remove from source parent
        sourceParent.children.splice(sourceIndex, 1);
        // Find target parent
        const targetParent = targetParentId ? findNodeById(graph, targetParentId) : null;
        if (!targetParent) return false;
        if (!targetParent.children) targetParent.children = [];
        // Adjust targetIndex if moving within same parent and sourceIndex < targetIndex
        let adjustedIndex = targetIndex;
        if (sourceParent === targetParent && sourceIndex < targetIndex) {
            adjustedIndex = targetIndex - 1;
        }
        // Insert at target index
        targetParent.children.splice(adjustedIndex, 0, node);
        return true;
    }

    // ── Alpine component ────────────────────────────────────────────

    return {
        init() {
            injectStyles();

            // Global functions for button clicks
            window.ewtDeleteNode = (id) => {
                const store = Alpine.store('flow');
                if (store && store.graph && deleteNodeFromTree(store.graph, id)) {
                    if (store.selectedId === id) store.selectedId = null;
                    this.render();
                    // Sync back to component state
                    if (window.__ewtOnChange) window.__ewtOnChange(store.graph);
                }
            };

            window.ewtEditAgent = async (agentId) => {
                if (!agentId) return;
                // Close the workflow editor and open the agent editor for this workflow
                const store = Alpine.store('flow');
                if (store && store.closeFlowView) {
                    store.closeFlowView();
                }
                // Dispatch event to open agent designer with this workflow
                window.dispatchEvent(new CustomEvent('open-agent-designer-with-id', { 
                    detail: { agentId: agentId } 
                }));
            };

            window.ewtAddChild = (parentId, childType) => {
                const store = Alpine.store('flow');
                if (store && store.graph && addChildToNode(store.graph, parentId, childType)) {
                    this.render();
                    if (window.__ewtOnChange) window.__ewtOnChange(store.graph);
                }
            };

            window.ewtMoveUp = (id) => {
                const store = Alpine.store('flow');
                if (store && store.graph && moveNodeUp(store.graph, id)) {
                    this.render();
                    if (window.__ewtOnChange) window.__ewtOnChange(store.graph);
                }
            };

            window.ewtMoveDown = (id) => {
                const store = Alpine.store('flow');
                if (store && store.graph && moveNodeDown(store.graph, id)) {
                    this.render();
                    if (window.__ewtOnChange) window.__ewtOnChange(store.graph);
                }
            };

            window.ewtDuplicate = (id) => {
                const store = Alpine.store('flow');
                if (store && store.graph) {
                    store.duplicateNode(id);
                    this.render();
                    if (window.__ewtOnChange) window.__ewtOnChange(store.graph);
                }
            };

            window.ewtAddConditionalBranch = (parentId) => {
                const condition = prompt('Enter condition value (use * for fallback):', '');
                if (condition === null) return; // user cancelled
                const store = Alpine.store('flow');
                if (store && store.graph) {
                    const parent = findNodeById(store.graph, parentId);
                    if (parent) {
                        if (!parent.children) parent.children = [];
                        const newNode = createNode('agent');
                        newNode.condition = condition || '*';
                        parent.children.push(newNode);
                        this.render();
                        if (window.__ewtOnChange) window.__ewtOnChange(store.graph);
                    }
                }
            };

            // Event delegation for inline edits and selection
            this.$el.addEventListener('change', (e) => {
                const field = e.target.dataset.field;
                const nodeId = e.target.dataset.nodeId;
                const store = Alpine.store('flow');
                if (field && nodeId && store && store.graph) {
                    const node = findNodeById(store.graph, nodeId);
                    if (node) {
                        let value = e.target.value;
                        if (field === 'max_iterations') value = parseInt(value) || 1;
                        node[field] = value;
                        if (window.__ewtOnChange) window.__ewtOnChange(store.graph);
                    }
                }
            });

            this.$el.addEventListener('input', (e) => {
                const field = e.target.dataset.field;
                const nodeId = e.target.dataset.nodeId;
                const store = Alpine.store('flow');
                if (field && nodeId && field !== 'agent_id' && field !== 'a2a_connector_id' && store && store.graph) {
                    const node = findNodeById(store.graph, nodeId);
                    if (node) {
                        node[field] = e.target.value;
                        if (window.__ewtOnChange) window.__ewtOnChange(store.graph);
                    }
                }
            });

            this.$el.addEventListener('click', (e) => {
                const nodeEl = e.target.closest('[data-node-id]');
                if (nodeEl) {
                    const store = Alpine.store('flow');
                    if (store) {
                        store.selectedId = nodeEl.dataset.nodeId;
                        this.render();
                    }
                }
            });

            // Drag and drop
            this.$el.addEventListener('dragstart', (e) => {
                const nodeEl = e.target.closest('[data-node-id]');
                if (nodeEl) {
                    e.dataTransfer.setData('ewt-reorder', nodeEl.dataset.nodeId);
                    e.dataTransfer.effectAllowed = 'move';
                    // Add dragging class for visual feedback
                    nodeEl.style.opacity = '0.5';
                }
            });

            this.$el.addEventListener('dragend', (e) => {
                const nodeEl = e.target.closest('[data-node-id]');
                if (nodeEl) {
                    nodeEl.style.opacity = '';
                }
                // Remove drop-target class from any element
                const dropTarget = this.$el.querySelector('.drop-target');
                if (dropTarget) dropTarget.classList.remove('drop-target');
            });

            this.$el.addEventListener('dragover', (e) => {
                e.preventDefault();
                const dropZone = e.target.closest('.ewt-node, .ewt-loop-wrap, .ewt-drop-zone');
                if (dropZone) {
                    dropZone.classList.add('drop-target');
                }
            });

            this.$el.addEventListener('dragleave', (e) => {
                const dropZone = e.target.closest('.ewt-node, .ewt-loop-wrap, .ewt-drop-zone');
                if (dropZone) {
                    dropZone.classList.remove('drop-target');
                }
            });

            this.$el.addEventListener('drop', (e) => {
                e.preventDefault();
                const dropZone = e.target.closest('[data-node-id]');
                const store = Alpine.store('flow');
                if (dropZone && store && store.graph) {
                    dropZone.classList.remove('drop-target');
                    const targetId = dropZone.dataset.nodeId;
                    const targetNode = findNodeById(store.graph, targetId);
                    if (!targetNode) return;
                    
                    // Check if this is a reorder drop
                    const sourceId = e.dataTransfer.getData('ewt-reorder');
                    if (sourceId) {
                        // Reorder: move source node to target parent
                        // Determine target parent and index
                        let targetParentId = targetId;
                        let targetIndex = 0;
                        // If target node is composite (has children), add as last child
                        if (targetNode.children) {
                            targetParentId = targetId;
                            targetIndex = targetNode.children.length;
                        } else {
                            // Target is leaf: insert as sibling after target
                            const targetInfo = findParentAndIndex(store.graph, targetId);
                            if (targetInfo && targetInfo.parent) {
                                targetParentId = targetInfo.parent.id;
                                targetIndex = targetInfo.index + 1;
                            } else {
                                return; // cannot move root?
                            }
                        }
                        if (moveNode(store.graph, sourceId, targetParentId, targetIndex)) {
                            this.render();
                            if (window.__ewtOnChange) window.__ewtOnChange(store.graph);
                        }
                        return;
                    }
                    
                    // Palette drop: add new node as child
                    const nodeType = e.dataTransfer.getData('flowNodeType');
                    if (nodeType) {
                        const parent = targetNode;
                        if (parent && parent.children) {
                            const newNode = createNode(nodeType);
                            if (nodeType === 'agent') {
                                newNode.agent_id = e.dataTransfer.getData('flowAgentId') || '';
                                newNode.label = e.dataTransfer.getData('flowAgentName') || '';
                            } else if (nodeType === 'remote') {
                                newNode.a2a_connector_id = e.dataTransfer.getData('flowConnectorId') || '';
                            }
                            parent.children.push(newNode);
                            this.render();
                            if (window.__ewtOnChange) window.__ewtOnChange(store.graph);
                        }
                    }
                }
            });

            // Keyboard shortcuts
            this.$el.addEventListener('keydown', (e) => {
                // Ignore if focus is on an input element
                if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA' || e.target.tagName === 'SELECT') return;
                const store = Alpine.store('flow');
                if (!store) return;
                const selectedId = store.selectedId;
                
                if (e.key === 'Delete' || e.key === 'Backspace') {
                    if (selectedId) {
                        e.preventDefault();
                        window.ewtDeleteNode(selectedId);
                    }
                } else if (e.key === 'ArrowUp' && e.ctrlKey) {
                    if (selectedId) {
                        e.preventDefault();
                        window.ewtMoveUp(selectedId);
                    }
                } else if (e.key === 'ArrowDown' && e.ctrlKey) {
                    if (selectedId) {
                        e.preventDefault();
                        window.ewtMoveDown(selectedId);
                    }
                } else if (e.key === 'z' && (e.ctrlKey || e.metaKey) && !e.shiftKey) {
                    e.preventDefault();
                    store.undo();
                } else if ((e.key === 'y' && (e.ctrlKey || e.metaKey)) || (e.key === 'z' && (e.ctrlKey || e.metaKey) && e.shiftKey)) {
                    e.preventDefault();
                    store.redo();
                } else if (e.key === 'd' && (e.ctrlKey || e.metaKey)) {
                    if (selectedId) {
                        e.preventDefault();
                        store.duplicateNode(selectedId);
                        this.render();
                        if (window.__ewtOnChange) window.__ewtOnChange(store.graph);
                    }
                } else if (e.key === 'ArrowLeft' && selectedId && !e.ctrlKey) {
                    const node = findNodeById(store.graph, selectedId);
                    if (node && node.children && node.children.length > 0 && !store.isCollapsed(selectedId)) {
                        e.preventDefault();
                        store.toggleCollapse(selectedId);
                    }
                } else if (e.key === 'ArrowRight' && selectedId && !e.ctrlKey) {
                    if (store.isCollapsed(selectedId)) {
                        e.preventDefault();
                        store.toggleCollapse(selectedId);
                    }
                } else if (e.key === '?') {
                    store._showHelp = !store._showHelp;
                } else if (e.key === 'Escape' && store._showHelp) {
                    store._showHelp = false;
                }
            });

            // Listen for workflow execution tracking from Test Panel or Main Chat
            window.addEventListener('test-panel-workflow-tracking', (e) => {
                const { active, completed, failed } = e.detail;
                const hasAnyActive = Object.keys(active).length > 0;
                
                // Manually apply global tracking CSS classes without full Alpine re-render
                this.$el.querySelectorAll('.ewt-node[data-node-type="agent"], .ewt-node[data-node-type="remote"]').forEach(el => {
                    const agentName = el.getAttribute('data-agent-name');
                    const nodeLabel = el.getAttribute('data-node-label');
                    
                    const nameToMatch1 = agentName || '';
                    const nameToMatch2 = nodeLabel || '';

                    const isActive = !!(active[nameToMatch1] || active[nameToMatch2]);
                    const isCompleted = !!(completed[nameToMatch1] || completed[nameToMatch2]);
                    const isFailed = !!(failed[nameToMatch1] || failed[nameToMatch2]);
                    const isInactive = !isActive && !isCompleted && !isFailed && hasAnyActive;
                    
                    el.classList.toggle('node-active', isActive);
                    el.classList.toggle('node-completed', isCompleted);
                    el.classList.toggle('node-failed', isFailed);
                    el.classList.toggle('node-inactive', isInactive);
                });
            });

            // Set up observer to re-render when store changes
            this._effect = Alpine.effect(() => {
                const store = Alpine.store('flow');
                if (store) {
                    // Access reactive properties to trigger re-render
                    void store.graph;
                    void store.selectedId;
                    void store._collapsed;
                    this.render();
                }
            });

            // Initial render after a tick
            this.$nextTick(() => this.render());
        },

        render() {
            const container = this.$el.querySelector('.ewt-pipeline-canvas');
            if (!container) return;

            const store = Alpine.store('flow');
            if (!store || !store.graph) {
                container.innerHTML = '<div class="ewt-empty">No workflow graph. Add a root node to start.</div>';
                return;
            }

            // Pass selectedId through the graph object for the renderer
            const graphWithSelection = { ...store.graph, _selectedId: store.selectedId };

            container.innerHTML = `
                <div style="display:flex;flex-direction:column;align-items:center;width:100%;">
                    ${renderPipelineNode(graphWithSelection, store.agents || [], store.connectors || [], 0, store)}
                </div>`;
        },

        // Compatibility methods called by agent-designer-dialog
        setGraph(graph) {
            // Already handled by Alpine store
            this.$nextTick(() => this.render());
        },

        setAgents(agents) {
            // Already handled by Alpine store
        },

        setConnectors(connectors) {
            // Already handled by Alpine store
        },

        onChange(callback) {
            // Store callback for syncing changes back to the component
            window.__ewtOnChange = callback;
        },

        // Public API for external use
        addRootNode(type) {
            if (this.graph) return;
            this.graph = createNode(type);
            this.render();
            this.notifyChange();
        },

        deleteSelectedNode() {
            if (this._selectedId && this.graph && deleteNodeFromTree(this.graph, this._selectedId)) {
                this._selectedId = null;
                this.render();
                this.notifyChange();
            }
        },

        getSelectedNode() {
            if (!this._selectedId || !this.graph) return null;
            return findNodeById(this.graph, this._selectedId);
        },
    };
};
