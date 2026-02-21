
/**
 * Tool State Machine Factory
 * Mirrors the backend state machine defined in domain/tool_lifecycle.go
 */

// State constants
export const TOOL_STATES = {
    IDLE: 'idle',
    EXECUTING: 'executing',
    AWAITING_INPUT: 'awaiting_input',
    AWAITING_AUTH: 'awaiting_auth',
    ANSWERED: 'answered',
    COMPLETED: 'completed',
    ERROR: 'error'
};

// Event constants
export const TOOL_EVENTS = {
    START: 'TOOL_START',
    SUCCESS: 'TOOL_SUCCESS',
    ERROR: 'TOOL_ERROR',
    NEEDS_INPUT: 'TOOL_NEEDS_INPUT',
    NEEDS_AUTH: 'TOOL_NEEDS_AUTH',
    INPUT_PROVIDED: 'INPUT_PROVIDED',
    AUTH_GRANTED: 'AUTH_GRANTED',
    AUTH_DENIED: 'AUTH_DENIED',
    CANCELLED: 'TOOL_CANCELLED'
};

/**
 * Creates an XState machine definition for a tool call
 * @param {string} toolCallId - ID of the tool call
 * @param {Object} initialContext - Initial context/data
 * @returns {Object} XState machine definition
 */
import { createMachine, assign } from 'xstate';

/**
 * Creates an XState machine definition for a tool call
 * @param {string} toolCallId - ID of the tool call
 * @param {Object} initialContext - Initial context/data
 * @returns {Object} XState machine definition
 */
export function createToolStateMachine(toolCallId, initialContext = {}) {
    return createMachine({
        id: `tool-${toolCallId}`,
        initial: TOOL_STATES.IDLE,
        context: {
            toolCallId,
            toolName: initialContext.toolName || '',
            result: null,
            error: null,
            elicitation: null,
            authSpec: null,
            userAnswer: null,
            ...initialContext
        },
        states: {
            [TOOL_STATES.IDLE]: {
                on: {
                    [TOOL_EVENTS.START]: TOOL_STATES.EXECUTING
                }
            },
            [TOOL_STATES.EXECUTING]: {
                on: {
                    [TOOL_EVENTS.SUCCESS]: {
                        target: TOOL_STATES.COMPLETED,
                        actions: assign({
                            result: ({ event }) => event.result
                        })
                    },
                    [TOOL_EVENTS.ERROR]: {
                        target: TOOL_STATES.ERROR,
                        actions: assign({
                            error: ({ event }) => event.error,
                            result: ({ event }) => event.result // Sometimes error events carry partial results
                        })
                    },
                    [TOOL_EVENTS.NEEDS_INPUT]: {
                        target: TOOL_STATES.AWAITING_INPUT,
                        actions: assign({
                            elicitation: ({ event }) => event.elicitation
                        })
                    },
                    [TOOL_EVENTS.NEEDS_AUTH]: {
                        target: TOOL_STATES.AWAITING_AUTH,
                        actions: assign({
                            authSpec: ({ event }) => event.authSpec
                        })
                    }
                }
            },
            [TOOL_STATES.AWAITING_INPUT]: {
                on: {
                    [TOOL_EVENTS.INPUT_PROVIDED]: {
                        target: TOOL_STATES.ANSWERED,
                        actions: assign({
                            userAnswer: ({ event }) => event.input || null
                        })
                    },
                    [TOOL_EVENTS.CANCELLED]: {
                        target: TOOL_STATES.ERROR,
                        actions: assign({
                            error: ({ event }) => 'Cancelled'
                        })
                    }
                }
            },
            [TOOL_STATES.AWAITING_AUTH]: {
                on: {
                    [TOOL_EVENTS.AUTH_GRANTED]: {
                        target: TOOL_STATES.EXECUTING,
                        actions: assign({
                            authSpec: null
                        })
                    },
                    [TOOL_EVENTS.AUTH_DENIED]: {
                        target: TOOL_STATES.ERROR,
                        actions: assign({
                            error: ({ event }) => 'Auth Denied'
                        })
                    },
                    [TOOL_EVENTS.CANCELLED]: {
                        target: TOOL_STATES.ERROR,
                        actions: assign({
                            error: ({ event }) => 'Cancelled'
                        })
                    }
                }
            },
            [TOOL_STATES.ANSWERED]: {
                type: 'final'
            },
            [TOOL_STATES.COMPLETED]: {
                type: 'final'
            },
            [TOOL_STATES.ERROR]: {
                type: 'final'
            }
        }
    });
}
