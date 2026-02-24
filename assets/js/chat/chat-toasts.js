/**
 * Chat Toast Notifications
 *
 * Toast notification system for user feedback.
 *
 * Alpine.js pattern: this.toasts is a reactive array — Alpine auto-tracks
 * push/filter mutations for re-rendering. The __announce() call bridges
 * to the accessibility module for screen reader support.
 */

/**
 * Create toast notification methods mixin
 * @returns {Object} Methods for toast notifications
 */
export function createToastMethods() {
    return {
        /**
         * Show a toast notification
         * @param {string} message - The message to display
         * @param {string} type - Type: 'success', 'error', 'info', 'warning'
         * @param {number} duration - Duration in ms (default: 4000, 0 = persistent)
         * @returns {number} Toast ID for manual dismissal
         */
        showToast(message, type = 'info', duration = 4000) {
            const id = ++this.__toastIdCounter;
            const toast = {
                id,
                message,
                type,
                visible: true,
                leaving: false
            };

            this.toasts.push(toast);

            if (duration > 0) {
                setTimeout(() => {
                    this.dismissToast(id);
                }, duration);
            }

            this.__announce(message, type === 'error' ? 'assertive' : 'polite');

            return id;
        },

        /**
         * Dismiss a toast notification
         * @param {number} id - Toast ID to dismiss
         */
        dismissToast(id) {
            const toast = this.toasts.find(t => t.id === id);
            if (toast && !toast.leaving) {
                toast.leaving = true;
                setTimeout(() => {
                    this.toasts = this.toasts.filter(t => t.id !== id);
                }, 200);
            }
        },
    };
}
