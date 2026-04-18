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
import { trackWorkflowEvent } from '../utils/workflow-tracker.js';

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



        /**
         * Handle AGENT_TRANSFER_STARTED — workflow agent transferred control.
         * Tracks the current active agent and full workflow context so new
         * messages get tagged with step/branch/iteration info.
         * Injects visual section markers for parallel and loop boundaries.
         */
        __handleWorkflowNodeCompleted(value, state) {
            trackWorkflowEvent({ type: 'WORKFLOW_NODE_COMPLETED', ...value }, state);
        },

        __handleStateChangeReported(value, state) {
            const { agentName, key, value: val } = value;
            if (!state.workflowOutputs) state.workflowOutputs = {};
            state.workflowOutputs = { ...state.workflowOutputs, [key]: { agentName, value: val } };
        },

        __handleAgentTransferStarted(value, state, threadId) {
            const { fromAgent, toAgent, workflowType, parentAgent, stepIndex, stepCount, iteration, maxIterations } = value;

            trackWorkflowEvent({ type: 'AGENT_TRANSFER_STARTED', ...value }, state);

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
            state.tokenUsage = mapTokenUsage(value);
        },

        // ============================================
        // Tool Lifecycle Handlers
        // ============================================

        __handleToolStart(value, state) {
            const { toolCallId, toolName, arguments: toolArgs, agentName } = value;

            this.__ensureToolCallMessage(state, toolCallId, toolName, agentName);

            // Sync arguments
            const msg = state.messages.find(m => m.id === toolCallId);
            if (msg && toolArgs) {
                msg.args = toolArgs;
                msg.parsedArgs = safeParseJSON(toolArgs);
            }

            this.transitionToolState(toolCallId, TOOL_EVENTS.START, { toolName });
        },

        __handleToolSuccess(value, state) {
            const { toolCallId, toolName, result, arguments: toolArgs, agentName } = value;

            this.__ensureToolCallMessage(state, toolCallId, toolName, agentName);

            // Check if this is a confirmation_required response
            let parsedResult;
            try {
                parsedResult = typeof result === 'string' ? JSON.parse(result) : result;
            } catch {
                parsedResult = null;
            }

            const isConfirmationRequired = parsedResult?.status === 'confirmation_required';

            // Mark streaming as complete and clean up
            const msg = state.messages.find(m => m.id === toolCallId);
            if (msg) {
                msg.isStreaming = false;
                delete msg.streamingContent;
                if (toolArgs) {
                    msg.args = toolArgs;
                    msg.parsedArgs = safeParseJSON(toolArgs);
                }
                // Store the result but don't override awaiting_input status
                if (parsedResult) {
                    msg.parsedResult = parsedResult;
                }
            }

            // Only transition to SUCCESS if NOT confirmation_required
            // (TOOL_NEEDS_INPUT already handled the state transition to AWAITING_INPUT)
            if (!isConfirmationRequired) {
                this.transitionToolState(toolCallId, TOOL_EVENTS.SUCCESS, { toolName, result });
            }

            // Notify artifacts panel to refresh
            window.dispatchEvent(new CustomEvent('artifacts-updated'));
        },

        __handleToolError(value, state) {
            const { toolCallId, toolName, error, arguments: toolArgs, agentName } = value;

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
            const { toolCallId, originalToolCallId, toolName, question, schema, agentName } = value;
            const effectiveId = originalToolCallId || toolCallId;

            this.__ensureToolCallMessage(state, effectiveId, toolName, agentName);

            // Single Source of Truth: 'TOOL_NEEDS_INPUT' always means structured input.
            const schemaObj = typeof schema === 'string' ? safeParseJSON(schema) : schema;
            
            this.transitionToolState(effectiveId, TOOL_EVENTS.NEEDS_INPUT, {
                elicitation: { question, schema: schemaObj }
            });

            const msg = state.messages.find(m => m.id === effectiveId);
            if (msg) {
                msg.status = 'awaiting_input';
                msg.parsedResult = { question_text: question, schema: schemaObj };
            }

            if (threadId === this.__currentThreadId) {
                this.showToast(window.i18n.t('Input required'), 'info');
            }
        },

        __handleToolNeedsConfirmation(value, state, threadId) {
            const { toolCallId, originalToolCallId, toolName, question, agentName } = value;
            const effectiveId = originalToolCallId || toolCallId;

            this.__ensureToolCallMessage(state, effectiveId, toolName, agentName);

            // Transition machine to awaiting confirmation state.
            this.transitionToolState(effectiveId, TOOL_EVENTS.NEEDS_CONFIRMATION, {
                elicitation: { question }
            });

            const msg = state.messages.find(m => m.id === effectiveId);
            if (msg) {
                msg.status = 'awaiting_confirmation'; // NEW specifically for binary
                msg.parsedResult = { question_text: question };
            }

            if (threadId === this.__currentThreadId) {
                this.showToast(window.i18n.t('Confirmation required'), 'info');
            }
        },

        __handleToolNeedsAuth(value, state) {
            const { toolCallId, toolName, authSpec, agentName } = value;

            this.__ensureToolCallMessage(state, toolCallId, toolName, agentName);
            this.transitionToolState(toolCallId, TOOL_EVENTS.NEEDS_AUTH, { authSpec });
        },

        __handleInputProvided(value) {
            const { toolCallId, input } = value;
            this.transitionToolState(toolCallId, TOOL_EVENTS.INPUT_PROVIDED, { input });
        },

        __handleAuthGranted(value) {
            const { toolCallId } = value;
            this.transitionToolState(toolCallId, TOOL_EVENTS.AUTH_GRANTED, {});
        },

        __handleAuthDenied(value) {
            const { toolCallId } = value;
            this.transitionToolState(toolCallId, TOOL_EVENTS.AUTH_DENIED, {});
        },

        __handleToolCancelled(value, state) {
            const { toolCallId, toolName } = value;

            this.transitionToolState(toolCallId, TOOL_EVENTS.CANCELLED, { toolName });
        },

        // ============================================
        // Text Message Handlers
        // ============================================

        /**
         * Handle TEXT_MESSAGE_START event
         */
        __handleTextMessageStart(event, state, threadId) {
            state.showTypingIndicator = false;
            state.currentStreamingId = event.messageId;

            this.__perfMetrics.messagesReceived++;

            const lastUserMsg = state.messages.slice().reverse().find(m => m.role === 'user');
            if (lastUserMsg?._sendTime) {
                const responseTime = Date.now() - lastUserMsg._sendTime;
                this.__perfMetrics.avgResponseTime =
                    (this.__perfMetrics.avgResponseTime * (this.__perfMetrics.messagesReceived - 1) + responseTime) /
                    this.__perfMetrics.messagesReceived;
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
                    }
                } else {
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
            state.activeAgentNames = {};
            state.activeWorkflowContext = null;
            state.lastWorkflowContext = null;
            state.parallelAgentMessages = {};
            state.parallelCoalesceMap = {};
            state.parallelParent = null;
            eventSource?.close();
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
            state.activeAgentNames = {};
            state.activeWorkflowContext = null;
            state.lastWorkflowContext = null;
            state.parallelAgentMessages = {};
            state.parallelCoalesceMap = {};
            state.parallelParent = null;

            if (threadId === this.__currentThreadId) {
                this.__showError(event.message || window.i18n.t('An error occurred'), threadId);
            }
            eventSource?.close();
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

        /**
         * Handle ARTIFACT_VERSION_SAVED event — an agent saved or updated a versioned
         * artifact via the ADK artifact service. Refresh the artifacts sidebar and
         * push a lightweight notification message into the chat.
         */
        __handleArtifactVersionSaved(value, state, threadId) {
            const { fileName, version, agentName } = value;

            state.messages.push({
                id: `artifact_saved_${fileName}_v${version}_${Date.now()}`,
                type: 'artifact_saved',
                fileName: fileName || '',
                version: version || 1,
                agentName: agentName || '',
                timestamp: new Date().toISOString(),
            });

            // Refresh the artifacts sidebar panel.
            window.dispatchEvent(new CustomEvent('artifacts-updated'));

            if (threadId === this.__currentThreadId && !this.__abortScroll) {
                this.$nextTick(() => this.__scrollToBottomStreaming());
            }
        },

        /**
         * Handle CODE_EXECUTION_RAN — the LLM provider's sandbox executed
         * code during this turn. Render a read-only card carrying the
         * source the model generated and the sandbox's output, so the user
         * can see exactly what ran. Code never runs on our servers — the
         * provider isolates the execution.
         */
        __handleCodeExecutionRan(value, state, threadId) {
            const { messageId, language, code, output, outcome, agentName } = value;
            const ok = !outcome || outcome === 'OUTCOME_OK';

            state.messages.push({
                id: `code_exec_${messageId || ''}_${Date.now()}`,
                type: 'code_execution',
                language: (language || 'python').toLowerCase(),
                code: code || '',
                output: output || '',
                outcome: outcome || '',
                ok,
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
