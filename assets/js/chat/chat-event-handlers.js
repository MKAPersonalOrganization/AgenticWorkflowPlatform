/**
 * Chat AG-UI Event Handlers
 *
 * Specific handler implementations for all AG-UI Protocol events and custom
 * tool lifecycle events. Called by the event dispatcher in chat-streaming.js
 * via the shared component scope (mixin pattern).
 *
 * Alpine.js pattern: all handlers mutate reactive state (state.messages,
 * state.isStreaming, etc.) which triggers automatic UI re-renders.
 * Tool state transitions use transitionToolState() which coalesces updates
 * via __batchUpdate.
 */

import { safeParseJSON, mapTokenUsage } from './chat-utils.js';
import { TOOL_EVENTS } from './tool-state-machine.js';

/**
 * Lifecycle content label generators — lookup table replaces switch statement.
 * Each entry is a function that takes the event and returns the display label.
 * @type {Record<string, (event: Object) => string>}
 */
const LIFECYCLE_LABELS = {
    'RUN_STARTED': () => window.i18n.t('Started'),
    'RUN_FINISHED': (e) => e.outcome === 'interrupt' ? window.i18n.t('Paused') : window.i18n.t('Completed'),
    'RUN_ERROR': (e) => `${window.i18n.t('Error')}: ${e.message || window.i18n.t('Unknown')}`,
    'STEP_STARTED': (e) => `${window.i18n.t('Step')}: ${e.stepName || window.i18n.t('Unknown')}`,
    'STEP_FINISHED': (e) => `${e.stepName || window.i18n.t('Step')} ${window.i18n.t('Done')}`,
    'TEXT_MESSAGE_START': () => window.i18n.t('Responding'),
    'TEXT_MESSAGE_END': () => window.i18n.t('Response Done'),
    'TOOL_CALL_START': (e) => `${window.i18n.t('Tool')}: ${e.toolCallName || window.i18n.t('Unknown')}`,
    'TOOL_CALL_ARGS': () => window.i18n.t('Args'),
    'TOOL_CALL_END': () => window.i18n.t('Tool Done'),
    'TOOL_CALL_RESULT': () => window.i18n.t('Result'),
    'STATE_SNAPSHOT': () => window.i18n.t('State Snapshot'),
    'STATE_DELTA': () => window.i18n.t('State Update'),
    'MESSAGES_SNAPSHOT': () => window.i18n.t('Messages Sync'),
    'ACTIVITY_SNAPSHOT': (e) => `${window.i18n.t('Activity')}: ${e.activityType || window.i18n.t('Unknown')}`,
    'ACTIVITY_DELTA': () => window.i18n.t('Activity Update'),
    'REASONING_START': () => window.i18n.t('Thinking'),
    'REASONING_MESSAGE_START': () => window.i18n.t('Reasoning'),
    'REASONING_MESSAGE_END': () => window.i18n.t('Thought Complete'),
    'REASONING_END': () => window.i18n.t('Thinking Done'),
    'META_EVENT': (e) => e.metaType || window.i18n.t('Meta'),
    'RAW': (e) => `${window.i18n.t('Raw')}: ${e.source || window.i18n.t('External')}`,
    'CUSTOM': (e) => e.name || window.i18n.t('Custom'),
};

/**
 * Create AG-UI event handler methods mixin
 * @returns {Object} Methods for handling specific AG-UI events
 */
