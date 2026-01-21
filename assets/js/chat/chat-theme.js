/**
 * Chat Theme Management
 *
 * Handles theme switching and persistence:
 * - Light/dark/system theme modes
 * - Local storage persistence
 * - System preference detection
 */

/**
 * Create theme management methods mixin
 * @returns {Object} Methods for theme management
 */
export function createThemeMethods() {
    /**
     * Private helper: Apply theme to document
     * @param {'light'|'dark'|'system'} theme - Theme to apply
     */
    const _applyThemeToDOM = (theme) => {
        const root = document.documentElement;

        if (theme === 'system') {
            const systemPrefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
            if (systemPrefersDark) {
                root.classList.add('dark');
            } else {
                root.classList.remove('dark');
            }
        } else if (theme === 'dark') {
            root.classList.add('dark');
        } else {
            root.classList.remove('dark');
        }
    };

    return {
        /**
         * Apply theme side effects (DOM, localStorage, Toast)
         * Should be called by watcher
         * @param {'light'|'dark'|'system'} theme 
         */
        applyThemeEffects(theme) {
            localStorage.setItem('theme', theme);
            _applyThemeToDOM(theme);

            // Update Alpine store if available
            if (Alpine.store('chatSettings')) {
                Alpine.store('chatSettings').theme = theme;
            }

            // Show toast notification
            const themeLabels = {
                light: 'Light mode',
                dark: 'Dark mode',
                system: 'System theme'
            };

            // Only show toast if initialized (avoid toast on load)
            if (this.__initialized && this.__themeInitialized) {
                this.showToast(`${themeLabels[theme]} enabled`, 'info', 2000);
            }
        },

        /**
         * Toggle theme: light -> dark -> system -> light
         * updates state only, watcher triggers effects
         */
        toggleTheme() {
            const currentTheme = this.theme;
            let nextTheme;

            if (currentTheme === 'light') {
                nextTheme = 'dark';
            } else if (currentTheme === 'dark') {
                nextTheme = 'system';
            } else {
                nextTheme = 'light';
            }

            this.theme = nextTheme;
        },

        /**
         * Set theme preference
         * updates state only, watcher triggers effects
         * @param {'light'|'dark'|'system'} theme - Theme to set
         */
        setTheme(theme) {
            this.theme = theme;
        },

        /**
         * Initialize theme from localStorage or system preference
         */
        __initTheme() {
            const savedTheme = localStorage.getItem('theme') || 'system';
            this.theme = savedTheme;
            _applyThemeToDOM(savedTheme);
            this.__themeInitialized = true;

            // Listen for system theme changes when in system mode
            const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
            mediaQuery.addEventListener('change', () => {
                if (this.theme === 'system') {
                    _applyThemeToDOM('system');
                }
            });
        }
    };
}
