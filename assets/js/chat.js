import { tryCatch, tryCatchAsync } from './error-handler.js';

// Alpine.js chat component
// Following Alpine.js UI library patterns and modern resilient practices:
//
// ═══════════════════════════════════════════════════════════════════════════
// CORE PATTERNS (from Alpine.js source)
// ═══════════════════════════════════════════════════════════════════════════
//
// NAMING & STRUCTURE:
// - __ prefix for internal state and methods (Alpine convention)
// - Simple getters for computed properties (reactive data exposure)
// - Modular helper functions with clear separation
//
// INITIALIZATION & LIFECYCLE:
// - queueMicrotask() for async initialization (Alpine scheduler.js pattern)
// - Initialization guards to prevent double-init (Alpine lifecycle.js pattern)
// - Lazy state initialization for thread isolation
//
// REACTIVITY & STATE:
// - $watch for reactive side effects (auto-scroll, state sync)
// - Direct state references for optimal reactivity
// - State validation guards before operations (Alpine defensive pattern)
//
// CLEANUP & MEMORY:
// - Defensive cleanup with while loops (Alpine mutation.js:46 pattern)
// - Array nullification for GC (Alpine mutation.js:216-219 pattern)
// - Proper resource disposal (EventSource, intervals, watchers)
// - Set for cleanup tracking (Alpine reactivity.js:37 pattern)
//
// RESILIENCE & SAFETY:
// - Duplicate prevention in queues (Alpine scheduler.js:9-10 pattern)
// - Defensive DOM reference checks (isConnected, optional chaining)
// - Comprehensive error boundaries with tryCatch
// - Event structure validation before processing
//
// PERFORMANCE:
// - Batch updates with queueMicrotask (Alpine batching pattern)
// - Memoization for expensive operations (Alpine evaluator.js:72-113 pattern)
// - Cache size limiting to prevent memory leaks (Alpine evaluator.js:138 pattern)
// - Throttled scroll for smooth UX
// - Lazy initialization of thread states
//
// ERROR HANDLING:
// - Structured error reporting (Alpine error.js pattern)
// - Async error throwing with setTimeout(0) (Alpine error.js:26 pattern)
// - Context-rich error objects for debugging
// - Warning format consistency (Alpine warn.js pattern)
//
// ═══════════════════════════════════════════════════════════════════════════
// PRODUCTION ENHANCEMENTS
// ═══════════════════════════════════════════════════════════════════════════
//
// CONNECTION RESILIENCE:
// - Exponential backoff retry (1s → 2s → 4s → 8s → 16s → 30s max)
// - Configurable retry limits (default: 5 attempts)
// - Connection health monitoring (status, retry count, error tracking)
// - Automatic reconnection with user feedback
//
// PERFORMANCE MONITORING:
// - Message send/receive tracking
// - Average response time calculation
// - Error rate monitoring
// - Reconnection attempts tracking
// - Cache size monitoring
//
// OBSERVABILITY:
// - Structured logging with context (timestamp, threadId, metrics)
// - Global debug utilities (window.__chatDebug)
// - Health check endpoint (isHealthy())
// - Diagnostic data export (exportDiagnostics())
// - Performance metrics API (getMetrics())
//
// DEVELOPER EXPERIENCE:
// - Debug utilities exposed globally
// - Comprehensive JSDoc comments
// - Clear separation of concerns
// - Easy metric reset for testing
// - Detailed diagnostic exports
//
// ═══════════════════════════════════════════════════════════════════════════
// ARCHITECTURE
// ═══════════════════════════════════════════════════════════════════════════
//
// - Event-driven with SSE streaming
// - Multi-threaded chat with isolated state
// - AG-UI Protocol integration
// - Production-ready error handling and monitoring

// AG-UI Protocol Lifecycle Event Types
const LIFECYCLE_EVENT_TYPES = [
    'RUN_STARTED', 'RUN_FINISHED', 'RUN_ERROR', 'STEP_STARTED', 'STEP_FINISHED',
    'TEXT_MESSAGE_START', 'TEXT_MESSAGE_END',
    'TOOL_CALL_START', 'TOOL_CALL_ARGS', 'TOOL_CALL_END', 'TOOL_CALL_RESULT',
    'STATE_SNAPSHOT', 'STATE_DELTA', 'MESSAGES_SNAPSHOT',
    'ACTIVITY_SNAPSHOT', 'ACTIVITY_DELTA',
    'RAW', 'CUSTOM',
    'REASONING_START', 'REASONING_MESSAGE_START', 'REASONING_MESSAGE_END', 'REASONING_END',
    'META_EVENT'
];

// Initialize markdown-it with highlight.js and Mathpix integration
let md = null;

if (window.markdownit) {
    md = window.markdownit({
        html: true,
        linkify: true,
        typographer: true,
        breaks: true,
        highlight: function (str, lang) {
            if (window.hljs && lang && window.hljs.getLanguage(lang)) {
                try {
                    return window.hljs.highlight(str, { language: lang }).value;
                } catch (__) { }
            }
            return ''; // use external default escaping
        }
    });
}

/**
 * Render markdown content safely
 * @param {string} content - The markdown content to render
 * @returns {string} - Rendered HTML
 */
function renderMarkdown(content) {
    if (!content) return '';

    // Use Mathpix renderer if available (supports LaTeX math with $$, \[\], etc.)
    if (typeof window.markdownToHTML === 'function') {
        try {
            const rendered = window.markdownToHTML(content, {
                htmlTags: true
            });
            return enhanceCodeBlocks(rendered);
        } catch (e) {
            console.error('Error rendering with Mathpix:', e);
            // Fallback to regular markdown-it
            if (md) {
                return enhanceCodeBlocks(md.render(content));
            }
        }
    }

    // Fallback to regular markdown-it
    if (md) {
        return enhanceCodeBlocks(md.render(content));
    }

    return content;
}

/**
 * Enhance code blocks with copy buttons
 * @param {string} html - Rendered HTML
 * @returns {string} - Enhanced HTML with copy buttons
 */
function enhanceCodeBlocks(html) {
    if (!html) return html;

    // Wrap code blocks in a relative container and add copy button
    return html.replace(
        /<pre><code([^>]*)>([\s\S]*?)<\/code><\/pre>/g,
        (match, attrs, code) => {
            return `<div class="relative group/codeblock">
                <button
                    onclick="copyCodeBlock(event)"
                    class="absolute top-2 right-2 opacity-0 group-hover/codeblock:opacity-100 transition-opacity inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded-md text-xs font-medium ring-offset-background bg-muted hover:bg-accent hover:text-accent-foreground border border-border h-7 px-2 shadow-sm touch-target"
                    type="button"
                    aria-label="Copy code"
                    title="Copy code"
                >
                    <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
                    </svg>
                    <span>Copy</span>
                </button>
                <pre><code${attrs}>${code}</code></pre>
            </div>`;
        }
    );
}

/**
 * Global function to copy code blocks (called from onclick)
 * @param {Event} event - Click event
 */
window.copyCodeBlock = async function (event) {
    const button = event.currentTarget;
    const codeBlock = button.closest('.group\\/codeblock')?.querySelector('code');
    if (!codeBlock) return;

    try {
        await navigator.clipboard.writeText(codeBlock.textContent);

        // Visual feedback
        const originalHTML = button.innerHTML;
        button.innerHTML = `
            <svg class="w-3 h-3 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
            </svg>
            <span class="text-primary">Copied!</span>
        `;

        setTimeout(() => {
            button.innerHTML = originalHTML;
        }, 2000);

        // Show toast if available
        const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
        if (chatComponent?.showToast) {
            chatComponent.showToast('Code copied to clipboard', 'success', 2000);
        }
    } catch (error) {
        console.error('Failed to copy code:', error);

        // Show error toast if available
        const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
        if (chatComponent?.showToast) {
            chatComponent.showToast('Failed to copy code', 'error');
        }
    }
}

/**
 * Chat Component
 * 
 * Multi-threaded chat interface with SSE streaming support.
 * Manages independent conversation threads with real-time updates.
 * 
 * Features:
 * - Thread isolation: Each thread has independent state
 * - Real-time streaming: SSE-based message streaming
 * - Event logging: Debug panel for AG-UI Protocol events
 * - Auto-scroll: Reactive scroll on new messages
 * 
 * State Management:
 * - Uses __ prefix for internal state (Alpine convention)
 * - Lazy thread state initialization
 * - Direct state references for optimal reactivity
 */
