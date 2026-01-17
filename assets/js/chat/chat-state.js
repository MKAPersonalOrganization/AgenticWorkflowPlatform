/**
 * Chat State Management
 *
 * Handles state initialization, thread state management, and computed properties.
 * Follows Alpine.js patterns for reactive state management.
 */

import { CONNECTION_CONFIG, DEFAULT_PERF_METRICS } from './constants.js';

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
        currentStreamingContent: '',
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
        humanInputToastId: null
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
            currentStreamingContent: '',
            awaitingHumanInput: false,
            humanInputQuestion: null,
            humanInputSchema: null,
            humanInputToastId: null
        },

        // User input
        currentMessage: '',

        // UI state
        sessions: [],
        sidebarOpen: true,
        showDebugPanel: false,
        showEventLog: false,
        eventLogCopied: false,
        hideLifecycleEvents: false,
        showKeyboardShortcuts: false,
        toasts: [],
        __toastIdCounter: 0,

        // Theme management
        theme: 'system',

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

        // Message queue for offline handling
        __messageQueue: []
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
        }
    };
}
