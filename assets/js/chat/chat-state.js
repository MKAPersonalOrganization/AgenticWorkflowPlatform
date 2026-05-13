/**
 * Chat State Management
 *
 * State initialization, thread state management, and tool state machines.
 *
 * Alpine.js patterns applied:
 * - State returned from Alpine.data() factory becomes reactive automatically
 *   (Alpine wraps it in Vue's reactive() — see packages/alpinejs/src/reactivity.js)
 * - Thread states use lazy initialization: __getThreadState() creates on first access
 * - Batch updates use queueMicrotask to coalesce multiple mutations into one render
 *   (mirrors Alpine's internal scheduler — see packages/alpinejs/src/scheduler.js)
 * - Tool machines use XState actors with Alpine-batched state sync
 */

import { CONNECTION_CONFIG, DEFAULT_PERF_METRICS } from './constants.js';
import { safeParseJSON } from './chat-utils.js';
import { createToolStateMachine } from './tool-state-machine.js';
import { isSpecialTool } from './chat-tool-registry.js';
import { createActor } from 'xstate';

/**
 * Create an empty thread state object
 * @returns {Object} New thread state object
 */
export function createEmptyThreadState() {
    return {
        messages: [],
        isStreaming: false,
        eventSource: null,
        events: [],
        toolCalls: {},
        pendingToolCallAgents: {},
        currentInteractionId: null,
        errorMessage: null,
        showTypingIndicator: false,
        currentStreamingId: null,
        // Connection tracking
        connectionState: null,
        heartbeatInterval: null,
        retryCount: 0,
        activeJobStreams: new Set(),
        jobRetryMap: new Map(),
        currentJobId: null,
        humanInputToastId: null,
        // Token usage tracking
        tokenUsage: null,
        // Workflow execution state
        activeAgentName: null,
        activeAgentNames: {},     // { [agentName]: true } — all currently executing agents
        activeWorkflowContext: null,
        lastWorkflowContext: null,
        // Workflow node completion tracking — reset at the start of each new run
        completedNodes: {},   // { [agentName]: { durationMs, timestamp } }
        failedNodes: {},      // { [agentName]: { timestamp } }
        workflowOutputs: {},  // { [key]: { agentName, value } }
        // Run lifecycle timestamps — drive the Activity panel's live elapsed
        // display and the "running / done / failed" hero state.
        runStartedAt: null,
        runEndedAt: null,
        // Parallel message coalescing — maps multiple SSE message IDs
        // from the same parallel branch agent into a single UI message.
        // parallelAgentMessages: { [agentName]: messageId } — canonical msg per agent
        // parallelCoalesceMap: { [newMessageId]: canonicalMessageId } — redirect
        // parallelParent: string|null — current parallel section's parentAgent
        parallelAgentMessages: {},
        parallelCoalesceMap: {},
        parallelParent: null
    };
}

/**
 * Create the initial state for the chat component.
 * Every property here becomes reactive when returned from Alpine.data().
 * @returns {Object} Initial component state
 */
