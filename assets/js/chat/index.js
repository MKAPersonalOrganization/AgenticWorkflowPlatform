/**
 * Chat Component — Main Entry Point
 *
 * Composes all chat functionality into a single Alpine.data('chat') component.
 *
 * Architecture (follows Alpine.js idioms):
 * - Alpine.plugin(chatPlugin) registers stores and debug utilities
 * - Alpine.data('chat', factory) registers the component
 * - Factory returns state + method mixins (spread), composed via JS modules
 * - init() sets up observers/watchers, destroy() tears them down
 * - $watch returns unwatch functions collected for destroy() cleanup
 * - Reactive state (messages, sessions, etc.) drives UI automatically
 *
 * Module structure:
 * - constants.js: Event types and configuration (pure data)
 * - markdown-renderer.js: Markdown → HTML pipeline (pure function)
 * - chat-utils.js: Utilities (pure functions, no Alpine dependency)
 * - chat-state.js: State initialization and thread management
 * - chat-sessions.js: Session CRUD
 * - chat-messaging.js: Message handling and AG-UI conversion
 * - chat-streaming.js: SSE streaming and event dispatch
 * - chat-ui.js: UI actions, toasts, clipboard, scroll
 * - chat-keyboard.js: Global keyboard shortcuts
 * - chat-theme.js: Theme management (light/dark/system)
 * - chat-accessibility.js: Screen reader ARIA support
 * - chat-health.js: Connection health monitoring
 * - alpine-extensions.js: Alpine.plugin for stores and debug
 */

// Import all modules
import { renderMarkdown } from './markdown-renderer.js';
import { throttle, formatSessionTime, formatTime, log, createErrorHandlers, isValidJSON, formatNumber } from './chat-utils.js';
import { createInitialState, createStateMethods, createEmptyThreadState } from './chat-state.js';
import { createSessionMethods } from './chat-sessions.js';
import { createMessagingMethods } from './chat-messaging.js';
import { createStreamingMethods } from './chat-streaming.js';
import { createUIMethods } from './chat-ui.js';
import { createKeyboardMethods } from './chat-keyboard.js';
import { createThemeMethods } from './chat-theme.js';
import { createAccessibilityMethods } from './chat-accessibility.js';
import { createHealthMethods } from './chat-health.js';
import { createToastMethods } from './chat-toasts.js';
import { createScrollMethods } from './chat-scroll.js';
import { createEventLogMethods } from './chat-event-log.js';
import { createActivityPanelMethods } from './chat-activity-panel.js';
import { createWorkflowStatusMethods } from './chat-workflow-status.js';
import { createInputMethods } from './chat-input.js';
import { createProfileMethods } from './chat-profile.js';
import { createEventHandlerMethods } from './chat-event-handlers.js';
import { createDebuggerMethods } from './chat-debugger.js';
import { chatPlugin } from './alpine-extensions.js';

/**
 * Register the chat component and extensions.
 *
 * Alpine.js idiom: use a single alpine:init listener to register all
 * extensions (plugins, stores, data components) in one place.
 * Alpine.plugin() receives the Alpine instance — it's the canonical
 * entry point for extensions (see packages/alpinejs/src/plugin.js).
 */
