/**
 * Chat Streaming
 *
 * Handles SSE streaming and AG-UI Protocol event processing including:
 * - Starting and managing SSE connections
 * - Processing all AG-UI event types
 * - Connection retry with exponential backoff
 * - Event logging
 */

import { LIFECYCLE_EVENT_TYPES } from './constants.js';
import { enrichToolResult } from './chat-utils.js';

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
         */
        async __streamResponse(message, threadId, agentName = '') {
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

                case 'TOOL_CALL_START':
                    this.__handleToolCallStart(event, state, threadId);
                    break;

                case 'TOOL_CALL_ARGS':
                    this.__handleToolCallArgs(event, state);
                    break;

                case 'TOOL_CALL_END':
                    this.__handleToolCallEnd(event, state);
                    break;

                case 'TOOL_CALL_RESULT':
                    this.__handleToolCallResult(event, state, threadId);
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
         * Handle CUSTOM event
         */
        __handleCustomEvent(event, state, threadId) {
            if (event.name === 'THREAD_TITLE_UPDATED') {
                const session = this.sessions.find(s => s.id === event.value.threadId);
                if (session) {
                    session.title = event.value.title;
                }
            } else if (event.name === 'TOOL_CALL_AGENT') {
                const { toolCallId, agentName } = event.value;
                state.pendingToolCallAgents = state.pendingToolCallAgents || {};
                state.pendingToolCallAgents[toolCallId] = agentName;

                if (state.toolCalls[toolCallId]) state.toolCalls[toolCallId].agentName = agentName;
                const msg = state.messages.find(m => m.id === toolCallId);
                if (msg) {
                    msg.agentName = agentName;
                    delete state.pendingToolCallAgents[toolCallId];
                }
            } else if (event.name === 'AGENT_CALLED') {
                const { callerAgent, calledAgent, reason } = event.value;
                console.log(`Agent collaboration: ${callerAgent} → ${calledAgent} (${reason})`);
            } else if (event.name === 'TOKEN_USAGE_REPORTED') {
                console.log('Token usage reported:', event.value);
                state.tokenUsage = {
                    total: event.value.total || 0,
                    prompt: event.value.prompt || 0,
                    completion: event.value.completion || 0,
                    cached: event.value.cached || 0,
                    reasoning: event.value.reasoning || 0,
                    audioInput: event.value.audioInput || 0,
                    audioOutput: event.value.audioOutput || 0,
                    acceptedPredict: event.value.acceptedPredict || 0,
                    rejectedPredict: event.value.rejectedPredict || 0,
                    model: event.value.model || 'N/A',
                    provider: event.value.provider || 'N/A',
                    messageId: event.value.messageId,
                    threadId: event.value.threadId
                };
            }
            // Note: HUMAN_INPUT_REQUESTED is no longer emitted by the backend.
            // Elicitation state is derived from pending ask_human tool calls in __convertBackendMessagesToUI
        },

        /**
         * Handle TOOL_CALL_START event
         */
        __handleToolCallStart(event, state, threadId) {
            const agentName = state.pendingToolCallAgents?.[event.toolCallId] || null;
            const parentMessageId = event.parentMessageId || null;

            state.toolCalls[event.toolCallId] = {
                id: event.toolCallId,
                name: event.toolCallName,
                args: '',
                status: 'streaming',
                timestamp: new Date(event.timestamp).toISOString(),
                agentName: agentName,
                parentMessageId: parentMessageId
            };

            state.messages.push({
                id: event.toolCallId,
                type: 'tool_call',
                name: event.toolCallName,
                args: '',
                parsedArgs: {},
                status: 'streaming',
                timestamp: new Date(event.timestamp).toISOString(),
                agentName: agentName,
                parentMessageId: parentMessageId
            });

            if (agentName) {
                delete state.pendingToolCallAgents[event.toolCallId];
            }

            if (threadId === this.__currentThreadId) {
                this.$nextTick(() => this.__scrollToBottom());
            }
        },

        /**
         * Handle TOOL_CALL_ARGS event
         */
        __handleToolCallArgs(event, state) {
            if (state.toolCalls[event.toolCallId]) {
                state.toolCalls[event.toolCallId].args += event.delta;

                const msgIndex = state.messages.findIndex(m => m.id === event.toolCallId);
                if (msgIndex !== -1) {
                    const msg = state.messages[msgIndex];
                    msg.args = state.toolCalls[event.toolCallId].args;
                    msg.parsedArgs = this.safeParseJSON(msg.args);
                }
            }
        },

        /**
         * Handle TOOL_CALL_END event
         */
        __handleToolCallEnd(event, state) {
            if (state.toolCalls[event.toolCallId]) {
                state.toolCalls[event.toolCallId].status = 'completed';

                const toolMsgIndex = state.messages.findIndex(m => m.id === event.toolCallId);
                if (toolMsgIndex !== -1) {
                    const toolMsg = state.messages[toolMsgIndex];
                    toolMsg.status = 'completed';

                    const argsString = toolMsg.args || '{}';
                    const parsed = this.__tryCatch('parse tool args', { toolCallId: event.toolCallId }, () => {
                        return JSON.parse(argsString);
                    });
                    toolMsg.parsedArgs = parsed || { raw: argsString };

                    // Detect elicitation: if this is an elicitation tool, allow component to react via derived state
                    if (['quiz_multiple_choice', 'quiz_numeric_scale', 'quiz_yes_no'].includes(toolMsg.name)) {
                        console.log('Elicitation tool call ended:', toolMsg.parsedArgs?.question_text || toolMsg.parsedArgs?.statement_to_verify);
                        this.__updateDerivedState();
                    }
                }
            }
        },

        /**
         * Handle TOOL_CALL_RESULT event
         */
        __handleToolCallResult(event, state, threadId) {
            const resultToolCallIndex = state.messages.findIndex(m => m.id === event.toolCallId);

            if (resultToolCallIndex !== -1) {
                state.messages[resultToolCallIndex].result = event.content;
                state.messages[resultToolCallIndex].resultTimestamp = new Date(event.timestamp).toISOString();

                // Use shared enrichment logic
                enrichToolResult(state.messages[resultToolCallIndex], event.content);
            }

            if (state.toolCalls[event.toolCallId]) {
                state.toolCalls[event.toolCallId].result = event.content;
            }

            if (threadId === this.__currentThreadId) {
                this.$nextTick(() => this.__scrollToBottom());
            }
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
                this.$nextTick(() => this.__scrollToBottom());
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

            if (threadId === this.__currentThreadId) {
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
        }
    };
}
