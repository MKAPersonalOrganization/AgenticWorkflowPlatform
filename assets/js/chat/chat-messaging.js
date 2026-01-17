/**
 * Chat Messaging
 *
 * Handles message operations including:
 * - Sending messages
 * - Loading messages from server
 * - Converting backend messages to UI format
 * - Message snapshot handling
 */

import { createUserMessage, safeParseJSON, generateInteractionId } from './chat-utils.js';

/**
 * Create messaging methods mixin
 * @returns {Object} Methods for message handling
 */
export function createMessagingMethods() {
    return {
        /**
         * Send a message
         */
        sendMessage() {
            return this.__tryCatch('send message', { messageLength: this.currentMessage.length }, () => {
                // Guard: Check initialization
                if (!this.__initialized) {
                    console.warn('Component not initialized');
                    return;
                }

                const state = this.__getThreadState();

                // Validation
                if (!this.canSendMessage) {
                    console.warn('Cannot send message: invalid state');
                    return;
                }

                if (!this.__currentThreadId) {
                    this.__showError('Please select or create a chat session');
                    return;
                }

                const userMessage = this.currentMessage.trim();
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

                // Clear input immediately for better UX
                this.currentMessage = '';

                // Clear human input state if responding
                if (state.humanInputToastId) {
                    this.dismissToast(state.humanInputToastId);
                }
                state.awaitingHumanInput = false;
                state.humanInputQuestion = null;
                state.humanInputToastId = null;

                // Focus management
                this.$nextTick(() => {
                    const input = this.$refs?.messageInput;
                    if (input?.isConnected && typeof input.focus === 'function') {
                        input.focus({ preventScroll: true });
                    }
                });

                // Start SSE connection
                this.__streamResponse(userMessage, this.__currentThreadId);
            });
        },

        /**
         * Handle keydown in message input
         * @param {KeyboardEvent} event - Keyboard event
         */
        handleKeydown(event) {
            if (event.key === 'Enter' && !event.shiftKey && this.canSendMessage) {
                event.preventDefault();
                this.sendMessage();
            }
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
                    state.messages = this.__convertBackendMessagesToUI(messages);

                    // Scroll to bottom after loading
                    if (id === this.__currentThreadId) {
                        this.$nextTick(() => this.__scrollToBottom());
                    }
                } else {
                    throw new Error(`Failed to fetch messages: ${response.status}`);
                }
            });
        },

        /**
         * Convert backend AG-UI messages to UI display format
         * @param {Array} backendMessages - Messages in AG-UI SDK format
         * @returns {Array} Converted messages for UI display
         */
        __convertBackendMessagesToUI(backendMessages) {
            // Guard: Validate input
            if (!Array.isArray(backendMessages)) {
                console.warn('Invalid messages array:', backendMessages);
                return [];
            }

            const uiMessages = [];

            for (const msg of backendMessages) {
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
                        if (msg.toolCalls && msg.toolCalls.length > 0) {
                            const agentName = msg.name || 'agent';

                            for (const toolCall of msg.toolCalls) {
                                const toolName = toolCall.function?.name || 'unknown';
                                const toolArgs = toolCall.function?.arguments || '{}';

                                uiMessages.push({
                                    id: toolCall.id,
                                    type: 'tool_call',
                                    name: toolName,
                                    args: toolArgs,
                                    status: 'completed',
                                    timestamp: msg.timestamp || new Date().toISOString(),
                                    parsedArgs: safeParseJSON(toolArgs),
                                    result: null,
                                    agentName: agentName
                                });
                            }
                        }
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
                        // console.log('[DEBUG] Raw Tool Message Keys:', Object.keys(msg));
                        const toolCallId = msg.toolCallId || msg.toolCallID || msg.tool_call_id || null;
                        uiMessages.push({
                            id: msg.id,
                            type: 'tool_result',
                            toolCallId: toolCallId,
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
                        toolCall.result = msg.content;
                    } else {
                        pairedMessages.push(msg);
                    }
                } else {
                    pairedMessages.push(msg);
                }
            }

            // Detect awaiting human input from last message
            if (pairedMessages.length > 0) {
                const lastMsg = pairedMessages[pairedMessages.length - 1];



                if (lastMsg.type === 'tool_call' && lastMsg.name === 'elicit_user_input') {

                    const state = this.__getThreadState();
                    state.awaitingHumanInput = true;
                    state.humanInputQuestion = lastMsg.parsedArgs?.question;
                    state.humanInputSchema = lastMsg.parsedArgs?.schema;


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
            state.messages = this.__convertBackendMessagesToUI(event.messages);

            console.log('[MESSAGES_SNAPSHOT] Processing', state.messages.length, 'messages');

            // Scroll to bottom
            if (threadId === this.__currentThreadId) {
                this.$nextTick(() => this.__scrollToBottom());
            }
        },

        /**
         * Safely parse JSON string
         * @param {string} jsonString - JSON string to parse
         * @returns {Object} Parsed object or empty object
         */
        safeParseJSON(jsonString) {
            return safeParseJSON(jsonString);
        }
    };
}
