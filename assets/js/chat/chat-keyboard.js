/**
 * Chat Keyboard Shortcuts
 *
 * Handles keyboard navigation and shortcuts:
 * - Global shortcuts (new chat, toggle sidebar, etc.)
 * - Modal dismissal
 * - Session navigation
 */

/**
 * Create keyboard shortcut methods mixin
 * @returns {Object} Methods for keyboard shortcuts
 */
export function createKeyboardMethods() {
    return {
        /**
         * Register keyboard shortcuts
         */
        __registerKeyboardShortcuts() {
            const handler = (e) => {
                // ? or Ctrl+/ - Show keyboard shortcuts (works even in inputs)
                if (e.key === '?' || ((e.ctrlKey || e.metaKey) && e.key === '/')) {
                    e.preventDefault();
                    this.toggleKeyboardShortcuts();
                    return;
                }

                // Escape - Close modals or cancel streaming
                if (e.key === 'Escape') {
                    e.preventDefault();
                    if (this.showKeyboardShortcuts) {
                        this.showKeyboardShortcuts = false;
                    } else if (this.isStreaming) {
                        this.__stopStreaming();
                    }
                    return;
                }

                // Don't trigger other shortcuts when typing in input fields
                if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') {
                    return;
                }

                // Ctrl/Cmd + Shift + N - New chat
                if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'N') {
                    e.preventDefault();
                    this.createNewSession();
                }

                // Ctrl/Cmd + B - Toggle sidebar
                if ((e.ctrlKey || e.metaKey) && e.key === 'b') {
                    e.preventDefault();
                    this.toggle();
                }

                // Ctrl/Cmd + Shift + D - Toggle debug panel
                if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'D') {
                    e.preventDefault();
                    this.toggleDebugPanel();
                }

                // Ctrl/Cmd + Shift + T - Toggle theme
                if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'T') {
                    e.preventDefault();
                    this.toggleTheme();
                }

                // Ctrl/Cmd + Up/Down - Navigate sessions
                if ((e.ctrlKey || e.metaKey) && (e.key === 'ArrowUp' || e.key === 'ArrowDown')) {
                    e.preventDefault();
                    this.__navigateSessions(e.key === 'ArrowUp' ? -1 : 1);
                }
            };

            document.addEventListener('keydown', handler);

            // Register cleanup
            this.__cleanupTasks.add(() => {
                document.removeEventListener('keydown', handler);
            });
        }
    };
}
