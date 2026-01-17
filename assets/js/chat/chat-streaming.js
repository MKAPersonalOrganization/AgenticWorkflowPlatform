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
         */
        async __streamResponse(message, threadId) {
            const state = this.__getThreadState(threadId);

            // Guard: Prevent duplicate connections
            if (state.eventSource) {
                console.warn('Stream already active for thread:', threadId);
                state.eventSource.close();
                state.eventSource = null;
            }

            state.isStreaming = true;
            state.showTypingIndicator = true;
            state.errorMessage = null;

            try {
                // Step 1: Enqueue the task (A2A compliant)
                const response = await fetch('/v1/message:send', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        contextId: threadId,
                        message: message,
                    }),
                });

                if (!response.ok) {
                    throw new Error(`Failed to enqueue task: ${response.statusText}`);
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
                    this.__showError(`Failed to send message: ${error.message}`, threadId);
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
         * Reconnect to all active jobs across sessions
         */
        async __reconnectToAllActiveJobs() {
            try {
                for (const session of this.sessions) {
                    await this.__reconnectToActiveJobs(session.id);
                }
            } catch (error) {
                console.error('Error reconnecting to all active jobs:', error);
            }
        },

        /**
         * Reconnect to active jobs for a thread
         * @param {string} threadId - Thread ID
         */
        async __reconnectToActiveJobs(threadId) {
            try {
                const response = await fetch(`/api/agui/threads/${encodeURIComponent(threadId)}/active-run`);

                if (!response.ok) {
                    console.warn('Failed to fetch active run:', response.statusText);
                    return;
                }

                const data = await response.json();

                if (!data) return;

                if (data.status === 'running') {
                    console.log('Reconnecting to active run:', data);
                    const state = this.__getThreadState(threadId);
                    state.isStreaming = true;
                    state.showTypingIndicator = true;
                    console.log('Active run detected:', data.runId, data.status);
                }
            } catch (error) {
                console.error('Error reconnecting to active jobs:', error);
            }
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
                        throw new Error(`Failed to cancel job: ${response.statusText}`);
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
                    this.__showError('Failed to cancel job', this.__currentThreadId);
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
                this.__announce('Streaming cancelled');
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
            } else if (event.name === 'HUMAN_INPUT_REQUESTED') {
                const { threadId, messageId, toolCallId, question, schema } = event.value;

                // Order matters: set data before enabling the UI to avoid partial/invalid state rendering
                state.humanInputQuestion = question;
                state.humanInputSchema = schema;
                state.awaitingHumanInput = true;

                console.log(`Human input requested for thread ${threadId}: ${question}`, schema);

                if (threadId === this.__currentThreadId) {
                    state.humanInputToastId = this.showToast(question || 'Specialist needs your help', 'warning', 0); // Persistent until dismiss or reply
                }
            }
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
                    state.messages[toolMsgIndex].status = 'completed';

                    const argsString = state.messages[toolMsgIndex].args || '{}';
                    const parsed = this.__tryCatch('parse tool args', { toolCallId: event.toolCallId }, () => {
                        return JSON.parse(argsString);
                    });
                    state.messages[toolMsgIndex].parsedArgs = parsed || { raw: argsString };
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

                // Handle whiteboard tool calls
                if (state.messages[resultToolCallIndex].name === 'write_whiteboard') {
                    this.__handleWhiteboardUpdate(state.messages[resultToolCallIndex], threadId);
                }
            }

            if (state.toolCalls[event.toolCallId]) {
                state.toolCalls[event.toolCallId].result = event.content;
            }

            if (threadId === this.__currentThreadId) {
                this.$nextTick(() => this.__scrollToBottom());
            }
        },

        /**
         * Handle whiteboard update from tool call
         */
        __handleWhiteboardUpdate(toolMessage, threadId) {
            const parsed = toolMessage.parsedArgs;
            if (parsed) {
                const { content, action } = parsed;
                const agentName = toolMessage.agentName || 'Agent';

                this.whiteboardTyping = `${agentName} is writing...`;
                setTimeout(() => { this.whiteboardTyping = null; }, 500);

                const session = this.sessions.find(s => s.id === threadId);
                if (session && content !== undefined) {
                    switch (action) {
                        case 'write':
                            session.whiteboardContent = content;
                            break;
                        case 'append':
                            session.whiteboardContent = (session.whiteboardContent || '') + '\n' + content;
                            break;
                        case 'clear':
                            session.whiteboardContent = '';
                            break;
                    }
                    this.whiteboardLastAgent = agentName;
                }
            }
        },

        /**
         * Handle TEXT_MESSAGE_START event
         */
        __handleTextMessageStart(event, state, threadId) {
            state.showTypingIndicator = false;
            state.currentStreamingId = event.messageId;
            state.currentStreamingContent = '';

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
                this.__announce('Assistant is responding');
            }

            state.messages.push({
                id: event.messageId,
                role: 'assistant',
                content: '',
                timestamp: new Date(event.timestamp).toISOString(),
                streaming: true
            });
        },

        /**
         * Handle TEXT_MESSAGE_CONTENT event
         */
        __handleTextMessageContent(event, state, threadId) {
            state.currentStreamingContent += event.delta;

            const msgIndex = state.messages.findIndex(m => m.id === event.messageId);
            if (msgIndex !== -1) {
                state.messages[msgIndex].content = state.currentStreamingContent;
            }

            if (threadId === this.__currentThreadId) {
                this.__scrollToBottomThrottled();
            }
        },

        /**
         * Handle TEXT_MESSAGE_END event
         */
        __handleTextMessageEnd(event, state, threadId) {
            const endMsgIndex = state.messages.findIndex(m => m.id === event.messageId);
            if (endMsgIndex !== -1) {
                state.messages[endMsgIndex].streaming = false;
            }

            if (threadId === this.__currentThreadId) {
                this.__announce('Response complete');
            }

            state.currentStreamingId = null;
            state.currentStreamingContent = '';
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
                this.__showError(event.message || 'An error occurred', threadId);
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
                case 'RUN_STARTED': return 'Started';
                case 'RUN_FINISHED': return event.outcome === 'interrupt' ? 'Paused' : 'Completed';
                case 'RUN_ERROR': return `Error: ${event.message || 'Unknown'}`;
                case 'STEP_STARTED': return `Step: ${event.stepName || 'Unknown'}`;
                case 'STEP_FINISHED': return `${event.stepName || 'Step'} Done`;
                case 'TEXT_MESSAGE_START': return 'Responding';
                case 'TEXT_MESSAGE_END': return 'Response Done';
                case 'TOOL_CALL_START': return `Tool: ${event.toolCallName || 'Unknown'}`;
                case 'TOOL_CALL_ARGS': return 'Args';
                case 'TOOL_CALL_END': return 'Tool Done';
                case 'TOOL_CALL_RESULT': return 'Result';
                case 'STATE_SNAPSHOT': return 'State Snapshot';
                case 'STATE_DELTA': return 'State Update';
                case 'MESSAGES_SNAPSHOT': return 'Messages Sync';
                case 'ACTIVITY_SNAPSHOT': return `Activity: ${event.activityType || 'Unknown'}`;
                case 'ACTIVITY_DELTA': return 'Activity Update';
                case 'REASONING_START': return 'Thinking';
                case 'REASONING_MESSAGE_START': return 'Reasoning';
                case 'REASONING_MESSAGE_END': return 'Thought Complete';
                case 'REASONING_END': return 'Thinking Done';
                case 'META_EVENT': return event.metaType || 'Meta';
                case 'RAW': return `Raw: ${event.source || 'External'}`;
                case 'CUSTOM': return event.name || 'Custom';
                default: return event.type;
            }
        }
    };
}
