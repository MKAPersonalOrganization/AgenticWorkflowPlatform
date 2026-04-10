/**
 * Workflow State Tracker
 * 
 * Shared utility to update reactive tracking state (active, completed, failed nodes)
 * based on SSE workflow orchestration events.
 * Used by both the main Chat UI and the Agent Designer Test Panel.
 */

export function trackWorkflowEvent(event, state) {
    if (event.type === 'AGENT_TRANSFER_STARTED') {
        const { fromAgent, toAgent, workflowType, parentAgent, stepIndex, stepCount, iteration, maxIterations } = event;

        // Reset node completion tracking when a new workflow run begins
        if (!state.activeWorkflowContext && workflowType) {
            state.completedNodes = {};
            state.failedNodes = {};
            state.workflowOutputs = {};
            state.activeAgentNames = {};
        }

        state.activeAgentName = toAgent;
        
        // Track all concurrently active agents
        const updatedActive = { ...state.activeAgentNames, [toAgent]: true };
        if (workflowType && workflowType !== 'parallel' && fromAgent) {
            delete updatedActive[fromAgent];
        }
        state.activeAgentNames = updatedActive;

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
    } 
    else if (event.type === 'WORKFLOW_NODE_COMPLETED') {
        const { agentName, durationMs, outcome } = event;
        
        if (outcome === 'error') {
            if (!state.failedNodes) state.failedNodes = {};
            state.failedNodes = { ...state.failedNodes, [agentName]: { timestamp: event.timestamp } };
        } else {
            if (!state.completedNodes) state.completedNodes = {};
            state.completedNodes = { ...state.completedNodes, [agentName]: { durationMs, timestamp: event.timestamp } };
        }
        
        // Remove from active set
        if (state.activeAgentNames?.[agentName]) {
            const { [agentName]: _, ...rest } = state.activeAgentNames;
            state.activeAgentNames = rest;
        }
    }
}

/**
 * Track a message for a specific agent in the workflow state.
 * Creates the agent's message array if it doesn't exist.
 */
export function trackAgentMessage(state, agentName, message) {
    if (!state.agentMessages) state.agentMessages = {};
    if (!state.agentMessages[agentName]) {
        state.agentMessages[agentName] = [];
    }
    state.agentMessages[agentName].push(message);
}

/**
 * Append content to the last message of an agent.
 */
export function appendAgentMessageContent(state, agentName, delta) {
    if (!state.agentMessages || !state.agentMessages[agentName]) return;
    const messages = state.agentMessages[agentName];
    const last = messages[messages.length - 1];
    if (last && last.streaming) {
        last.content += delta;
    }
}

/**
 * Mark the last message of an agent as complete (not streaming).
 */
export function finishAgentMessage(state, agentName) {
    if (!state.agentMessages || !state.agentMessages[agentName]) return;
    const messages = state.agentMessages[agentName];
    const last = messages[messages.length - 1];
    if (last && last.streaming) {
        last.streaming = false;
    }
}
