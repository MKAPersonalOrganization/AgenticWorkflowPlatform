import { tryCatch, tryCatchAsync } from './error-handler.js';

// Alpine.js chat component
// Following Alpine.js UI library patterns:
// - __ prefix for internal state and methods (Alpine convention)
// - queueMicrotask() for async initialization
// - Simple getters for computed properties (reactive data exposure)
// - $watch for reactive side effects (auto-scroll, state sync)
// - Direct state references for optimal reactivity
// - Modular helper functions with clear separation
// - Event-driven architecture with SSE streaming
// - Lazy state initialization for thread isolation

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
            currentInteractionId: null,
            errorMessage: null,
            showTypingIndicator: false,
            currentStreamingId: null,
            currentStreamingContent: ''
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

        // Internal tracking (Alpine pattern)
        __watchers: [],
        __cleanupTasks: new Set(),
        __updateQueue: [],
        __updatePending: false,

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

        // ============================================
        // Lifecycle Methods
        // ============================================
        init() {
            // Alpine pattern: use queueMicrotask for async initialization
            queueMicrotask(() => {
                this.__loadSessions()
            })

            // Initialize throttled scroll (Alpine pattern)
            this.__scrollToBottomThrottled = this.__throttle(
                () => this.__scrollToBottom(),
                100
            )

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
            // Clean up all watchers (Alpine pattern)
            this.__watchers?.forEach(unwatch => {
                try {
                    unwatch()
                } catch (e) {
                    console.error('Watcher cleanup error:', e)
                }
            })
            this.__watchers = []

            // Execute all registered cleanup tasks
            this.__cleanupTasks?.forEach(task => {
                try {
                    task()
                } catch (e) {
                    console.error('Cleanup task error:', e)
                }
            })
            this.__cleanupTasks.clear()

            // Cleanup: close all EventSource connections and heartbeats
            Object.values(this.__threadStates).forEach(state => {
                if (state.heartbeatInterval) {
                    clearInterval(state.heartbeatInterval)
                }
                if (state.eventSource) {
                    state.eventSource.close()
                    state.eventSource = null
                }
            })

            // Clear state
            this.__threadStates = {}
            this.__currentState = null
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
                currentInteractionId: null,
                errorMessage: null,
                showTypingIndicator: false,
                currentStreamingId: null,
                currentStreamingContent: '',
                // Connection tracking (Alpine pattern)
                connectionState: null,
                heartbeatInterval: null,
                retryCount: 0
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
                            lastMessage: thread.updatedAt
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
                        this.__loadMessages(savedThreadId);
                    }
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
                    if (state && state.eventSource) {
                        state.eventSource.close();
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
            const date = new Date(timestamp);
            const now = new Date();
            const diffMs = now - date;
            const diffMins = Math.floor(diffMs / 60000);

            if (diffMins < 1) return 'Just now';
            if (diffMins < 60) return `${diffMins}m ago`;

            const diffHours = Math.floor(diffMins / 60);
            if (diffHours < 24) return `${diffHours}h ago`;

            const diffDays = Math.floor(diffHours / 24);
            if (diffDays < 7) return `${diffDays}d ago`;

            return date.toLocaleDateString();
        },

        formatTime(timestamp) {
            return new Date(timestamp).toLocaleTimeString('en-US', {
                hour: 'numeric',
                minute: '2-digit'
            });
        },

        isValidJSON(str) {
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
         * Centralized error handler with context (Alpine pattern)
         * @param {string} operation - Operation being performed
         * @param {Object} context - Additional context
         * @param {Function} callback - Function to execute
         * @returns {*} Result of callback or null on error
         */
        __tryCatch(operation, context, callback) {
            return tryCatch(operation, context, callback, {
                threadId: this.__currentThreadId,
                component: this
            });
        },

        /**
         * Async error handler with context
         * @param {string} operation - Operation being performed
         * @param {Object} context - Additional context
         * @param {Function} callback - Async function to execute
         * @returns {Promise<*>} Result of callback or null on error
         */
        async __tryCatchAsync(operation, context, callback) {
            return await tryCatchAsync(operation, context, callback, {
                threadId: this.__currentThreadId,
                component: this
            });
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
            this.__updateQueue.push(callback)
            if (!this.__updatePending) {
                this.__updatePending = true
                queueMicrotask(() => {
                    this.__updateQueue.forEach(fn => fn())
                    this.__updateQueue = []
                    this.__updatePending = false
                })
            }
        },

        // ============================================
        // Keyboard Shortcuts (Alpine UI pattern)
        // ============================================

        __registerKeyboardShortcuts() {
            const handler = (e) => {
                // Don't trigger shortcuts when typing in input fields
                if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
                    // Allow Escape even in input fields
                    if (e.key !== 'Escape') return;
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

                // Escape - Cancel streaming
                if (e.key === 'Escape' && this.isStreaming) {
                    e.preventDefault()
                    this.__stopStreaming()
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
                // Show info message instead of error
                console.log('Streaming cancelled by user')
            }
        },

        __notifyNewMessage(message) {
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

            // Focus message input
            this.$nextTick(() => {
                this.$refs.messageInput?.focus({ preventScroll: true })
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
            const uiMessages = [];

            for (const msg of backendMessages) {
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
                            // Add tool call messages
                            for (const toolCall of msg.toolCalls) {
                                uiMessages.push({
                                    id: toolCall.id,
                                    type: 'tool_call',
                                    name: toolCall.function?.name || 'unknown',
                                    args: toolCall.function?.arguments || '{}',
                                    status: 'completed',
                                    timestamp: msg.timestamp || new Date().toISOString(),
                                    parsedArgs: this.safeParseJSON(toolCall.function?.arguments)
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
                        // Tool result message
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
                        uiMessages.push({
                            id: msg.id,
                            role: msg.role,
                            content: msg.content || '',
                            timestamp: msg.timestamp || new Date().toISOString(),
                            streaming: false
                        });
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

            return uiMessages;
        },

        sendMessage() {
            return this.__tryCatch('send message', { messageLength: this.currentMessage.length }, () => {
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

                // Create and add user message using factory method
                const userMsg = this.__createUserMessage(userMessage, interactionId);
                state.messages.push(userMsg);

                // Store current interaction ID for this thread
                state.currentInteractionId = interactionId;

                // Update session title with first message
                this.__updateSessionTitle();

                // Clear input immediately for better UX
                this.currentMessage = '';

                // Focus management
                this.$nextTick(() => {
                    this.$refs.messageInput?.focus({ preventScroll: true });
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

        __streamResponse(message, threadId) {
            const state = this.__getThreadState(threadId);
            state.isStreaming = true;
            state.showTypingIndicator = true;
            state.errorMessage = null;

            const url = `/send?threadId=${encodeURIComponent(threadId)}&message=${encodeURIComponent(message)}`;
            const eventSource = new EventSource(url);

            // Store EventSource in thread state so we can close it later
            state.eventSource = eventSource;

            eventSource.onmessage = (event) => {
                const data = JSON.parse(event.data);
                this.__handleEvent(data, eventSource, threadId);
            };

            eventSource.onerror = (error) => {
                console.error('SSE error:', error);
                eventSource.close();
                state.eventSource = null;
                state.isStreaming = false;
                state.showTypingIndicator = false;

                // Only show error if this is the current thread
                if (threadId === this.__currentThreadId) {
                    this.__showError('Connection error. Please try again.', threadId);
                }
            };
        },

        __handleEvent(event, eventSource, threadId) {
            const state = this.__getThreadState(threadId);

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
                    }
                    break;

                case 'TOOL_CALL_START':
                    // Tool call started - create a new tool call message
                    state.toolCalls[event.toolCallId] = {
                        id: event.toolCallId,
                        name: event.toolCallName,
                        args: '',
                        status: 'streaming',
                        timestamp: new Date(event.timestamp).toISOString()
                    };

                    state.messages.push({
                        id: event.toolCallId,
                        type: 'tool_call',
                        name: event.toolCallName,
                        args: '',
                        status: 'streaming',
                        timestamp: new Date(event.timestamp).toISOString()
                    });

                    if (threadId === this.__currentThreadId) {
                        this.$nextTick(() => this.__scrollToBottom());
                    }
                    break;

                case 'TOOL_CALL_ARGS':
                    // Tool call arguments received - accumulate them
                    if (state.toolCalls[event.toolCallId]) {
                        state.toolCalls[event.toolCallId].args += event.delta;

                        // Update the message
                        const toolMsgIndex = state.messages.findIndex(m => m.id === event.toolCallId);
                        if (toolMsgIndex !== -1) {
                            state.messages[toolMsgIndex].args = state.toolCalls[event.toolCallId].args;
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

                            // Try to parse args as JSON for display
                            try {
                                state.messages[toolMsgIndex].parsedArgs = JSON.parse(state.messages[toolMsgIndex].args);
                            } catch (e) {
                                state.messages[toolMsgIndex].parsedArgs = { raw: state.messages[toolMsgIndex].args };
                            }
                        }
                    }
                    break;

                case 'TOOL_CALL_RESULT':
                    // Tool call result received - insert result message right after the tool call
                    const toolCallIndex = state.messages.findIndex(m => m.id === event.toolCallId);

                    if (toolCallIndex !== -1) {
                        // Insert the result right after the tool call message
                        state.messages.splice(toolCallIndex + 1, 0, {
                            id: event.messageId,
                            type: 'tool_result',
                            toolCallId: event.toolCallId,
                            content: event.content,
                            timestamp: new Date(event.timestamp).toISOString()
                        });
                    } else {
                        // Fallback: if tool call not found, append to end
                        state.messages.push({
                            id: event.messageId,
                            type: 'tool_result',
                            toolCallId: event.toolCallId,
                            content: event.content,
                            timestamp: new Date(event.timestamp).toISOString()
                        });
                    }

                    if (threadId === this.__currentThreadId) {
                        this.$nextTick(() => this.__scrollToBottom());
                    }
                    break;

                case 'TEXT_MESSAGE_START':
                    state.showTypingIndicator = false;
                    state.currentStreamingId = event.messageId;
                    state.currentStreamingContent = '';

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

                    // Auto-scroll only if this is the current thread
                    if (threadId === this.__currentThreadId) {
                        this.$nextTick(() => this.__scrollToBottom());
                    }
                    break;

                case 'TEXT_MESSAGE_END':
                    // Mark as not streaming
                    const endMsgIndex = state.messages.findIndex(m => m.id === event.messageId);
                    if (endMsgIndex !== -1) {
                        state.messages[endMsgIndex].streaming = false;
                    }

                    state.currentStreamingId = null;
                    state.currentStreamingContent = '';
                    break;

                case 'RUN_FINISHED':
                    // Run completed successfully
                    state.isStreaming = false;
                    state.eventSource = null;
                    eventSource.close();
                    break;

                case 'RUN_ERROR':
                    // Handle error
                    console.error('Run error:', event.message);
                    state.showTypingIndicator = false;
                    state.isStreaming = false;
                    state.eventSource = null;

                    // Only show error if this is the current thread
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

            if (!event.messages || !Array.isArray(event.messages)) {
                console.warn('Invalid MESSAGES_SNAPSHOT: missing or invalid messages array');
                return;
            }

            const state = this.__getThreadState(threadId);

            // Use shared conversion method
            state.messages = this.__convertBackendMessagesToUI(event.messages);

            // Scroll to bottom after snapshot (only if this is the current thread)
            if (threadId === this.__currentThreadId) {
                this.$nextTick(() => this.__scrollToBottom());
            }
        },

        safeParseJSON(jsonString) {
            if (!jsonString) return {};
            try {
                return JSON.parse(jsonString);
            } catch (e) {
                return { raw: jsonString };
            }
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
            if (type.startsWith('RUN_')) return 'text-purple-600 dark:text-purple-400';
            if (type.startsWith('TEXT_MESSAGE_')) return 'text-blue-600 dark:text-blue-400';
            if (type.startsWith('TOOL_CALL_')) return 'text-green-600 dark:text-green-400';
            if (type.startsWith('STATE_')) return 'text-orange-600 dark:text-orange-400';
            if (type.startsWith('STEP_')) return 'text-yellow-600 dark:text-yellow-400';
            return 'text-gray-600 dark:text-gray-400';
        },

        toggleEventLog() {
            this.showEventLog = !this.showEventLog;
            if (this.showEventLog) {
                this.$nextTick(() => {
                    const logContainer = this.$refs.eventLogContainer;
                    if (logContainer) {
                        logContainer.scrollTop = logContainer.scrollHeight;
                    }
                });
            }
        },

        clear() {
            const state = this.__getThreadState()
            state.events = []
        },

        async copy() {
            const state = this.__getThreadState();

            try {
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
            } catch (err) {
                console.error('Failed to copy event log:', err);
                alert('Failed to copy: ' + err.message);
            }
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
            const container = this.$refs.messageList;
            if (container) {
                // Smooth scroll to bottom
                container.scrollTo({
                    top: container.scrollHeight,
                    behavior: 'smooth'
                });
            }
        }
    }));
});
