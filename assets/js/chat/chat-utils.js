/**
 * Chat Utilities
 *
 * Pure utility functions — no Alpine dependency, no component state access.
 * These are imported by modules that compose into the Alpine.data('chat') component.
 *
 * Alpine.js design principle: keep utilities as pure functions that receive
 * their inputs as arguments. This makes them testable and decoupled from
 * the reactive system.
 */


/**
 * Throttle function execution
 * @param {Function} func - Function to throttle
 * @param {number} limit - Minimum time between calls in ms
 * @returns {Function} Throttled function
 */
export function throttle(func, limit) {
    let inThrottle;
    let lastArgs;
    let lastThis;
    return function (...args) {
        if (!inThrottle) {
            func.apply(this, args);
            inThrottle = true;
            setTimeout(() => {
                inThrottle = false;
                // Fire trailing call if one was queued during throttle window
                if (lastArgs) {
                    func.apply(lastThis, lastArgs);
                    lastArgs = null;
                    lastThis = null;
                }
            }, limit);
        } else {
            // Queue the latest call for trailing execution
            lastArgs = args;
            lastThis = this;
        }
    };
}

/**
 * Debounce function execution
 * @param {Function} func - Function to debounce
 * @param {number} wait - Wait time in ms
 * @returns {Function} Debounced function
 */
export function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

/**
 * Generate a unique thread ID
 * @returns {string} Thread ID
 */
export function generateThreadId() {
    return `thread_${Date.now()}_${Math.random().toString(36).substring(2, 11)}`;
}

/**
 * Generate a unique message ID
 * @returns {string} Message ID
 */
export function generateMessageId() {
    return `msg_${Date.now()}`;
}

/**
 * Generate a unique interaction ID
 * @returns {string} Interaction ID
 */
export function generateInteractionId() {
    return `interaction_${Date.now()}`;
}

/**
 * Format timestamp for display
 * @param {string} timestamp - ISO timestamp
 * @returns {string} Formatted time string
 */
export function formatTime(timestamp) {
    return new Date(timestamp).toLocaleTimeString('en-US', {
        hour: 'numeric',
        minute: '2-digit'
    });
}

/**
 * Format session time with memoization support
 * @param {string} timestamp - ISO timestamp
 * @param {Object} cache - Memoization cache object
 * @returns {string} Formatted relative time
 */
export function formatSessionTime(timestamp, cache = {}) {
    const cacheKey = `sessionTime_${timestamp}`;

    // Cache for 1 minute to balance freshness and performance
    if (cache[cacheKey]) {
        const cached = cache[cacheKey];
        if (Date.now() - cached.time < 60000) {
            return cached.value;
        }
    }

    const date = new Date(timestamp);
    const now = new Date();
    const diffMs = now - date;
    const diffMins = Math.floor(diffMs / 60000);

    let result;
    if (diffMins < 1) result = 'Just now';
    else if (diffMins < 60) result = `${diffMins}m ago`;
    else {
        const diffHours = Math.floor(diffMins / 60);
        if (diffHours < 24) result = `${diffHours}h ago`;
        else {
            const diffDays = Math.floor(diffHours / 24);
            if (diffDays < 7) result = `${diffDays}d ago`;
            else result = date.toLocaleDateString();
        }
    }

    // Cache result (limit cache size to prevent memory leaks)
    if (Object.keys(cache).length > 100) {
        const entries = Object.entries(cache);
        entries.slice(0, 50).forEach(([key]) => delete cache[key]);
    }

    cache[cacheKey] = { value: result, time: Date.now() };
    return result;
}

/**
 * Check if string is valid JSON
 * @param {string} str - String to check
 * @returns {boolean} True if valid JSON
 */
export function isValidJSON(str) {
    if (!str) return false;
    try {
        JSON.parse(str);
        return true;
    } catch (e) {
        return false;
    }
}

/**
 * Safely parse JSON string
 * @param {string} jsonString - JSON string to parse
 * @returns {Object} Parsed object or empty object on failure
 */
export function safeParseJSON(jsonString) {
    if (!jsonString) return {};
    try {
        return JSON.parse(jsonString);
    } catch (e) {
        // Silently fail - expected during streaming when JSON is incomplete
        return {};
    }
}

/**
 * Format number with thousand separators
 * @param {number} num - Number to format
 * @returns {string} Formatted number string
 */
export function formatNumber(num) {
    if (num === null || num === undefined) return '0';
    return num.toLocaleString('en-US');
}

