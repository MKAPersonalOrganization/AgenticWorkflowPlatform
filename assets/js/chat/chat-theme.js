/**
 * Chat Theme Management
 *
 * Handles theme switching and persistence:
 * - Light/dark/system theme modes
 * - Local storage persistence
 * - System preference detection
 *
 * Alpine.js pattern: preferences.theme on the component is the single source
 * of truth. The $watch in init() triggers applyThemeEffects on changes,
 * which handles DOM updates and persistence. No store mirroring needed —
 * Alpine's scope chain makes this.preferences.theme accessible to all
 * child components and templates.
 */

/**
 * Apply theme to document root.
 * Pure function — no Alpine dependency.
 * @param {'light'|'dark'|'system'} theme
 */
function applyThemeToDOM(theme) {
    const root = document.documentElement;

    if (theme === 'system') {
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        root.classList.toggle('dark', prefersDark);
    } else {
        root.classList.toggle('dark', theme === 'dark');
    }
}

/**
 * Create theme management methods mixin
 * @returns {Object} Methods for theme management
 */
export function createThemeMethods() {
    return {
        /**
         * Apply theme side effects (DOM + localStorage).
         * Called by the $watch('preferences.theme') watcher in init().
         * @param {'light'|'dark'|'system'} theme
         */
        applyThemeEffects(theme) {
            localStorage.setItem('theme', theme);
            applyThemeToDOM(theme);

            // Keep global store in sync for cross-component reads (e.g. standalone widgets)
            const store = Alpine.store('chatSettings');
            if (store) store.theme = theme;

            // Only show toast after initial load (avoid toast on hydration)
            if (this._themeReady) {
                const labels = { light: 'Light mode', dark: 'Dark mode', system: 'System theme' };
                this.showToast(`${labels[theme]} enabled`, 'info', 2000);
            }
        },

        /**
         * Cycle theme: light → dark → system → light.
         * Only updates state — the watcher triggers side effects.
         */
        toggleTheme() {
            const cycle = { light: 'dark', dark: 'system', system: 'light' };
            this.preferences.theme = cycle[this.preferences.theme] || 'light';
        },

        /**
         * Set theme preference.
         * Only updates state — the watcher triggers side effects.
         * @param {'light'|'dark'|'system'} theme
         */
        setTheme(theme) {
            this.preferences.theme = theme;
        },

        /**
         * Initialize theme from localStorage or system preference.
         * Called once from init().
         */
        __initTheme() {
            const saved = localStorage.getItem('theme') || 'system';
            this.preferences.theme = saved;
            applyThemeToDOM(saved);

            // Mark ready after initial application (prevents toast on load)
            this._themeReady = true;

            // React to OS-level theme changes when in "system" mode
            const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
            const onSystemChange = () => {
                if (this.preferences.theme === 'system') {
                    applyThemeToDOM('system');
                }
            };
            mediaQuery.addEventListener('change', onSystemChange);

            // Alpine pattern: register cleanup via destroy() lifecycle
            this.__cleanupTasks.add(() => {
                mediaQuery.removeEventListener('change', onSystemChange);
            });
        }
    };
}
