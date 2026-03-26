/**
 * Workflow Graph Panel
 *
 * Floating execution panel that appears when a workflow agent runs.
 * Shows the pipeline graph with live active-node highlighting.
 *
 * Lifecycle:
 *   - Appears as soon as streaming starts for a workflow agent (has .graph)
 *   - Highlights the currently executing node via activeAgentName
 *   - Shows step/iteration progress during sequential or loop workflows
 *   - Stays visible after completion until manually dismissed
 */
// ── Panel CSS (injected once, mirrors workflow-tree-visualizer pattern) ────────

function injectPanelStyles() {
    if (document.getElementById('wf-panel-styles')) return;
    const style = document.createElement('style');
    style.id = 'wf-panel-styles';
    style.textContent = `
        /* ── Workflow panel semantic tokens ── */
        .wf-panel {
            /* Live / running state */
            --panel-live:        oklch(0.55 0.20 260);
            --panel-live-bg:     oklch(0.55 0.20 260 / 10%);
            --panel-live-border: oklch(0.55 0.20 260 / 28%);
            /* Done / completed state */
            --panel-done:        oklch(0.52 0.17 160);
            --panel-done-bg:     oklch(0.52 0.17 160 / 10%);
            --panel-done-border: oklch(0.52 0.17 160 / 28%);
            /* Progress bar gradients */
            --panel-bar-seq:     linear-gradient(to right, oklch(0.55 0.20 260), oklch(0.62 0.16 220));
            --panel-bar-loop:    linear-gradient(to right, oklch(0.58 0.20 300), oklch(0.55 0.20 260));
            /* Accent bar at top */
            --panel-accent-bar:  linear-gradient(to right, oklch(0.55 0.20 260), oklch(0.62 0.16 230), oklch(0.68 0.15 200));
            --panel-done-bar:    linear-gradient(to right, oklch(0.52 0.17 160), oklch(0.58 0.16 175), oklch(0.64 0.15 200));
        }
        .dark .wf-panel {
            --panel-live:        oklch(0.72 0.17 260);
            --panel-live-bg:     oklch(0.72 0.17 260 / 14%);
            --panel-live-border: oklch(0.72 0.17 260 / 32%);
            --panel-done:        oklch(0.72 0.15 160);
            --panel-done-bg:     oklch(0.72 0.15 160 / 14%);
            --panel-done-border: oklch(0.72 0.15 160 / 32%);
            --panel-bar-seq:     linear-gradient(to right, oklch(0.72 0.17 260), oklch(0.74 0.14 230));
            --panel-bar-loop:    linear-gradient(to right, oklch(0.74 0.16 300), oklch(0.72 0.17 260));
            --panel-accent-bar:  linear-gradient(to right, oklch(0.72 0.17 260), oklch(0.74 0.14 235), oklch(0.76 0.12 205));
            --panel-done-bar:    linear-gradient(to right, oklch(0.72 0.15 160), oklch(0.74 0.13 178), oklch(0.76 0.12 205));
        }

        /* ── Pulsing dot for live badge ── */
        @keyframes wfPanelPulse {
            0%, 100% { opacity: 1; transform: scale(1); }
            50%       { opacity: 0.5; transform: scale(0.75); }
        }
        .wf-panel-pulse {
            animation: wfPanelPulse 1.8s ease-in-out infinite;
        }
    `;
    document.head.appendChild(style);
}

