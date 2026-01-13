/**
 * Tool Handlers Module
 * 
 * This module contains specialized logic for handling different tool calls
 * from the AGUI message stream. Each tool has its own handler to keep
 * the main chat.js clean and maintainable.
 */

/**
 * Whiteboard Tool Handler
 * Reconstructs whiteboard state from write_whiteboard tool calls
 */
export const WhiteboardHandler = {
    /**
     * Reconstruct whiteboard content from message history
     * This replays all write_whiteboard tool calls to rebuild the whiteboard state
     * @param {Object} session - Session object with whiteboardContent property
     * @param {Array} messages - UI messages array
     * @returns {Object} - { content: string, lastAgent: string|null }
     */
    reconstructFromMessages(session, messages) {
        if (!session) {
            console.warn('[Whiteboard] Session not found for reconstruction');
            return { content: '', lastAgent: null };
        }

        // Reset whiteboard content
        let content = '';
        let lastAgent = null;

        // Replay all write_whiteboard tool calls in order
        for (const msg of messages) {
            if (msg.type === 'tool_call' && msg.name === 'write_whiteboard' && msg.parsedArgs) {
                const { content: toolContent, action } = msg.parsedArgs;
                const agentName = msg.agentName || 'Agent';

                if (toolContent !== undefined) {
                    switch (action) {
                        case 'write':
                            content = toolContent;
                            break;
                        case 'append':
                            content = (content || '') + '\n' + toolContent;
                            break;
                        case 'clear':
                            content = '';
                            break;
                    }
                    lastAgent = agentName;
                }
            }
        }

        console.log('[Whiteboard] Reconstructed from', messages.length, 'messages - final length:', content?.length || 0);

        return { content, lastAgent };
    },

    /**
     * Apply reconstruction result to session
     * @param {Object} session - Session object to update
     * @param {Object} result - Result from reconstructFromMessages
     */
    applyToSession(session, result) {
        if (session) {
            session.whiteboardContent = result.content;
        }
        return result.lastAgent;
    }
};

/**
 * Tool Handler Registry
 * Add new tool handlers here as they are implemented
 */
export const ToolHandlers = {
    whiteboard: WhiteboardHandler,

    // Future tool handlers can be added here:
    // todo: TodoHandler,
    // diagram: DiagramHandler,
    // etc.
};
