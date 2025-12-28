/**
 * Centralized Error Handler
 * Alpine.js pattern for consistent error handling with context
 */

/**
 * Try-catch wrapper with enriched error context
 * @param {string} operation - Operation being performed
 * @param {Object} context - Additional context
 * @param {Function} callback - Function to execute
 * @param {Object} options - Optional configuration
 * @returns {*} Result of callback or null on error
 */
export function tryCatch(operation, context, callback, options = {}) {
    const {
        showError = true,
        errorCallback = null,
        threadId = null,
        component = null
    } = options;

    try {
        return callback();
    } catch (error) {
        const enrichedError = {
            message: error.message,
            stack: error.stack,
            operation,
            context,
            threadId,
            timestamp: new Date().toISOString()
        };

        console.error(`Error [${operation}]:`, enrichedError);

        // Call custom error callback if provided
        if (errorCallback) {
            errorCallback(enrichedError);
        }

        // Show error to user if component reference provided and showError is true
        if (showError && component && typeof component.__showError === 'function') {
            component.__showError(`Failed to ${operation}`);
        }

        return null;
    }
}

/**
 * Async try-catch wrapper with enriched error context
 * @param {string} operation - Operation being performed
 * @param {Object} context - Additional context
 * @param {Function} callback - Async function to execute
 * @param {Object} options - Optional configuration
 * @returns {Promise<*>} Result of callback or null on error
 */
export async function tryCatchAsync(operation, context, callback, options = {}) {
    const {
        showError = true,
        errorCallback = null,
        threadId = null,
        component = null
    } = options;

    try {
        return await callback();
    } catch (error) {
        const enrichedError = {
            message: error.message,
            stack: error.stack,
            operation,
            context,
            threadId,
            timestamp: new Date().toISOString()
        };

        console.error(`Error [${operation}]:`, enrichedError);

        // Call custom error callback if provided
        if (errorCallback) {
            errorCallback(enrichedError);
        }

        // Show error to user if component reference provided and showError is true
        if (showError && component && typeof component.__showError === 'function') {
            component.__showError(`Failed to ${operation}`);
        }

        return null;
    }
}

/**
 * Handle error with context (Alpine pattern)
 * @param {Error} error - Error object
 * @param {Object} context - Error context
 */
export function handleError(error, context = {}) {
    const enrichedError = Object.assign(
        error ?? { message: 'No error message given.' },
        {
            ...context,
            timestamp: new Date().toISOString()
        }
    );

    console.error(
        `Error: ${enrichedError.message}\n\n${context.expression ? 'Expression: "' + context.expression + '"\n\n' : ''}`,
        enrichedError
    );

    // Throw error asynchronously to not interfere with error boundary
    setTimeout(() => {
        throw enrichedError;
    }, 0);
}
