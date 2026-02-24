/**
 * Chat Keyboard Shortcuts
 *
 * Global keyboard navigation and shortcuts.
 *
 * Alpine.js pattern: event listeners registered in init() are cleaned up
 * via __cleanupTasks in destroy(). This follows Alpine's lifecycle convention
 * where init() sets up resources and destroy() tears them down.
 *
 * Input guard: shortcuts that conflict with text entry are gated by a
 * target-tag check to avoid firing inside INPUT/TEXTAREA elements.
 */

/**
 * Create keyboard shortcut methods mixin
 * @returns {Object} Methods for keyboard shortcuts
 */
export function createKeyboardMethods() {
    return {
        /**
         * Register global keyboard shortcuts.
         * Called once from init(). Cleanup is registered for destroy().
         */
        __registerKeyboardShortcuts() {
            const controller = new AbortController();

            document.addEventListener('keydown', (e) => {
                // Ctrl+/ — Toggle keyboard shortcuts help
                if ((e.ctrlKey || e.metaKey) && e.key === '/') {
                    e.preventDefault();
                    this.toggleKeyboardShortcuts();
                    return;
                }

                // Escape — Close modals or cancel streaming
                if (e.key === 'Escape') {
                    e.preventDefault();
                    if (this.showKeyboardShortcuts) {
                        this.showKeyboardShortcuts = false;
                    } else if (this.isStreaming) {
                        this.__stopStreaming();
                    }
                    return;
                }

                // Guard: Don't trigger navigation shortcuts inside text fields
                if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
                    return;
                }

                const mod = e.ctrlKey || e.metaKey;

                if (mod && e.shiftKey && e.key === 'N') {
                    e.preventDefault();
                    this.createNewSession();
                } else if (mod && e.key === 'b') {
                    e.preventDefault();
                    this.toggle();
                } else if (mod && e.shiftKey && e.key === 'D') {
                    e.preventDefault();
                    this.toggleDebugPanel();
                } else if (mod && e.shiftKey && e.key === 'T') {
                    e.preventDefault();
                    this.toggleTheme();
                } else if (mod && (e.key === 'ArrowUp' || e.key === 'ArrowDown')) {
                    e.preventDefault();
                    this.__navigateSessions(e.key === 'ArrowUp' ? -1 : 1);
                }
            }, { signal: controller.signal });

            // AbortController.abort() removes all listeners registered with this signal
            this.__cleanupTasks.add(() => controller.abort());
        }
    };
}
