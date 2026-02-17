/**
 * Chat State Management
 *
 * Handles state initialization, thread state management, and computed properties.
 * Follows Alpine.js patterns for reactive state management.
 */

import { CONNECTION_CONFIG, DEFAULT_PERF_METRICS } from './constants.js';
import { normalizeElicitation, safeParseJSON } from './chat-utils.js';
import { createToolStateMachine, TOOL_EVENTS, TOOL_STATES } from './tool-state-machine.js';
import { isElicitationTool, isSpecialTool } from './chat-tool-registry.js';
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
        awaitingHumanInput: false,
        humanInputQuestion: null,
        humanInputSchema: null,
        humanInputToastId: null,
        // Token usage tracking
        tokenUsage: null
    };
}

/**
 * Create the initial state for the chat component
 * @returns {Object} Initial component state
 */
export function createInitialState() {
    return {
        // Thread management (internal)
        __threadStates: {},
        __currentThreadId: null,
        __currentState: {
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
            awaitingHumanInput: false,
            humanInputQuestion: null,
            humanInputSchema: null,
            humanInputToastId: null,
            tokenUsage: null
        },
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

        selectedAgent: '',

        // UI state
        sessions: [],
        sessionsNextCursor: null,
        sessionsHasMore: false,
        sessionsLoading: false,
        sidebarOpen: true,
        showDebugPanel: false,
        showEventLog: false,
        eventLogCopied: false,
        hideLifecycleEvents: false,
        showKeyboardShortcuts: false,
        showSettings: false,
        profile: {
            learning_style: '',
            struggles: '',
            strengths: '',
            goals: ''
        },
        profileLoading: false,
        toasts: [],
        __toastIdCounter: 0,

        // Global Banner
        banner: null, // { id: string, message: string, persistable: boolean }
        bannerDismissedIds: JSON.parse(localStorage.getItem('bannerDismissedIds') || '[]'),

        // Theme management
        // Theme management
        theme: localStorage.getItem('theme') || 'system',

        // Accessibility
        prefersReducedMotion: window.matchMedia('(prefers-reduced-motion: reduce)').matches,

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
        markdownEnabled: true,
        autoScroll: true,
        scrolledUp: false, // Track if user has scrolled up from bottom
        textToSpeechEnabled: false,

        // Message queue for offline handling
        __messageQueue: [],

        // Sidepanel view state
        activeView: 'artifacts'
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
         * Batch update helper
         * @param {Function} callback - Update callback
         */
        __batchUpdate(callback) {
            // Prevent duplicate callbacks
            if (!this.__updateQueue.includes(callback)) {
                this.__updateQueue.push(callback);
            }

            if (!this.__updatePending) {
                this.__updatePending = true;

                queueMicrotask(() => {
                    while (this.__updateQueue.length > 0) {
                        const fn = this.__updateQueue.shift();
                        this.__tryCatch('batch update', {}, () => fn?.());
                    }
                    this.__updatePending = false;
                });
            }
        },

        /**
         * Update derived state based on current messages.
         * This ensures a Single Source of Truth for UI state.
         */
        /**
         * Synchronize tool machine state and context to the corresponding UI message.
         * Centralizes mapping of status, result, and elicitation data.
         * @param {Object} threadState - Target thread state
         * @param {string} toolCallId - ID of the tool call
         * @param {string} stateValue - Current machine state value
         * @param {Object} context - Machine context
         */
        __syncToolStateToMessage(threadState, toolCallId, stateValue, context) {
            if (!threadState.messages) return;

            const msg = threadState.messages.find(m => m.id === toolCallId);
            if (!msg) return;

            // Update status
            msg.status = stateValue;

            // Sync result/args
            if (context.result !== undefined && context.result !== null) {
                msg.result = context.result;
                msg.parsedResult = safeParseJSON(msg.result);
            }

            // Sync elicitation (for UI component configuration)
            if (context.elicitation) {
                const { question, schema } = context.elicitation;
                msg.parsedResult = normalizeElicitation(question, schema);
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
                // This ensures __updateDerivedState finds the state immediately even before the listener fires
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
                        this.__syncToolStateToMessage(targetState, toolCallId, snapshot.value, snapshot.context);

                        // Force derived state update
                        this.__updateDerivedState();
                    });
                });

                actor.start();
                this.__toolStateServices[toolCallId] = actor;
            }

            const actor = this.__toolStateServices[toolCallId];
            actor.send({ type: event, ...payload });
        },

        // Registry Helpers (exposed for UI templates)
        isSpecialTool,
        isElicitationTool,

        /**
         * Update derived state (awaiting input, questions, etc.)
         */
        __updateDerivedState() {
            const state = this.__getThreadState();
            const msgs = state.messages;

            // Default: reset state
            let isWaiting = false;
            let question = null;
            let schema = null;

            if (msgs && msgs.length > 0) {
                // Iterate backwards to find the most recent tool call that is awaiting input
                for (let i = msgs.length - 1; i >= 0; i--) {
                    const m = msgs[i];
                    if (m.type === 'tool_call') {
                        // Check XState machine state if available
                        let machineState = null;
                        if (state.toolCalls && state.toolCalls[m.id]) {
                            machineState = state.toolCalls[m.id].state;
                        }

                        // Strict XState check: only show input if machine is in 'awaiting_input'
                        if (machineState === TOOL_STATES.AWAITING_INPUT) {
                            isWaiting = true;
                            question = m.parsedResult?.question_text ||
                                m.parsedResult?.statement_to_verify || null;
                            schema = m.parsedResult?.schema || null;
                            break; // Stop after finding the first one (most recent)
                        }
                    }
                }
            }

            // Batch update derived properties
            this.__batchUpdate(() => {
                state.awaitingHumanInput = isWaiting;
                state.humanInputQuestion = question;
                state.humanInputSchema = schema;
            });
        }
    };
}
