/**
 * Chat Utilities
 *
 * Utility functions for the chat component including:
 * - Performance utilities (throttle, debounce)
 * - Formatting utilities (time, JSON)
 * - ID generators
 * - Error handling wrappers
 */

import { tryCatch, tryCatchAsync } from '../error-handler.js';

/**
 * Throttle function execution
 * @param {Function} func - Function to throttle
 * @param {number} limit - Minimum time between calls in ms
 * @returns {Function} Throttled function
 */
export function throttle(func, limit) {
    let inThrottle;
    return function (...args) {
        if (!inThrottle) {
            func.apply(this, args);
            inThrottle = true;
            setTimeout(() => inThrottle = false, limit);
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
 * Create a user message object
 * @param {string} content - Message content
 * @param {string} interactionId - Interaction ID
 * @returns {Object} User message object
 */
export function createUserMessage(content, interactionId) {
    return {
        id: generateMessageId(),
        role: 'user',
        content: content,
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

    // Async error throwing (Alpine error.js pattern: setTimeout(0))
    setTimeout(() => {
        if (window.onerror) {
            const errorWithContext = Object.assign(
                error || new Error('Unknown error'),
                { operation, context: errorReport.context }
            );
            throw errorWithContext;
        }
    }, 0);
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
                return tryCatch(operation, context, callback, {
                    threadId: component.__currentThreadId,
                    component
                });
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
                return await tryCatchAsync(operation, context, callback, {
                    threadId: component.__currentThreadId,
                    component
                });
            }
        }
    };
}
