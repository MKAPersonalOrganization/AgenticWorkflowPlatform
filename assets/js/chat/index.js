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
                            this.AVAILABLE_AGENTS = data.agents;

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
                        if (tc?.state === 'awaiting_input') return m;
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
                return this.currentMessage.trim() !== '' && !this.isStreaming;
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

                // Cleanup previous observers
                if (this.__mutationObserver) {
                    this.__mutationObserver.disconnect();
                }

                // Setup MutationObserver to handle dynamic height changes (KaTeX, Markdown rendering, images)
                // Throttled to prevent layout thrashing during rapid DOM mutations.
                this.$nextTick(() => {
                    const container = this.$refs?.messageList;
                    if (container) {
                        const throttledMutationScroll = throttle(() => {
                            // During streaming, the streaming handler is the single scroll source.
                            // Letting MutationObserver also scroll causes competing scrollIntoView
                            // calls that produce visible flickering/jitter.
                            if (this.isStreaming) return;
                            if (this.__abortScroll) return;
                            const isNearBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 200;
                            if (isNearBottom) {
                                this.__scrollToBottom({ behavior: 'auto' });
                            }
                        }, 150);

                        this.__mutationObserver = new MutationObserver(() => {
                            throttledMutationScroll();
                        });
                        this.__mutationObserver.observe(container, {
                            childList: true,
                            subtree: true,
                            characterData: true
                        });

                        // Abort auto-scroll when user manually scrolls up during streaming
                        container.addEventListener('scroll', () => {
                            if (!this.isStreaming) {
                                // Not streaming — just track position, don't set abort
                                return;
                            }
                            const distanceFromBottom = container.scrollHeight - container.scrollTop - container.clientHeight;
                            if (distanceFromBottom > 150) {
                                // User scrolled away from bottom during streaming
                                this.__abortScroll = true;
                            } else {
                                // User scrolled back near bottom — re-engage
                                this.__abortScroll = false;
                            }
                        }, { passive: true });
                    }
                });

                // Load sessions and profile asynchronously
                queueMicrotask(() => {
                    this.__loadSessions();
                    this.fetchProfile();
                    this.fetchBanner();
                });

                // Setup IntersectionObserver for scroll button visibility
                this.$nextTick(() => {
                    const anchor = this.$refs?.messagesEnd;
                    if (anchor) {
                        this.__scrollButtonObserver = new IntersectionObserver(([entry]) => {
                            this.scrolledUp = !entry.isIntersecting;
                        }, {
                            root: this.$refs.messageList,
                            threshold: 0.1
                        });
                        this.__scrollButtonObserver.observe(anchor);
                    }
                });

                // Set up infinite scroll for sidebar sessions (handled by x-intersect in nav.templ)
                /*
                this.$nextTick(() => {
                    const sidebarContent = document.querySelector('[data-tui-sidebar="content"]');
                    if (sidebarContent) {
                        sidebarContent.addEventListener('scroll', (e) => this.__onSidebarScroll(e));
                    }
                });
                */

                // Initialize accessibility
                this.__initScreenReaderAnnouncer();

                // Initialize throttled scroll for streaming — uses direct scrollTop
                // assignment (no scrollIntoView) to avoid animation jitter.
                // 150ms matches LibreChat's streaming scroll cadence.
                this.__scrollToBottomThrottled = throttle(
                    () => this.__scrollToBottomStreaming(),
                    150
                );

                // Set up watchers
                const unwatchMessages = this.$watch('messages', (newMessages, oldMessages) => {
                    // During streaming, TEXT_MESSAGE_CONTENT handler drives scroll via
                    // __scrollToBottomThrottled. Don't duplicate it here.
                    if (!this.isStreaming) {
                        // Also scroll if near bottom even when not streaming (for late updates or historical load)
                        this.$nextTick(() => {
                            const container = this.$refs?.messageList;
                            if (container) {
                                const isNearBottom = container.scrollHeight - container.scrollTop - container.clientHeight < 150;
                                const isMajorUpdate = Math.abs(newMessages.length - oldMessages.length) > 5;

                                if (isNearBottom || isMajorUpdate) {
                                    this.__scrollToBottom({ behavior: 'auto', force: true });
                                }
                            }
                        });
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
                        // Reset scroll state for the new thread
                        this.__abortScroll = false;
                        this.__clearScrollTimeouts();

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

                const unwatchTheme = this.$watch('theme', (newTheme) => {
                    // Centralized watcher for theme side effects
                    // Triggered by x-model, toggleTheme, or direct assignment
                    this.applyThemeEffects(newTheme);
                });
                this.__watchers.push(unwatchTheme);

                // Register keyboard shortcuts
                this.__registerKeyboardShortcuts();
            },

            destroy() {
                // Clear deferred scroll timeouts
                this.__clearScrollTimeouts();

                if (this.__mutationObserver) {
                    this.__mutationObserver.disconnect();
                }
                if (this.__scrollButtonObserver) {
                    this.__scrollButtonObserver.disconnect();
                }

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
