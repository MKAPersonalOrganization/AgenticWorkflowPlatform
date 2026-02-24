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
    };
}
