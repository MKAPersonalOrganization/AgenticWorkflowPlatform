/**
 * Alpine.js Extensions
 *
 * Custom Alpine.js magics, global store, and debug utilities.
 *
 * Design principles (from Alpine.js source):
 * - Magics are lazily evaluated on access, not eagerly computed
 * - Use Alpine's scope chain ($data) rather than fragile DOM queries
 * - Stores are for truly global cross-component state only
 * - Keep the API surface minimal — don't duplicate component methods
 */

/**
 * Register custom Alpine.js magics
 */
export function registerAlpineMagics() {
    document.addEventListener('alpine:init', () => {
        // $formatTime — pure utility, no component dependency
        Alpine.magic('formatTime', () => {
            return (timestamp) => {
                if (!timestamp) return '';
                return new Date(timestamp).toLocaleTimeString('en-US', {
                    hour: 'numeric',
                    minute: '2-digit'
                });
            };
        });

        // $copyToClipboard — pure utility, no component dependency
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
 * Only stores truly global settings that outlive any single component.
 * Theme is managed by the chat component's preferences.theme — the store
 * mirrors it for cross-component reads (e.g. standalone widgets).
 */
export function registerAlpineStore() {
    document.addEventListener('alpine:init', () => {
        Alpine.store('chatSettings', {
            init() {
                this.theme = localStorage.getItem('theme') ||
                    (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light');
            },

            // Mirrored from component (set by applyThemeEffects)
            theme: 'light',

            // Independent global settings
            soundEnabled: true,
            notificationsEnabled: true,
            showTimestamps: true,
            compactMode: false,

            toggleSound() {
                this.soundEnabled = !this.soundEnabled;
            },

            toggleNotifications() {
                this.notificationsEnabled = !this.notificationsEnabled;
            },

            toggleCompactMode() {
                this.compactMode = !this.compactMode;
            }
        });
    });
}

/**
 * Register global debug utilities (development only)
 */
export function registerDebugUtilities() {
    if (typeof window === 'undefined') return;

    window.__chatDebug = {
        getComponent() {
            const el = document.querySelector('[x-data*="chat"]');
            return el ? Alpine.$data(el) : null;
        },
        getMetrics() {
            return this.getComponent()?.getMetrics?.() ?? null;
        },
        getHealth() {
            return this.getComponent()?.getConnectionHealth?.() ?? null;
        },
        exportDiagnostics() {
            return this.getComponent()?.exportDiagnostics?.() ?? null;
        },
        resetMetrics() {
            return this.getComponent()?.resetMetrics?.();
        },
        getSettings() {
            return Alpine.store('chatSettings');
        },
        isHealthy() {
            return this.getComponent()?.isHealthy?.() ?? false;
        }
    };

    console.log('[Chat] Debug: window.__chatDebug available');
}

/**
 * Initialize all Alpine extensions
 */
export function initAlpineExtensions() {
    registerAlpineMagics();
    registerAlpineStore();
    registerDebugUtilities();
}
