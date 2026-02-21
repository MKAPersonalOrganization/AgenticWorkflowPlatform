import { TOOL_STATES } from './tool-state-machine.js';

export function isSpecialTool(toolName) {
    return window.SPECIAL_TOOLS && window.SPECIAL_TOOLS.includes(toolName);
}

/**
 * Check if a schema type is for elicitation
 */
export function isElicitationTool(schemaType) {
    return window.ELICITATION_SCHEMAS && window.ELICITATION_SCHEMAS.includes(schemaType);
}

/**
 * Hydrate a tool's XState machine to match the persisted backend state.
 * Instead of replaying transitions (idle → executing → awaiting_input → ...),
 * this creates the machine directly in the target state with full context.
 *
 * @param {string} toolId - Tool call ID
 * @param {string} threadId - Thread ID
 * @param {Object} context - Backend state context
 * @param {Function} hydrateFn - (toolId, targetState, contextData, threadId)
 */
export function syncToolState(toolId, threadId, context, hydrateFn) {
    const { toolName, status, toolResult, elicitation, userAnswer } = context;

    // Map backend status names to XState state values
    const stateMap = {
        'executing': TOOL_STATES.EXECUTING,
        'completed': TOOL_STATES.COMPLETED,
        'error': TOOL_STATES.ERROR,
        'awaiting_input': TOOL_STATES.AWAITING_INPUT,
        'awaiting_auth': TOOL_STATES.AWAITING_AUTH,
        'input_provided': TOOL_STATES.ANSWERED,
        'answered': TOOL_STATES.ANSWERED,
        'auth_granted': TOOL_STATES.EXECUTING,
    };

    const targetState = stateMap[status] || TOOL_STATES.EXECUTING;

    hydrateFn(toolId, targetState, {
        toolName: toolName || '',
        result: toolResult || null,
        elicitation: elicitation || null,
        userAnswer: userAnswer || null,
    }, threadId);
}
