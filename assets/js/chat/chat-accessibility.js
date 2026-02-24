/**
 * Chat Accessibility
 *
 * Screen reader support via ARIA live regions.
 *
 * Alpine.js pattern: the announcer DOM element is created in init() and
 * removed in destroy() — matching Alpine's component lifecycle.
 * The __announce() method uses aria-live for polite/assertive announcements.
 */

/**
 * Create accessibility methods mixin
 * @returns {Object} Methods for accessibility
 */
export function createAccessibilityMethods() {
    return {
        /**
         * Create a hidden ARIA live region for screen reader announcements.
         * Called once from init(). Cleanup is registered for destroy().
         */
        __initScreenReaderAnnouncer() {
            const el = document.createElement('div');
            el.setAttribute('role', 'status');
            el.setAttribute('aria-live', 'polite');
            el.setAttribute('aria-atomic', 'true');
            el.className = 'sr-only';
            document.body.appendChild(el);

            this.__srAnnouncer = el;

            // Alpine pattern: register cleanup for destroy() lifecycle
            this.__cleanupTasks.add(() => this.__srAnnouncer?.remove());
        },

        /**
         * Announce a message to screen readers via the ARIA live region.
         * Clears → waits → sets text to ensure AT detects the change.
         * @param {string} message - Text to announce
         * @param {'polite'|'assertive'} priority - Urgency level
         */
        __announce(message, priority = 'polite') {
            const el = this.__srAnnouncer;
            if (!el) return;

            if (el.getAttribute('aria-live') !== priority) {
                el.setAttribute('aria-live', priority);
            }

            // Clear then set on next frame — ensures screen readers detect the update
            el.textContent = '';
            setTimeout(() => { el.textContent = message; }, 100);
        }
    };
}