document.addEventListener('alpine:init', () => {
    // Register plugin first — stores must exist before component init()
    Alpine.plugin(chatPlugin);

    Alpine.data('chat', () => {
        // Create initial state
        const state = createInitialState();

        // Compose all methods from modules
        const stateMethods = createStateMethods();
        const sessionMethods = createSessionMethods();
        const messagingMethods = createMessagingMethods();
        const streamingMethods = createStreamingMethods();
        const uiMethods = createUIMethods();
        const keyboardMethods = createKeyboardMethods();
        const themeMethods = createThemeMethods();
        const accessibilityMethods = createAccessibilityMethods();
        const healthMethods = createHealthMethods();
        const toastMethods = createToastMethods();
        const scrollMethods = createScrollMethods();
        const eventLogMethods = createEventLogMethods();
        const activityPanelMethods = createActivityPanelMethods();
        const workflowStatusMethods = createWorkflowStatusMethods();
        const inputMethods = createInputMethods();
        const profileMethods = createProfileMethods();
        const eventHandlerMethods = createEventHandlerMethods();
        const debuggerMethods = createDebuggerMethods();

        return {
            // Spread initial state
            ...state,

            // Spread all method mixins
            ...stateMethods,
            ...sessionMethods,
            ...messagingMethods,
            ...streamingMethods,
            ...uiMethods,
            ...keyboardMethods,
            ...themeMethods,
            ...accessibilityMethods,
            ...healthMethods,
            ...toastMethods,
            ...scrollMethods,
            ...eventLogMethods,
            ...activityPanelMethods,
            ...workflowStatusMethods,
            ...inputMethods,
            ...profileMethods,
            ...eventHandlerMethods,
            ...debuggerMethods,

            // Constants
            AVAILABLE_AGENTS: [],

            // Fetch agents from backend
            async fetchAgents() {
                this.__tryCatchAsync('fetch agents', {}, async () => {
                    const response = await fetch('/v1/agents');
                    if (response.ok) {
                        const data = await response.json();
                        if (data.agents && Array.isArray(data.agents)) {
                            this.AVAILABLE_AGENTS = data.agents;

                            // Fetch the server-side saved preference (cross-device source of truth)
                            let serverAgent = '';
                            try {
                                const prefResp = await fetch('/api/preferences/selected_agent');
                                if (prefResp.ok) {
                                    const prefData = await prefResp.json();
                                    serverAgent = prefData.value || '';
                                }
                            } catch { /* ignore — fall back to localStorage or first agent */ }

                            // Priority: server preference > localStorage cache > first agent
                            const candidate = serverAgent || localStorage.getItem('selectedAgent') || '';
                            const isValid = candidate &&
                                this.AVAILABLE_AGENTS.some(a => a.id === candidate);

                            if (isValid) {
                                this.selectAgent(candidate);
                            } else if (this.AVAILABLE_AGENTS.length > 0) {
                                // Fall back to the first agent (oldest by creation time)
                                this.selectAgent(this.AVAILABLE_AGENTS[0].id);
                            } else {
                                // No agents available — stop the spinner with empty data
                                this.currentAgentData = { id: '', name: 'Agent', description: '', icon: '' };
                            }
                        } else {
                            // Malformed response — stop the spinner
                            this.currentAgentData = { id: '', name: 'Agent', description: '', icon: '' };
                        }
                    } else {
                        // Fetch failed — stop the spinner
                        this.currentAgentData = { id: '', name: 'Agent', description: '', icon: '' };
                    }
                });
            },

            // Explictly select an agent and fetch its data
            async selectAgent(agentId) {
                this.selectedAgent = agentId;
                this.__persistAgentPreference(agentId);

                // Fetch the agent details from the backend
                try {
                    const response = await fetch('/v1/agents/' + agentId);
                    if (response.ok) {
                        const data = await response.json();
                        this.currentAgentData = data;
                    } else {
                        // Fetch failed — set minimal data so the UI doesn't spin forever
                        console.error('Failed to fetch agent details, status:', response.status);
                        this.currentAgentData = { id: agentId, name: 'Agent', description: '', icon: '' };
                    }
                } catch (err) {
                    console.error('Failed to resolve current agent details:', err);
                    this.currentAgentData = { id: agentId, name: 'Agent', description: '', icon: '' };
                }

                // Best-effort design-time analysis. Failures are silent;
                // the Activity panel just won't show red dots until the
                // user re-selects the agent.
                this.__fetchAgentLineage();
            },

            // __fetchAgentLineage runs the static state-flow analyzer
            // for the currently selected agent's workflow graph. The
            // result drives the inline red-dot warnings on Activity-
            // panel leaves whose instructions reference unreachable
            // keys. No-op for non-workflow agents.
            async __fetchAgentLineage() {
                const graph = this.currentAgentData?.graph;
                if (!graph) {
                    this.currentAgentLineage = { issues: [], producible_before: {}, produces: {}, references: {} };
                    return;
                }
                try {
                    const response = await fetch('/v1/agent-definitions/analyze-state-lineage', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ graph }),
                    });
                    if (!response.ok) return;
                    const data = await response.json();
                    this.currentAgentLineage = {
                        issues: Array.isArray(data.issues) ? data.issues : [],
                        producible_before: data.producible_before || {},
                        produces: data.produces || {},
                        references: data.references || {},
                    };
                } catch (err) {
                    console.error('Failed to analyze state lineage:', err);
                }
            },

            // lineageIssuesByAgent buckets the latest analysis result
            // by agent ID so the Activity-panel leaf-render can look up
            // its leaf in O(1). Issues with no agent_id (e.g. a
            // conditional with a missing evaluator output_key on a
            // node-only basis) bucket under '_node:<node_id>' so a
            // separate lookup can surface them.
            get lineageIssuesByAgent() {
                const out = {};
                const issues = this.currentAgentLineage?.issues || [];
                for (const issue of issues) {
                    const k = issue.agent_id || ('_node:' + (issue.node_id || ''));
                    if (!out[k]) out[k] = [];
                    out[k].push(issue);
                }
                return out;
            },

            // Persist agent preference to server and localStorage cache
            __persistAgentPreference(agentId) {
                localStorage.setItem('selectedAgent', agentId);
                fetch('/api/preferences/selected_agent', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ value: agentId })
                }).catch(() => { /* best-effort — localStorage is the fast fallback */ });
            },

            // ============================================
            // Computed Properties (Getters must be defined directly)
            // ============================================
            get messages() {
                return this.__currentState.messages;
            },

            /**
             * Grouped message view for rendering.
             *
             * Pass 1 — Collapse consecutive parallel messages (same parentAgent)
             * into a single parallel_group entry with per-branch arrays so the
             * template can render them as side-by-side columns.
             *
             * Pass 2 — Bucket multi-agent turns into (activity_card, final).
             * A "turn" is everything between consecutive user messages. Within
             * each turn, the *last* assistant-style entry is the final answer
             * and gets the main bubble; everything else (intermediate sequential
             * steps, parallel_groups, workflow_markers, tool calls) is folded
             * into one premium activity_card that the user can expand to see
             * the work. Single-agent turns pass through unchanged.
             */
            get displayMessages() {
                const msgs = this.__currentState.messages;
                if (!msgs || msgs.length === 0) return [];

                // ── Pass 1: collapse parallel sections ──────────────────────
                const flat = [];
                let i = 0;
                while (i < msgs.length) {
                    const msg = msgs[i];
                    if (msg.type === 'workflow_marker' && msg.markerType === 'parallel') {
                        const parentAgent = msg.parentAgent;
                        const stepCount = msg.stepCount || 2;
                        const branches = {};
                        const agentToBranch = {};
                        i++;
                        while (i < msgs.length) {
                            const m = msgs[i];
                            if (m.workflowType === 'parallel' && m.parentAgent === parentAgent) {
                                const branch = m.stepIndex || 0;
                                if (!branches[branch]) branches[branch] = [];
                                branches[branch].push(m);
                                if (m.agentName) agentToBranch[m.agentName] = branch;
                                i++;
                            } else if (m.type === 'tool_call' && m.agentName && m.agentName in agentToBranch) {
                                const branch = agentToBranch[m.agentName];
                                if (!branches[branch]) branches[branch] = [];
                                branches[branch].push(m);
                                i++;
                            } else {
                                break;
                            }
                        }
                        flat.push({
                            id: msg.id,
                            type: 'parallel_group',
                            parentAgent,
                            stepCount,
                            branches,
                            timestamp: msg.timestamp,
                        });
                    } else {
                        flat.push(msg);
                        i++;
                    }
                }

                // ── Pass 2: bucket each turn into activity + final ──────────
                // A turn starts at a user message and runs until the next user
                // message. Within the turn:
                //   • If any message carries workflow context, the whole run
                //     is treated as multi-agent. EVERY workflow message is
                //     folded into an activity_card (which is hidden from the
                //     main thread and only surfaced in the side panel).
                //   • While intermediate agents run, the main thread shows
                //     a `streaming_placeholder` — calm progress instead of
                //     a parade of in-flight agents.
                //   • As soon as the workflow's *final* agent starts
                //     streaming, its message is promoted to the main bubble
                //     and tokens flow into the chat live. The final agent
                //     is identified as the last leaf in workflowGraphRows
                //     (DFS pre-order on the composition tree).
                //   • Single-agent turns (no workflowType anywhere) stream
                //     into the main thread unchanged.
                const isStreaming = !!this.__currentState.isStreaming;
                const isWorkflowArtefact = (m) => (
                    m.type === 'parallel_group' ||
                    m.type === 'workflow_marker' ||
                    m.type === 'tool_call' ||
                    (m.role === 'assistant' && !!m.workflowType)
                );

                // Resolve the workflow's final agent name from the static
                // composition tree (last leaf in DFS pre-order). Used to
                // promote that agent's in-flight message to the main bubble
                // while still streaming. Null for ad-hoc / non-composed
                // agents — those keep the placeholder until the run ends.
                let finalAgentName = null;
                {
                    const rows = this.workflowGraphRows;
                    if (rows && rows.length > 0) {
                        for (let k = rows.length - 1; k >= 0; k--) {
                            if (rows[k].isLeaf && rows[k].agentName) {
                                finalAgentName = rows[k].agentName;
                                break;
                            }
                        }
                    }
                }
                const out = [];
                let j = 0;
                while (j < flat.length) {
                    const head = flat[j];
                    if (head.role !== 'user') {
                        out.push(head);
                        j++;
                        continue;
                    }
                    // Push the user message; then scan its turn.
                    out.push(head);
                    j++;

                    const turnStart = j;
                    while (j < flat.length && flat[j].role !== 'user') j++;
                    const turnEnd = j;

                    // "Streaming" applies only to the LAST turn — older
                    // turns are definitively closed even while a new run
                    // is in flight, so their final bubble must stay put
                    // and not be replaced by a placeholder.
                    const isLastTurn = turnEnd === flat.length;
                    const turnIsStreaming = isStreaming && isLastTurn;

                    // Detect whether this is a multi-agent turn. We define it
                    // as "the turn contains any workflow-tagged message" —
                    // even a single workflow message means the run came from
                    // a composed agent, so the activity surface is the right
                    // place for everything that isn't the final answer.
                    let hasAnyWorkflow = false;
                    for (let k = turnStart; k < turnEnd; k++) {
                        if (isWorkflowArtefact(flat[k])) { hasAnyWorkflow = true; break; }
                    }

                    // Single-agent fast path: pass through unchanged.
                    if (!hasAnyWorkflow) {
                        for (let k = turnStart; k < turnEnd; k++) out.push(flat[k]);
                        continue;
                    }

                    // Multi-agent turn: pick out the last assistant message
                    // in the turn — for a finished run this *is* the final
                    // answer; for an in-flight run it's the currently-
                    // streaming agent (which may or may not be the final
                    // agent — we promote it to the main bubble only when
                    // its agentName matches finalAgentName).
                    let lastAssistantIdx = -1;
                    for (let k = turnEnd - 1; k >= turnStart; k--) {
                        if (flat[k].role === 'assistant') { lastAssistantIdx = k; break; }
                    }

                    // Decide what to render in the main slot for this turn:
                    //   • finished turn → the last assistant (the final).
                    //   • streaming turn whose last assistant message
                    //     belongs to the workflow's final agent → that
                    //     message streams into the main bubble live, and
                    //     stays there even after its TEXT_MESSAGE_END but
                    //     before RUN_FINISHED (so we don't flip back to a
                    //     placeholder for the tail of the run).
                    //   • streaming turn whose last assistant is an
                    //     intermediate agent → placeholder.
                    const lastMsg = lastAssistantIdx >= 0 ? flat[lastAssistantIdx] : null;
                    const lastIsFinalAgent = !!(
                        lastMsg
                        && finalAgentName
                        && lastMsg.agentName === finalAgentName
                    );
                    const promoteFinalToMain = !turnIsStreaming || lastIsFinalAgent;

                    const activitySteps = [];
                    const nonWorkflowTail = []; // errors/system → keep inline
                    for (let k = turnStart; k < turnEnd; k++) {
                        const m = flat[k];
                        // The promoted-final message is rendered in the
                        // main slot, not inside the activity card.
                        if (promoteFinalToMain && k === lastAssistantIdx) continue;
                        if (isWorkflowArtefact(m)) {
                            activitySteps.push(m);
                        } else {
                            nonWorkflowTail.push(m);
                        }
                    }

                    if (activitySteps.length > 0) {
                        const first = activitySteps[0];
                        const lastStep = activitySteps[activitySteps.length - 1];
                        out.push({
                            id: 'activity-' + head.id,
                            type: 'activity_card',
                            steps: activitySteps,
                            stepCount: activitySteps.length,
                            startedAt: first.timestamp,
                            updatedAt: lastStep.timestamp,
                            timestamp: first.timestamp,
                        });
                    }

                    // Inline non-workflow rows (errors, etc.) come before
                    // the final/placeholder so they aren't hidden under it.
                    for (const m of nonWorkflowTail) out.push(m);

                    if (promoteFinalToMain && lastMsg) {
                        // Stream the final agent's message live into the
                        // main thread (or render the settled answer
                        // post-run). Tag for the attribution footer.
                        out.push({ ...lastMsg, _isFinalOfTurn: true });
                    } else if (turnIsStreaming) {
                        // Waiting for the final agent to start. Show a calm
                        // placeholder so the main thread doesn't churn with
                        // intermediate agent text.
                        out.push({
                            id: 'placeholder-' + head.id,
                            type: 'streaming_placeholder',
                            timestamp: flat[lastAssistantIdx >= 0 ? lastAssistantIdx : turnStart]?.timestamp,
                        });
                    }
                }
                return out;
            },

            get isStreaming() {
                return this.__currentState.isStreaming;
            },

            /**
             * All activity_card entries from displayMessages, oldest → newest.
             * Drives the Activity side panel; intentionally separate from the
             * chat thread so the main bubble stack stays focused on the final
             * answer.
             */
            get activityCards() {
                return this.displayMessages.filter(m => m.type === 'activity_card');
            },

            /** True iff there's at least one activity_card to show. */
            get hasActivity() {
                return this.activityCards.length > 0;
            },

            /**
             * Flat-list view of the current agent's composition graph.
             *
             * The static workflow tree (sequential / parallel / loop /
             * conditional composites with agent leaves) is flattened into
             * one row per node, in pre-order traversal, with `depth` and
             * `isLast` siblings so the template can draw the rail glyphs.
             *
             * Each leaf's `agentName` is resolved by looking up the leaf's
             * `agent_id` in `AVAILABLE_AGENTS` — the same list the agent
             * picker uses. Runtime status comes from `agentStatusFor()`.
             *
             * Returns [] for single-agent (non-composed) runs; the chat
             * thread is already the right surface for those.
             */
            get workflowGraphRows() {
                const graph = this.currentAgentData?.graph;
                if (!graph || !graph.type) return [];
                const rows = [];
                const agentMap = {};
                for (const a of (this.AVAILABLE_AGENTS || [])) {
                    if (a && a.id) agentMap[a.id] = a;
                }
                const walk = (node, depth, isLast, ancestorIsLastChain) => {
                    if (!node || !node.type) return;
                    const isLeaf = node.type === 'agent' || node.type === 'remote';
                    let agentName = null;
                    let outputKey = null;
                    let description = null;
                    if (isLeaf && node.agent_id) {
                        const sub = agentMap[node.agent_id];
                        agentName = sub?.name || null;
                        outputKey = sub?.outputKey || null;
                        description = sub?.description || null;
                    }
                    rows.push({
                        id: node.id || ('node_' + rows.length),
                        depth,
                        kind: node.type, // 'sequential' | 'parallel' | 'loop' | 'conditional' | 'agent' | 'remote'
                        label: agentName || node.label || node.id || node.type,
                        agentName,
                        // agentId lets the Activity panel join the row
                        // against currentAgentLineage.issues — keyed by
                        // agent_id, not node_id — so red-dot warnings
                        // light up on the leaves that own the problem.
                        agentId: node.agent_id || null,
                        outputKey,
                        description,
                        condition: node.condition || null,
                        maxIterations: node.max_iterations || null,
                        isLeaf,
                        isLast,
                        ancestorIsLastChain,
                    });
                    if (Array.isArray(node.children) && node.children.length > 0) {
                        for (let i = 0; i < node.children.length; i++) {
                            const last = i === node.children.length - 1;
                            walk(node.children[i], depth + 1, last, [...ancestorIsLastChain, isLast]);
                        }
                    }
                };
                walk(graph, 0, true, []);
                return rows;
            },

            /** True iff the current agent has a composition graph worth rendering. */
            get hasWorkflowGraph() {
                return this.workflowGraphRows.length > 0;
            },

            /**
             * High-level summary of the current workflow run — drives the
             * hero card on top of the Activity panel.
             *
             * status: 'idle' (nothing started) | 'running' | 'success' | 'error'
             * elapsedMs: live during run, frozen on finish/error
             */
            get workflowRunSummary() {
                const rows = this.workflowGraphRows;
                const leaves = rows.filter(r => r.isLeaf && r.agentName);
                let done = 0, active = 0, error = 0;
                let currentName = null;
                for (const r of leaves) {
                    const s = this.agentStatusFor(r.agentName);
                    if (s === 'done') done++;
                    else if (s === 'active') { active++; currentName = r.agentName; }
                    else if (s === 'error') error++;
                }
                const pending = leaves.length - done - active - error;
                const state = this.__currentState || {};
                let status = 'idle';
                if (error > 0) status = 'error';
                else if (this.isStreaming || active > 0) status = 'running';
                else if (leaves.length > 0 && done === leaves.length) status = 'success';
                else if (done > 0) status = 'success'; // partial-done counts as success when no longer streaming
                // Live-ticking elapsed: while running, use _activityNowTick which
                // updates every second; after end, freeze at runEndedAt.
                let elapsedMs = 0;
                if (state.runStartedAt) {
                    const end = (status === 'running')
                        ? (this._activityNowTick || Date.now())
                        : (state.runEndedAt || Date.now());
                    elapsedMs = Math.max(0, end - state.runStartedAt);
                }
                return {
                    status,
                    total: leaves.length,
                    done, active, error, pending,
                    currentAgentName: currentName,
                    elapsedMs,
                    elapsedLabel: this.__formatElapsed(elapsedMs),
                };
            },

            __formatElapsed(ms) {
                if (!ms || ms <= 0) return '0s';
                if (ms < 1000) return ms + 'ms';
                if (ms < 60000) return (ms / 1000).toFixed(1) + 's';
                const m = Math.floor(ms / 60000);
                const s = Math.floor((ms % 60000) / 1000);
                return `${m}m ${s}s`;
            },

            // Live tick — bumped by an interval started in init() (see below).
            // Drives the elapsed-time display in the Activity hero card.
            _activityNowTick: Date.now(),
            
            get activeAgentName() {
                return this.__currentState.activeAgentName || null;
            },

            get activeAgentNames() {
                return this.__currentState.activeAgentNames || {};
            },
            
            get activeWorkflowContext() {
                return this.__currentState.activeWorkflowContext || null;
            },

            get completedNodes() {
                return this.__currentState.completedNodes || {};
            },

            get failedNodes() {
                return this.__currentState.failedNodes || {};
            },

            get workflowOutputs() {
                return this.__currentState.workflowOutputs || {};
            },

            get events() {
                return this.__currentState.events;
            },

            get errorMessage() {
                return this.__currentState.errorMessage;
            },

            get threadId() {
                return this.__currentThreadId;
            },

            get showTypingIndicator() {
                return this.__currentState.showTypingIndicator;
            },

            get hasMessages() {
                return this.__currentState.messages.length > 0;
            },

            get awaitingHumanInput() {
                return this.__awaitingToolCall !== null;
            },

            get humanInputQuestion() {
                const tc = this.__awaitingToolCall;
                return tc?.parsedResult?.question_text ||
                    tc?.parsedResult?.statement_to_verify || null;
            },

            get humanInputSchema() {
                const tc = this.__awaitingToolCall;
                return tc?.parsedResult?.schema || null;
            },

            get humanInputToolCallId() {
                const tc = this.__awaitingToolCall;
                return tc?.id || null;
            },

            /**
             * Find the most recent tool call message that is currently awaiting input.
             * This is the single source of truth — all human-input getters derive from it.
             * @private
             */
            get __awaitingToolCall() {
                const state = this.__currentState;
                const msgs = state?.messages;
                if (!msgs || msgs.length === 0) return null;

                for (let i = msgs.length - 1; i >= 0; i--) {
                    const m = msgs[i];
                    if (m.type === 'tool_call') {
                        const tc = state.toolCalls?.[m.id];
                        if (tc?.state === 'awaiting_input' || tc?.state === 'awaiting_confirmation') return m;
                    }
                }
                return null;
            },

            get hasEvents() {
                return this.__currentState.events.length > 0;
            },

            get tokenUsage() {
                return this.__currentState.tokenUsage;
            },

            get canSendMessage() {
                const hasContent = this.currentMessage.trim() !== '' || this.files.some(f => f.status === 'ready');
                const uploading = this.files.some(f => f.status === 'uploading');
                return hasContent && !this.isStreaming && !uploading;
            },

            get currentThread() {
                return this.sessions.find(s => s.id === this.__currentThreadId) || null;
            },

            getCurrentSessionTitle() {
                const currentThread = this.currentThread;
                return currentThread ? currentThread.title : 'New Chat';
            },

            getTimeBasedGreeting() {
                const hour = new Date().getHours();
                if (hour < 12) {
                    return 'Good morning!';
                } else if (hour < 18) {
                    return 'Good afternoon!';
                } else {
                    return 'Good evening!';
                }
            },

            /**
             * Extract message date for grouping
             * @param {Object} message - Message object
             * @returns {string} Date string in YYYY-MM-DD format
             */
            getMessageDate(message) {
                if (!message || !message.timestamp) return '';
                return new Date(message.timestamp).toDateString();
            },

            /**
             * Format relative date for message grouping
             * @param {string} dateString - Date string
             * @returns {string} Formatted relative date
             */
            formatRelativeDate(dateString) {
                if (!dateString) return '';

                const date = new Date(dateString);
                const today = new Date();
                const yesterday = new Date(today);
                yesterday.setDate(yesterday.getDate() - 1);

                const isToday = date.toDateString() === today.toDateString();
                const isYesterday = date.toDateString() === yesterday.toDateString();

                if (isToday) {
                    return 'Today';
                } else if (isYesterday) {
                    return 'Yesterday';
                } else {
                    // For older dates, show the actual date
                    return date.toLocaleDateString('en-US', {
                        weekday: 'long',
                        year: 'numeric',
                        month: 'long',
                        day: 'numeric'
                    });
                }
            },

            /**
             * Format time for display
             * @param {string|number} timestamp - Timestamp to format
             * @returns {string} Formatted time string
             */
            formatTime(timestamp) {
                return formatTime(timestamp);
            },

            // ============================================
            // Lifecycle Methods
            // ============================================

            init() {
                // Guard: Alpine guarantees init() is called once per instance,
                // but this protects against edge cases (HMR, re-mount).
                if (this.__initialized) {
                    console.warn('Chat component already initialized');
                    return;
                }
                this.__initialized = true;

                // Create error handlers bound to this component
                const errorHandlers = createErrorHandlers(this);
                this.__tryCatch = errorHandlers.tryCatch.bind(this);
                this.__tryCatchAsync = errorHandlers.tryCatchAsync.bind(this);

                // Initialize theme (reads localStorage, applies to DOM)
                this.__initTheme();

                // Fetch available agents
                this.fetchAgents();

                // Refresh agents when agents are added from the Agent Templates or edited
                window.addEventListener('agents-updated', () => this.fetchAgents());

                // ── DOM Observers (deferred to next tick for DOM readiness) ──
                this.$nextTick(() => {
                    this.__setupScrollObservers();
                    this.__setupIntersectionObserver();
                });

                // ── Async data loading (via microtask to not block init) ──
                queueMicrotask(() => {
                    this.__loadSessions();
                    this.fetchProfile();
                    this.fetchBanner();
                });

                // Initialize accessibility (screen reader live region)
                this.__initScreenReaderAnnouncer();

                // ── Payment success: detect Stripe redirect ──
                // After confirmPayment, Stripe redirects to /?redirect_status=succeeded
                // with additional params: payment_intent, payment_intent_client_secret.
                // We call the server-side verify endpoint as a fallback to confirm
                // the payment. The webhook is the canonical confirmation path.
                this.$nextTick(() => {
                    const params = new URLSearchParams(window.location.search);
                    if (params.get('redirect_status') === 'succeeded' || params.get('payment') === 'success') {
                        const piId = params.get('payment_intent');
                        if (piId) {
                            fetch('/api/billing/verify-payment', {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                credentials: 'same-origin',
                                body: JSON.stringify({ paymentIntent: piId }),
                            }).then(r => {
                                if (r.ok) {
                                    this.showToast('🎉 Payment successful! Your account is now fully activated.', 'success', 6000);
                                } else {
                                    // Webhook will handle confirmation — show optimistic message
                                    this.showToast('🎉 Payment received! Activation may take a moment.', 'success', 6000);
                                }
                            }).catch(() => {
                                this.showToast('🎉 Payment received! Activation may take a moment.', 'success', 6000);
                            });
                        } else {
                            this.showToast('🎉 Payment successful! Your account is now fully activated.', 'success', 6000);
                        }
                        // Clean the URL so a refresh doesn't re-show the toast
                        const url = new URL(window.location);
                        url.searchParams.delete('payment');
                        url.searchParams.delete('payment_intent');
                        url.searchParams.delete('payment_intent_client_secret');
                        url.searchParams.delete('redirect_status');
                        window.history.replaceState({}, '', url.pathname);
                    }
                });

                // Throttled scroll for streaming — direct scrollTop assignment
                // avoids scrollIntoView animation jitter. 150ms matches LibreChat.
                this.__scrollToBottomThrottled = throttle(
                    () => this.__scrollToBottomStreaming(),
                    150
                );

                // ── Watchers ($watch returns unwatch fn — collected for destroy) ──
                this.__watchers.push(
                    this.$watch('messages', (newMessages, oldMessages) => {
                        if (!this.isStreaming) {
                            this.$nextTick(() => {
                                const container = this.$refs?.messageList;
                                if (!container) return;
                                const isNearBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 150;
                                const isMajorUpdate = Math.abs(newMessages.length - oldMessages.length) > 5;
                                if (isNearBottom || isMajorUpdate) {
                                    this.__scrollToBottom({ behavior: 'auto', force: true });
                                }
                            });
                        }
                        if (newMessages.length > oldMessages.length) {
                            const lastMsg = newMessages[newMessages.length - 1];
                            if (lastMsg.role === 'assistant' && !document.hasFocus()) {
                                this.__notifyNewMessage(lastMsg);
                            }
                        }
                    }),

                    this.$watch('__currentThreadId', (newThreadId, oldThreadId) => {
                        if (newThreadId) {
                            this.__abortScroll = false;
                            this.__clearScrollTimeouts();
                            this.__currentState = this.__getThreadState(newThreadId);
                            this.__onThreadSwitch(newThreadId, oldThreadId);
                        }
                    }),

                    this.$watch('errorMessage', (error) => {
                        if (error) {
                            setTimeout(() => {
                                if (this.errorMessage === error) {
                                    this.__currentState.errorMessage = null;
                                }
                            }, 5000);
                        }
                    }),

                    // Theme watcher: centralized side-effect trigger
                    this.$watch('preferences.theme', (newTheme) => {
                        this.applyThemeEffects(newTheme);
                    }),

                    // Persist activeView (sidepanel tab) to localStorage
                    this.$watch('activeView', (newView) => {
                        localStorage.setItem('activeView', newView || '');
                    }),
                );

                // Register keyboard shortcuts
                this.__registerKeyboardShortcuts();

                // Live tick for the Activity panel's elapsed timer. Only
                // bumps state while a run is in flight so we don't churn
                // reactivity on idle threads.
                const activityTick = setInterval(() => {
                    if (this.isStreaming) {
                        this._activityNowTick = Date.now();
                    }
                }, 500);
                this.__cleanupTasks.add(() => clearInterval(activityTick));
            },

            /**
             * Set up MutationObserver and scroll listeners for the message list.
             * Uses AbortController for clean listener removal in destroy().
             * @private
             */
            __setupScrollObservers() {
                const container = this.$refs?.messageList;
                if (!container) return;

                // Shared AbortController for all container event listeners
                const controller = new AbortController();
                this.__cleanupTasks.add(() => controller.abort());

                // MutationObserver: auto-scroll on DOM changes (KaTeX, images)
                const throttledScroll = throttle(() => {
                    if (this.isStreaming || this.__abortScroll) return;
                    const isNearBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 200;
                    if (isNearBottom) {
                        this.__scrollToBottom({ behavior: 'auto' });
                    }
                }, 150);

                this.__mutationObserver = new MutationObserver(() => throttledScroll());
                this.__mutationObserver.observe(container, {
                    childList: true,
                    subtree: true,
                    characterData: true
                });

                // Scroll abort: user scrolled up during streaming — pause auto-scroll
                container.addEventListener('scroll', () => {
                    if (!this.isStreaming) return;
                    const distanceFromBottom = container.scrollHeight - container.scrollTop - container.clientHeight;
                    this.__abortScroll = distanceFromBottom > 150;
                }, { passive: true, signal: controller.signal });

                // Code copy events from markdown code blocks (event delegation)
                container.addEventListener('code-copied', (e) => {
                    if (e.detail?.success) {
                        this.showToast(window.i18n.t('Code copied to clipboard'), 'success', 2000);
                    } else {
                        this.showToast(window.i18n.t('Failed to copy code'), 'error');
                    }
                }, { signal: controller.signal });
            },

            /**
             * Set up IntersectionObserver for scroll-to-bottom button visibility.
             * @private
             */
            __setupIntersectionObserver() {
                const anchor = this.$refs?.messagesEnd;
                if (!anchor) return;

                this.__scrollButtonObserver = new IntersectionObserver(([entry]) => {
                    this.scrolledUp = !entry.isIntersecting;
                }, {
                    root: this.$refs.messageList,
                    threshold: 0.1
                });
                this.__scrollButtonObserver.observe(anchor);
            },

            destroy() {
                // ── Observers ──
                this.__clearScrollTimeouts();
                this.__mutationObserver?.disconnect();
                this.__scrollButtonObserver?.disconnect();

                // ── Watchers (Alpine $watch returns unwatch functions) ──
                this.__watchers.forEach(unwatch => {
                    this.__tryCatch('cleanup watcher', {}, () => unwatch?.());
                });
                this.__watchers.length = 0;

                // ── Cleanup tasks (AbortControllers, media queries, DOM elements) ──
                for (const task of this.__cleanupTasks) {
                    this.__tryCatch('cleanup task', {}, () => task?.());
                }
                this.__cleanupTasks.clear();

                // Close all EventSource connections
                const threadStates = Object.values(this.__threadStates);
                for (const threadState of threadStates) {
                    if (threadState?.heartbeatInterval) {
                        clearInterval(threadState.heartbeatInterval);
                        threadState.heartbeatInterval = null;
                    }
                    if (threadState?.eventSource) {
                        threadState.eventSource.close();
                        threadState.eventSource = null;
                    }
                }

                // Stop all XState tool actors
                if (this.__toolStateServices) {
                    for (const actor of Object.values(this.__toolStateServices)) {
                        actor?.stop();
                    }
                    this.__toolStateServices = null;
                }

                // Clear state for GC
                this.__threadStates = null;
                this.__currentState = null;
                this.__updateQueue = null;
            },

            // ============================================
            // Utility Methods (bound to component)
            // ============================================

            formatSessionTime(timestamp) {
                return formatSessionTime(timestamp, this.__memoCache);
            },

            renderMarkdown(content) {
                return renderMarkdown(content);
            },

            isValidJSON(str) {
                return isValidJSON(str);
            },

            formatNumber(num) {
                return formatNumber(num);
            },

            /**
             * Get parallel tool call info for display
             * @param {Object} message - The tool call message
             * @returns {Object} { count: number, position: number, isParallel: boolean }
             */
            getParallelToolCallInfo(message) {
                if (!message?.parentMessageId) {
                    return { count: 1, position: 1, isParallel: false };
                }
                const siblings = this.messages.filter(m =>
                    m.type === 'tool_call' && m.parentMessageId === message.parentMessageId
                );
                const position = siblings.findIndex(m => m.id === message.id) + 1;
                return {
                    count: siblings.length,
                    position: position,
                    isParallel: siblings.length > 1
                };
            },

            __log(level, message, context = {}) {
                log(level, message, {
                    ...context,
                    threadId: this.__currentThreadId,
                    metrics: this.__perfMetrics
                });
            }
        };
    });
});