export function createInitialState() {
    return {
        // Thread management (internal)
        __threadStates: {},
        __currentThreadId: null,
        __currentState: createEmptyThreadState(),
        __toolStateServices: {}, // Map of toolCallId -> XState Service

        // User input
        currentMessage: '',
        files: [],

        // Slash Command Menu
        showSlashMenu: false,
        slashMenuIndex: 0,
        slashQuery: '',
        filteredPrompts: [],
        prompts: [
            { command: '/study', description: window.i18n.t('Create a study plan'), content: window.i18n.t('Can you help me create a structured study plan for satisfying the following goals:\n\n') },
            { command: '/explain', description: window.i18n.t('Explain a complex concept'), content: window.i18n.t('Please explain the following concept in simple terms suitable for a beginner:\n\n') },
            { command: '/quiz', description: window.i18n.t('Generate a quiz'), content: window.i18n.t('Generate a 5-question multiple choice quiz about the following topic, including an answer key at the end:\n\n') },
            { command: '/summary', description: window.i18n.t('Summarize text or notes'), content: window.i18n.t('Please provide a concise summary of the following text, highlighting the key points and actionable takeaways:\n\n') },
            { command: '/analogy', description: window.i18n.t('Explain with an analogy'), content: window.i18n.t('Please explain this concept using a real-world analogy to make it easier to understand:\n\n') },
            { command: '/code-review', description: window.i18n.t('Review code for best practices'), content: window.i18n.t('Please review the following code for best practices, security vulnerabilities, and potential performance optimizations:\\n\\n') },
            { command: '/debug', description: window.i18n.t('Help debug an error'), content: window.i18n.t('I am encountering the following error. Can you help me diagnose the cause and suggest a fix?\\n\\nError:\\n\\nCode context:\\n') },
            { command: '/email', description: window.i18n.t('Draft a professional email'), content: window.i18n.t('Draft a professional email to [recipient] regarding [topic]. The tone should be [tone].\\n\\nKey points to include:\\n- ') },
            { command: '/brainstorm', description: window.i18n.t('Brainstorm ideas'), content: window.i18n.t('Help me brainstorm 10 creative ideas for [topic]. Prioritize uniqueness and feasibility.\\n\\n') },
        ],
        // Search
        showSearch: false,
        searchQuery: '',
        threadSearchQuery: '',

        renamingSessionId: null,
        renameTitle: '',

        // Thread preview (hover in history dialog)
        __previewSessionId: null,
        __previewMessages: [],
        __previewLoading: false,
        __previewCache: {},
        __previewAbort: null,

        selectedAgent: localStorage.getItem('selectedAgent') || '',
        currentAgentData: null,

        // Static state-lineage analysis for the current agent's
        // workflow graph. Populated by fetchAgentLineage after
        // selectAgent succeeds. Shape mirrors the Go StateLineage
        // struct: { issues: [{kind, node_id, agent_id, key, message}],
        // producible_before: {...}, produces: {...}, references: {...} }.
        // Drives the inline red-dot warnings on leaves with missing refs.
        currentAgentLineage: { issues: [], producible_before: {}, produces: {}, references: {} },

        // UI state
        sessions: [],
        sessionsNextCursor: null,
        sessionsHasMore: false,
        sessionsLoading: false,
        sidebarOpen: false,
        showDebugPanel: false,
        showEventLog: false,
        eventLogEnabled: localStorage.getItem('eventLogEnabled') !== 'false',
        eventLogCopied: false,
        showKeyboardShortcuts: false,
        showSettings: false,
        profile: {
            learning_style: '',
            struggles: '',
            strengths: '',
            goals: ''
        },
        profileLoading: false,

        // User Preferences (consolidated)
        preferences: {
            markdownEnabled: true,
            autoScroll: true,
            theme: localStorage.getItem('theme') || 'system',
            prefersReducedMotion: window.matchMedia('(prefers-reduced-motion: reduce)').matches,
            textToSpeechEnabled: false
        },

        toasts: [],
        __toastIdCounter: 0,

        // Global Banner
        banner: null, // { id: string, message: string, persistable: boolean }
        bannerDismissedIds: JSON.parse(localStorage.getItem('bannerDismissedIds') || '[]'),

        // Internal tracking
        __watchers: [],
        __cleanupTasks: new Set(),
        __updateQueue: [],
        __updatePending: false,
        __srAnnouncer: null,
        __initialized: false,

        // Memoization cache
        __memoCache: {},

        // Performance monitoring
        __perfMetrics: { ...DEFAULT_PERF_METRICS },

        // Connection resilience config
        __connectionConfig: { ...CONNECTION_CONFIG },

        // Message parsing state
        scrolledUp: false, // Track if user has scrolled up from bottom
        __abortScroll: false, // User scrolled up during streaming — stop auto-scroll
        __scrollTimeouts: [], // Track deferred scroll setTimeout handles for cleanup
        __messageQueue: [],

        // Sidepanel view state
        activeView: localStorage.getItem('activeView') || 'artifacts',

        // Right-click context menu state
        contextMenuOpen: false,
        contextMenuX: 0,
        contextMenuY: 0,
        __contextMenuMessage: null,
    };
}

/**
 * State management methods mixin
 * @returns {Object} Methods for state management
 */