export function createEventHandlerMethods() {
    return {
        // ============================================
        // Custom Event Handlers
        // ============================================

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

        /**
         * Handle AGENT_TRANSFER_STARTED — workflow agent transferred control.
         * Tracks the current active agent and full workflow context so new
         * messages get tagged with step/branch/iteration info.
         * Injects visual section markers for parallel and loop boundaries.
         */
        __handleWorkflowNodeCompleted(value, state) {
            const { agentName, durationMs, outcome } = value;
            if (outcome === 'error') {
                if (!state.failedNodes) state.failedNodes = {};
                state.failedNodes = { ...state.failedNodes, [agentName]: { timestamp: value.timestamp } };
            } else {
                if (!state.completedNodes) state.completedNodes = {};
                state.completedNodes = { ...state.completedNodes, [agentName]: { durationMs, timestamp: value.timestamp } };
            }
        },

        __handleStateChangeReported(value, state) {
            const { agentName, key, value: val } = value;
            if (!state.workflowOutputs) state.workflowOutputs = {};
            state.workflowOutputs = { ...state.workflowOutputs, [key]: { agentName, value: val } };
        },

        __handleAgentTransferStarted(value, state, threadId) {
            const { fromAgent, toAgent, workflowType, parentAgent, stepIndex, stepCount, iteration, maxIterations } = value;
            console.log(`[Workflow] Agent transfer: ${fromAgent} → ${toAgent} (${workflowType || 'direct'}, step ${(stepIndex || 0) + 1}/${stepCount || '?'})`);

            // Reset node completion tracking when a new workflow run begins.
            // activeWorkflowContext being null means no prior workflow context existed
            // in this thread state — i.e. this is the first transfer of a fresh run.
            if (!state.activeWorkflowContext && workflowType) {
                state.completedNodes = {};
                state.failedNodes = {};
                state.workflowOutputs = {};
            }

            state.activeAgentName = toAgent;

            let orchestratorId = state.activeWorkflowContext?.orchestratorId;
            if (workflowType) {
                if (!orchestratorId) {
                    orchestratorId = fromAgent;
                }
            } else {
                orchestratorId = null;
            }

            state.activeWorkflowContext = workflowType
                ? { workflowType, parentAgent, stepIndex, stepCount, iteration, maxIterations, orchestratorId }
                : null;

            // Reset parallel coalesce maps when leaving a parallel section
            // or entering a different parallel section.
            if (workflowType === 'parallel') {
                if (state.parallelParent !== parentAgent) {
                    state.parallelAgentMessages = {};
                    state.parallelCoalesceMap = {};
                    state.parallelParent = parentAgent;
                }
            } else {
                state.parallelAgentMessages = {};
                state.parallelCoalesceMap = {};
                state.parallelParent = null;
            }

            // Inject workflow section markers when entering a new workflow
            // section or starting a new loop iteration.
            if (workflowType) {
                const prev = state.lastWorkflowContext;
                const isNewSection = !prev || prev.parentAgent !== parentAgent || prev.workflowType !== workflowType;
                const isNewIteration = workflowType === 'loop' && prev?.parentAgent === parentAgent && iteration > (prev.iteration || 0);

                if (isNewSection && (workflowType === 'parallel' || workflowType === 'loop' || workflowType === 'conditional')) {
                    state.messages.push({
                        id: `wf_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
                        type: 'workflow_marker',
                        markerType: workflowType,
                        parentAgent,
                        stepCount,
                        maxIterations,
                        iteration: iteration || 1,
                        timestamp: new Date().toISOString(),
                    });
                } else if (isNewIteration) {
                    state.messages.push({
                        id: `wf_iter_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`,
                        type: 'workflow_marker',
                        markerType: 'loop_iteration',
                        parentAgent,
                        iteration,
                        maxIterations,
                        timestamp: new Date().toISOString(),
                    });
                }

                state.lastWorkflowContext = { workflowType, parentAgent, stepIndex, stepCount, iteration, maxIterations };
            }
        },

        __handleTokenUsageReported(value, state) {
            console.log('Token usage reported:', value);
            state.tokenUsage = mapTokenUsage(value);
        },

        // ============================================
        // Tool Lifecycle Handlers
        // ============================================

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
         * Handle TOOL_CONTENT_STREAMED event — streams tool output tokens in real-time.
         *
         * Two streaming phases:
         *   1. LLM argument generation: field='__arguments__' — raw JSON deltas
         *   2. Tool execution: field='answer', 'thoughts', etc. — post-execution streaming
         */
        __handleToolContentStreamed(value, state, threadId) {
            const { toolCallId, toolName, delta, field } = value;

            // Lazily create the tool call message if it doesn't exist yet.
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
            if (fieldKey === '__arguments__') {
                msg.args = (msg.args || '') + delta;
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
         * Handles incomplete JSON gracefully.
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
            if (!msg.parsedArgs) msg.parsedArgs = {};

            // Generic extraction for any string field in progress
            const fieldPattern = /"(\w+)"\s*:\s*"((?:[^"\\]|\\.)*)$/;
            const match = raw.match(fieldPattern);
            if (match) {
                const [, key, partialValue] = match;
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

            // Update the message in the list to trigger UI rendering
            const msg = state.messages.find(m => m.id === toolCallId);
            if (msg) {
                msg.status = 'awaiting_input';
                msg.parsedResult = { question_text: question, schema: schema };
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

        // ============================================
        // Text Message Handlers
        // ============================================

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
                const wfType = state.activeWorkflowContext?.workflowType;
                const agentName = state.activeAgentName;

                // Parallel coalescing: if this agent already has a message
                // in the current parallel section, redirect to it instead
                // of creating a new one-word message bubble.
                if (wfType === 'parallel' && agentName && state.parallelAgentMessages[agentName]) {
                    const canonicalId = state.parallelAgentMessages[agentName];
                    state.parallelCoalesceMap[event.messageId] = canonicalId;
                    const canonical = state.messages.find(m => m.id === canonicalId);
                    if (canonical) {
                        canonical.streaming = true;
                        console.log(`[STREAM] Coalescing parallel msg ${event.messageId} → ${canonicalId} (${agentName})`);
                    }
                } else {
                    console.log(`[STREAM] Adding new assistant message for ID: ${event.messageId}`);
                    state.messages.push({
                        id: event.messageId,
                        role: 'assistant',
                        content: '',
                        timestamp: new Date(event.timestamp || Date.now()).toISOString(),
                        streaming: true,
                        agentName: agentName || null,
                        workflowType: state.activeWorkflowContext?.workflowType || null,
                        parentAgent: state.activeWorkflowContext?.parentAgent || null,
                        stepIndex: state.activeWorkflowContext?.stepIndex ?? null,
                        stepCount: state.activeWorkflowContext?.stepCount ?? null,
                        iteration: state.activeWorkflowContext?.iteration ?? null,
                        maxIterations: state.activeWorkflowContext?.maxIterations ?? null,
                    });

                    // Track as the canonical message for this parallel agent
                    if (wfType === 'parallel' && agentName) {
                        state.parallelAgentMessages[agentName] = event.messageId;
                    }
                }
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
            // Resolve coalesced parallel message ID
            const resolvedId = state.parallelCoalesceMap[event.messageId] || event.messageId;
            let msgIndex = state.messages.findIndex(m => m.id === resolvedId);
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

            // Log very selectively (first few chars) to avoid log spam but confirm it's working
            if (!this.__tokenBuffer?.[resolvedId] && state.messages[msgIndex].content.length === 0) {
                console.log(`[STREAM] First token for ${event.messageId}: "${event.delta}"`);
            }

            // Buffer delta into a plain (non-reactive) map — avoids triggering Alpine
            // reactivity (and the full markdown render pipeline) on every single token.
            // A single requestAnimationFrame flush caps re-renders at ~60fps.
            if (!this.__tokenBuffer) this.__tokenBuffer = {};
            this.__tokenBuffer[resolvedId] = (this.__tokenBuffer[resolvedId] || '') + event.delta;

            if (!this.__tokenRafId) {
                this.__tokenRafId = requestAnimationFrame(() => {
                    this.__flushTokenBuffer(state);
                    this.__tokenRafId = null;
                });
            }

            if (threadId === this.__currentThreadId && !this.__abortScroll) {
                this.__scrollToBottomThrottled();
            }

            // Drive the navbar brand logo rotation — 1° per token
            window.dispatchEvent(new CustomEvent('ai-token'));
        },

        /**
         * Flush buffered token deltas into Alpine reactive state.
         * Called by the RAF callback and on TEXT_MESSAGE_END.
         */
        __flushTokenBuffer(state) {
            if (!this.__tokenBuffer) return;
            for (const [resolvedId, delta] of Object.entries(this.__tokenBuffer)) {
                const idx = state.messages.findIndex(m => m.id === resolvedId);
                if (idx !== -1) state.messages[idx].content += delta;
            }
            this.__tokenBuffer = {};
        },

        /**
         * Handle TEXT_MESSAGE_END event
         */
        __handleTextMessageEnd(event, state, threadId) {
            // Resolve coalesced parallel message ID
            const resolvedId = state.parallelCoalesceMap[event.messageId] || event.messageId;
            console.log(`[STREAM] TEXT_MESSAGE_END for messageId: ${event.messageId}${resolvedId !== event.messageId ? ` (coalesced → ${resolvedId})` : ''}`);

            // Cancel any pending RAF and flush remaining buffered tokens.
            // The KaTeX cache + output cache in renderMarkdown make this final
            // render fast — most expressions are already cached from streaming.
            if (this.__tokenRafId) {
                cancelAnimationFrame(this.__tokenRafId);
                this.__tokenRafId = null;
            }
            this.__flushTokenBuffer(state);

            const endMsgIndex = state.messages.findIndex(m => m.id === resolvedId);
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

        // ============================================
        // Run Lifecycle Handlers
        // ============================================

        /**
         * Handle RUN_FINISHED event
         */
        __handleRunFinished(state, eventSource) {
            state.activeJobStreams.clear();
            state.jobRetryMap.clear();
            state.jobSeqIdMap?.clear();
            state.currentJobId = null;
            state.isStreaming = false;
            state.showTypingIndicator = false;
            state.eventSource = null;
            state.retryCount = 0;
            state.activeAgentName = null;
            state.activeWorkflowContext = null;
            state.lastWorkflowContext = null;
            state.parallelAgentMessages = {};
            state.parallelCoalesceMap = {};
            state.parallelParent = null;
            eventSource.close();
        },

        /**
         * Handle RUN_ERROR event
         */
        __handleRunError(event, state, threadId, eventSource) {
            console.error('Run error:', event.message);

            state.activeJobStreams.clear();
            state.jobSeqIdMap?.clear();
            state.isStreaming = false;
            state.showTypingIndicator = false;
            state.eventSource = null;
            state.activeAgentName = null;
            state.activeWorkflowContext = null;
            state.lastWorkflowContext = null;
            state.parallelAgentMessages = {};
            state.parallelCoalesceMap = {};
            state.parallelParent = null;

            if (threadId === this.__currentThreadId) {
                this.__showError(event.message || window.i18n.t('An error occurred'), threadId);
            }
            eventSource.close();
        },

        // ============================================
        // Lifecycle Message Display
        // ============================================

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
         * Get human-readable lifecycle event content.
         * Uses lookup table instead of switch for maintainability.
         */
        __getLifecycleContent(event) {
            const labelFn = LIFECYCLE_LABELS[event.type];
            return labelFn ? labelFn(event) : event.type;
        },

        // ============================================
        // Artifact Handler
        // ============================================

        /**
         * Handle ARTIFACT event — renders a document/file produced by any agent as a card.
         * Generic UI affordance; no knowledge of the underlying transport (A2A, tool, etc.).
         */
        __handleArtifact(value, state, threadId) {
            const { id, title, content, mimeType, agentName } = value;

            state.messages.push({
                id: `artifact_${id || Date.now()}`,
                type: 'artifact',
                artifactId: id || '',
                title: title || '',
                content: content || '',
                mimeType: mimeType || 'text/plain',
                agentName: agentName || '',
                timestamp: new Date().toISOString(),
            });

            if (threadId === this.__currentThreadId && !this.__abortScroll) {
                this.$nextTick(() => this.__scrollToBottomStreaming());
            }
        },

        /**
         * Handle FILE_ATTACHMENT event — renders a downloadable file card.
         * Generic UI affordance; no knowledge of the underlying transport.
         */
        __handleFileAttachment(value, state, threadId) {
            const { fileUrl, fileName, mimeType, agentName } = value;

            state.messages.push({
                id: `file_${Date.now()}`,
                type: 'file_attachment',
                fileUrl: fileUrl || '',
                fileName: fileName || '',
                mimeType: mimeType || 'application/octet-stream',
                agentName: agentName || '',
                timestamp: new Date().toISOString(),
            });

            if (threadId === this.__currentThreadId && !this.__abortScroll) {
                this.$nextTick(() => this.__scrollToBottomStreaming());
            }
        },

        // ============================================
        // ============================================
        // Tool Call Message Helpers
        // ============================================

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
                agentName: agentName,
                workflowType: state.activeWorkflowContext?.workflowType || null,
                parentAgent: state.activeWorkflowContext?.parentAgent || null,
                stepIndex: state.activeWorkflowContext?.stepIndex ?? null,
                stepCount: state.activeWorkflowContext?.stepCount ?? null,
            });
        }
    };
}
