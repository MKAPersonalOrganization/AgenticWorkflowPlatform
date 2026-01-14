/**
 * Chat Accessibility
 *
 * Handles accessibility features:
 * - Screen reader announcements
 * - Live regions for dynamic content
 */

/**
 * Create accessibility methods mixin
 * @returns {Object} Methods for accessibility
 */
export function createAccessibilityMethods() {
    return {
        /**
         * Initialize screen reader announcer element
         */
        __initScreenReaderAnnouncer() {
            // Create hidden live region for screen reader announcements
            this.__srAnnouncer = document.createElement('div');
            this.__srAnnouncer.setAttribute('role', 'status');
            this.__srAnnouncer.setAttribute('aria-live', 'polite');
            this.__srAnnouncer.setAttribute('aria-atomic', 'true');
            this.__srAnnouncer.className = 'sr-only';
            document.body.appendChild(this.__srAnnouncer);

            // Register cleanup
            this.__cleanupTasks.add(() => {
                if (this.__srAnnouncer) {
                    this.__srAnnouncer.remove();
                }
            });
        },

        /**
         * Announce message to screen readers
         * @param {string} message - Message to announce
         * @param {string} priority - 'polite' (default) or 'assertive'
         */
        __announce(message, priority = 'polite') {
            if (!this.__srAnnouncer) return;

            // Update aria-live if priority changed
            if (this.__srAnnouncer.getAttribute('aria-live') !== priority) {
                this.__srAnnouncer.setAttribute('aria-live', priority);
            }

            // Clear previous announcement
            this.__srAnnouncer.textContent = '';

            // Announce new message (timeout ensures screen readers detect the change)
            setTimeout(() => {
                this.__srAnnouncer.textContent = message;
            }, 100);
        }
    };
}
