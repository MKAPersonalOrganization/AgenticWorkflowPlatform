/**
 * Chat Scroll Management
 *
 * Scroll-to-bottom logic for streaming, thread switching, and user interaction.
 *
 * Alpine.js patterns applied:
 * - $refs for DOM element access (messageList, messagesEnd)
 * - Prefers-reduced-motion respected for accessibility
 * - Streaming scroll uses direct scrollTop (no scrollIntoView animation jitter)
 * - Deferred scrolls are tracked and cleaned up in destroy()
 */

/**
 * Create scroll management methods mixin
 * @returns {Object} Methods for scroll management
 */
export function createScrollMethods() {
    return {
        /**
         * Resolve the effective scroll behavior, respecting prefers-reduced-motion.
         * @param {string} requested - The requested behavior ('auto', 'smooth', 'instant')
         * @returns {string} The resolved behavior
         */
        __resolveScrollBehavior(requested) {
            if (this.preferences?.prefersReducedMotion) {
                return 'auto'; // instant equivalent — no animation
            }
            return requested || 'smooth';
        },

        /**
         * Clear all pending scroll timeouts (prevents stale scrolls on thread switch).
         */
        __clearScrollTimeouts() {
            if (this.__scrollTimeouts?.length) {
                this.__scrollTimeouts.forEach(id => clearTimeout(id));
                this.__scrollTimeouts = [];
            }
        },

        /**
         * Schedule a deferred scroll that is tracked for cleanup.
         * @param {Function} fn - Function to call
         * @param {number} delay - Delay in ms
         * @param {string} threadId - Thread ID to guard against stale calls
         */
        __deferScroll(fn, delay, threadId) {
            const id = setTimeout(() => {
                // Guard: only fire if still on the same thread
                if (this.__currentThreadId === threadId) {
                    fn();
                }
                // Remove from tracking
                this.__scrollTimeouts = (this.__scrollTimeouts || []).filter(t => t !== id);
            }, delay);
            (this.__scrollTimeouts = this.__scrollTimeouts || []).push(id);
        },

        /**
         * Fast scroll-to-bottom for streaming. Uses a direct scrollTop assignment
         * which is synchronous, single-frame, and produces zero animation jitter.
         * This should be the ONLY scroll method called during active token streaming.
         */
        __scrollToBottomStreaming() {
            if (this.__abortScroll) return;

            const container = this.$refs?.messageList;
            if (!container || !container.isConnected) return;

            // Direct assignment — no scrollIntoView, no behavior animation, no reflow fight
            container.scrollTop = container.scrollHeight;
        },

        /**
         * Scroll to bottom of message list (for non-streaming contexts:
         * thread switch, history load, button click, post-stream).
         * @param {Object} options - Scroll options
         * @param {string} options.behavior - Scroll behavior ('auto', 'smooth', 'instant')
         * @param {boolean} options.force - If true, use deferred retries to ensure layout is ready
         */
        __scrollToBottom(options = { behavior: 'smooth', force: false }) {
            // Respect user's abort during streaming
            if (this.__abortScroll && this.isStreaming) {
                return;
            }

            const container = this.$refs?.messageList;
            const anchor = this.$refs?.messagesEnd;

            if (!container || !container.isConnected) {
                return;
            }

            const behavior = this.__resolveScrollBehavior(options.behavior);

            const scroll = () => {
                this.__tryCatch('scroll to bottom', { behavior }, () => {
                    if (anchor && typeof anchor.scrollIntoView === 'function') {
                        anchor.scrollIntoView({
                            behavior: behavior === 'instant' ? 'auto' : behavior,
                            block: 'end'
                        });
                    } else if (typeof container.scrollTo === 'function') {
                        container.scrollTo({
                            top: container.scrollHeight,
                            behavior: behavior
                        });
                    }
                });
            };

            const threadId = this.__currentThreadId;

            if (options.force || behavior === 'auto' || behavior === 'instant') {
                scroll();
                this.__deferScroll(scroll, 50, threadId);
                this.__deferScroll(scroll, 150, threadId);
            } else {
                scroll();
            }
        },

        /**
         * Public scroll to bottom method (for button click).
         * Clears the abort flag so user explicitly re-engages auto-scroll.
         */
        scrollToBottom() {
            this.__abortScroll = false;
            this.__scrollToBottom({ behavior: 'smooth' });
        },

        /**
         * Notify on new message (screen reader announcement).
         * @param {Object} message - Message object
         */
        __notifyNewMessage(message) {
            this.__announce(window.i18n.t('New message received from assistant'));
            console.log('New message received:', message.id);
        },
    };
}
