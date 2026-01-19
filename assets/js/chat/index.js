/**
 * Chat Component - Main Entry Point
 *
 * This module composes all chat functionality into a single Alpine.js component.
 *
 * Architecture:
 * - Event-driven with SSE streaming
 * - Multi-threaded chat with isolated state
 * - AG-UI Protocol integration
 * - Production-ready error handling and monitoring
 *
 * Module Structure:
 * - constants.js: Event types and configuration
 * - markdown-renderer.js: Markdown rendering utilities
 * - chat-utils.js: Utility functions (throttle, format, etc.)
 * - chat-state.js: State initialization and management
 * - chat-sessions.js: Session CRUD operations
 * - chat-messaging.js: Message handling and conversion
 * - chat-streaming.js: SSE streaming and event processing
 * - chat-ui.js: UI actions, toasts, clipboard
 * - chat-keyboard.js: Keyboard shortcuts
 * - chat-theme.js: Theme management
 * - chat-accessibility.js: Screen reader support
 * - chat-health.js: Connection health monitoring
 * - alpine-extensions.js: Custom magics and stores
 */

// Import all modules
import { renderMarkdown } from './markdown-renderer.js';
import { throttle, formatSessionTime, log, createErrorHandlers, isValidJSON } from './chat-utils.js';
import { createInitialState, createStateMethods, createEmptyThreadState } from './chat-state.js';
import { createSessionMethods } from './chat-sessions.js';
import { createMessagingMethods } from './chat-messaging.js';
import { createStreamingMethods } from './chat-streaming.js';
import { createUIMethods } from './chat-ui.js';
import { createKeyboardMethods } from './chat-keyboard.js';
import { createThemeMethods } from './chat-theme.js';
import { createAccessibilityMethods } from './chat-accessibility.js';
import { createHealthMethods } from './chat-health.js';
import { initAlpineExtensions } from './alpine-extensions.js';

// Initialize Alpine extensions (magics, stores, debug utils)
initAlpineExtensions();

/**
 * Register the chat Alpine.js component
 */
document.addEventListener('alpine:init', () => {
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

            // Constants
            AVAILABLE_AGENTS: [],

            // Fetch agents from backend
            async fetchAgents() {
                this.__tryCatchAsync('fetch agents', {}, async () => {
                    const response = await fetch('/v1/agents');
                    if (response.ok) {
                        const data = await response.json();
                        if (data.agents && Array.isArray(data.agents)) {
                            this.AVAILABLE_AGENTS = data.agents.map(id => ({
                                id: id,
                                name: id.split('-')
                                    .map(word => word.charAt(0).toUpperCase() + word.slice(1))
                                    .join(' ')
                            }));

                            // Set default if not set
                            if (!this.selectedAgent && this.AVAILABLE_AGENTS.length > 0) {
                                // Prefer math-instructor as default
                                const defaultAgent = this.AVAILABLE_AGENTS.find(a => a.id === 'math-instructor') || this.AVAILABLE_AGENTS[0];
                                this.selectedAgent = defaultAgent.id;
                            }
                        }
                    }
                });
            },

            // ============================================
            // Computed Properties (Getters must be defined directly)
            // ============================================
            get messages() {
                return this.__currentState.messages;
            },

            get isStreaming() {
                return this.__currentState.isStreaming;
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
                return this.__currentState.awaitingHumanInput;
            },

            get humanInputQuestion() {
                return this.__currentState.humanInputQuestion;
            },

            get humanInputSchema() {
                return this.__currentState.humanInputSchema;
            },

            get hasEvents() {
                return this.__currentState.events.length > 0;
            },

            get canSendMessage() {
                return this.currentMessage.trim() !== '' && !this.isStreaming && !this.awaitingHumanInput;
            },

            get canSendElicitation() {
                // Allow replying if we have an answer AND:
                // 1. We are NOT streaming (standard case)
                // OR
                // 2. We ARE streaming BUT we are explicitly waiting for human input (blocking tool call)
                // This fixes the bug where the reply button was disabled during the 'ask_human' state
                return this.elicitationAnswer.trim() !== '' && (!this.isStreaming || this.awaitingHumanInput);
            },

            get currentThread() {
                return this.sessions.find(s => s.id === this.__currentThreadId) || null;
            },

            // ============================================
            // Lifecycle Methods
            // ============================================

            init() {
                // Guard: Prevent double initialization
                if (this.__initialized) {
                    console.warn('Chat component already initialized');
                    return;
                }
                this.__initialized = true;

                // Create error handlers bound to this component
                const errorHandlers = createErrorHandlers(this);
                this.__tryCatch = errorHandlers.tryCatch.bind(this);
                this.__tryCatchAsync = errorHandlers.tryCatchAsync.bind(this);

                // Initialize theme
                this.__initTheme();

                // Fetch available agents
                this.fetchAgents();

                // Load sessions asynchronously
                queueMicrotask(() => {
                    this.__loadSessions();
                });

                // Initialize accessibility
                this.__initScreenReaderAnnouncer();

                // Initialize throttled scroll
                this.__scrollToBottomThrottled = throttle(
                    () => this.__scrollToBottom(),
                    100
                );

                // Bind resize handlers


                // Set up watchers
                const unwatchMessages = this.$watch('messages', (newMessages, oldMessages) => {
                    if (this.isStreaming) {
                        this.__scrollToBottomThrottled();
                    }
                    if (newMessages.length > oldMessages.length) {
                        const lastMsg = newMessages[newMessages.length - 1];
                        if (lastMsg.role === 'assistant' && !document.hasFocus()) {
                            this.__notifyNewMessage(lastMsg);
                        }
                    }
                });
                this.__watchers.push(unwatchMessages);

                const unwatchThread = this.$watch('__currentThreadId', (newThreadId, oldThreadId) => {
                    if (newThreadId) {
                        this.__currentState = this.__getThreadState(newThreadId);
                        this.__onThreadSwitch(newThreadId, oldThreadId);
                    }
                });
                this.__watchers.push(unwatchThread);

                const unwatchError = this.$watch('errorMessage', (error) => {
                    if (error) {
                        setTimeout(() => {
                            if (this.errorMessage === error) {
                                this.__currentState.errorMessage = null;
                            }
                        }, 5000);
                    }
                });
                this.__watchers.push(unwatchError);

                // Register keyboard shortcuts
                this.__registerKeyboardShortcuts();
            },

            destroy() {
                // Clean up all watchers
                while (this.__watchers?.length) {
                    const unwatch = this.__watchers.pop();
                    this.__tryCatch('cleanup watcher', {}, () => unwatch?.());
                }

                // Execute all cleanup tasks
                while (this.__cleanupTasks?.size > 0) {
                    const tasks = Array.from(this.__cleanupTasks);
                    this.__cleanupTasks.clear();
                    tasks.forEach(task => {
                        this.__tryCatch('cleanup task', {}, () => task?.());
                    });
                }

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