document.addEventListener('alpine:init', () => {
    Alpine.data('chat', () => ({
        // ============================================
        // State Properties
        // ============================================

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
            whiteboardContent: '',
            whiteboardLastAgent: null
        },

        // User input
        currentMessage: '',

        // Whiteboard state
        whiteboardTyping: null, // Shows "agent is writing..." message
        whiteboardLastAgent: null, // Last agent who updated whiteboard
        showWhiteboard: true, // Controls whiteboard visibility
        showThread: true, // Controls thread/chat visibility

        // Split panel state
        splitPosition: 50, // Percentage of width for left panel (thread)
        __isResizing: false, // Internal: tracking resize drag state
        __resizeStartX: 0, // Internal: starting X position of resize
        __resizeStartSplit: 50, // Internal: starting split position

        // UI state
        sessions: [],
        sidebarOpen: true,
        showDebugPanel: false,
        showEventLog: false,
        eventLogCopied: false,
        hideLifecycleEvents: false,
        showKeyboardShortcuts: false, // Keyboard shortcuts help modal
        toasts: [], // Toast notifications queue
        __toastIdCounter: 0, // Internal toast ID counter

        // Theme management: 'light', 'dark', or 'system'
        theme: 'system',

        // Accessibility: Check for reduced motion preference
        prefersReducedMotion: window.matchMedia('(prefers-reduced-motion: reduce)').matches,

        // Internal tracking (Alpine pattern)
        __watchers: [],
        __cleanupTasks: new Set(),
        __updateQueue: [],
        __updatePending: false,
        __srAnnouncer: null,
        __initialized: false,

        // Production: Memoization cache (Alpine evaluator pattern)
        __memoCache: {},

        // Production: Performance monitoring
        __perfMetrics: {
            messagesSent: 0,
            messagesReceived: 0,
            errors: 0,
            reconnects: 0,
            avgResponseTime: 0
        },

        // Production: Connection resilience
        __connectionConfig: {
            maxRetries: 5,
            baseDelay: 1000,
            maxDelay: 30000,
            backoffMultiplier: 2
        },

        // Production: Message queue for offline handling
        __messageQueue: [],

        // ============================================
        // Computed Properties (Simple Getters)
        // ============================================
        get messages() {
            return this.__currentState.messages
        },

        get isStreaming() {
            return this.__currentState.isStreaming
        },

        get events() {
            return this.__currentState.events
        },

        get errorMessage() {
            return this.__currentState.errorMessage
        },

        get threadId() {
            return this.__currentThreadId
        },

        get showTypingIndicator() {
            return this.__currentState.showTypingIndicator
        },

        get hasMessages() {
            return this.__currentState.messages.length > 0
        },

        get hasEvents() {
            return this.__currentState.events.length > 0
        },

        get canSendMessage() {
            return this.currentMessage.trim() !== '' && !this.isStreaming
        },

        get currentThread() {
            return this.sessions.find(s => s.id === this.__currentThreadId) || null
        },

        /**
         * Check if a specific thread is streaming (for template access)
         * @param {string} threadId - Thread ID to check
         * @returns {boolean} True if thread is streaming
         */
        isThreadStreaming(threadId) {
            if (!threadId) return false;
            const state = this.__threadStates[threadId];
            return state?.isStreaming || false;
        },

        // ============================================
        // Lifecycle Methods
        // ============================================
        init() {
            // Guard: Prevent double initialization (Alpine defensive pattern)
            if (this.__initialized) {
                console.warn('Chat component already initialized');
                return;
            }
            this.__initialized = true;

            // Initialize theme from saved preference
            this.__initTheme()

            // Alpine pattern: use queueMicrotask for async initialization
            queueMicrotask(() => {
                this.__loadSessions()
            })

            // Initialize screen reader announcer
            this.__initScreenReaderAnnouncer()

            // Initialize throttled scroll (Alpine pattern)
            this.__scrollToBottomThrottled = this.__throttle(
                () => this.__scrollToBottom(),
                100
            )

            // Bind resize handlers to maintain correct 'this' context
            this.__handleResize = this.__handleResize.bind(this)
            this.__stopResize = this.__stopResize.bind(this)

            // Alpine UI pattern: Use $watch for declarative reactive side effects
            // Store watchers for cleanup
            const unwatchMessages = this.$watch('messages', (newMessages, oldMessages) => {
                if (this.isStreaming) {
                    this.__scrollToBottomThrottled()
                }
                // Notify on new messages when window not focused
                if (newMessages.length > oldMessages.length) {
                    const lastMsg = newMessages[newMessages.length - 1]
                    if (lastMsg.role === 'assistant' && !document.hasFocus()) {
                        this.__notifyNewMessage(lastMsg)
                    }
                }
            })
            this.__watchers.push(unwatchMessages)

            // Watch for thread changes to ensure state consistency
            const unwatchThread = this.$watch('__currentThreadId', (newThreadId, oldThreadId) => {
                if (newThreadId) {
                    this.__currentState = this.__getThreadState(newThreadId)
                    this.__onThreadSwitch(newThreadId, oldThreadId)
                }
            })
            this.__watchers.push(unwatchThread)

            // Watch for errors to auto-clear
            const unwatchError = this.$watch('errorMessage', (error) => {
                if (error) {
                    setTimeout(() => {
                        if (this.errorMessage === error) {
                            this.__currentState.errorMessage = null
                        }
                    }, 5000)
                }
            })
            this.__watchers.push(unwatchError)

            // Register keyboard shortcuts (Alpine UI pattern)
            this.__registerKeyboardShortcuts()
        },

        destroy() {
            // Clean up all watchers (Alpine defensive cleanup pattern)
            // Use while loop instead of forEach - more resilient, empties array as it processes
            while (this.__watchers?.length) {
                const unwatch = this.__watchers.pop()
                this.__tryCatch('cleanup watcher', {}, () => unwatch?.())
            }

            // Execute all registered cleanup tasks (Alpine pattern)
            while (this.__cleanupTasks?.size > 0) {
                const tasks = Array.from(this.__cleanupTasks)
                this.__cleanupTasks.clear()
                tasks.forEach(task => {
                    this.__tryCatch('cleanup task', {}, () => task?.())
                })
            }

            // Cleanup: close all EventSource connections and heartbeats
            const threadStates = Object.values(this.__threadStates)
            for (const state of threadStates) {
                if (state?.heartbeatInterval) {
                    clearInterval(state.heartbeatInterval)
                    state.heartbeatInterval = null
                }
                if (state?.eventSource) {
                    state.eventSource.close()
                    state.eventSource = null
                }
            }

            // Clear state and nullify for GC (Alpine memory management pattern)
            this.__threadStates = null
            this.__currentState = null
            this.__updateQueue = null
        },

        // ============================================
        // Thread State Management
        // ============================================

        /**
         * Get or lazily create thread state
         * @param {string|null} threadId - Thread ID or null for current
         * @returns {Object} Thread state object
         */
        __getThreadState(threadId = null) {
            const id = threadId || this.__currentThreadId
            if (!id) return this.__currentState

            // Lazy initialization pattern (Alpine UI convention)
            if (!this.__threadStates[id]) {
                this.__threadStates[id] = this.__createEmptyState()
            }

            return this.__threadStates[id]
        },

        /**
         * Factory method for creating empty thread state
         * @returns {Object} New thread state object
         */
        __createEmptyState() {
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
                // Connection tracking (Alpine pattern)
                connectionState: null,
                heartbeatInterval: null,
                retryCount: 0,
                activeJobStreams: new Set(), // Track active job streams to prevent duplicates
                jobRetryMap: new Map(), // Track retry attempts per job (jobId -> retryCount)
                currentJobId: null // Track current job for cancellation
            }
        },

        // ============================================
        // UI Actions (Alpine pattern: simple action verbs)
        // ============================================

        toggle() {
            this.sidebarOpen = !this.sidebarOpen
        },

        toggleDebugPanel() {
            this.showDebugPanel = !this.showDebugPanel
        },

        toggleEventLog() {
            this.showEventLog = !this.showEventLog
            if (this.showEventLog) {
                this.$nextTick(() => {
                    const logContainer = this.$refs.eventLogContainer
                    if (logContainer) {
                        logContainer.scrollTop = logContainer.scrollHeight
                    }
                })
            }
        },

        toggleWhiteboard() {
            this.showWhiteboard = !this.showWhiteboard
            // Reset split position when toggling
            if (this.showWhiteboard && this.showThread) {
                this.splitPosition = 50
            }
        },

        toggleThread() {
            this.showThread = !this.showThread
            // Reset split position when toggling
            if (this.showWhiteboard && this.showThread) {
                this.splitPosition = 50
            }
        },

        toggleKeyboardShortcuts() {
            this.showKeyboardShortcuts = !this.showKeyboardShortcuts
        },

        // ============================================
        // Toast Notification System
        // ============================================

        /**
         * Show a toast notification
         * @param {string} message - The message to display
         * @param {string} type - Type: 'success', 'error', 'info', 'warning'
         * @param {number} duration - Duration in ms (default: 4000, 0 = persistent)
         * @returns {number} Toast ID for manual dismissal
         */
        showToast(message, type = 'info', duration = 4000) {
            const id = ++this.__toastIdCounter
            const toast = {
                id,
                message,
                type,
                visible: true,
                leaving: false
            }

            this.toasts.push(toast)

            // Auto-dismiss after duration (unless duration is 0)
            if (duration > 0) {
                setTimeout(() => {
                    this.dismissToast(id)
                }, duration)
            }

            // Announce to screen readers
            this.__announce(message, type === 'error' ? 'assertive' : 'polite')

            return id
        },

        /**
         * Dismiss a toast notification
         * @param {number} id - Toast ID to dismiss
         */
        dismissToast(id) {
            const toast = this.toasts.find(t => t.id === id)
            if (toast && !toast.leaving) {
                toast.leaving = true
                // Wait for animation to complete before removing
                setTimeout(() => {
                    this.toasts = this.toasts.filter(t => t.id !== id)
                }, 200)
            }
        },

        /**
         * Get toast icon based on type
         */
        getToastIcon(type) {
            switch (type) {
                case 'success':
                    return '<svg class="w-5 h-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>'
                case 'error':
                    return '<svg class="w-5 h-5 text-destructive" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>'
                case 'warning':
                    return '<svg class="w-5 h-5 text-accent-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path></svg>'
                default:
                    return '<svg class="w-5 h-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>'
            }
        },

        // ============================================
        // Copy to Clipboard Utilities
        // ============================================

        /**
         * Copy text to clipboard with toast feedback
         * @param {string} text - Text to copy
         * @param {string} successMessage - Message to show on success
         */
        async copyToClipboard(text, successMessage = 'Copied to clipboard') {
            try {
                await navigator.clipboard.writeText(text)
                this.showToast(successMessage, 'success', 2000)
                return true
            } catch (error) {
                console.error('Failed to copy:', error)
                this.showToast('Failed to copy to clipboard', 'error')
                return false
            }
        },

        /**
         * Copy message content to clipboard
         * @param {string} messageId - ID of the message to copy
         */
        async copyMessage(messageId) {
            const state = this.__getThreadState()
            const message = state.messages.find(m => m.id === messageId)
            if (message) {
                const content = message.content || message.args || ''
                await this.copyToClipboard(content, 'Message copied')
            }
        },

        /**
         * Copy code block content
         * @param {Event} event - Click event from code block
         */
        async copyCodeBlock(event) {
            const button = event.currentTarget
            const codeBlock = button.closest('pre')?.querySelector('code')
            if (codeBlock) {
                await this.copyToClipboard(codeBlock.textContent, 'Code copied')
            }
        },

        // ============================================
        // Resizable Split Panel Methods
        // ============================================

        startResize(event) {
            // Handle both mouse and touch events
            const isTouch = event.type === 'touchstart'

            // Only handle left mouse button for mouse events
            if (!isTouch && event.button !== 0) return

            // Prevent default to avoid scrolling on touch devices
            if (isTouch) {
                event.preventDefault()
            }

            this.__isResizing = true
            this.__resizeStartX = isTouch ? event.touches[0].clientX : event.clientX
            this.__resizeStartSplit = this.splitPosition

            // Prevent text selection during resize
            document.body.style.userSelect = 'none'
            document.body.style.cursor = 'col-resize'

            // Add global event listeners for both mouse and touch
            document.addEventListener('mousemove', this.__handleResize)
            document.addEventListener('mouseup', this.__stopResize)
            document.addEventListener('touchmove', this.__handleResize, { passive: false })
            document.addEventListener('touchend', this.__stopResize)
            document.addEventListener('touchcancel', this.__stopResize)
        },

        __handleResize(event) {
            if (!this.__isResizing) return

            // Prevent scrolling during touch resize
            if (event.type === 'touchmove') {
                event.preventDefault()
            }

            const container = document.querySelector('[x-ref="splitContainer"]') ||
                document.querySelector('.flex-1.flex.h-screen.overflow-hidden')
            if (!container) return

            const containerRect = container.getBoundingClientRect()
            const clientX = event.type === 'touchmove' ? event.touches[0].clientX : event.clientX
            const deltaX = clientX - this.__resizeStartX
            const deltaPercent = (deltaX / containerRect.width) * 100

            // Calculate new split position with constraints (20% - 80%)
            let newSplit = this.__resizeStartSplit + deltaPercent
            newSplit = Math.max(20, Math.min(80, newSplit))

            this.splitPosition = newSplit
        },

        __stopResize() {
            if (!this.__isResizing) return

            this.__isResizing = false
            document.body.style.userSelect = ''
            document.body.style.cursor = ''

            // Remove all global event listeners
            document.removeEventListener('mousemove', this.__handleResize)
            document.removeEventListener('mouseup', this.__stopResize)
            document.removeEventListener('touchmove', this.__handleResize)
            document.removeEventListener('touchend', this.__stopResize)
            document.removeEventListener('touchcancel', this.__stopResize)
        },

        // Double-click to reset split position to 50%
        resetSplitPosition() {
            this.splitPosition = 50
        },

        // ============================================
        // Session Management
        // ============================================

        async __loadSessions() {
            return await this.__tryCatchAsync('load sessions', {}, async () => {
                const response = await fetch('/api/threads');
                if (response.ok) {
                    const threads = await response.json();
                    // Only show threads that have messages
                    this.sessions = threads
                        .filter(thread => thread.messages && thread.messages.length > 0)
                        .map(thread => ({
                            id: thread.id,
                            title: thread.title,
                            created: thread.createdAt,
                            lastMessage: thread.updatedAt,
                            whiteboardContent: thread.whiteboardContent || '',
                            currentInstructor: thread.currentInstructor || null
                        }));

                    // Get or create current thread ID
                    const savedThreadId = localStorage.getItem('currentThreadId');
                    if (!savedThreadId || !this.sessions.find(s => s.id === savedThreadId)) {
                        if (this.sessions.length > 0) {
                            // Use the most recent session
                            this.switchSession(this.sessions[0].id);
                        } else {
                            // Create a new session
                            this.createNewSession();
                        }
                    } else {
                        // Load existing messages for this thread
                        this.__currentThreadId = savedThreadId;
                        this.__currentState = this.__getThreadState(savedThreadId);
                        await this.__loadMessages(savedThreadId);
                    }

                    // Reconnect to active jobs for ALL threads (not just current)
                    await this.__reconnectToAllActiveJobs();
                } else {
                    throw new Error(`Failed to fetch threads: ${response.status}`);
                }
            }) || this.createNewSession();
        },

        createNewSession() {
            const newSessionId = this.__generateThreadId();
            const newState = this.__createEmptyState();

            this.__threadStates[newSessionId] = newState;
            this.__currentThreadId = newSessionId;
            this.__currentState = newState;
            localStorage.setItem('currentThreadId', newSessionId);

            // Announce to screen readers
            this.__announce('New chat session created');
        },

        switchSession(sessionId) {
            // Allow switching even if current or other threads are streaming - threads are independent
            this.__currentThreadId = sessionId;
            localStorage.setItem('currentThreadId', sessionId);

            // Note: __currentState is updated automatically via $watch on __currentThreadId
            const state = this.__getThreadState(sessionId);

            // Load messages if not already loaded
            if (state.messages.length === 0) {
                this.__loadMessages(sessionId);
            }

            // Scroll to bottom after switching
            this.$nextTick(() => this.__scrollToBottom());
        },

        async deleteSession(sessionId) {
            // Allow deletion even if streaming - threads are independent

            // Confirm deletion
            if (!confirm('Are you sure you want to delete this session?')) {
                return;
            }

            return await this.__tryCatchAsync('delete session', { sessionId }, async () => {
                const response = await fetch(`/api/threads/${sessionId}`, {
                    method: 'DELETE'
                });

                if (response.ok) {
                    // Close EventSource if this thread is streaming
                    const state = this.__threadStates[sessionId];
                    if (state?.eventSource) {
                        state.eventSource.close();
                        state.eventSource = null;
                    }
                    if (state?.heartbeatInterval) {
                        clearInterval(state.heartbeatInterval);
                        state.heartbeatInterval = null;
                    }

                    // Clear large arrays for GC (Alpine memory management pattern)
                    if (state) {
                        state.messages?.splice(0);
                        state.events?.splice(0);
                    }

                    // Remove thread state
                    delete this.__threadStates[sessionId];

                    // Remove session from list
                    this.sessions = this.sessions.filter(s => s.id !== sessionId);

                    // If we deleted the current session, switch to another or create new
                    if (this.__currentThreadId === sessionId) {
                        if (this.sessions.length > 0) {
                            this.switchSession(this.sessions[0].id);
                        } else {
                            this.createNewSession();
                        }
                    }
                } else {
                    throw new Error(`Failed to delete session: ${response.status}`);
                }
            });
        },

        async __updateSessionTitle() {
            let session = this.sessions.find(s => s.id === this.__currentThreadId);

            // If session doesn't exist in the list yet, create it
            const state = this.__getThreadState();
            if (!session && state.messages.length > 0) {
                const now = new Date().toISOString();
                session = {
                    id: this.__currentThreadId,
                    title: 'New Chat',
                    created: now,
                    lastMessage: now
                };
                this.sessions.unshift(session);
                // Title will be auto-generated by backend AI and updated via CUSTOM event
            }
        },

        // ============================================
        // Utility / Helper Methods
        // ============================================

        __generateThreadId() {
            return `thread_${Date.now()}_${Math.random().toString(36).substring(2, 11)}`;
        },

        __generateMessageId() {
            return `msg_${Date.now()}`;
        },

        __generateInteractionId() {
            return `interaction_${Date.now()}`;
        },

        __createUserMessage(content, interactionId) {
            return {
                id: this.__generateMessageId(),
                role: 'user',
                content: content,
                timestamp: new Date().toISOString(),
                interactionId: interactionId,
                lifecycleEvents: []
            };
        },

        formatSessionTime(timestamp) {
            // Production: Memoize expensive date formatting (Alpine evaluator pattern)
            const cacheKey = `sessionTime_${timestamp}`;

            // Cache for 1 minute to balance freshness and performance
            if (this.__memoCache[cacheKey]) {
                const cached = this.__memoCache[cacheKey];
                if (Date.now() - cached.time < 60000) {
                    return cached.value;
                }
            }

            const date = new Date(timestamp);
            const now = new Date();
            const diffMs = now - date;
            const diffMins = Math.floor(diffMs / 60000);

            let result;
            if (diffMins < 1) result = 'Just now';
            else if (diffMins < 60) result = `${diffMins}m ago`;
            else {
                const diffHours = Math.floor(diffMins / 60);
                if (diffHours < 24) result = `${diffHours}h ago`;
                else {
                    const diffDays = Math.floor(diffHours / 24);
                    if (diffDays < 7) result = `${diffDays}d ago`;
                    else result = date.toLocaleDateString();
                }
            }

            // Cache result (Alpine pattern: prevent memory leaks by limiting cache size)
            if (Object.keys(this.__memoCache).length > 100) {
                // Clear old cache entries
                const entries = Object.entries(this.__memoCache);
                entries.slice(0, 50).forEach(([key]) => delete this.__memoCache[key]);
            }

            this.__memoCache[cacheKey] = { value: result, time: Date.now() };
            return result;
        },

        formatTime(timestamp) {
            return new Date(timestamp).toLocaleTimeString('en-US', {
                hour: 'numeric',
                minute: '2-digit'
            });
        },

        isValidJSON(str) {
            if (!str) return false;
            try {
                JSON.parse(str);
                return true;
            } catch (e) {
                return false;
            }
        },

        debounce(func, wait) {
            let timeout;
            return function executedFunction(...args) {
                const later = () => {
                    clearTimeout(timeout);
                    func(...args);
                };
                clearTimeout(timeout);
                timeout = setTimeout(later, wait);
            };
        },

        // ============================================
        // Error Handling (Alpine pattern)
        // ============================================

        /**
         * Production: Structured logging utility (Alpine warn.js pattern)
         * @param {string} level - Log level (info, warn, error)
         * @param {string} message - Log message
         * @param {Object} context - Additional context
         */
        __log(level, message, context = {}) {
            const logData = {
                timestamp: new Date().toISOString(),
                level,
                message,
                threadId: this.__currentThreadId,
                metrics: this.__perfMetrics,
                ...context
            };

            // Production: Use structured logging for better observability
            const logMethod = console[level] || console.log;
            logMethod(`[Chat] ${message}`, logData);

            // Production: Could send to external logging service
            // this.__sendToLoggingService(logData);
        },

        /**
         * Production: Enhanced error reporting (Alpine error.js pattern)
         * @param {Error} error - Error object
         * @param {string} operation - Operation that failed
         * @param {Object} context - Additional context
         */
        __reportError(error, operation, context = {}) {
            this.__perfMetrics.errors++;

            const errorReport = {
                error: {
                    message: error?.message || 'Unknown error',
                    stack: error?.stack,
                    name: error?.name
                },
                operation,
                context: {
                    threadId: this.__currentThreadId,
                    url: window.location.href,
                    userAgent: navigator.userAgent,
                    timestamp: new Date().toISOString(),
                    ...context
                },
                metrics: { ...this.__perfMetrics }
            };

            // Log structured error
            this.__log('error', `Operation failed: ${operation}`, errorReport);

            // Production: Async error throwing (Alpine error.js pattern: setTimeout(0))
            // Allows error to be caught by global error handlers without blocking
            setTimeout(() => {
                if (window.onerror) {
                    const errorWithContext = Object.assign(
                        error || new Error('Unknown error'),
                        { operation, context: errorReport.context }
                    );
                    // Global error handler can catch this
                    throw errorWithContext;
                }
            }, 0);
        },

        /**
         * Centralized error handler with context (Alpine pattern + production enhancements)
         * @param {string} operation - Operation being performed
         * @param {Object} context - Additional context
         * @param {Function} callback - Function to execute
         * @returns {*} Result of callback or null on error
         */
        __tryCatch(operation, context, callback) {
            try {
                return callback();
            } catch (error) {
                this.__reportError(error, operation, context);
                // Call original tryCatch for compatibility
                return tryCatch(operation, context, callback, {
                    threadId: this.__currentThreadId,
                    component: this
                });
            }
        },

        /**
         * Async error handler with context
         * @param {string} operation - Operation being performed
         * @param {Object} context - Additional context
         * @param {Function} callback - Async function to execute
         * @returns {Promise<*>} Result of callback or null on error
         */
        async __tryCatchAsync(operation, context, callback) {
            try {
                return await callback();
            } catch (error) {
                this.__reportError(error, operation, context);
                // Call original tryCatchAsync for compatibility
                return await tryCatchAsync(operation, context, callback, {
                    threadId: this.__currentThreadId,
                    component: this
                });
            }
        },

        // ============================================
        // Performance Utilities (Alpine pattern)
        // ============================================

        __throttle(func, limit) {
            let inThrottle
            return function (...args) {
                if (!inThrottle) {
                    func.apply(this, args)
                    inThrottle = true
                    setTimeout(() => inThrottle = false, limit)
                }
            }
        },

        __batchUpdate(callback) {
            // Prevent duplicate callbacks (Alpine scheduler pattern)
            if (!this.__updateQueue.includes(callback)) {
                this.__updateQueue.push(callback)
            }

            if (!this.__updatePending) {
                this.__updatePending = true

                queueMicrotask(() => {
                    // Alpine pattern: detect last flush in event loop
                    // Process all batched updates in one go
                    while (this.__updateQueue.length > 0) {
                        const fn = this.__updateQueue.shift()
                        this.__tryCatch('batch update', {}, () => fn?.())
                    }
                    this.__updatePending = false
                })
            }
        },

        // ============================================
        // Accessibility (Alpine UI pattern)
        // ============================================

        /**
         * Initialize screen reader announcer element
         */
        __initScreenReaderAnnouncer() {
            // Create hidden live region for screen reader announcements
            this.__srAnnouncer = document.createElement('div')
            this.__srAnnouncer.setAttribute('role', 'status')
            this.__srAnnouncer.setAttribute('aria-live', 'polite')
            this.__srAnnouncer.setAttribute('aria-atomic', 'true')
            this.__srAnnouncer.className = 'sr-only'
            document.body.appendChild(this.__srAnnouncer)

            // Register cleanup
            this.__cleanupTasks.add(() => {
                if (this.__srAnnouncer) {
                    this.__srAnnouncer.remove()
                }
            })
        },

        /**
         * Announce message to screen readers
         * @param {string} message - Message to announce
         * @param {string} priority - 'polite' (default) or 'assertive'
         */
        __announce(message, priority = 'polite') {
            if (!this.__srAnnouncer) return

            // Update aria-live if priority changed
            if (this.__srAnnouncer.getAttribute('aria-live') !== priority) {
                this.__srAnnouncer.setAttribute('aria-live', priority)
            }

            // Clear previous announcement
            this.__srAnnouncer.textContent = ''

            // Announce new message (timeout ensures screen readers detect the change)
            setTimeout(() => {
                this.__srAnnouncer.textContent = message
            }, 100)
        },

        // ============================================
        // Keyboard Shortcuts (Alpine UI pattern)
        // ============================================

        __registerKeyboardShortcuts() {
            const handler = (e) => {
                // ? or Ctrl+/ - Show keyboard shortcuts (works even in inputs)
                if (e.key === '?' || ((e.ctrlKey || e.metaKey) && e.key === '/')) {
                    e.preventDefault()
                    this.toggleKeyboardShortcuts()
                    return
                }

                // Escape - Close modals or cancel streaming
                if (e.key === 'Escape') {
                    e.preventDefault()
                    if (this.showKeyboardShortcuts) {
                        this.showKeyboardShortcuts = false
                    } else if (this.isStreaming) {
                        this.__stopStreaming()
                    }
                    return
                }

                // Don't trigger other shortcuts when typing in input fields
                if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
                    return
                }

                // Ctrl/Cmd + Shift + N - New chat (avoid browser Ctrl+N)
                if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'N') {
                    e.preventDefault()
                    this.createNewSession()
                }

                // Ctrl/Cmd + B - Toggle sidebar (common pattern)
                if ((e.ctrlKey || e.metaKey) && e.key === 'b') {
                    e.preventDefault()
                    this.toggle()
                }

                // Ctrl/Cmd + Shift + D - Toggle debug panel
                if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'D') {
                    e.preventDefault()
                    this.toggleDebugPanel()
                }

                // Ctrl/Cmd + Shift + T - Toggle theme
                if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'T') {
                    e.preventDefault()
                    this.toggleTheme()
                }

                // Ctrl/Cmd + W - Toggle whiteboard
                if ((e.ctrlKey || e.metaKey) && e.key === 'w') {
                    // Only if we have messages (otherwise let browser handle it)
                    if (this.hasMessages || this.isStreaming) {
                        e.preventDefault()
                        this.toggleWhiteboard()
                    }
                }

                // Ctrl/Cmd + Up/Down - Navigate sessions (more intuitive than Alt)
                if ((e.ctrlKey || e.metaKey) && (e.key === 'ArrowUp' || e.key === 'ArrowDown')) {
                    e.preventDefault()
                    this.__navigateSessions(e.key === 'ArrowUp' ? -1 : 1)
                }
            }

            document.addEventListener('keydown', handler)

            // Register cleanup
            this.__cleanupTasks.add(() => {
                document.removeEventListener('keydown', handler)
            })
        },

        // ============================================
        // Theme Management
        // ============================================

        /**
         * Toggle theme: light -> dark -> system -> light
         */
        toggleTheme() {
            const currentTheme = this.theme
            let nextTheme

            if (currentTheme === 'light') {
                nextTheme = 'dark'
            } else if (currentTheme === 'dark') {
                nextTheme = 'system'
            } else {
                nextTheme = 'light'
            }

            this.setTheme(nextTheme)
        },

        /**
         * Set theme preference
         * @param {'light'|'dark'|'system'} theme
         */
        setTheme(theme) {
            this.theme = theme
            localStorage.setItem('theme', theme)
            this.__applyTheme(theme)

            // Update Alpine store if available
            if (Alpine.store('chatSettings')) {
                Alpine.store('chatSettings').theme = theme
            }

            // Show toast notification
            const themeLabels = {
                light: 'Light mode',
                dark: 'Dark mode',
                system: 'System theme'
            }
            this.showToast(`${themeLabels[theme]} enabled`, 'info', 2000)
        },

        /**
         * Apply theme to document
         * @param {'light'|'dark'|'system'} theme
         */
        __applyTheme(theme) {
            const root = document.documentElement

            if (theme === 'system') {
                // Remove explicit class, let system preference apply
                const systemPrefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
                if (systemPrefersDark) {
                    root.classList.add('dark')
                } else {
                    root.classList.remove('dark')
                }
            } else if (theme === 'dark') {
                root.classList.add('dark')
            } else {
                root.classList.remove('dark')
            }
        },

        /**
         * Initialize theme from localStorage or system preference
         */
        __initTheme() {
            const savedTheme = localStorage.getItem('theme') || 'system'
            this.theme = savedTheme
            this.__applyTheme(savedTheme)

            // Listen for system theme changes when in system mode
            const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
            mediaQuery.addEventListener('change', (e) => {
                if (this.theme === 'system') {
                    this.__applyTheme('system')
                }
            })
        },

        __navigateSessions(direction) {
            if (!this.sessions.length) return

            const currentIndex = this.sessions.findIndex(s => s.id === this.__currentThreadId)
            const nextIndex = (currentIndex + direction + this.sessions.length) % this.sessions.length

            this.switchSession(this.sessions[nextIndex].id)
        },

        __stopStreaming() {
            const state = this.__currentState
            if (state.eventSource) {
                state.eventSource.close()
                state.eventSource = null
                state.isStreaming = false
                if (state.heartbeatInterval) {
                    clearInterval(state.heartbeatInterval)
                    state.heartbeatInterval = null
                }
                // Announce to screen readers
                this.__announce('Streaming cancelled')
                console.log('Streaming cancelled by user')
            }
        },

        __notifyNewMessage(message) {
            // Announce new message to screen readers
            this.__announce('New message received from assistant')
            // Could be enhanced with browser notifications
            console.log('New message received:', message.id)
        },

        __onThreadSwitch(newThreadId, oldThreadId) {
            // Pause/background old thread if streaming
            if (oldThreadId) {
                const oldState = this.__threadStates[oldThreadId]
                if (oldState?.eventSource) {
                    oldState.isBackground = true
                }
            }

            // Announce thread switch to screen readers
            const session = this.sessions.find(s => s.id === newThreadId)
            if (session) {
                this.__announce(`Switched to ${session.title}`)
            }

            // Focus message input (Alpine defensive pattern)
            this.$nextTick(() => {
                const input = this.$refs?.messageInput;
                if (input?.isConnected && typeof input.focus === 'function') {
                    input.focus({ preventScroll: true });
                }
            })
        },

        // ============================================
        // Message Loading & Conversion
        // ============================================

        async __loadMessages(threadId = null) {
            const id = threadId || this.__currentThreadId;
            if (!id) return;

            return await this.__tryCatchAsync('load messages', { threadId: id }, async () => {
                const response = await fetch(`/api/threads/${id}/messages`);
                if (response.ok) {
                    const messages = await response.json();
                    const state = this.__getThreadState(id);
                    state.messages = this.__convertBackendMessagesToUI(messages);

                    // Scroll to bottom after loading (only if this is the current thread)
                    if (id === this.__currentThreadId) {
                        this.$nextTick(() => this.__scrollToBottom());
                    }
                } else {
                    throw new Error(`Failed to fetch messages: ${response.status}`);
                }
            });
        },

        __convertBackendMessagesToUI(backendMessages) {
            // Convert backend/AG-UI messages to UI format
            // Handles: user, assistant (with tool calls), tool, system, developer messages

            // Guard: Validate input (Alpine defensive pattern)
            if (!Array.isArray(backendMessages)) {
                console.warn('Invalid messages array:', backendMessages);
                return [];
            }

            const uiMessages = [];

            for (const msg of backendMessages) {
                // Guard: Skip invalid messages
                if (!msg || !msg.role) {
                    console.warn('Skipping invalid message:', msg);
                    continue;
                }
                switch (msg.role) {
                    case 'user':
                        uiMessages.push({
                            id: msg.id,
                            role: 'user',
                            content: msg.content || '',
                            timestamp: msg.timestamp || new Date().toISOString(),
                            streaming: false
                        });
                        break;

                    case 'assistant':
                        // Check for tool calls first
                        if (msg.toolCalls && msg.toolCalls.length > 0) {
                            // Add tool call messages - results will be paired later
                            // Per AG-UI protocol: msg.name contains the agent name for assistant messages
                            const agentName = msg.name || msg.agentName || 'agent';
                            for (const toolCall of msg.toolCalls) {
                                uiMessages.push({
                                    id: toolCall.id,
                                    type: 'tool_call',
                                    name: toolCall.function?.name || 'unknown',
                                    args: toolCall.function?.arguments || '{}',
                                    status: 'completed',
                                    timestamp: msg.timestamp || new Date().toISOString(),
                                    parsedArgs: this.safeParseJSON(toolCall.function?.arguments),
                                    result: null, // Will be filled by pairing logic
                                    agentName: agentName
                                });
                            }
                        }

                        // Add assistant text message if content exists
                        if (msg.content) {
                            uiMessages.push({
                                id: msg.id,
                                role: 'assistant',
                                content: msg.content,
                                timestamp: msg.timestamp || new Date().toISOString(),
                                streaming: false
                            });
                        }
                        break;

                    case 'tool':
                        // Tool result - will be paired with tool call
                        uiMessages.push({
                            id: msg.id,
                            type: 'tool_result',
                            toolCallId: msg.toolCallId,
                            content: msg.content || '',
                            timestamp: msg.timestamp || new Date().toISOString()
                        });
                        break;

                    case 'system':
                    case 'developer':
                        // Check if this is an agent handoff message (special format)
                        if (msg.content && msg.content.startsWith('AGENT_HANDOFF|')) {
                            const parts = msg.content.split('|');
                            uiMessages.push({
                                id: msg.id,
                                type: 'agent_handoff',
                                fromAgent: parts[1] || '',
                                toAgent: parts[2] || '',
                                reason: parts[3] || '',
                                timestamp: msg.timestamp || new Date().toISOString()
                            });
                        } else {
                            uiMessages.push({
                                id: msg.id,
                                role: msg.role,
                                content: msg.content || '',
                                timestamp: msg.timestamp || new Date().toISOString(),
                                streaming: false
                            });
                        }
                        break;

                    default:
                        // Fallback for unknown roles
                        console.warn('Unknown message role:', msg.role);
                        uiMessages.push({
                            id: msg.id,
                            role: msg.role,
                            content: msg.content || '',
                            timestamp: msg.timestamp || new Date().toISOString(),
                            streaming: false
                        });
                }
            }

            // Pair tool calls with their results
            const toolCallMap = new Map();
            const pairedMessages = [];

            for (const msg of uiMessages) {
                if (msg.type === 'tool_call') {
                    toolCallMap.set(msg.id, msg);
                    pairedMessages.push(msg);
                } else if (msg.type === 'tool_result') {
                    const toolCall = toolCallMap.get(msg.toolCallId);
                    if (toolCall) {
                        // Attach result to the tool call
                        toolCall.result = msg.content;
                        // Don't add the tool_result as a separate message
                    } else {
                        // Orphaned result - add it anyway
                        pairedMessages.push(msg);
                    }
                } else {
                    pairedMessages.push(msg);
                }
            }

            return pairedMessages;
        },

        async cancelJob() {
            return this.__tryCatchAsync('cancel job', {}, async () => {
                const state = this.__getThreadState();

                if (!state.currentJobId) {
                    console.warn('No active job to cancel');
                    return;
                }

                const jobId = state.currentJobId;
                console.log('Cancelling job:', jobId);

                try {
                    const response = await fetch(`/api/jobs/${jobId}`, {
                        method: 'DELETE',
                    });

                    if (!response.ok) {
                        throw new Error(`Failed to cancel job: ${response.statusText}`);
                    }

                    // Close event source
                    if (state.eventSource) {
                        state.eventSource.close();
                        state.eventSource = null;
                    }

                    // Clean up state
                    state.activeJobStreams.clear();
                    state.jobRetryMap.clear();
                    state.currentJobId = null;
                    state.isStreaming = false;
                    state.showTypingIndicator = false;

                    console.log('Job cancelled successfully');
                } catch (error) {
                    console.error('Failed to cancel job:', error);
                    this.__showError('Failed to cancel job', this.__currentThreadId);
                }
            });
        },

        async cancelJob() {
            return this.__tryCatchAsync('cancel job', {}, async () => {
                const state = this.__getThreadState();

                if (!state.currentJobId) {
                    console.warn('No active job to cancel');
                    return;
                }

                const jobId = state.currentJobId;
                console.log('Cancelling job:', jobId);

                try {
                    const response = await fetch(`/api/jobs/${jobId}`, {
                        method: 'DELETE',
                    });

                    if (!response.ok) {
                        throw new Error(`Failed to cancel job: ${response.statusText}`);
                    }

                    // Close event source
                    if (state.eventSource) {
                        state.eventSource.close();
                        state.eventSource = null;
                    }

                    // Clean up state
                    state.activeJobStreams.clear();
                    state.jobRetryMap.clear();
                    state.currentJobId = null;
                    state.isStreaming = false;
                    state.showTypingIndicator = false;

                    console.log('Job cancelled successfully');
                } catch (error) {
                    console.error('Failed to cancel job:', error);
                    this.__showError('Failed to cancel job', this.__currentThreadId);
                }
            });
        },

        sendMessage() {
            return this.__tryCatch('send message', { messageLength: this.currentMessage.length }, () => {
                // Guard: Check initialization (Alpine defensive pattern)
                if (!this.__initialized) {
                    console.warn('Component not initialized');
                    return;
                }

                const state = this.__getThreadState();

                // Validation (can also use canSendMessage getter)
                if (!this.canSendMessage) {
                    console.warn('Cannot send message: invalid state');
                    return;
                }

                if (!this.__currentThreadId) {
                    this.__showError('Please select or create a chat session');
                    return;
                }

                const userMessage = this.currentMessage.trim();
                const interactionId = this.__generateInteractionId();

                // Production: Track performance metrics
                this.__perfMetrics.messagesSent++;
                const sendTime = Date.now();

                // Create and add user message using factory method
                const userMsg = this.__createUserMessage(userMessage, interactionId);
                userMsg._sendTime = sendTime; // Internal tracking
                state.messages.push(userMsg);

                // Store current interaction ID for this thread
                state.currentInteractionId = interactionId;

                // Update session title with first message
                this.__updateSessionTitle();

                // Clear input immediately for better UX
                this.currentMessage = '';

                // Focus management (Alpine defensive pattern)
                this.$nextTick(() => {
                    const input = this.$refs?.messageInput;
                    if (input?.isConnected && typeof input.focus === 'function') {
                        input.focus({ preventScroll: true });
                    }
                });

                // Note: Auto-scroll is handled by $watch on messages
                // Start SSE connection for this thread
                this.__streamResponse(userMessage, this.__currentThreadId);
            });
        },

        handleKeydown(event) {
            // Alpine UI pattern: Extract keyboard handling to dedicated method
            // Send on Enter (without Shift), allow Shift+Enter for newlines
            if (event.key === 'Enter' && !event.shiftKey && this.canSendMessage) {
                event.preventDefault();
                this.sendMessage();
            }
        },

        // ============================================
        // SSE Streaming & Event Handling
        // ============================================

        async __streamResponse(message, threadId) {
            const state = this.__getThreadState(threadId);

            // Guard: Prevent duplicate connections (Alpine defensive pattern)
            if (state.eventSource) {
                console.warn('Stream already active for thread:', threadId);
                state.eventSource.close();
                state.eventSource = null;
            }

            state.isStreaming = true;
            state.showTypingIndicator = true;
            state.errorMessage = null;

            try {
                // Step 1: Enqueue the job
                const response = await fetch(`/send?threadId=${encodeURIComponent(threadId)}&message=${encodeURIComponent(message)}`, {
                    method: 'GET',
                });

                if (!response.ok) {
                    throw new Error(`Failed to enqueue job: ${response.statusText}`);
                }

                const jobData = await response.json();
                const jobId = jobData.jobId;

                console.log('Job enqueued:', jobId);

                // Store current job ID for cancellation
                state.currentJobId = jobId;

                // Step 2: Connect to SSE stream for job events
                this.__connectToJobStream(jobId, threadId);

            } catch (error) {
                console.error('Failed to start stream:', error);
                this.__perfMetrics.errors++;

                state.isStreaming = false;
                state.showTypingIndicator = false;

                if (threadId === this.__currentThreadId) {
                    this.__showError(`Failed to send message: ${error.message}`, threadId);
                }
            }
        },

        __connectToJobStream(jobId, threadId) {
            const state = this.__getThreadState(threadId);

            // Prevent duplicate connections to the same job
            if (state.activeJobStreams.has(jobId)) {
                console.log('Already streaming job:', jobId);
                return;
            }

            state.activeJobStreams.add(jobId);

            const streamUrl = `/api/jobs/${jobId}/stream`;
            const eventSource = new EventSource(streamUrl);

            // Store EventSource in thread state so we can close it later
            state.eventSource = eventSource;

            eventSource.onmessage = (event) => {
                // Reset retry count on successful message
                state.jobRetryMap.delete(jobId);

                // Defensive parsing (Alpine pattern)
                const data = this.__tryCatch('parse SSE event', { threadId, jobId }, () => {
                    return JSON.parse(event.data);
                });

                if (data) {
                    this.__handleEvent(data, eventSource, threadId);
                }
            };

            eventSource.onerror = (error) => {
                console.error('SSE error for job:', jobId, error);

                this.__perfMetrics.errors++;

                const wasConnected = eventSource.readyState === EventSource.OPEN;
                eventSource.close();
                state.eventSource = null;
                state.activeJobStreams.delete(jobId);

                // Update streaming state based on active jobs
                state.isStreaming = state.activeJobStreams.size > 0;
                state.showTypingIndicator = state.isStreaming;

                // Only retry if connection was established (not a 404/500)
                if (wasConnected || eventSource.readyState === EventSource.CONNECTING) {
                    const currentRetry = state.jobRetryMap.get(jobId) || 0;
                    state.jobRetryMap.set(jobId, currentRetry + 1);

                    // Exponential backoff: 1s, 2s, 4s, 8s, 16s
                    const backoffDelay = Math.min(1000 * Math.pow(2, currentRetry), 16000);

                    console.log(`Retrying job ${jobId} in ${backoffDelay}ms (attempt ${currentRetry + 1}/5)`);

                    setTimeout(() => {
                        // Only retry if not manually stopped
                        if (!state.activeJobStreams.has(jobId)) {
                            this.__connectToJobStream(jobId, threadId);
                        }
                    }, backoffDelay);
                } else {
                    // Job not found or server error - don't retry
                    console.warn('Job stream not available:', jobId);
                    state.jobRetryMap.delete(jobId);
                }
            };
        }, async __reconnectToAllActiveJobs() {
            try {
                // Check active jobs for all sessions
                for (const session of this.sessions) {
                    await this.__reconnectToActiveJobs(session.id);
                }
            } catch (error) {
                console.error('Error reconnecting to all active jobs:', error);
            }
        },

        async __reconnectToActiveJobs(threadId) {
            try {
                const response = await fetch(`/api/jobs/active?threadId=${encodeURIComponent(threadId)}`);

                if (!response.ok) {
                    console.warn('Failed to fetch active jobs:', response.statusText);
                    return;
                }

                const activeJobs = await response.json();

                if (activeJobs && activeJobs.length > 0) {
                    console.log(`Reconnecting to ${activeJobs.length} active job(s)`, activeJobs);

                    const state = this.__getThreadState(threadId);
                    state.isStreaming = true;
                    state.showTypingIndicator = true;

                    // Connect to all active jobs (they will all stream to the same thread)
                    // The most recent job's ID is stored, but all will contribute events
                    for (const job of activeJobs) {
                        console.log('Connecting to job:', job.id, job.status);
                        this.__connectToJobStream(job.id, threadId);
                    }

                    // Store the most recent job ID
                    if (activeJobs.length > 0) {
                        state.currentJobId = activeJobs[activeJobs.length - 1].id;
                    }
                }
            } catch (error) {
                console.error('Error reconnecting to active jobs:', error);
            }
        },

        __handleEvent(event, eventSource, threadId) {
            // Guard: Validate event structure (Alpine defensive pattern)
            if (!event || !event.type) {
                console.warn('Invalid event received:', event);
                return;
            }

            const state = this.__getThreadState(threadId);
            if (!state) {
                console.warn('No state found for thread:', threadId);
                return;
            }

            // Log all events for debugging
            this.__addEvent(event, threadId);

            // Add lifecycle event as a message if not hidden
            if (!this.hideLifecycleEvents && LIFECYCLE_EVENT_TYPES.includes(event.type)) {
                this.__addLifecycleMessage(event, threadId);
            }

            switch (event.type) {
                case 'RUN_STARTED':
                    // Run started - we can show a loading indicator if needed
                    break;

                case 'STEP_STARTED':
                    // Step started within agent run
                    break;

                case 'STEP_FINISHED':
                    // Step finished within agent run
                    break;

                case 'STATE_SNAPSHOT':
                    // Complete state snapshot received
                    break;

                case 'STATE_DELTA':
                    // Incremental state update received
                    break;

                case 'MESSAGES_SNAPSHOT':
                    // Messages snapshot received - replace entire conversation with snapshot
                    this.__handleMessagesSnapshot(event, threadId);
                    break;

                case 'ACTIVITY_SNAPSHOT':
                    // Activity snapshot received
                    break;

                case 'ACTIVITY_DELTA':
                    // Activity delta received
                    break;

                case 'REASONING_START':
                    // Reasoning started
                    break;

                case 'REASONING_MESSAGE_START':
                    // Reasoning message started
                    break;

                case 'REASONING_MESSAGE_END':
                    // Reasoning message ended
                    break;

                case 'REASONING_END':
                    // Reasoning ended
                    break;

                case 'META_EVENT':
                    // Meta event received (e.g., user feedback)
                    break;

                case 'RAW':
                    // Raw event from external system
                    break;

                case 'CUSTOM':
                    // Custom application-specific event
                    if (event.name === 'THREAD_TITLE_UPDATED') {
                        // Update session title in real-time
                        const session = this.sessions.find(s => s.id === event.value.threadId);
                        if (session) {
                            session.title = event.value.title;
                        }
                    } else if (event.name === 'WHITEBOARD_UPDATED') {
                        // Update whiteboard content in real-time
                        const { agentName, content, action } = event.value;

                        // Show typing indicator
                        this.whiteboardTyping = `${agentName} is writing...`;
                        setTimeout(() => { this.whiteboardTyping = null; }, 500);

                        // Update whiteboard content based on action
                        const session = this.sessions.find(s => s.id === threadId);
                        if (session) {
                            switch (action) {
                                case 'write':
                                    session.whiteboardContent = content;
                                    break;
                                case 'append':
                                    session.whiteboardContent = (session.whiteboardContent || '') + '\n' + content;
                                    break;
                                case 'clear':
                                    session.whiteboardContent = '';
                                    break;
                            }
                            this.whiteboardLastAgent = agentName;
                        }
                    } else if (event.name === 'TOOL_CALL_AGENT') {
                        // Tool call agent pairing - store agent name for upcoming TOOL_CALL_START
                        const { toolCallId, agentName } = event.value;
                        console.log(`[TOOL_CALL_AGENT] toolCallId=${toolCallId}, agentName=${agentName}`);

                        // Initialize pending map if needed
                        if (!state.pendingToolCallAgents) {
                            state.pendingToolCallAgents = {};
                        }
                        state.pendingToolCallAgents[toolCallId] = agentName;

                        // Check if tool call already exists but wasn't rendered yet (TOOL_CALL_START arrived first)
                        const existingToolCall = state.toolCalls[toolCallId];
                        if (existingToolCall && !existingToolCall.hasAgentName) {
                            // Update the tool call with agent name
                            existingToolCall.agentName = agentName;
                            existingToolCall.hasAgentName = true;

                            // Try to parse existing args if available
                            let parsedArgs = {};
                            if (existingToolCall.args) {
                                try {
                                    parsedArgs = JSON.parse(existingToolCall.args);
                                } catch (e) {
                                    // Ignore - JSON is incomplete while streaming
                                }
                            }

                            // Now add to messages array for rendering
                            state.messages.push({
                                id: existingToolCall.id,
                                type: 'tool_call',
                                name: existingToolCall.name,
                                args: existingToolCall.args,
                                parsedArgs: parsedArgs,
                                status: existingToolCall.status,
                                timestamp: existingToolCall.timestamp,
                                agentName: agentName
                            });

                            // Clean up pending agent name
                            delete state.pendingToolCallAgents[toolCallId];

                            if (threadId === this.__currentThreadId) {
                                this.$nextTick(() => this.__scrollToBottom());
                            }
                        } else if (existingToolCall) {
                            // Tool call exists and was already rendered, just update the agent name
                            existingToolCall.agentName = agentName;
                            const toolMsgIndex = state.messages.findIndex(m => m.id === toolCallId);
                            if (toolMsgIndex !== -1) {
                                state.messages[toolMsgIndex].agentName = agentName;
                            }
                        }
                        // If tool call doesn't exist yet, the pending map will be used by TOOL_CALL_START
                    } else if (event.name === 'AGENT_CALLED') {
                        // Agent collaboration event
                        const { callerAgent, calledAgent, reason } = event.value;
                        console.log(`Agent collaboration: ${callerAgent} → ${calledAgent} (${reason})`);
                    }
                    break;

                case 'TOOL_CALL_START':
                    // Tool call started - check if we have agent name from TOOL_CALL_AGENT event
                    const agentName = state.pendingToolCallAgents?.[event.toolCallId];
                    console.log(`[TOOL_CALL_START] toolCallId=${event.toolCallId}, agentName=${agentName}, hasPending=${!!agentName}`);

                    // Store tool call in state (internal tracking)
                    state.toolCalls[event.toolCallId] = {
                        id: event.toolCallId,
                        name: event.toolCallName,
                        args: '',
                        status: 'streaming',
                        timestamp: new Date(event.timestamp).toISOString(),
                        agentName: agentName || 'agent',
                        hasAgentName: !!agentName // Track if we have the agent name
                    };

                    // Only add to messages array (for rendering) if we have the agent name
                    if (agentName) {
                        state.messages.push({
                            id: event.toolCallId,
                            type: 'tool_call',
                            name: event.toolCallName,
                            args: '',
                            parsedArgs: {},
                            status: 'streaming',
                            timestamp: new Date(event.timestamp).toISOString(),
                            agentName: agentName
                        });

                        // Clean up pending agent name
                        delete state.pendingToolCallAgents[event.toolCallId];

                        if (threadId === this.__currentThreadId) {
                            this.$nextTick(() => this.__scrollToBottom());
                        }
                    }
                    // If no agent name yet, the TOOL_CALL_AGENT event will add it to messages when it arrives
                    break;

                case 'TOOL_CALL_ARGS':
                    // Tool call arguments received - accumulate them
                    if (state.toolCalls[event.toolCallId]) {
                        state.toolCalls[event.toolCallId].args += event.delta;

                        // Update the message
                        const toolMsgIndex = state.messages.findIndex(m => m.id === event.toolCallId);
                        if (toolMsgIndex !== -1) {
                            state.messages[toolMsgIndex].args = state.toolCalls[event.toolCallId].args;

                            // Try to parse incrementally for live preview (silently fail on incomplete JSON)
                            try {
                                const parsed = JSON.parse(state.messages[toolMsgIndex].args);
                                state.messages[toolMsgIndex].parsedArgs = parsed;
                            } catch (e) {
                                // Ignore - JSON is incomplete while streaming
                            }
                        }
                    }
                    break;

                case 'TOOL_CALL_END':
                    // Tool call ended - mark as complete
                    if (state.toolCalls[event.toolCallId]) {
                        state.toolCalls[event.toolCallId].status = 'completed';

                        const toolMsgIndex = state.messages.findIndex(m => m.id === event.toolCallId);
                        if (toolMsgIndex !== -1) {
                            state.messages[toolMsgIndex].status = 'completed';

                            // Parse args for display
                            const argsString = state.messages[toolMsgIndex].args || '{}';
                            const parsed = this.__tryCatch('parse tool args', { toolCallId: event.toolCallId, args: argsString.substring(0, 100) }, () => {
                                return JSON.parse(argsString);
                            });
                            state.messages[toolMsgIndex].parsedArgs = parsed || { raw: argsString };
                        }
                    }
                    break;

                case 'TOOL_CALL_RESULT':
                    // Tool call result received - update the tool call message with result
                    const resultToolCallIndex = state.messages.findIndex(m => m.id === event.toolCallId);

                    if (resultToolCallIndex !== -1) {
                        // Update the existing tool call message with the result
                        state.messages[resultToolCallIndex].result = event.content;
                        state.messages[resultToolCallIndex].resultTimestamp = new Date(event.timestamp).toISOString();
                    }

                    // Also update the tool call in toolCalls map
                    if (state.toolCalls[event.toolCallId]) {
                        state.toolCalls[event.toolCallId].result = event.content;
                    }

                    if (threadId === this.__currentThreadId) {
                        this.$nextTick(() => this.__scrollToBottom());
                    }
                    break;

                case 'TEXT_MESSAGE_START':
                    state.showTypingIndicator = false;
                    state.currentStreamingId = event.messageId;
                    state.currentStreamingContent = '';

                    // Production: Track response metrics
                    this.__perfMetrics.messagesReceived++;

                    // Calculate response time if we have the original send time
                    const lastUserMsg = state.messages.slice().reverse().find(m => m.role === 'user');
                    if (lastUserMsg?._sendTime) {
                        const responseTime = Date.now() - lastUserMsg._sendTime;
                        // Update running average (Alpine pattern: efficient calculation)
                        this.__perfMetrics.avgResponseTime =
                            (this.__perfMetrics.avgResponseTime * (this.__perfMetrics.messagesReceived - 1) + responseTime) /
                            this.__perfMetrics.messagesReceived;

                        console.log(`Response time: ${responseTime}ms (avg: ${Math.round(this.__perfMetrics.avgResponseTime)}ms)`);
                    }

                    // Announce to screen readers (only for current thread)
                    if (threadId === this.__currentThreadId) {
                        this.__announce('Assistant is responding');
                    }

                    // Add empty assistant message
                    state.messages.push({
                        id: event.messageId,
                        role: 'assistant',
                        content: '',
                        timestamp: new Date(event.timestamp).toISOString(),
                        streaming: true
                    });
                    break;

                case 'TEXT_MESSAGE_CONTENT':
                    state.currentStreamingContent += event.delta;

                    // Update the streaming message
                    const msgIndex = state.messages.findIndex(m => m.id === event.messageId);
                    if (msgIndex !== -1) {
                        state.messages[msgIndex].content = state.currentStreamingContent;
                    }

                    // Throttled auto-scroll only if this is the current thread
                    // This prevents excessive scroll adjustments during rapid streaming
                    if (threadId === this.__currentThreadId) {
                        this.__scrollToBottomThrottled();
                    }
                    break;

                case 'TEXT_MESSAGE_END':
                    // Mark as not streaming
                    const endMsgIndex = state.messages.findIndex(m => m.id === event.messageId);
                    if (endMsgIndex !== -1) {
                        state.messages[endMsgIndex].streaming = false;
                    }

                    // Announce completion to screen readers (only for current thread)
                    if (threadId === this.__currentThreadId) {
                        this.__announce('Response complete');
                    }

                    state.currentStreamingId = null;
                    state.currentStreamingContent = '';
                    break;

                case 'RUN_FINISHED':
                    // Run completed
                    state.activeJobStreams.clear();
                    state.jobRetryMap.clear();
                    state.currentJobId = null;
                    state.isStreaming = false;
                    state.showTypingIndicator = false;
                    state.eventSource = null;
                    state.retryCount = 0;

                    eventSource.close();
                    break; case 'RUN_ERROR':
                    console.error('Run error:', event.message);

                    state.activeJobStreams.clear();
                    state.isStreaming = false;
                    state.showTypingIndicator = false;
                    state.eventSource = null;

                    if (threadId === this.__currentThreadId) {
                        this.__showError(event.message || 'An error occurred', threadId);
                    }
                    eventSource.close();
                    break;
            }
        },

        __addLifecycleMessage(event, threadId) {
            const state = this.__getThreadState(threadId);

            // Find the last user message with the current interaction ID
            const userMessageIndex = state.messages.findIndex(m =>
                m.role === 'user' && m.interactionId === state.currentInteractionId
            );

            if (userMessageIndex !== -1) {
                const lifecycleEvent = {
                    id: `lifecycle_${Date.now()}_${Math.random()}`,
                    type: event.type,
                    content: this.__getLifecycleContent(event),
                    timestamp: new Date().toISOString()
                };

                // Add to the user message's lifecycle events array
                if (!state.messages[userMessageIndex].lifecycleEvents) {
                    state.messages[userMessageIndex].lifecycleEvents = [];
                }

                // Create a new array reference to ensure Alpine detects the change
                state.messages[userMessageIndex].lifecycleEvents = [
                    ...state.messages[userMessageIndex].lifecycleEvents,
                    lifecycleEvent
                ];
            }
        },

        __getLifecycleContent(event) {
            switch (event.type) {
                // Lifecycle Events
                case 'RUN_STARTED':
                    return 'Started';
                case 'RUN_FINISHED':
                    return event.outcome === 'interrupt' ? 'Paused' : 'Completed';
                case 'RUN_ERROR':
                    return `Error: ${event.message || 'Unknown'}`;
                case 'STEP_STARTED':
                    return `Step: ${event.stepName || 'Unknown'}`;
                case 'STEP_FINISHED':
                    return `${event.stepName || 'Step'} Done`;
                // Text Message Events
                case 'TEXT_MESSAGE_START':
                    return 'Responding';
                case 'TEXT_MESSAGE_END':
                    return 'Response Done';
                // Tool Call Events
                case 'TOOL_CALL_START':
                    return `Tool: ${event.toolCallName || 'Unknown'}`;
                case 'TOOL_CALL_ARGS':
                    return 'Args';
                case 'TOOL_CALL_END':
                    return 'Tool Done';
                case 'TOOL_CALL_RESULT':
                    return 'Result';
                // State Management Events
                case 'STATE_SNAPSHOT':
                    return 'State Snapshot';
                case 'STATE_DELTA':
                    return 'State Update';
                case 'MESSAGES_SNAPSHOT':
                    return 'Messages Sync';
                // Activity Events
                case 'ACTIVITY_SNAPSHOT':
                    return `Activity: ${event.activityType || 'Unknown'}`;
                case 'ACTIVITY_DELTA':
                    return 'Activity Update';
                // Reasoning Events (Draft)
                case 'REASONING_START':
                    return 'Thinking';
                case 'REASONING_MESSAGE_START':
                    return 'Reasoning';
                case 'REASONING_MESSAGE_END':
                    return 'Thought Complete';
                case 'REASONING_END':
                    return 'Thinking Done';
                // Meta Events (Draft)
                case 'META_EVENT':
                    return event.metaType || 'Meta';
                // Special Events
                case 'RAW':
                    return `Raw: ${event.source || 'External'}`;
                case 'CUSTOM':
                    return event.name || 'Custom';
                default:
                    return event.type;
            }
        },

        __handleMessagesSnapshot(event, threadId) {
            // Per AG-UI Protocol spec: MESSAGES_SNAPSHOT replaces entire message array
            // This ensures UI stays synchronized with backend state

            // Guard: Validate event structure (Alpine defensive pattern)
            if (!event?.messages || !Array.isArray(event.messages)) {
                console.warn('Invalid MESSAGES_SNAPSHOT: missing or invalid messages array');
                return;
            }

            const state = this.__getThreadState(threadId);
            if (!state) {
                console.warn('No state found for MESSAGES_SNAPSHOT:', threadId);
                return;
            }

            // Clear old messages for GC (Alpine memory management)
            const oldMessages = state.messages;
            if (oldMessages?.length > 0) {
                oldMessages.splice(0);
            }

            // Use shared conversion method (now includes handoffs from backend)
            const convertedMessages = this.__convertBackendMessagesToUI(event.messages);

            // Set messages from backend (includes handoffs)
            state.messages = convertedMessages;

            // Scroll to bottom after snapshot (only if this is the current thread)
            if (threadId === this.__currentThreadId) {
                this.$nextTick(() => this.__scrollToBottom());
            }
        },

        safeParseJSON(jsonString) {
            if (!jsonString) return {};
            return this.__tryCatch('parse JSON', { jsonString: jsonString?.substring(0, 100) }, () => {
                return JSON.parse(jsonString);
            }) || { raw: jsonString };
        },

        // ============================================
        // Event Logging & Debugging
        // ============================================

        __addEvent(event, threadId) {
            const state = this.__getThreadState(threadId);

            // Add all events to the log (append-only)
            state.events.push({
                id: `event_${Date.now()}_${Math.random()}`,
                type: event.type,
                time: new Date().toLocaleTimeString('en-US', {
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit',
                    fractionalSecondDigits: 3
                }),
                data: event
            });
        },

        getEventDetails(event) {
            switch (event.type) {
                case 'RUN_STARTED':
                    return `Run ${event.runId?.slice(-8)}`;
                case 'TEXT_MESSAGE_START':
                    return `Message ${event.messageId?.slice(-8)}`;
                case 'TEXT_MESSAGE_CONTENT':
                    return `${event.delta?.length || 0} chars`;
                case 'TEXT_MESSAGE_END':
                    return `Complete`;
                case 'RUN_FINISHED':
                    return `Complete`;
                case 'RUN_ERROR':
                    return `Error: ${event.code}`;
                default:
                    return '';
            }
        },

        getEventTypeColor(type) {
            if (type.startsWith('RUN_')) return 'text-primary';
            if (type.startsWith('TEXT_MESSAGE_')) return 'text-primary';
            if (type.startsWith('TOOL_CALL_')) return 'text-accent-foreground';
            if (type.startsWith('STATE_')) return 'text-accent-foreground';
            if (type.startsWith('STEP_')) return 'text-accent-foreground';
            return 'text-muted-foreground';
        },

        toggleEventLog() {
            this.showEventLog = !this.showEventLog;
            if (this.showEventLog) {
                this.$nextTick(() => {
                    // Defensive DOM reference (Alpine pattern)
                    const logContainer = this.$refs?.eventLogContainer;
                    if (logContainer?.isConnected) {
                        logContainer.scrollTop = logContainer.scrollHeight;
                    }
                });
            }
        },

        clear() {
            const state = this.__getThreadState()
            // Clear and nullify for GC (Alpine memory management pattern)
            const oldEvents = state.events
            state.events = []
            // Help GC by nullifying large arrays
            oldEvents.length = 0
        },

        async copy() {
            const state = this.__getThreadState();

            return await this.__tryCatchAsync('copy event log', { eventCount: state.events.length }, async () => {
                const logText = state.events.map(event => {
                    let details = `[${event.time}] ${event.type} (ID: ${event.id})`;

                    // Add all event data fields
                    if (Object.keys(event.data).length > 0) {
                        details += '\n';
                        for (const [key, value] of Object.entries(event.data)) {
                            if (typeof value === 'object') {
                                details += `  ${key}: ${JSON.stringify(value, null, 2).split('\n').join('\n  ')}\n`;
                            } else {
                                details += `  ${key}: ${value}\n`;
                            }
                        }
                    }

                    return details;
                }).join('\n---\n');

                await navigator.clipboard.writeText(logText);
                console.log('Event log copied to clipboard!');

                // Show success feedback
                this.eventLogCopied = true;
                setTimeout(() => {
                    this.eventLogCopied = false;
                }, 2000);
            });
        },

        __showError(message, threadId = null) {
            const state = this.__getThreadState(threadId);
            state.errorMessage = message;

            // Scroll to show the error (only if this is the current thread)
            if (!threadId || threadId === this.__currentThreadId) {
                this.$nextTick(() => this.__scrollToBottom());
            }

            // Auto-hide error after 5 seconds
            setTimeout(() => {
                state.errorMessage = null;
            }, 5000);
        },

        __scrollToBottom() {
            // Defensive DOM reference handling (Alpine pattern)
            const container = this.$refs?.messageList;
            if (!container || !container.isConnected) {
                return;
            }

            // Additional safety check for scroll capability
            if (typeof container.scrollTo !== 'function') {
                console.warn('Container does not support scrollTo');
                return;
            }

            // Smooth scroll to bottom
            this.__tryCatch('scroll to bottom', {}, () => {
                container.scrollTo({
                    top: container.scrollHeight,
                    behavior: 'smooth'
                });
            });
        },

        // ============================================
        // Production: Connection Health Monitoring
        // ============================================

        /**
         * Production: Check connection health
         * @returns {Object} Health status
         */
        getConnectionHealth() {
            const state = this.__getThreadState();
            const isConnected = state?.eventSource?.readyState === EventSource.OPEN;
            const isConnecting = state?.eventSource?.readyState === EventSource.CONNECTING;

            return {
                status: isConnected ? 'connected' : (isConnecting ? 'connecting' : 'disconnected'),
                isStreaming: state?.isStreaming || false,
                retryCount: state?.retryCount || 0,
                maxRetries: this.__connectionConfig.maxRetries,
                hasErrors: this.__perfMetrics.errors > 0
            };
        },

        /**
         * Production: Get performance metrics (for debugging/monitoring)
         * @returns {Object} Performance metrics
         */
        getMetrics() {
            return {
                ...this.__perfMetrics,
                connectionHealth: this.getConnectionHealth(),
                cacheSize: Object.keys(this.__memoCache).length,
                activeThreads: Object.keys(this.__threadStates).length,
                queueSize: this.__updateQueue.length
            };
        },

        /**
         * Production: Reset metrics (useful for testing)
         */
        resetMetrics() {
            this.__perfMetrics = {
                messagesSent: 0,
                messagesReceived: 0,
                errors: 0,
                reconnects: 0,
                avgResponseTime: 0
            };
            this.__log('info', 'Metrics reset');
        },

        /**
         * Production: Health check endpoint (can be called periodically)
         * @returns {boolean} True if healthy
         */
        isHealthy() {
            const health = this.getConnectionHealth();
            const isOverRetrying = (health.retryCount / health.maxRetries) > 0.8;

            return health.status === 'connected' && !isOverRetrying;
        },

        /**
         * Production: Export diagnostic data
         * @returns {Object} Complete diagnostic data
         */
        exportDiagnostics() {
            return {
                timestamp: new Date().toISOString(),
                version: '1.0.0', // Update with your version
                initialized: this.__initialized,
                currentThread: this.__currentThreadId,
                sessions: this.sessions.length,
                threads: Object.keys(this.__threadStates).map(id => ({
                    id,
                    messageCount: this.__threadStates[id]?.messages?.length || 0,
                    isStreaming: this.__threadStates[id]?.isStreaming || false,
                    retryCount: this.__threadStates[id]?.retryCount || 0
                })),
                metrics: this.getMetrics(),
                health: this.getConnectionHealth(),
                config: {
                    retryConfig: this.__connectionConfig
                }
            };
        },

        // ============================================
        // Markdown Rendering
        // ============================================

        /**
         * Render markdown content to HTML
         * @param {string} content - The markdown content to render
         * @returns {string} - Rendered HTML
         */
        renderMarkdown(content) {
            return renderMarkdown(content);
        }
    }));
});

