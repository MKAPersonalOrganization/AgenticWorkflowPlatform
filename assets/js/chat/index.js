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
import { createInputMethods } from './chat-input.js';
import { createProfileMethods } from './chat-profile.js';
import { createEventHandlerMethods } from './chat-event-handlers.js';
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
        const inputMethods = createInputMethods();
        const profileMethods = createProfileMethods();
        const eventHandlerMethods = createEventHandlerMethods();

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
            ...inputMethods,
            ...profileMethods,
            ...eventHandlerMethods,

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
             * Consecutive parallel messages (same parentAgent) are collapsed
             * into a single parallel_group entry with per-branch arrays
             * so the template can render them as side-by-side columns.
             * Non-parallel messages pass through unchanged.
             */
            get displayMessages() {
                const msgs = this.__currentState.messages;
                if (!msgs || msgs.length === 0) return [];

                const result = [];
                let i = 0;

                while (i < msgs.length) {
                    const msg = msgs[i];

                    // When we hit a parallel workflow_marker, group everything
                    // that belongs to this parallel section into branches.
                    if (msg.type === 'workflow_marker' && msg.markerType === 'parallel') {
                        const parentAgent = msg.parentAgent;
                        const stepCount = msg.stepCount || 2;
                        const branches = {};
                        const agentToBranch = {}; // agentName → stepIndex lookup for tool calls
                        i++; // skip the marker itself

                        while (i < msgs.length) {
                            const m = msgs[i];
                            if (m.workflowType === 'parallel' && m.parentAgent === parentAgent) {
                                // Assistant message (or tagged tool call) — direct branch match
                                const branch = m.stepIndex || 0;
                                if (!branches[branch]) branches[branch] = [];
                                branches[branch].push(m);
                                if (m.agentName) agentToBranch[m.agentName] = branch;
                                i++;
                            } else if (m.type === 'tool_call' && m.agentName && m.agentName in agentToBranch) {
                                // Untagged tool call — match by agent name
                                const branch = agentToBranch[m.agentName];
                                if (!branches[branch]) branches[branch] = [];
                                branches[branch].push(m);
                                i++;
                            } else {
                                break; // end of parallel section
                            }
                        }

                        result.push({
                            id: msg.id,
                            type: 'parallel_group',
                            parentAgent,
                            stepCount,
                            branches,
                            timestamp: msg.timestamp,
                        });
                    } else {
                        result.push(msg);
                        i++;
                    }
                }
                return result;
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
