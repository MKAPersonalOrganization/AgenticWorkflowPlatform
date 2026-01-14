/**
 * Alpine.js Extensions
 *
 * Custom Alpine.js magics, global store, and debug utilities.
 */

/**
 * Register custom Alpine.js magics
 */
export function registerAlpineMagics() {
    document.addEventListener('alpine:init', () => {
        // Custom magic: $chatMetrics - Easy access to performance metrics
        Alpine.magic('chatMetrics', () => {
            return () => {
                const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
                return chatComponent?.getMetrics?.() || null;
            };
        });

        // Custom magic: $chatHealth - Check connection health
        Alpine.magic('chatHealth', () => {
            return () => {
                const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
                return chatComponent?.getConnectionHealth?.() || null;
            };
        });

        // Custom magic: $chatDiagnostics - Export full diagnostics
        Alpine.magic('chatDiagnostics', () => {
            return () => {
                const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
                return chatComponent?.exportDiagnostics?.() || null;
            };
        });

        // Custom magic: $formatTime - Utility for time formatting
        Alpine.magic('formatTime', () => {
            return (timestamp) => {
                if (!timestamp) return '';
                return new Date(timestamp).toLocaleTimeString('en-US', {
                    hour: 'numeric',
                    minute: '2-digit'
                });
            };
        });

        // Custom magic: $copyToClipboard - Easy clipboard access
        Alpine.magic('copyToClipboard', () => {
            return async (text) => {
                try {
                    await navigator.clipboard.writeText(text);
                    return true;
                } catch (error) {
                    console.error('Failed to copy to clipboard:', error);
                    return false;
                }
            };
        });
    });
}

/**
 * Register Alpine.js global store
 */
export function registerAlpineStore() {
    document.addEventListener('alpine:init', () => {
        Alpine.store('chatSettings', {
            init() {
                this.theme = window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
            },

            // Settings
            theme: 'light',
            soundEnabled: true,
            notificationsEnabled: true,
            showTimestamps: true,
            compactMode: false,

            // Actions
            toggleSound() {
                this.soundEnabled = !this.soundEnabled;
            },

            toggleNotifications() {
                this.notificationsEnabled = !this.notificationsEnabled;
            },

            toggleTheme() {
                this.theme = this.theme === 'light' ? 'dark' : 'light';
            },

            toggleCompactMode() {
                this.compactMode = !this.compactMode;
            }
        });
    });
}

/**
 * Register global debug utilities
 */
export function registerDebugUtilities() {
    if (typeof window !== 'undefined') {
        window.__chatDebug = {
            getMetrics: () => {
                const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
                return chatComponent?.getMetrics?.();
            },
            getHealth: () => {
                const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
                return chatComponent?.getConnectionHealth?.();
            },
            exportDiagnostics: () => {
                const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
                return chatComponent?.exportDiagnostics?.();
            },
            resetMetrics: () => {
                const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
                return chatComponent?.resetMetrics?.();
            },
            getSettings: () => Alpine.store('chatSettings'),
            isHealthy: () => {
                const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
                return chatComponent?.isHealthy?.() || false;
            }
        };

        console.log('[Chat] Debug utilities available:');
        console.log('  - window.__chatDebug (legacy)');
        console.log('  - $chatMetrics() in templates');
        console.log('  - $chatHealth() in templates');
        console.log('  - $chatDiagnostics() in templates');
        console.log('  - $formatTime(timestamp) in templates');
        console.log('  - $copyToClipboard(text) in templates');
        console.log('  - $store.chatSettings (global settings)');
    }
}

/**
 * Initialize all Alpine extensions
 */
export function initAlpineExtensions() {
    registerAlpineMagics();
    registerAlpineStore();
    registerDebugUtilities();
}
