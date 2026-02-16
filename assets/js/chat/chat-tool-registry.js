import { safeParseJSON, normalizeElicitation } from './chat-utils.js';

/**
 * Default tool configuration (Base component logic)
 */
const DEFAULT_TOOL_CONFIG = {
    type: 'standard',
    category: 'general',

    /**
     * Data Mapping Hook (Pure)
     * Transforms raw tool data into UI message properties.
     */
    onMap: (toolCall, toolResult, elicitation) => {
        return {
            parsedResult: toolResult ? safeParseJSON(toolResult) : null
        };
    },

    /**
     * State Synchronization Hook (Side-effect)
     * Handles tool-specific stateful updates (e.g. XState transitions).
     */
    onSync: (toolId, threadId, context, transitionFn) => {
        const { toolName, status, toolResult, elicitation, TOOL_EVENTS } = context;

        // Base XState lifecycle: START -> (SUCCESS | ERROR | NEEDS_INPUT | NEEDS_AUTH)
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
            transitionFn(toolId, TOOL_EVENTS.NEEDS_INPUT, { elicitation }, threadId);
            transitionFn(toolId, TOOL_EVENTS.INPUT_PROVIDED, {}, threadId);
        }
    }
};

/**
 * Specialized Elicitation logic
 */
const ELICITATION_BASE_CONFIG = {
    ...DEFAULT_TOOL_CONFIG,
    type: 'elicitation',
    onMap: (toolCall, toolResult, elicitation) => {
        return {
            parsedResult: elicitation ? normalizeElicitation(elicitation.question, elicitation.schema) : null
        };
    }
};

/**
 * Registry of known tools and their metadata
 */
export const TOOL_REGISTRY = {
    // Collaboration
    'transfer_to_agent': { type: 'special', category: 'collaboration' },
    'save_memory': { type: 'special', category: 'memory' },
    'forget_memory': { type: 'special', category: 'memory' },
    'list_memories': { type: 'special', category: 'memory' },
    'clear_memories': { type: 'special', category: 'memory' },

    // Utilities
    // Content & Search
    'browse_library': { type: 'special', category: 'knowledge' },
    'lookup_section': { type: 'special', category: 'knowledge' },
    'search_text_in_file': { type: 'special', category: 'knowledge' },
    'query_knowledge_base': { type: 'special', category: 'knowledge' },

    // Content Creation
    'create_markdown': { type: 'special', category: 'content' },
    'update_markdown': { type: 'special', category: 'content' },
    'delete_markdown': { type: 'special', category: 'content' },

    // Mathematics & Analysis (Component-owned mapping for display consistency)
    'add': { type: 'special', category: 'math' },
    'subtract': { type: 'special', category: 'math' },
    'multiply': { type: 'special', category: 'math' },
    'divide': { type: 'special', category: 'math' },
    'power': { type: 'special', category: 'math' },
    'sqrt': { type: 'special', category: 'math' },
    'calculate': { type: 'special', category: 'math' },

    'plot': { type: 'special', category: 'visualization' },

    // Statistics
    'mean': { type: 'special', category: 'stats' },
    'median': { type: 'special', category: 'stats' },
    'mode': { type: 'special', category: 'stats' },
    'min': { type: 'special', category: 'stats' },
    'max': { type: 'special', category: 'stats' },
    'sum': { type: 'special', category: 'stats' },
    'variance': { type: 'special', category: 'stats' },
    'standard_deviation': { type: 'special', category: 'stats' },
    'quartile': { type: 'special', category: 'stats' },
    'percentile': { type: 'special', category: 'stats' },
    'interquartile_range': { type: 'special', category: 'stats' },
    'pearson_correlation': { type: 'special', category: 'stats' },
    'euclidean_distance': { type: 'special', category: 'stats' },
    'manhattan_distance': { type: 'special', category: 'stats' },
    'sample': { type: 'special', category: 'stats' },
    'quartile_outliers': { type: 'special', category: 'stats' },
    'normal_pdf': { type: 'special', category: 'stats' },
    'normal_cdf': { type: 'special', category: 'stats' },
    'normal_ppf': { type: 'special', category: 'stats' },
    'normal_mean': { type: 'special', category: 'stats' },
    'normal_median': { type: 'special', category: 'stats' },
    'normal_std': { type: 'special', category: 'stats' },
    'normal_var': { type: 'special', category: 'stats' },
    'normal_interval': { type: 'special', category: 'stats' },
    'binomial_ncr': { type: 'special', category: 'stats' },
    'linear_regression': { type: 'special', category: 'stats' },
};

/**
 * Registry of elicitation schema types
 */
export const ELICITATION_REGISTRY = {
    'choose_multiple': { ...ELICITATION_BASE_CONFIG },
    'true_false': { ...ELICITATION_BASE_CONFIG },
    'multiple_choice': { ...ELICITATION_BASE_CONFIG },
    'short_answer': { ...ELICITATION_BASE_CONFIG }
};

/**
 * Get the full configuration for a tool
 * @param {string} toolName - Name of the tool
 * @param {string|null} schemaType - Optional elicitation schema type
 * @returns {Object} Merged tool configuration
 */
export function getToolConfig(toolName, schemaType = null) {
    const elicitationConfig = schemaType ? ELICITATION_REGISTRY[schemaType] : null;
    const toolConfig = TOOL_REGISTRY[toolName] || null;

    return {
        ...DEFAULT_TOOL_CONFIG,
        ...(toolConfig || {}),
        ...(elicitationConfig || {})
    };
}

/**
 * Check if a tool has special rendering
 */
export function isSpecialTool(toolName) {
    return !!TOOL_REGISTRY[toolName];
}

/**
 * Check if a schema type is for elicitation
 */
export function isElicitationTool(schemaType) {
    return !!ELICITATION_REGISTRY[schemaType];
}
