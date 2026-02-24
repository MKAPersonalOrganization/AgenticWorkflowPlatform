/**
 * Chat UI Actions
 *
 * Minimal UI utility methods: toggles, clipboard, and error display.
 * Larger concerns have been extracted to focused modules:
 * - chat-toasts.js: Toast notification system
 * - chat-scroll.js: Scroll management
 * - chat-event-log.js: AG-UI event logging
 * - chat-input.js: Slash commands and file handling
 * - chat-profile.js: Profile and banner CRUD
 */

/**
 * Create UI action methods mixin
 * @returns {Object} Methods for UI actions
 */
export function createUIMethods() {
    return {
        // ============================================
        // Toggle Actions
        // ============================================

        toggle() {
            this.sidebarOpen = !this.sidebarOpen;
        },

        toggleDebugPanel() {
            this.showDebugPanel = !this.showDebugPanel;
        },

        toggleKeyboardShortcuts() {
            this.showKeyboardShortcuts = !this.showKeyboardShortcuts;
        },

        // ============================================
        // Clipboard Operations
        // ============================================

        /**
         * Copy text to clipboard with toast feedback
         * @param {string} text - Text to copy
         * @param {string} successMessage - Message to show on success
         * @returns {Promise<boolean>} Success status
         */
        async copyToClipboard(text, successMessage = window.i18n.t('Copied to clipboard')) {
            try {
                await navigator.clipboard.writeText(text);
                this.showToast(successMessage, 'success', 2000);
                return true;
            } catch (error) {
                console.error('Failed to copy:', error);
                this.showToast(window.i18n.t('Failed to copy to clipboard'), 'error');
                return false;
            }
        },

        /**
         * Copy message content to clipboard
         * @param {string} messageId - ID of the message to copy
         */
        async copyMessage(messageId) {
            const state = this.__getThreadState();
            const message = state.messages.find(m => m.id === messageId);
            if (message) {
                const content = message.content || message.args || '';
                await this.copyToClipboard(content, window.i18n.t('Message copied'));
            }
        },

        /**
         * Copy code block content
         * @param {Event} event - Click event from code block
         */
        async copyCodeBlock(event) {
            const button = event.currentTarget;
            const codeBlock = button.closest('pre')?.querySelector('code');
            if (codeBlock) {
                await this.copyToClipboard(codeBlock.textContent, window.i18n.t('Code copied'));
            }
        },

        // ============================================
        // Error Display
        // ============================================

        /**
         * Show error message with auto-dismiss
         * @param {string} message - Error message
         * @param {string|null} threadId - Thread ID
         */
        __showError(message, threadId = null) {
            const state = this.__getThreadState(threadId);
            state.errorMessage = message;

            if (!threadId || threadId === this.__currentThreadId) {
                this.$nextTick(() => this.__scrollToBottom());
            }

            setTimeout(() => {
                state.errorMessage = null;
            }, 5000);
        },
    };
}