export function createStateMethods() {
    return {
        /**
         * Get or lazily create thread state
         * @param {string|null} threadId - Thread ID or null for current
         * @returns {Object} Thread state object
         */
        __getThreadState(threadId = null) {
            const id = threadId || this.__currentThreadId;
            if (!id) return this.__currentState;

            // Lazy initialization pattern
            if (!this.__threadStates[id]) {
                this.__threadStates[id] = createEmptyThreadState();
            }

            return this.__threadStates[id];
        },

        /**
         * Check if a specific thread is streaming
         * @param {string} threadId - Thread ID to check
         * @returns {boolean} True if thread is streaming
         */
        isThreadStreaming(threadId) {
            if (!threadId) return false;
            const state = this.__threadStates[threadId];
            return state?.isStreaming || false;
        },

        /**
         * Coalesce multiple state mutations into one reactive flush.
         * Mirrors Alpine's internal scheduler pattern (queueMicrotask + dedup).
         * @param {Function} callback - Update callback
         */
        __batchUpdate(callback) {
            if (!this.__updateQueue.includes(callback)) {
                this.__updateQueue.push(callback);
            }

            if (this.__updatePending) return;
            this.__updatePending = true;

            queueMicrotask(() => {
                while (this.__updateQueue.length > 0) {
                    const fn = this.__updateQueue.shift();
                    this.__tryCatch('batch update', {}, () => fn?.());
                }
                this.__updatePending = false;
            });
        },

        /**
         * Update derived state based on current messages.
         * This ensures a Single Source of Truth for UI state.
         */
        __syncToolStateToMessage(id, snapshot) {
            const state = this.__currentState;
            const messageIndex = state.messages.findIndex(m => m.id === id);
            if (messageIndex === -1) return;

            const msg = state.messages[messageIndex];
            const status = snapshot.value;
            const context = snapshot.context;

            // map machine state to display status
            if (status === 'awaiting_input') {
                msg.status = 'awaiting_input';
            } else if (status === 'awaiting_confirmation') {
                msg.status = 'awaiting_confirmation';
            } else if (status === 'awaiting_auth') {
                msg.status = 'awaiting_auth';
            } else if (status === 'success') {
                msg.status = 'completed';
            } else if (status === 'error') {
                msg.status = 'error';
            }

            // Sync result/args
            if (context.result !== undefined && context.result !== null) {
                msg.result = context.result;
                msg.parsedResult = safeParseJSON(msg.result);
            }

            // Sync elicitation data for tool confirmation
            if (context.elicitation) {
                const { question, schema } = context.elicitation;
                msg.parsedResult = { question_text: question, schema: schema };
            }

            // Sync user answer (for answered quiz display)
            if (context.userAnswer !== undefined && context.userAnswer !== null) {
                msg.userAnswer = context.userAnswer;
            }
        },

        /**
         * Transition a tool's state machine
         * @param {string|null} threadId - (Optional) Thread ID for state lookup
         * @param {string} toolCallId
         * @param {string} event
         * @param {Object} payload
         */
        transitionToolState(toolCallId, event, payload = {}, threadId = null) {
            const threadState = this.__getThreadState(threadId);

            // Lazy init service
            if (!this.__toolStateServices[toolCallId]) {
                const machine = createToolStateMachine(toolCallId, { toolName: payload.toolName });
                const actor = createActor(machine);

                // Initialize state synchronously to prevent race conditions during hydration/streaming
                // This ensures computed getters find the state immediately even before the listener fires
                if (!threadState.toolCalls) threadState.toolCalls = {};
                threadState.toolCalls[toolCallId] = {
                    state: actor.getSnapshot().value,
                    context: actor.getSnapshot().context
                };

                actor.subscribe(snapshot => {
                    // Update Alpine state on transition
                    this.__batchUpdate(() => {
                        const targetState = this.__getThreadState(threadId);
                        if (!targetState.toolCalls) targetState.toolCalls = {};

                        targetState.toolCalls[toolCallId] = {
                            state: snapshot.value,
                            context: snapshot.context
                        };

                        // Sync to message object for UI binding
                        this.__syncToolStateToMessage(toolCallId, snapshot);
                    });
                });

                actor.start();
                this.__toolStateServices[toolCallId] = actor;
            }

            const actor = this.__toolStateServices[toolCallId];
            actor.send({ type: event, ...payload });
        },

        /**
         * Hydrate a tool's state machine directly into a target state.
         * Used on page reload to restore XState machines without replaying transitions.
         * @param {string} toolCallId
         * @param {string} targetState - XState state value (e.g. 'awaiting_input')
         * @param {Object} contextData - Context values (toolName, elicitation, userAnswer, etc.)
         * @param {string|null} threadId
         */
        hydrateToolState(toolCallId, targetState, contextData = {}, threadId = null) {
            const threadState = this.__getThreadState(threadId);

            // Stop any existing actor for this tool
            if (this.__toolStateServices[toolCallId]) {
                this.__toolStateServices[toolCallId].stop();
                delete this.__toolStateServices[toolCallId];
            }

            // Create machine starting directly in the target state with full context
            const machine = createToolStateMachine(toolCallId, {
                initialState: targetState,
                ...contextData,
            });
            const actor = createActor(machine);

            // Initialize state synchronously
            if (!threadState.toolCalls) threadState.toolCalls = {};
            threadState.toolCalls[toolCallId] = {
                state: actor.getSnapshot().value,
                context: actor.getSnapshot().context
            };

            actor.subscribe(snapshot => {
                this.__batchUpdate(() => {
                    const ts = this.__getThreadState(threadId);
                    if (!ts.toolCalls) ts.toolCalls = {};
                    ts.toolCalls[toolCallId] = {
                        state: snapshot.value,
                        context: snapshot.context
                    };
                    this.__syncToolStateToMessage(toolCallId, snapshot);
                });
            });

            actor.start();
            this.__toolStateServices[toolCallId] = actor;

            // Sync to message immediately (not via microtask) for correct initial render
            this.__syncToolStateToMessage(toolCallId, actor.getSnapshot());
        },

        // Registry Helpers (exposed for UI templates)
        isSpecialTool,
    };
}
