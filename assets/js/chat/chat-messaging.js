/**
 * Chat Messaging
 *
 * Handles message operations including:
 * - Sending messages
 * - Loading messages from server
 * - Converting backend messages to UI format
 * - Message snapshot handling
 */

import { createUserMessage, safeParseJSON, generateInteractionId, enrichToolResult, normalizeElicitation } from './chat-utils.js';
import { TOOL_EVENTS, TOOL_STATES } from './tool-state-machine.js';
import { isElicitationTool, isSpecialTool } from './chat-tool-registry.js';
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
                    state.messages = this.__convertBackendMessagesToUI(messages, data.toolStates || {}, id);

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
        __convertBackendMessagesToUI(backendMessages, toolStatesMap = {}, targetThreadId = null) {
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
                        if (msg.content) {
                            uiMessages.push({
                                id: msg.id,
                                role: 'assistant',
                                content: msg.content,
                                timestamp: msg.timestamp || new Date().toISOString(),
                                streaming: false
                            });
                        }
                        if (msg.toolCalls && msg.toolCalls.length > 0) {
                            const agentName = msg.name || 'agent';

                            for (const toolCall of msg.toolCalls) {
                                const toolName = toolCall.function?.name || 'unknown';
                                const toolArgs = toolCall.function?.arguments || '{}';

                                // Check metadata for tool state overrides
                                let status = 'completed';
                                let agent = agentName;
                                let toolResult = null;
                                let elicitation = null;
                                let parsedToolArgs = safeParseJSON(toolArgs);
                                // Check global tool states map first
                                if (toolStatesMap && toolStatesMap[toolCall.id]) {
                                    const stateObj = toolStatesMap[toolCall.id];
                                    // With XState, we don't need to manually set status here as the machine will do it
                                    // But we might need agent name
                                    if (stateObj.agentName) agent = stateObj.agentName;
                                    if (stateObj.result) toolResult = stateObj.result;
                                    if (stateObj.state) status = stateObj.state;
                                    if (stateObj.elicitation) elicitation = stateObj.elicitation;
                                }

                                // Determine if this is a special tool call or an elicitation tool call
                                const isSpecial = isSpecialTool(toolName);
                                const isElicitation = isElicitationTool(elicitation?.schema?.type);

                                // Initialize and sync XState machine
                                if (toolCall.id) {
                                    // Always start the machine (IDLE -> EXECUTING)
                                    // The machine will synchronously initialize its state in the thread state,
                                    // preventing race conditions during hydration.
                                    this.transitionToolState(toolCall.id, TOOL_EVENTS.START, { toolName }, targetThreadId);

                                    // Sync to specific state if needed
                                    if (status === 'completed') {
                                        this.transitionToolState(toolCall.id, TOOL_EVENTS.SUCCESS, { result: toolResult }, targetThreadId);
                                    } else if (status === 'error') {
                                        this.transitionToolState(toolCall.id, TOOL_EVENTS.ERROR, { error: toolResult || 'Unknown error' }, targetThreadId);
                                    } else if (status === 'awaiting_input') {
                                        this.transitionToolState(toolCall.id, TOOL_EVENTS.NEEDS_INPUT, { elicitation }, targetThreadId);
                                    } else if (status === 'awaiting_auth') {
                                        this.transitionToolState(toolCall.id, TOOL_EVENTS.NEEDS_AUTH, {}, targetThreadId);
                                    } else if (status === 'input_provided') {
                                        // Maintain history: It WAS awaiting input, then provided. 
                                        // The current state should be EXECUTING or COMPLETED depending on subsequent events?
                                        // Actually 'input_provided' is a transient event in backend, but might be a state in our view?
                                        // If backend says 'input_provided', it means we are past the wait.
                                        // So we should be in EXECUTING (after input).
                                        this.transitionToolState(toolCall.id, TOOL_EVENTS.NEEDS_INPUT, { elicitation }, targetThreadId);
                                        this.transitionToolState(toolCall.id, TOOL_EVENTS.INPUT_PROVIDED, {}, targetThreadId);
                                    }
                                }

                                // Normalize result/elicitation
                                let parsedResult = toolResult;
                                if (isElicitation && elicitation) {
                                    parsedResult = normalizeElicitation(elicitation.question, elicitation.schema);
                                } else if (toolResult) {
                                    parsedResult = safeParseJSON(toolResult);
                                }

                                uiMessages.push({
                                    id: toolCall.id,
                                    type: 'tool_call',
                                    name: toolName,
                                    args: toolArgs,
                                    parsedArgs: safeParseJSON(toolArgs),
                                    result: toolResult,
                                    parsedResult: parsedResult,
                                    status: status,
                                    agentName: agent,
                                    timestamp: toolCall.timestamp || new Date().toISOString()
                                });
                            }
                        }
                        break;

                    case 'tool':
                        console.log('[DEBUG] Raw Tool Message Keys:', Object.keys(msg));
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
                        // Use shared enrichment logic
                        enrichToolResult(toolCall, msg.content);
                    } else {
                        pairedMessages.push(msg);
                    }
                } else {
                    pairedMessages.push(msg);
                }
            }

            // Note: awaitingHumanInput is derived centrally via __updateDerivedState() set in chat-state.js

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
            state.messages = this.__convertBackendMessagesToUI(event.messages, {}, threadId);

            // Update derived state
            this.__updateDerivedState();

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
