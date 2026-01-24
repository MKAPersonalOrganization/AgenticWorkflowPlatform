/**
 * Chat Tool Registry
 * 
 * Central registry for tool definitions, rendering types, and normalization logic.
 * This decouples the message processing from specific tool implementations.
 */

/**
 * Registry of known tools and their metadata
 */
export const TOOL_REGISTRY = {
    // Collaboration
    'transfer_to_agent': {
        type: 'special',
        category: 'collaboration'
    },

    // Utilities
    'write_todos': {
        type: 'special',
        category: 'productivity'
    },

    // Content & Search
    'browse_library': {
        type: 'special',
        category: 'knowledge'
    },
    'lookup_section': {
        type: 'special',
        category: 'knowledge'
    },
    'search_text_in_file': {
        type: 'special',
        category: 'knowledge'
    },

    // Mathematics & Analysis
    'calculate': { type: 'special', category: 'math' },
    'add': { type: 'special', category: 'math' },
    'subtract': { type: 'special', category: 'math' },
    'multiply': { type: 'special', category: 'math' },
    'divide': { type: 'special', category: 'math' },
    'power': { type: 'special', category: 'math' },
    'sqrt': { type: 'special', category: 'math' },
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
    'choose_multiple': { type: 'elicitation' },
    'numeric_range': { type: 'elicitation' },
    'true_false': { type: 'elicitation' }
};

/**
 * Check if a tool has special rendering
 * @param {string} toolName - Name of the tool
 * @returns {boolean} True if tool has special rendering
 */
export function isSpecialTool(toolName) {
    return !!TOOL_REGISTRY[toolName];
}

/**
 * Check if a schema type is for elicitation
 * @param {string} schemaType - Type of schema
 * @returns {boolean} True if schema is for elicitation
 */
export function isElicitationTool(schemaType) {
    return !!ELICITATION_REGISTRY[schemaType];
}

/**
 * Get tool metadata
 * @param {string} toolName - Name of the tool
 * @returns {Object|null} Tool metadata or null
 */
export function getToolMetadata(toolName) {
    return TOOL_REGISTRY[toolName] || null;
}
