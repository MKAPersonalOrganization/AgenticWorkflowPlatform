/**
 * Chat Constants
 *
 * AG-UI Protocol event types and configuration constants.
 */

// AG-UI Protocol Lifecycle Event Types
export const LIFECYCLE_EVENT_TYPES = [
    'RUN_STARTED', 'RUN_FINISHED', 'RUN_ERROR', 'STEP_STARTED', 'STEP_FINISHED',
    'TEXT_MESSAGE_START', 'TEXT_MESSAGE_END',
    'TOOL_CALL_START', 'TOOL_CALL_ARGS', 'TOOL_CALL_END', 'TOOL_CALL_RESULT',
    'STATE_SNAPSHOT', 'STATE_DELTA', 'MESSAGES_SNAPSHOT',
    'ACTIVITY_SNAPSHOT', 'ACTIVITY_DELTA',
    'RAW', 'CUSTOM',
    'REASONING_START', 'REASONING_MESSAGE_START', 'REASONING_MESSAGE_END', 'REASONING_END',
    'META_EVENT'
];

// Connection resilience configuration
export const CONNECTION_CONFIG = {
    maxRetries: 5,
    baseDelay: 1000,
    maxDelay: 30000,
    backoffMultiplier: 2
};

// Default performance metrics structure
export const DEFAULT_PERF_METRICS = {
    messagesSent: 0,
    messagesReceived: 0,
    errors: 0,
    reconnects: 0,
    avgResponseTime: 0
};

// Available agents for selection
export const AVAILABLE_AGENTS = [
    { id: '', name: 'Auto (Default)' },
    { id: 'math-instructor', name: 'Math Instructor' },
    { id: 'physics-instructor', name: 'Physics Instructor' },
    { id: 'chemistry-instructor', name: 'Chemistry Instructor' },
    { id: 'statistics-instructor', name: 'Statistics Instructor' },
    { id: 'diagramming-instructor', name: 'Diagramming Instructor' }
];
