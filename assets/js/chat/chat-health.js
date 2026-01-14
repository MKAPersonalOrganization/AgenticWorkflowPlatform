/**
 * Chat Connection Health
 *
 * Handles connection monitoring and diagnostics:
 * - Connection health checks
 * - Performance metrics
 * - Diagnostic exports
 */

import { DEFAULT_PERF_METRICS } from './constants.js';

/**
 * Create connection health methods mixin
 * @returns {Object} Methods for connection health monitoring
 */
export function createHealthMethods() {
    return {
        /**
         * Check connection health
         * @returns {Object} Health status
         */
        getConnectionHealth() {
            const state = this.__getThreadState();
            const isConnected = state?.eventSource?.readyState === EventSource.OPEN;
            const isConnecting = state?.eventSource?.readyState === EventSource.CONNECTING;

            return {
                status: isConnected ? 'connected' : (isConnecting ? 'connecting' : 'disconnected'),
                isStreaming: state?.isStreaming || false,
                retryCount: state?.retryCount || 0,
                maxRetries: this.__connectionConfig.maxRetries,
                hasErrors: this.__perfMetrics.errors > 0
            };
        },

        /**
         * Get performance metrics
         * @returns {Object} Performance metrics
         */
        getMetrics() {
            return {
                ...this.__perfMetrics,
                connectionHealth: this.getConnectionHealth(),
                cacheSize: Object.keys(this.__memoCache).length,
                activeThreads: Object.keys(this.__threadStates).length,
                queueSize: this.__updateQueue.length
            };
        },

        /**
         * Reset performance metrics
         */
        resetMetrics() {
            this.__perfMetrics = { ...DEFAULT_PERF_METRICS };
            this.__log('info', 'Metrics reset');
        },

        /**
         * Health check
         * @returns {boolean} True if healthy
         */
        isHealthy() {
            const health = this.getConnectionHealth();
            const isOverRetrying = (health.retryCount / health.maxRetries) > 0.8;

            return health.status === 'connected' && !isOverRetrying;
        },

        /**
         * Export diagnostic data
         * @returns {Object} Complete diagnostic data
         */
        exportDiagnostics() {
            return {
                timestamp: new Date().toISOString(),
                version: '1.0.0',
                initialized: this.__initialized,
                currentThread: this.__currentThreadId,
                sessions: this.sessions.length,
                threads: Object.keys(this.__threadStates).map(id => ({
                    id,
                    messageCount: this.__threadStates[id]?.messages?.length || 0,
                    isStreaming: this.__threadStates[id]?.isStreaming || false,
                    retryCount: this.__threadStates[id]?.retryCount || 0
                })),
                metrics: this.getMetrics(),
                health: this.getConnectionHealth(),
                config: {
                    retryConfig: this.__connectionConfig
                }
            };
        }
    };
}
