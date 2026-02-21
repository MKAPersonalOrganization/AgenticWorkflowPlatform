import { safeParseJSON, normalizeElicitation } from './chat-utils.js';

/**
 * Default tool configuration (Base component logic)
 */
const DEFAULT_TOOL_CONFIG = {
    type: 'standard',
    category: 'general',

    onMap: (toolCall, toolResult, elicitation) => {
        return {
            parsedResult: toolResult ? safeParseJSON(toolResult) : null
        };
    },

    onSync: (toolId, threadId, context, transitionFn) => {
        const { toolName, status, toolResult, elicitation, TOOL_EVENTS } = context;

        transitionFn(toolId, TOOL_EVENTS.START, { toolName }, threadId);

        const statusEventMap = {
            'completed': { event: TOOL_EVENTS.SUCCESS, payload: { result: toolResult } },
            'error': { event: TOOL_EVENTS.ERROR, payload: { error: toolResult || 'Unknown error' } },
            'awaiting_input': { event: TOOL_EVENTS.NEEDS_INPUT, payload: { elicitation } },
            'awaiting_auth': { event: TOOL_EVENTS.NEEDS_AUTH, payload: {} }
        };

        if (statusEventMap[status]) {
            const { event, payload } = statusEventMap[status];
            transitionFn(toolId, event, payload, threadId);
        } else if (status === 'input_provided') {
            // Replay: idle → executing → awaiting_input → answered
            transitionFn(toolId, TOOL_EVENTS.NEEDS_INPUT, { elicitation }, threadId);
            transitionFn(toolId, TOOL_EVENTS.INPUT_PROVIDED, { input: context.userAnswer }, threadId);
        } else if (status === 'answered') {
            // Already in terminal answered state on reload
            transitionFn(toolId, TOOL_EVENTS.NEEDS_INPUT, { elicitation }, threadId);
            transitionFn(toolId, TOOL_EVENTS.INPUT_PROVIDED, { input: context.userAnswer }, threadId);
        }
    }
};

const ELICITATION_BASE_CONFIG = {
    ...DEFAULT_TOOL_CONFIG,
    type: 'elicitation',
    onMap: (toolCall, toolResult, elicitation) => {
        return {
            parsedResult: elicitation ? normalizeElicitation(elicitation.question, elicitation.schema) : null
        };
    }
};

export function getToolConfig(toolName, schemaType = null) {
    const elicitationConfig = (schemaType && isElicitationTool(schemaType)) ? ELICITATION_BASE_CONFIG : null;

    return {
        ...DEFAULT_TOOL_CONFIG,
        ...(elicitationConfig || {})
    };
}

export function isSpecialTool(toolName) {
    return window.SPECIAL_TOOLS && window.SPECIAL_TOOLS.includes(toolName);
}

/**
 * Check if a schema type is for elicitation
 */
export function isElicitationTool(schemaType) {
    return window.ELICITATION_SCHEMAS && window.ELICITATION_SCHEMAS.includes(schemaType);
}
