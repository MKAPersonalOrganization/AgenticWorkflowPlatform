/**
 * Chat Messaging
 *
 * Message send, load, format conversion (backend AG-UI → UI), and snapshot handling.
 *
 * Alpine.js patterns applied:
 * - state.messages is a reactive array; push/splice trigger re-renders
 * - $nextTick() for focus management after DOM updates
 * - $refs for safe DOM element access (messageInput)
 * - Tool state machines (XState) are synced via __batchUpdate to coalesce
 *   multiple transitions into one reactive flush
 */

import { createUserMessage, safeParseJSON, generateInteractionId, enrichToolResult, mapTokenUsage } from './chat-utils.js';
import { TOOL_EVENTS, TOOL_STATES } from './tool-state-machine.js';
import { syncToolState } from './chat-tool-registry.js';
/**
 * Create messaging methods mixin
 * @returns {Object} Methods for message handling
 */
export function createMessagingMethods() {
    return {
        /**
         * Cancel/stop the current streaming job
         */
        cancelJob() {
            this.__stopStreaming();
        },

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
                } else {
                    userMessage = this.currentMessage.trim();
                }

                // Capture attached files — use artifact URLs (pre-uploaded), not base64.
                const attachedFiles = this.files
                    .filter(f => f.type && f.status === 'ready' && f.url)
                    .map(f => ({ url: f.url, type: f.type, name: f.name }));

                // Block send if any files are still uploading.
                const uploading = this.files.some(f => f.status === 'uploading');
                if (uploading) {
                    this.showToast(window.i18n?.t?.('Please wait for file uploads to complete') || 'Please wait for file uploads to complete', 'info');
                    return;
                }

                // Validation — need text OR files
                if (!userMessage && attachedFiles.length === 0) {
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

                // Create and add user message (including image previews)
                const userMsg = createUserMessage(userMessage, interactionId, attachedFiles);
                userMsg._sendTime = sendTime;
                state.messages.push(userMsg);

                // Store current interaction ID
                state.currentInteractionId = interactionId;

                // Capture toolCallId before clearing state (for elicitation correlation)
                const toolCallId = this.humanInputToolCallId || null;

                // Immediately transition the tool's state machine so the quiz
                // buttons hide without waiting for the backend SSE round-trip.
                if (toolCallId) {
                    this.transitionToolState(toolCallId, TOOL_EVENTS.INPUT_PROVIDED, { input: userMessage });
                }

                // Update session title with first message
                this.__updateSessionTitle();

                // Clear the input buffer
                if (overrideMessage === null) {
                    this.currentMessage = '';
                }

                // Clear attached files
                this.files.splice(0);

                // Clear human input state
                if (state.humanInputToastId) {
                    this.dismissToast(state.humanInputToastId);
                }
                state.humanInputToastId = null;

                // Focus management (back to main input)
                this.$nextTick(() => {
                    const input = this.$refs?.messageInput;
                    if (input?.isConnected && typeof input.focus === 'function') {
                        input.focus({ preventScroll: true });
                    }
                });

                // Start SSE connection
                this.__streamResponse(userMessage, this.__currentThreadId, this.selectedAgent, toolCallId, attachedFiles);
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
                    const uiMessages = this.__convertBackendMessagesToUI(messages, data.toolStates || {}, id, data.messageAttachments || {});
                    state.messages = uiMessages;

                    // Side Effect Phase: Sync stateful components (XState)
                    this.__syncThreadToolStates(uiMessages, data.toolStates || {}, id);

                    // Load token usage if available
                    if (data.tokenUsage) {
                        state.tokenUsage = mapTokenUsage(data.tokenUsage);
                    }

                    // Scroll to bottom after loading (tracked deferred retries)
                    if (id === this.__currentThreadId) {
                        this.$nextTick(() => this.__scrollToBottom({ behavior: 'auto', force: true }));
                        this.__deferScroll(() => this.__scrollToBottom({ behavior: 'auto', force: true }), 150, id);
                        this.__deferScroll(() => this.__scrollToBottom({ behavior: 'auto', force: true }), 400, id);
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
         * @param {Object} messageAttachments - Map of messageID → attachment refs from backend (optional)
         * @returns {Array} Converted messages for UI display
         */
        __convertBackendMessagesToUI(backendMessages, toolStatesMap = {}, targetThreadId = null, messageAttachments = {}) {
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
                        uiMessages.push(this.__handleUserMessage(msg, messageAttachments));
                        break;

                    case 'assistant':
                        uiMessages.push(...this.__handleAssistantMessage(msg, toolStatesMap, targetThreadId));
                        break;

                    case 'tool':
                        uiMessages.push(this.__handleToolMessage(msg));
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
        __handleUserMessage(msg, messageAttachments = {}) {
            // Merge persisted attachments from backend (artifact URLs)
            let attachments = msg.attachments || [];
            if (messageAttachments[msg.id] && messageAttachments[msg.id].length > 0) {
                attachments = messageAttachments[msg.id].map(ref => ({
                    data: ref.url,    // artifact URL like /v1/threads/.../artifacts/file-xxx.ext
                    type: ref.mimeType || 'application/octet-stream',
                    name: ref.fileName || ref.url.split('/').pop() || 'file'
                }));
            }
            return {
                id: msg.id,
                role: 'user',
                content: msg.content || '',
                attachments: attachments,
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

            return {
                id: toolCall.id,
                type: 'tool_call',
                name: toolName,
                args: toolArgs,
                parsedArgs: safeParseJSON(toolArgs),
                result: toolResult,
                status: status,
                agentName: agent,
                elicitation: elicitation || null,
                timestamp: toolCall.timestamp || new Date().toISOString(),
            };
        },

        /**
         * State Hydration Phase
         * Creates XState machines directly in their target states, avoiding
         * replay transitions. Uses the dedicated syncToolState function.
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
                const userAnswer = toolStatesMap[toolId]?.input || null;

                syncToolState(toolId, threadId, {
                    toolName, status, toolResult, elicitation, userAnswer
                }, (id, state, ctx, tid) => this.hydrateToolState(id, state, ctx, tid));
            }
        },

        /**
         * Handle tool result messages
         * @private
         */
        __handleToolMessage(msg) {
            return {
                id: msg.id,
                type: 'tool_result',
                role: 'tool',
                toolCallId: msg.toolCallId,
                content: msg.content || '',
                timestamp: msg.timestamp || new Date().toISOString(),
                streaming: false
            };
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

            // Scroll to bottom
            if (threadId === this.__currentThreadId) {
                this.$nextTick(() => this.__scrollToBottom({ behavior: 'auto' }));
            }
        },

    };
}
