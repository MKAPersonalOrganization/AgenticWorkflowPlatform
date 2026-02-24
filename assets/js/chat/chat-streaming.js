/**
 * Chat Streaming
 *
 * SSE streaming and AG-UI Protocol event processing.
 *
 * Alpine.js patterns applied:
 * - State mutations (state.isStreaming, state.messages) are on reactive objects
 *   so Alpine auto-updates the UI on change
 * - __scrollToBottomThrottled batches scroll calls during rapid token streaming
 *   (mirrors Alpine's internal scheduler pattern)
 * - Tool state transitions use __batchUpdate for coalesced reactive flushes
 * - Event handlers follow a dispatch-table pattern for maintainability
 */

import { LIFECYCLE_EVENT_TYPES } from './constants.js';
import { enrichToolResult, normalizeElicitation, safeParseJSON } from './chat-utils.js';
import { TOOL_EVENTS } from './tool-state-machine.js';
import { isElicitationTool } from './chat-tool-registry.js';

/**
 * Create streaming methods mixin
 * @returns {Object} Methods for SSE streaming and event handling
 */
export function createStreamingMethods() {
    return {
        /**
         * Start SSE streaming for a message
         * @param {string} message - User message
         * @param {string} threadId - Thread ID
         * @param {string} agentName - Optional agent name to target
         * @param {string|null} toolCallId - Optional tool call ID for elicitation answers
         */
        async __streamResponse(message, threadId, agentName = '', toolCallId = null) {
            const state = this.__getThreadState(threadId);

            // Close existing stream if any (expected during elicitation responses)
            if (state.eventSource) {
                console.log('Closing existing stream for thread:', threadId);
                state.eventSource.close();
                state.eventSource = null;
            }

            state.isStreaming = true;
            state.showTypingIndicator = true;
            state.errorMessage = null;

            try {
                // Step 1: Enqueue the task (A2A compliant)
                const body = {
                    contextId: threadId,
                    message: message,
                };

                if (agentName) {
                    body.agentName = agentName;
                }

                if (toolCallId) {
                    body.toolCallId = toolCallId;
                }

                const response = await fetch('/v1/message:send', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body),
                });

                if (!response.ok) {
                    throw new Error(`${window.i18n.t('Failed to enqueue task')}: ${response.statusText}`);
                }

                const task = await response.json();
                const taskId = task.id;
                const streamUrl = task.metadata?.streamUrl || `/v1/tasks/${taskId}:subscribe`;

                console.log('Task created:', taskId);
                console.log('State:', task.status.state);
                console.log('Stream URL:', streamUrl);

                state.currentJobId = taskId;

                // Step 2: Connect to SSE stream
                this.__connectToJobStream(taskId, threadId);

            } catch (error) {
                console.error('Failed to start stream:', error);
                this.__perfMetrics.errors++;

                state.isStreaming = false;
                state.showTypingIndicator = false;

                if (threadId === this.__currentThreadId) {
                    this.__showError(`${window.i18n.t('Failed to send message')}: ${error.message}`, threadId);
                }
            }
        },

        /**
         * Connect to job SSE stream
         * @param {string} jobId - Job ID
         * @param {string} threadId - Thread ID
         */
        __connectToJobStream(jobId, threadId) {
            const state = this.__getThreadState(threadId);

            // Prevent duplicate connections
            if (state.activeJobStreams.has(jobId)) {
                console.log('Already streaming job:', jobId);
                return;
            }

            state.activeJobStreams.add(jobId);

            const streamUrl = `/v1/tasks/${jobId}:subscribe`;
            const eventSource = new EventSource(streamUrl);

            state.eventSource = eventSource;

            eventSource.onmessage = (event) => {
                state.jobRetryMap.delete(jobId);

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

                state.isStreaming = state.activeJobStreams.size > 0;
                state.showTypingIndicator = state.isStreaming;

                // Retry with exponential backoff
                if (wasConnected || eventSource.readyState === EventSource.CONNECTING) {
                    const currentRetry = state.jobRetryMap.get(jobId) || 0;
                    state.jobRetryMap.set(jobId, currentRetry + 1);

                    const backoffDelay = Math.min(1000 * Math.pow(2, currentRetry), 16000);
                    console.log(`Retrying job ${jobId} in ${backoffDelay}ms (attempt ${currentRetry + 1}/5)`);

                    setTimeout(() => {
                        if (!state.activeJobStreams.has(jobId)) {
                            this.__connectToJobStream(jobId, threadId);
                        }
                    }, backoffDelay);
                } else {
                    console.warn('Job stream not available:', jobId);
                    state.jobRetryMap.delete(jobId);
                }
            };
        },


        /**
         * Cancel the current job
         */
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
                    const response = await fetch(`/v1/tasks/${jobId}:cancel`, {
                        method: 'POST',
                    });

                    if (!response.ok) {
                        throw new Error(`${window.i18n.t('Failed to cancel job')}: ${response.statusText}`);
                    }

                    if (state.eventSource) {
                        state.eventSource.close();
                        state.eventSource = null;
                    }

                    state.activeJobStreams.clear();
                    state.jobRetryMap.clear();
                    state.currentJobId = null;
                    state.isStreaming = false;
                    state.showTypingIndicator = false;

                    console.log('Job cancelled successfully');
                } catch (error) {
                    console.error('Failed to cancel job:', error);
                    this.__showError(window.i18n.t('Failed to cancel job'), this.__currentThreadId);
                }
            });
        },

        /**
         * Stop streaming manually
         */
        __stopStreaming() {
            const state = this.__currentState;
            if (state.eventSource) {
                state.eventSource.close();
                state.eventSource = null;
                state.isStreaming = false;
                if (state.heartbeatInterval) {
                    clearInterval(state.heartbeatInterval);
                    state.heartbeatInterval = null;
                }
                this.__announce(window.i18n.t('Streaming cancelled'));
                console.log('Streaming cancelled by user');
            }
        },

        /**
         * Handle incoming AG-UI event
         * @param {Object} event - Event object
         * @param {EventSource} eventSource - Event source
         * @param {string} threadId - Thread ID
         */
        __handleEvent(event, eventSource, threadId) {
            if (!event || !event.type) {
                console.warn('Invalid event received:', event);
                return;
            }

            const state = this.__getThreadState(threadId);
            if (!state) {
                console.warn('No state found for thread:', threadId);
                return;
            }

            // Log all events
            this.__addEvent(event, threadId);

            // Add lifecycle event as message if not hidden
            if (!this.hideLifecycleEvents && LIFECYCLE_EVENT_TYPES.includes(event.type)) {
                this.__addLifecycleMessage(event, threadId);
            }

            switch (event.type) {
                case 'RUN_STARTED':
                case 'STEP_STARTED':
                case 'STEP_FINISHED':
                case 'STATE_SNAPSHOT':
                case 'STATE_DELTA':
                case 'ACTIVITY_SNAPSHOT':
                case 'ACTIVITY_DELTA':
                case 'REASONING_START':
                case 'REASONING_MESSAGE_START':
                case 'REASONING_MESSAGE_END':
                case 'REASONING_END':
                case 'META_EVENT':
                case 'RAW':
                    // These events are logged but don't require specific handling
                    break;

                case 'MESSAGES_SNAPSHOT':
                    this.__handleMessagesSnapshot(event, threadId);
                    break;

                case 'CUSTOM':
                    this.__handleCustomEvent(event, state, threadId);
                    break;


                case 'TEXT_MESSAGE_START':
                    this.__handleTextMessageStart(event, state, threadId);
                    break;

                case 'TEXT_MESSAGE_CONTENT':
                    this.__handleTextMessageContent(event, state, threadId);
                    break;

                case 'TEXT_MESSAGE_END':
                    this.__handleTextMessageEnd(event, state, threadId);
                    break;

                case 'RUN_FINISHED':
                    this.__handleRunFinished(state, eventSource);
                    break;

                case 'RUN_ERROR':
                    this.__handleRunError(event, state, threadId, eventSource);
                    break;
            }
        },

        /**
         * Handle CUSTOM event by dispatching to specific handlers
         */
        __handleCustomEvent(event, state, threadId) {
            switch (event.name) {
                case 'THREAD_TITLE_UPDATED':
                    this.__handleThreadTitleUpdated(event.value);
                    break;
                case 'TOOL_CALL_AGENT':
                    this.__handleToolCallAgent(event.value, state);
                    break;
                case 'AGENT_CALLED':
                    this.__handleAgentCalled(event.value);
                    break;
                case 'TOKEN_USAGE_REPORTED':
                    this.__handleTokenUsageReported(event.value, state);
                    break;
                // New specific tool lifecycle events
                case 'TOOL_START':
                    this.__handleToolStart(event.value, state);
                    break;
                case 'TOOL_CONTENT_STREAMED':
                    this.__handleToolContentStreamed(event.value, state, threadId);
                    break;
                case 'TOOL_SUCCESS':
                    this.__handleToolSuccess(event.value, state);
                    break;
                case 'TOOL_ERROR':
                    this.__handleToolError(event.value, state);
                    break;
                case 'TOOL_NEEDS_INPUT':
                    this.__handleToolNeedsInput(event.value, state, threadId);
                    break;
                case 'TOOL_NEEDS_AUTH':
                    this.__handleToolNeedsAuth(event.value, state);
                    break;
                case 'INPUT_PROVIDED':
                    this.__handleInputProvided(event.value);
                    break;
                case 'AUTH_GRANTED':
                    this.__handleAuthGranted(event.value);
                    break;
                case 'AUTH_DENIED':
                    this.__handleAuthDenied(event.value);
                    break;
                case 'TOOL_CANCELLED':
                    this.__handleToolCancelled(event.value, state);
                    break;
                default:
                    console.warn('[Streaming] Unknown custom event:', event.name);
            }
        },

        __handleThreadTitleUpdated(value) {
            const session = this.sessions.find(s => s.id === value.threadId);
            if (session) {
                session.title = value.title;
            }
        },

        __handleToolCallAgent(value, state) {
            const { toolCallId, agentName } = value;
            state.pendingToolCallAgents = state.pendingToolCallAgents || {};
            state.pendingToolCallAgents[toolCallId] = agentName;

            if (state.toolCalls[toolCallId]) state.toolCalls[toolCallId].agentName = agentName;
            const msg = state.messages.find(m => m.id === toolCallId);
            if (msg) {
                msg.agentName = agentName;
                delete state.pendingToolCallAgents[toolCallId];
            }
        },

        __handleAgentCalled(value) {
            const { callerAgent, calledAgent, reason } = value;
            console.log(`Agent collaboration: ${callerAgent} → ${calledAgent} (${reason})`);
        },

        __handleTokenUsageReported(value, state) {
            console.log('Token usage reported:', value);
            state.tokenUsage = {
                total: value.total || 0,
                prompt: value.prompt || 0,
                completion: value.completion || 0,
                cached: value.cached || 0,
                reasoning: value.reasoning || 0,
                audioInput: value.audioInput || 0,
                audioOutput: value.audioOutput || 0,
                acceptedPredict: value.acceptedPredict || 0,
                rejectedPredict: value.rejectedPredict || 0,
                model: value.model || 'N/A',
                provider: value.provider || 'N/A',
                messageId: value.messageId,
                threadId: value.threadId
            };
        },

        __handleToolStart(value, state) {
            const { toolCallId, toolName, arguments: toolArgs, agentName } = value;
            console.log(`[Tool] ${toolName} (${toolCallId}) started`);

            this.__ensureToolCallMessage(state, toolCallId, toolName, agentName);

            // Sync arguments
            const msg = state.messages.find(m => m.id === toolCallId);
            if (msg && toolArgs) {
                msg.args = toolArgs;
                msg.parsedArgs = safeParseJSON(toolArgs);
            }

            this.transitionToolState(toolCallId, TOOL_EVENTS.START, { toolName });
        },

        /**
         * Handle TOOL_CONTENT_STREAMED event - streams tool output tokens in real-time.
         *
         * Two streaming phases exist:
         *   1. LLM argument generation: field='__arguments__' — raw JSON argument
         *      deltas are accumulated and partially parsed to extract display fields
         *      (e.g. thoughts for the think tool).
         *   2. Tool execution: field='answer', 'thoughts', etc. — post-execution
         *      streaming from tools that call external APIs (e.g. knowledge base).
         */
        __handleToolContentStreamed(value, state, threadId) {
            const { toolCallId, toolName, delta, field } = value;

            // Lazily create the tool call message if it doesn't exist yet.
            // During LLM argument streaming, TOOL_START may not have arrived yet.
            this.__ensureToolCallMessage(state, toolCallId, toolName);

            const msg = state.messages.find(m => m.id === toolCallId);
            if (!msg) return;

            // Initialize streaming state on the message if not present
            if (!msg.streamingContent) {
                msg.streamingContent = {};
            }
            msg.isStreaming = true;

            // Accumulate streamed content by field
            const fieldKey = field || 'default';
            if (!msg.streamingContent[fieldKey]) {
                msg.streamingContent[fieldKey] = '';
            }
            msg.streamingContent[fieldKey] += delta;

            // --- Phase 1: LLM argument streaming ---
            // The StreamProcessor emits argument deltas with field='__arguments__'.
            // We accumulate raw JSON and try to extract display-ready fields.
            if (fieldKey === '__arguments__') {
                // Update the raw args string
                msg.args = (msg.args || '') + delta;

                // Try to extract fields from partial JSON.
                // For the think tool the schema is {"thoughts": "..."} — we can
                // extract the thoughts value even from incomplete JSON.
                this.__extractStreamingArgs(msg, toolName);
            }
            // --- Phase 2: Tool execution streaming ---
            else if (fieldKey === 'thoughts') {
                if (!msg.parsedArgs) msg.parsedArgs = {};
                msg.parsedArgs.thoughts = msg.streamingContent[fieldKey];
            } else if (fieldKey === 'answer') {
                if (!msg.parsedResult) msg.parsedResult = {};
                msg.parsedResult.answer = msg.streamingContent[fieldKey];
                msg.result = msg.streamingContent[fieldKey];
            }

            // Scroll to bottom during streaming
            if (threadId === this.__currentThreadId && !this.__abortScroll) {
                this.__scrollToBottomThrottled();
            }
        },

        /**
         * Extract display-ready fields from partially streamed JSON arguments.
         * Handles incomplete JSON gracefully — we try to pull out string values
         * even when the JSON is not yet valid.
         */
        __extractStreamingArgs(msg, toolName) {
            const raw = msg.args || '';
            if (!raw) return;

            // Try full JSON parse first
            const parsed = safeParseJSON(raw);
            if (parsed) {
                msg.parsedArgs = parsed;
                return;
            }

            // Fallback: extract partial string values from incomplete JSON.
            // For {"thoughts": "some text that is still being generated...
            // We look for the pattern "key": "value... and extract value.
            if (!msg.parsedArgs) msg.parsedArgs = {};

            // Generic extraction for any string field in progress
            const fieldPattern = /"(\w+)"\s*:\s*"((?:[^"\\]|\\.)*)$/;
            const match = raw.match(fieldPattern);
            if (match) {
                const [, key, partialValue] = match;
                // Unescape JSON escapes
                try {
                    msg.parsedArgs[key] = JSON.parse('"' + partialValue + '"');
                } catch {
                    msg.parsedArgs[key] = partialValue;
                }
            }
        },

        __handleToolSuccess(value, state) {
            const { toolCallId, toolName, result, arguments: toolArgs, agentName } = value;
            console.log(`[Tool] ${toolName} (${toolCallId}) completed successfully`);

            this.__ensureToolCallMessage(state, toolCallId, toolName, agentName);

            // Mark streaming as complete and clean up
            const msg = state.messages.find(m => m.id === toolCallId);
            if (msg) {
                msg.isStreaming = false;
                delete msg.streamingContent;
                if (toolArgs) {
                    msg.args = toolArgs;
                    msg.parsedArgs = safeParseJSON(toolArgs);
                }
            }

            this.transitionToolState(toolCallId, TOOL_EVENTS.SUCCESS, { toolName, result });

            // Notify artifacts panel to refresh
            window.dispatchEvent(new CustomEvent('artifacts-updated'));
        },

        __handleToolError(value, state) {
            const { toolCallId, toolName, error, arguments: toolArgs, agentName } = value;
            console.log(`[Tool] ${toolName} (${toolCallId}) error:`, error);

            this.__ensureToolCallMessage(state, toolCallId, toolName, agentName);

            // Sync arguments if provided
            const msg = state.messages.find(m => m.id === toolCallId);
            if (msg && toolArgs) {
                msg.args = toolArgs;
                msg.parsedArgs = safeParseJSON(toolArgs);
            }

            this.transitionToolState(toolCallId, TOOL_EVENTS.ERROR, { toolName, error });
        },

        __handleToolNeedsInput(value, state, threadId) {
            const { toolCallId, toolName, question, schema, agentName } = value;
            console.log(`[Tool] ${toolName} needs input: ${question}`);

            this.__ensureToolCallMessage(state, toolCallId, toolName, agentName);

            // Transition machine
            this.transitionToolState(toolCallId, TOOL_EVENTS.NEEDS_INPUT, {
                elicitation: { question, schema }
            });

            // Update the message in the list to trigger elicitation UI rendering
            const msg = state.messages.find(m => m.id === toolCallId);
            if (msg) {
                msg.status = 'awaiting_input';
                msg.parsedResult = normalizeElicitation(question, schema);
            }

            if (threadId === this.__currentThreadId) {
                this.showToast(window.i18n.t('Input required'), 'info');
            }
        },

        __handleToolNeedsAuth(value, state) {
            const { toolCallId, toolName, authSpec, agentName } = value;
            console.log(`[Tool] ${toolName} needs authorization: ${authSpec.reason}`);

            this.__ensureToolCallMessage(state, toolCallId, toolName, agentName);
            this.transitionToolState(toolCallId, TOOL_EVENTS.NEEDS_AUTH, { authSpec });
        },

        __handleInputProvided(value) {
            const { toolCallId, input } = value;
            console.log(`[Tool] Input provided for ${toolCallId}`);
            this.transitionToolState(toolCallId, TOOL_EVENTS.INPUT_PROVIDED, { input });
        },

        __handleAuthGranted(value) {
            const { toolCallId } = value;
            console.log(`[Tool] Authorization granted for ${toolCallId}`);
            this.transitionToolState(toolCallId, TOOL_EVENTS.AUTH_GRANTED, {});
        },

        __handleAuthDenied(value) {
            const { toolCallId } = value;
            console.log(`[Tool] Authorization denied for ${toolCallId}`);
            this.transitionToolState(toolCallId, TOOL_EVENTS.AUTH_DENIED, {});
        },

        __handleToolCancelled(value, state) {
            const { toolCallId, toolName } = value;
            console.log(`[Tool] ${toolName} (${toolCallId}) cancelled`);

            this.transitionToolState(toolCallId, TOOL_EVENTS.CANCELLED, { toolName });
        },



        /**
         * Handle TEXT_MESSAGE_START event
         */
        __handleTextMessageStart(event, state, threadId) {
            console.log(`[STREAM] TEXT_MESSAGE_START for messageId: ${event.messageId}`);
            state.showTypingIndicator = false;
            state.currentStreamingId = event.messageId;

            this.__perfMetrics.messagesReceived++;

            const lastUserMsg = state.messages.slice().reverse().find(m => m.role === 'user');
            if (lastUserMsg?._sendTime) {
                const responseTime = Date.now() - lastUserMsg._sendTime;
                this.__perfMetrics.avgResponseTime =
                    (this.__perfMetrics.avgResponseTime * (this.__perfMetrics.messagesReceived - 1) + responseTime) /
                    this.__perfMetrics.messagesReceived;
                console.log(`Response time: ${responseTime}ms (avg: ${Math.round(this.__perfMetrics.avgResponseTime)}ms)`);
            }

            if (threadId === this.__currentThreadId) {
                this.__announce(window.i18n.t('Assistant is responding'));
            }

            // Check if we already have this message (e.g. from history or snapshot)
            const existingMsg = state.messages.find(m => m.id === event.messageId);
            if (!existingMsg) {
                console.log(`[STREAM] Adding new assistant message for ID: ${event.messageId}`);
                state.messages.push({
                    id: event.messageId,
                    role: 'assistant',
                    content: '',
                    timestamp: new Date(event.timestamp || Date.now()).toISOString(),
                    streaming: true
                });
            } else {
                console.log(`[STREAM] Message already exists for ID: ${event.messageId}, marking as streaming`);
                existingMsg.streaming = true;
            }

            if (threadId === this.__currentThreadId) {
                // Reset abort flag at the start of a new streaming message
                this.__abortScroll = false;
                this.$nextTick(() => this.__scrollToBottomStreaming());
            }
        },

        /**
         * Handle TEXT_MESSAGE_CONTENT event
         */
        __handleTextMessageContent(event, state, threadId) {
            let msgIndex = state.messages.findIndex(m => m.id === event.messageId);
            if (msgIndex === -1) {
                console.warn(`[STREAM] TEXT_MESSAGE_CONTENT for unknown ID: ${event.messageId}, lazily creating segment.`);
                state.messages.push({
                    id: event.messageId,
                    role: 'assistant',
                    content: '',
                    timestamp: new Date().toISOString(),
                    streaming: true
                });
                msgIndex = state.messages.length - 1;
            }

            // Append directly to the message's content property
            state.messages[msgIndex].content += event.delta;

            // Log very selectively (first few chars) to avoid log spam but confirm it's working
            if (state.messages[msgIndex].content.length <= event.delta.length) {
                console.log(`[STREAM] First token for ${event.messageId}: "${event.delta}"`);
            }

            if (threadId === this.__currentThreadId && !this.__abortScroll) {
                this.__scrollToBottomThrottled();
            }
        },

        /**
         * Handle TEXT_MESSAGE_END event
         */
        __handleTextMessageEnd(event, state, threadId) {
            console.log(`[STREAM] TEXT_MESSAGE_END for messageId: ${event.messageId}`);
            const endMsgIndex = state.messages.findIndex(m => m.id === event.messageId);
            if (endMsgIndex !== -1) {
                state.messages[endMsgIndex].streaming = false;
            }

            if (threadId === this.__currentThreadId) {
                this.__announce(window.i18n.t('Response complete'));
            }

            if (state.currentStreamingId === event.messageId) {
                state.currentStreamingId = null;
            }
        },

        /**
         * Handle RUN_FINISHED event
         */
        __handleRunFinished(state, eventSource) {
            state.activeJobStreams.clear();
            state.jobRetryMap.clear();
            state.currentJobId = null;
            state.isStreaming = false;
            state.showTypingIndicator = false;
            state.eventSource = null;
            state.retryCount = 0;
            eventSource.close();
        },

        /**
         * Handle RUN_ERROR event
         */
        __handleRunError(event, state, threadId, eventSource) {
            console.error('Run error:', event.message);

            state.activeJobStreams.clear();
            state.isStreaming = false;
            state.showTypingIndicator = false;
            state.eventSource = null;

            if (threadId === this.__currentThreadId) {
                this.__showError(event.message || window.i18n.t('An error occurred'), threadId);
            }
            eventSource.close();
        },

        /**
         * Add lifecycle message to user message
         */
        __addLifecycleMessage(event, threadId) {
            const state = this.__getThreadState(threadId);

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

                if (!state.messages[userMessageIndex].lifecycleEvents) {
                    state.messages[userMessageIndex].lifecycleEvents = [];
                }

                state.messages[userMessageIndex].lifecycleEvents = [
                    ...state.messages[userMessageIndex].lifecycleEvents,
                    lifecycleEvent
                ];
            }
        },

        /**
         * Get human-readable lifecycle event content
         */
        __getLifecycleContent(event) {
            switch (event.type) {
                case 'RUN_STARTED': return window.i18n.t('Started');
                case 'RUN_FINISHED': return event.outcome === 'interrupt' ? window.i18n.t('Paused') : window.i18n.t('Completed');
                case 'RUN_ERROR': return `${window.i18n.t('Error')}: ${event.message || window.i18n.t('Unknown')}`;
                case 'STEP_STARTED': return `${window.i18n.t('Step')}: ${event.stepName || window.i18n.t('Unknown')}`;
                case 'STEP_FINISHED': return `${event.stepName || window.i18n.t('Step')} ${window.i18n.t('Done')}`;
                case 'TEXT_MESSAGE_START': return window.i18n.t('Responding');
                case 'TEXT_MESSAGE_END': return window.i18n.t('Response Done');
                case 'TOOL_CALL_START': return `${window.i18n.t('Tool')}: ${event.toolCallName || window.i18n.t('Unknown')}`;
                case 'TOOL_CALL_ARGS': return window.i18n.t('Args');
                case 'TOOL_CALL_END': return window.i18n.t('Tool Done');
                case 'TOOL_CALL_RESULT': return window.i18n.t('Result');
                case 'STATE_SNAPSHOT': return window.i18n.t('State Snapshot');
                case 'STATE_DELTA': return window.i18n.t('State Update');
                case 'MESSAGES_SNAPSHOT': return window.i18n.t('Messages Sync');
                case 'ACTIVITY_SNAPSHOT': return `${window.i18n.t('Activity')}: ${event.activityType || window.i18n.t('Unknown')}`;
                case 'ACTIVITY_DELTA': return window.i18n.t('Activity Update');
                case 'REASONING_START': return window.i18n.t('Thinking');
                case 'REASONING_MESSAGE_START': return window.i18n.t('Reasoning');
                case 'REASONING_MESSAGE_END': return window.i18n.t('Thought Complete');
                case 'REASONING_END': return window.i18n.t('Thinking Done');
                case 'META_EVENT': return event.metaType || window.i18n.t('Meta');
                case 'RAW': return `${window.i18n.t('Raw')}: ${event.source || window.i18n.t('External')}`;
                case 'CUSTOM': return event.name || window.i18n.t('Custom');
                default: return event.type;
            }
        },

        /**
         * Ensure a tool call message exists in state
         */
        __ensureToolCallMessage(state, toolCallId, toolName, agentName) {
            if (state.toolCalls[toolCallId]) return;

            console.log(`[STREAM] Lazily creating tool call message for ${toolName} (${toolCallId})`);

            state.toolCalls[toolCallId] = {
                id: toolCallId,
                name: toolName,
                args: '',
                status: 'executing',
                timestamp: new Date().toISOString(),
                agentName: agentName
            };

            state.messages.push({
                id: toolCallId,
                type: 'tool_call',
                name: toolName,
                args: '',
                parsedArgs: {},
                parsedResult: null,
                status: 'executing',
                timestamp: new Date().toISOString(),
                agentName: agentName
            });
        }
    };
}
