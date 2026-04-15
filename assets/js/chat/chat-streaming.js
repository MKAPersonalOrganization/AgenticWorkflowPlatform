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
         * Start SSE streaming for a message using single POST + SSE flow
         * @param {string} message - User message
         * @param {string} threadId - Thread ID
         * @param {string} agentName - Optional agent name to target
         * @param {string|null} toolCallId - Optional tool call ID for elicitation answers
         * @param {Array} attachedFiles - Optional array of file objects [{data, type, name}]
         */
        async __streamResponse(message, threadId, agentName = '', toolCallId = null, attachedFiles = []) {
            const state = this.__getThreadState(threadId);
            
            // Create abort controller for this request
            state.abortController = new AbortController();

            // Close existing stream if any (expected during elicitation responses)
            if (state.eventSource) {
                state.eventSource.close();
                state.eventSource = null;
            }

            state.isStreaming = true;
            state.showTypingIndicator = true;
            state.errorMessage = null;

            try {
                // Build request body
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

                // Attach files as pre-uploaded artifact references (compact, no base64).
                if (attachedFiles && attachedFiles.length > 0) {
                    body.attachments = attachedFiles.map(f => ({
                        url: f.url,
                        mimeType: f.type,
                        fileName: f.name,
                    }));
                }

                // Single POST request that returns SSE stream inline
                // The response is a text/event-stream with AG-UI events
                const response = await fetch('/v1/message:send', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body),
                    signal: state.abortController.signal,
                });

                if (!response.ok) {
                    const errorText = await response.text();
                    throw new Error(`${window.i18n.t('Failed to send message')}: ${response.statusText} - ${errorText}`);
                }

                // Get the content-type to verify it's an SSE stream
                const contentType = response.headers.get('content-type');
                if (!contentType || !contentType.includes('text/event-stream')) {
                    throw new Error('Unexpected response content-type: ' + contentType);
                }

                // Process the SSE stream directly from the response body
                this.__processSSEStream(response.body, threadId);

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
         * Process SSE stream directly from response body (no EventSource)
         * @param {ReadableStream} stream - Response body stream
         * @param {string} threadId - Thread ID
         */
        __processSSEStream(stream, threadId) {
            const state = this.__getThreadState(threadId);
            const decoder = new TextDecoder('utf-8');
            let buffer = '';
            let eventId = '';

            // Use ReadableStream default reader to process the stream
            const reader = stream.getReader();

            const processChunk = () => {
                reader.read().then(({ done, value }) => {
                    if (done) {
                        state.isStreaming = false;
                        state.showTypingIndicator = false;
                        return;
                    }

                    // Decode the chunk and append to buffer
                    buffer += decoder.decode(value, { stream: true });

                    // Process complete SSE events (double newline)
                    const lines = buffer.split('\n');
                    buffer = lines.pop() || ''; // Keep incomplete line in buffer

                    for (const line of lines) {
                        if (line.startsWith('id:')) {
                            eventId = line.slice(3).trim();
                        } else if (line.startsWith('data:')) {
                            const data = line.slice(5).trim();
                            if (data) {
                                try {
                                    const event = JSON.parse(data);
                                    // Track last event ID for potential resume
                                    if (eventId) {
                                        state.lastEventId = eventId;
                                        eventId = '';
                                    }
                                    this.__handleEvent(event, null, threadId);
                                } catch (e) {
                                    console.error('[SSE] Failed to parse event:', e);
                                }
                            }
                        }
                    }

                    // Continue reading
                    processChunk();
                }).catch(error => {
                    console.error('[SSE] Stream error:', error);
                    this.__perfMetrics.errors++;
                    state.isStreaming = false;
                    state.showTypingIndicator = false;
                });
            };

            processChunk();
        },

        /**
         * Stop streaming manually (called when user presses Escape)
         */
        __stopStreaming() {
            const state = this.__currentState;
            
            // Abort the fetch request to stop the SSE stream
            if (state.abortController) {
                state.abortController.abort();
                state.abortController = null;
            }
            
            state.isStreaming = false;
            if (state.heartbeatInterval) {
                clearInterval(state.heartbeatInterval);
                state.heartbeatInterval = null;
            }
            this.__announce(window.i18n.t('Streaming cancelled'));
        },

        /**
         * Handle incoming AG-UI event
         * @param {Object} event - Event object
         * @param {EventSource|null} eventSource - Event source (can be null for direct stream)
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

            if (LIFECYCLE_EVENT_TYPES.includes(event.type)) {
                this.__addLifecycleMessage(event, threadId);
            }

            switch (event.type) {
                case 'SESSION_START':
                    if (event.contextId && !state.threadId) {
                        state.threadId = event.contextId;
                    }
                    break;

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
                case 'TOOL_START':
                    this.__handleToolStart(event.value, state);
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
                case 'TOOL_NEEDS_CONFIRMATION':
                    this.__handleToolNeedsConfirmation(event.value, state, threadId);
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
                case 'AGENT_TRANSFER_STARTED':
                    this.__handleAgentTransferStarted(event.value, state, threadId);
                    break;
                case 'WORKFLOW_NODE_COMPLETED':
                    this.__handleWorkflowNodeCompleted(event.value, state);
                    break;
                case 'STATE_CHANGE_REPORTED':
                    this.__handleStateChangeReported(event.value, state);
                    break;
                case 'ARTIFACT':
                    this.__handleArtifact(event.value, state, threadId);
                    break;
                case 'FILE_ATTACHMENT':
                    this.__handleFileAttachment(event.value, state, threadId);
                    break;
                case 'ARTIFACT_VERSION_SAVED':
                    this.__handleArtifactVersionSaved(event.value, state, threadId);
                    break;
                default:
                    console.warn('[Streaming] Unknown custom event:', event.name);
            }
        },
    };
}