/**
 * Enrich a tool result message or object with parsed content.
 * This is the SINGLE place where parsedResult is set during message hydration.
 * For tools with elicitation data (confirmation in awaiting_input state), parsedResult
 * is set from the elicitation schema. For standard tools, it's JSON-parsed result.
 * @param {Object} toolResult - The tool result object/message
 * @param {string} content - The JSON string content
 * @returns {Object} The enriched tool result object
 */
export function enrichToolResult(toolResult, content) {
    if (!toolResult) return toolResult;

    // Always set the raw result
    toolResult.result = content;

    // For tools with elicitation data (e.g. confirmation in awaiting_input state),
    // parsedResult must carry the schema so the UI component can render the confirmation.
    if (toolResult.elicitation) {
        const { question, schema } = toolResult.elicitation;
        toolResult.parsedResult = { question_text: question, schema: schema };
    } else {
        toolResult.parsedResult = safeParseJSON(content);
    }

    return toolResult;
}

/**
 * Map raw token usage data to a normalized structure.
 * Deduplicates the mapping between streaming and message loading.
 * @param {Object} raw - Raw token usage data from backend
 * @returns {Object} Normalized token usage
 */
export function mapTokenUsage(raw) {
    return {
        total: raw.total || 0,
        prompt: raw.prompt || 0,
        completion: raw.completion || 0,
        cached: raw.cached || 0,
        reasoning: raw.reasoning || 0,
        audioInput: raw.audioInput || 0,
        audioOutput: raw.audioOutput || 0,
        acceptedPredict: raw.acceptedPredict || 0,
        rejectedPredict: raw.rejectedPredict || 0,
        model: raw.model || 'N/A',
        provider: raw.provider || 'N/A',
        messageId: raw.messageId,
        threadId: raw.threadId
    };
}

/**
 * Create a user message object
 * @param {string} content - Message content
 * @param {string} interactionId - Interaction ID
 * @param {Array} attachments - File attachments [{url, type, name}]
 * @returns {Object} User message object
 */
export function createUserMessage(content, interactionId, attachments = []) {
    return {
        id: generateMessageId(),
        role: 'user',
        content: content,
        // Map to the format expected by the message template:
        //   data = artifact URL (for audio/file playback via streaming)
        //   type = MIME type, name = filename
        attachments: attachments.map(a => ({
            data: a.url || a.data, // prefer artifact URL, fallback to base64
            type: a.type,
            name: a.name,
        })),
        timestamp: new Date().toISOString(),
        interactionId: interactionId,
        lifecycleEvents: []
    };
}

/**
 * Structured logging utility
 * @param {string} level - Log level (info, warn, error)
 * @param {string} message - Log message
 * @param {Object} context - Additional context
 */
export function log(level, message, context = {}) {
    const logData = {
        timestamp: new Date().toISOString(),
        level,
        message,
        ...context
    };

    const logMethod = console[level] || console.log;
    logMethod(`[Chat] ${message}`, logData);
}

/**
 * Enhanced error reporting
 * @param {Error} error - Error object
 * @param {string} operation - Operation that failed
 * @param {Object} context - Additional context
 * @param {Object} metrics - Performance metrics object
 */
export function reportError(error, operation, context = {}, metrics = null) {
    if (metrics) {
        metrics.errors++;
    }

    const errorReport = {
        error: {
            message: error?.message || 'Unknown error',
            stack: error?.stack,
            name: error?.name
        },
        operation,
        context: {
            url: window.location.href,
            userAgent: navigator.userAgent,
            timestamp: new Date().toISOString(),
            ...context
        }
    };

    log('error', `Operation failed: ${operation}`, errorReport);
}

/**
 * Create error handler wrapper for component
 * @param {Object} component - Component instance
 * @returns {Object} Object with tryCatch and tryCatchAsync methods
 */
export function createErrorHandlers(component) {
    return {
        tryCatch(operation, context, callback) {
            try {
                return callback();
            } catch (error) {
                reportError(error, operation, {
                    ...context,
                    threadId: component.__currentThreadId
                }, component.__perfMetrics);
                if (typeof component.__showError === 'function') {
                    component.__showError(`Failed to ${operation}`);
                }
                return null;
            }
        },

        async tryCatchAsync(operation, context, callback) {
            try {
                return await callback();
            } catch (error) {
                reportError(error, operation, {
                    ...context,
                    threadId: component.__currentThreadId
                }, component.__perfMetrics);
                if (typeof component.__showError === 'function') {
                    component.__showError(`Failed to ${operation}`);
                }
                return null;
            }
        }
    };
}