// ═══════════════════════════════════════════════════════════════════════════
// ALPINE.JS CUSTOM MAGICS (Alpine extending.md pattern)
// ═══════════════════════════════════════════════════════════════════════════

document.addEventListener('alpine:init', () => {
    // Custom magic: $chatMetrics - Easy access to performance metrics
    Alpine.magic('chatMetrics', () => {
        return () => {
            const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
            return chatComponent?.getMetrics?.() || null;
        };
    });

    // Custom magic: $chatHealth - Check connection health
    Alpine.magic('chatHealth', () => {
        return () => {
            const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
            return chatComponent?.getConnectionHealth?.() || null;
        };
    });

    // Custom magic: $chatDiagnostics - Export full diagnostics
    Alpine.magic('chatDiagnostics', () => {
        return () => {
            const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
            return chatComponent?.exportDiagnostics?.() || null;
        };
    });

    // Custom magic: $formatTime - Utility for time formatting
    Alpine.magic('formatTime', () => {
        return (timestamp) => {
            if (!timestamp) return '';
            return new Date(timestamp).toLocaleTimeString('en-US', {
                hour: 'numeric',
                minute: '2-digit'
            });
        };
    });

    // Custom magic: $copyToClipboard - Easy clipboard access
    Alpine.magic('copyToClipboard', () => {
        return async (text) => {
            try {
                await navigator.clipboard.writeText(text);
                return true;
            } catch (error) {
                console.error('Failed to copy to clipboard:', error);
                return false;
            }
        };
    });
});