export function workflowGraphPanel() {
    return {
        graph: null,
        agentsList: [],
        isLoading: false,
        /** 'idle' | 'running' | 'completed' | 'error' */
        executionStatus: 'idle',
        /** { current, total, iteration, maxIterations, workflowType } | null */
        stepInfo: null,
        orchestratorName: '',
        isMinimized: false,
        /** Tracks which agent id we last loaded so we don't reload on each effect run */
        _loadedForId: null,

        init() {
            injectPanelStyles();
        },

        // ── Public handler called by x-effect in the template ──────────────────

        /**
         * Reactive handler — called whenever any watched value changes.
         * @param {boolean}     isStreaming
         * @param {string|null} orchestratorId    — set after first AGENT_TRANSFER_STARTED
         * @param {string}      selectedAgent     — agent id chosen by user
         * @param {Object|null} currentAgentData  — full agent metadata (includes .graph)
         * @param {Object|null} workflowContext   — activeWorkflowContext from chat state
         * @param {Array}       availableAgents   — AVAILABLE_AGENTS from parent chat scope
         */
        async handleWorkflowChange(isStreaming, orchestratorId, selectedAgent, currentAgentData, workflowContext, availableAgents) {
            // Always sync the full agents list from the parent scope so the visualizer
            // can resolve every agent_id → name (prevents "Unknown Agent" nodes).
            if (availableAgents && availableAgents.length > 0) {
                this.agentsList = availableAgents;
            }

            if (isStreaming) {
                this.executionStatus = 'running';

                // Update step / iteration progress indicator
                if (workflowContext && (workflowContext.stepCount > 1 || workflowContext.maxIterations > 1)) {
                    this.stepInfo = {
                        current: (workflowContext.stepIndex || 0) + 1,
                        total: workflowContext.stepCount || null,
                        iteration: workflowContext.iteration || null,
                        maxIterations: workflowContext.maxIterations || null,
                        workflowType: workflowContext.workflowType,
                    };
                }

                // Determine which agent's graph to show.
                // orchestratorId is set on first transfer; before that use the
                // selected agent — which IS the root orchestrator that just started.
                const targetId = orchestratorId ||
                    (currentAgentData?.graph ? (currentAgentData.id || selectedAgent) : null);

                if (!targetId) return;

                // Avoid redundant reloads — only re-fetch if the orchestrator changed
                if (targetId === this._loadedForId && this.graph) return;
                this._loadedForId = targetId;

                // Fast path: currentAgentData already carries the graph
                if (
                    currentAgentData?.graph &&
                    (currentAgentData.id === targetId || currentAgentData.name === targetId)
                ) {
                    this.graph = currentAgentData.graph;
                    this.orchestratorName = currentAgentData.name || '';
                    return;
                }

                await this.loadGraph(targetId);

            } else if (this.executionStatus === 'running') {
                // Streaming just ended — mark completed, keep panel visible
                this.executionStatus = 'completed';
                this.stepInfo = null;
            }
        },

        // ── Dismiss (user action) ────────────────────────────────────────────────

        dismiss() {
            this.graph = null;
            this.executionStatus = 'idle';
            this.stepInfo = null;
            this.orchestratorName = '';
            this._loadedForId = null;
            this.isMinimized = false;
        },

        // ── Data fetching ────────────────────────────────────────────────────────

        async fetchInitialAgents() {
            try {
                const res = await fetch('/v1/agents');
                if (res.ok) {
                    const data = await res.json();
                    this.agentsList = data.agents || [];
                }
            } catch (err) {
                console.error('[WorkflowPanel] Failed to fetch agents:', err);
            }
        },

        async loadGraph(agentIdentifier) {
            this.isLoading = true;
            this.graph = null;
            try {
                if (!this.agentsList || this.agentsList.length === 0) {
                    await this.fetchInitialAgents();
                }

                let targetId = agentIdentifier;
                const agentMeta = this.agentsList.find(
                    a => a.name === agentIdentifier || a.id === agentIdentifier
                );
                if (agentMeta) {
                    targetId = agentMeta.id;
                    this.orchestratorName = agentMeta.name || '';
                    if (agentMeta.graph) {
                        this.graph = agentMeta.graph;
                        return;
                    }
                }

                if (!targetId) return;

                const res = await fetch('/v1/agents/' + encodeURIComponent(targetId));
                if (res.ok) {
                    const details = await res.json();
                    this.graph = details.graph || null;
                    this.orchestratorName = details.name || this.orchestratorName;
                }
            } catch (err) {
                console.error('[WorkflowPanel] Failed to load graph for:', agentIdentifier, err);
                this.graph = null;
            } finally {
                this.isLoading = false;
            }
        },
    };
}
