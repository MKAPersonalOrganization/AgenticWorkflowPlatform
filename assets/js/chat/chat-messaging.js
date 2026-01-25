/**
 * Chat Messaging
 *
 * Handles message operations including:
 * - Sending messages
 * - Loading messages from server
 * - Converting backend messages to UI format
 * - Message snapshot handling
 */

import { createUserMessage, safeParseJSON, generateInteractionId, enrichToolResult } from './chat-utils.js';
import { TOOL_EVENTS, TOOL_STATES } from './tool-state-machine.js';
import { getToolConfig } from './chat-tool-registry.js';
/**
 * Create messaging methods mixin
 * @returns {Object} Methods for message handling
 */
export function createMessagingMethods() {
    return {
        /**
         * Send a message
         * @param {string|null} overrideMessage - Optional message to send (e.g. from elicitation)
         */
        sendMessage(overrideMessage = null) {
            return this.__tryCatch('send message', { isElicitation: overrideMessage !== null }, () => {
                // Guard: Check initialization
                if (!this.__initialized) {
                    console.warn('Component not initialized');
                    return;
                }

                const state = this.__getThreadState();

                // Determine which message to send
                let userMessage;
                if (overrideMessage !== null) {
                    userMessage = overrideMessage.trim();
                } else if (!this.awaitingHumanInput) {
                    userMessage = this.currentMessage.trim();
                } else {
                    console.warn('Cannot send message: awaiting human input through specific controls');
                    return;
                }

                // Validation
                if (!userMessage) {
                    return;
                }

                if (!this.canSendMessage && overrideMessage === null) {
                    console.warn('Cannot send regular message: invalid state');
                    return;
                }

                if (!this.__currentThreadId) {
                    this.__showError(window.i18n.t('Please select or create a chat session'));
                    return;
                }

                const interactionId = generateInteractionId();

                // Track performance metrics
                this.__perfMetrics.messagesSent++;
                const sendTime = Date.now();

                // Create and add user message
                const userMsg = createUserMessage(userMessage, interactionId);
                userMsg._sendTime = sendTime;
                state.messages.push(userMsg);

                // Store current interaction ID
                state.currentInteractionId = interactionId;

                // Update session title with first message
                this.__updateSessionTitle();

                // Clear the input buffer
                if (overrideMessage === null) {
                    this.currentMessage = '';
                }

                // Clear human input state
                if (state.humanInputToastId) {
                    this.dismissToast(state.humanInputToastId);
                }
                state.humanInputToastId = null;

                // Update derived state (will clear waiting status since calling push() added a user message)
                this.__updateDerivedState();

                // Focus management (back to main input)
                this.$nextTick(() => {
                    const input = this.$refs?.messageInput;
                    if (input?.isConnected && typeof input.focus === 'function') {
                        input.focus({ preventScroll: true });
                    }
                });

                // Start SSE connection
                this.__streamResponse(userMessage, this.__currentThreadId, this.selectedAgent);
            });
        },

        /**
         * Handle keydown in message input
         * @param {KeyboardEvent} event - Keyboard event
         */
        handleKeydown(event) {
            // Shift+Enter to send
            if (event.key === 'Enter' && event.shiftKey && this.canSendMessage) {
                event.preventDefault();
                this.sendMessage();
            }
            // Enter alone inserts newline (default behavior), so we don't need to prevent default
        },

        /**
         * Load messages for a thread
         * @param {string|null} threadId - Thread ID or null for current
         */
        async __loadMessages(threadId = null) {
            const id = threadId || this.__currentThreadId;
            if (!id) return;

            return await this.__tryCatchAsync('load messages', { threadId: id }, async () => {
                const response = await fetch(`/api/agui/threads/${id}/messages`);
                if (response.ok) {
                    const data = await response.json();
                    const state = this.__getThreadState(id);
                    const messages = Array.isArray(data) ? data : (data.messages || []);
                    const uiMessages = this.__convertBackendMessagesToUI(messages, data.toolStates || {}, id);
                    state.messages = uiMessages;

                    // Side Effect Phase: Sync stateful components (XState)
                    this.__syncThreadToolStates(uiMessages, data.toolStates || {}, id);

                    // Load token usage if available
                    if (data.tokenUsage) {
                        state.tokenUsage = {
                            total: data.tokenUsage.total || 0,
                            prompt: data.tokenUsage.prompt || 0,
                            completion: data.tokenUsage.completion || 0,
                            cached: data.tokenUsage.cached || 0,
                            reasoning: data.tokenUsage.reasoning || 0,
                            audioInput: data.tokenUsage.audioInput || 0,
                            audioOutput: data.tokenUsage.audioOutput || 0,
                            acceptedPredict: data.tokenUsage.acceptedPredict || 0,
                            rejectedPredict: data.tokenUsage.rejectedPredict || 0,
                            model: data.tokenUsage.model || 'N/A',
                            provider: data.tokenUsage.provider || 'N/A',
                            messageId: data.tokenUsage.messageId,
                            threadId: data.tokenUsage.threadId
                        };
                        console.log('Token usage loaded from history:', state.tokenUsage);
                    }

                    // Update derived state based on loaded messages
                    this.__updateDerivedState();

                    // Scroll to bottom after loading
                    if (id === this.__currentThreadId) {
                        this.$nextTick(() => this.__scrollToBottom());
                    }
                } else {
                    throw new Error(`${window.i18n.t('Failed to fetch messages')}: ${response.status}`);
                }
            });
        },

        /**
         * Convert backend AG-UI messages to UI display format
         * @param {Array} backendMessages - Messages in AG-UI SDK format
         * @param {Object} toolStatesMap - Map of tool states by toolCallId (optional)
         * @param {string|null} targetThreadId - Thread ID for state syncing (optional)
         * @returns {Array} Converted messages for UI display
         */
        /**
         * Convert backend AG-UI messages to UI display format
         * @param {Array} backendMessages - Messages in AG-UI SDK format
         * @param {Object} toolStatesMap - Map of tool states by toolCallId (optional)
         * @param {string|null} targetThreadId - Thread ID for state syncing (optional)
         * @returns {Array} Converted messages for UI display
         */
        __convertBackendMessagesToUI(backendMessages, toolStatesMap = {}, targetThreadId = null) {
            if (!Array.isArray(backendMessages)) {
                console.warn('[Messaging] Invalid messages array:', backendMessages);
                return [];
            }

            const uiMessages = [];

            // Phase 1: Convert individual backend messages to flat UI message segments
            for (const msg of backendMessages) {
                if (!msg?.role) continue;

                switch (msg.role) {
                    case 'user':
                        uiMessages.push(this.__handleUserMessage(msg));
                        break;

                    case 'assistant':
                        uiMessages.push(...this.__handleAssistantMessage(msg, toolStatesMap, targetThreadId));
                        break;

                    case 'system':
                    case 'developer':
                        uiMessages.push(this.__handleSystemMessage(msg));
                        break;

                    default:
                        console.warn('[Messaging] Unknown message role:', msg.role);
                        uiMessages.push(this.__handleSystemMessage(msg)); // Fallback to system styling
                }
            }

            // Phase 2: Pair tool calls with their results for unified display
            return this.__pairToolCallsWithResults(uiMessages);
        },

        /**
         * Handle conversion of a user message
         * @private
         */
        __handleUserMessage(msg) {
            return {
                id: msg.id,
                role: 'user',
                content: msg.content || '',
                timestamp: msg.timestamp || new Date().toISOString(),
                streaming: false
            };
        },

        /**
         * Handle conversion of an assistant message (text + tool calls)
         * EXPLICIT ORDER: CONTENT -> TOOL CALLS
         * @private
         */
        __handleAssistantMessage(msg, toolStatesMap, targetThreadId) {
            const segments = [];

            // 1. Text Content ALWAYS first (ensures consistent UI order after refresh)
            if (msg.content) {
                segments.push({
                    id: msg.id,
                    role: 'assistant',
                    content: msg.content,
                    timestamp: msg.timestamp || new Date().toISOString(),
                    streaming: false
                });
            }

            // 2. Tool Calls second
            if (msg.toolCalls?.length > 0) {
                const agentName = msg.name || 'agent';
                for (const toolCall of msg.toolCalls) {
                    segments.push(this.__processToolCall(toolCall, agentName, toolStatesMap, targetThreadId));
                }
            }

            return segments;
        },

        /**
         * Process a single tool call from an assistant message
         * @private
         */
        __processToolCall(toolCall, agentName, toolStatesMap, targetThreadId) {
            const toolName = toolCall.function?.name || 'unknown';
            const toolArgs = toolCall.function?.arguments || '{}';

            // Default state
            let status = 'completed';
            let agent = agentName;
            let toolResult = null;
            let elicitation = null;

            // Sync with persistent tool states from projection
            if (toolStatesMap?.[toolCall.id]) {
                const stateObj = toolStatesMap[toolCall.id];
                if (stateObj.agentName) agent = stateObj.agentName;
                if (stateObj.result) toolResult = stateObj.result;
                if (stateObj.state) status = stateObj.state;
                if (stateObj.elicitation) elicitation = stateObj.elicitation;
            }

            // Component-owned mapping (React-like)
            const config = getToolConfig(toolName, elicitation?.schema?.type);
            const mappedData = config.onMap(toolCall, toolResult, elicitation);

            return {
                id: toolCall.id,
                type: 'tool_call',
                name: toolName,
                args: toolArgs,
                parsedArgs: safeParseJSON(toolArgs),
                result: toolResult,
                status: status,
                agentName: agent,
                timestamp: toolCall.timestamp || new Date().toISOString(),
                ...mappedData
            };
        },

        /**
         * State Synchronization Phase (React-like design)
         * Decouples side effects (XState transitions) from data mapping.
         * @param {Array} uiMessages - Converted UI messages
         * @param {Object} toolStatesMap - Map of tool states by toolCallId
         * @param {string} threadId - Thread ID
         * @private
         */
        __syncThreadToolStates(uiMessages, toolStatesMap, threadId) {
            for (const msg of uiMessages) {
                if (msg.type !== 'tool_call') continue;

                const toolId = msg.id;
                const toolName = msg.name;
                const status = msg.status;
                const toolResult = msg.result;
                const elicitation = toolStatesMap[toolId]?.elicitation;

                // Component-owned synchronization (React-like)
                const config = getToolConfig(toolName, elicitation?.schema?.type);
                config.onSync(toolId, threadId, {
                    toolName,
                    status,
                    toolResult,
                    elicitation,
                    TOOL_EVENTS
                }, (id, ev, pay, tid) => this.transitionToolState(id, ev, pay, tid));
            }
        },

        /**
         * Handle system/developer messages
         * @private
         */
        __handleSystemMessage(msg) {
            return {
                id: msg.id,
                role: msg.role,
                content: msg.content || '',
                timestamp: msg.timestamp || new Date().toISOString(),
                streaming: false
            };
        },

        /**
         * Pair tool calls with their results for unified display
         * @private
         */
        __pairToolCallsWithResults(uiMessages) {
            const toolCallMap = new Map();
            const pairedMessages = [];

            for (const msg of uiMessages) {
                if (msg.type === 'tool_call') {
                    toolCallMap.set(msg.id, msg);
                    pairedMessages.push(msg);
                } else if (msg.type === 'tool_result') {
                    const toolCall = toolCallMap.get(msg.toolCallId);
                    if (toolCall) {
                        enrichToolResult(toolCall, msg.content);
                    } else {
                        pairedMessages.push(msg); // Keep orphaned results visible
                    }
                } else {
                    pairedMessages.push(msg);
                }
            }

            return pairedMessages;
        },

        /**
         * Handle messages snapshot event
         * @param {Object} event - MESSAGES_SNAPSHOT event
         * @param {string} threadId - Thread ID
         */
        __handleMessagesSnapshot(event, threadId) {
            console.log('[MESSAGES_SNAPSHOT] Received for thread', threadId, 'with', event?.messages?.length || 0, 'messages');

            if (!event?.messages || !Array.isArray(event.messages)) {
                console.warn('Invalid MESSAGES_SNAPSHOT:', event);
                return;
            }

            const state = this.__getThreadState(threadId);
            if (!state) {
                console.warn('No state found for MESSAGES_SNAPSHOT:', threadId);
                return;
            }

            // Clear old messages for GC
            const oldMessages = state.messages;
            if (oldMessages?.length > 0) {
                oldMessages.splice(0);
            }

            // Convert and set messages
            const uiMessages = this.__convertBackendMessagesToUI(event.messages, {}, threadId);
            state.messages = uiMessages;

            // Sync stateful components (XState)
            this.__syncThreadToolStates(uiMessages, {}, threadId);

            // Update derived state
            this.__updateDerivedState();

            console.log('[MESSAGES_SNAPSHOT] Processing', state.messages.length, 'messages');

            // Scroll to bottom
            if (threadId === this.__currentThreadId) {
                this.$nextTick(() => this.__scrollToBottom());
            }
        },

    };
}