// ═══════════════════════════════════════════════════════════════════════════
// ALPINE.JS GLOBAL STORE (Alpine store.md pattern)
// ═══════════════════════════════════════════════════════════════════════════

document.addEventListener('alpine:init', () => {
    // Global chat settings store
    Alpine.store('chatSettings', {
        // Initialize with sensible defaults
        init() {
            this.theme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
        },

        // Settings
        theme: 'light',
        soundEnabled: true,
        notificationsEnabled: true,
        showTimestamps: true,
        compactMode: false,

        // Actions
        toggleSound() {
            this.soundEnabled = !this.soundEnabled;
        },

        toggleNotifications() {
            this.notificationsEnabled = !this.notificationsEnabled;
        },

        toggleTheme() {
            this.theme = this.theme === 'light' ? 'dark' : 'light';
        },

        toggleCompactMode() {
            this.compactMode = !this.compactMode;
        }
    });
});

// ═══════════════════════════════════════════════════════════════════════════
// PRODUCTION DEBUG UTILITIES (Alpine pattern: expose utilities)
// ═══════════════════════════════════════════════════════════════════════════

if (typeof window !== 'undefined') {
    window.__chatDebug = {
        getMetrics: () => {
            const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
            return chatComponent?.getMetrics?.();
        },
        getHealth: () => {
            const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
            return chatComponent?.getConnectionHealth?.();
        },
        exportDiagnostics: () => {
            const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
            return chatComponent?.exportDiagnostics?.();
        },
        resetMetrics: () => {
            const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
            return chatComponent?.resetMetrics?.();
        },
        // Access Alpine store
        getSettings: () => Alpine.store('chatSettings'),
        // Quick health check
        isHealthy: () => {
            const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
            return chatComponent?.isHealthy?.() || false;
        }
    };

    console.log('[Chat] Debug utilities available:');
    console.log('  - window.__chatDebug (legacy)');
    console.log('  - $chatMetrics() in templates');
    console.log('  - $chatHealth() in templates');
    console.log('  - $chatDiagnostics() in templates');
    console.log('  - $formatTime(timestamp) in templates');
    console.log('  - $copyToClipboard(text) in templates');
    console.log('  - $store.chatSettings (global settings)');
}
