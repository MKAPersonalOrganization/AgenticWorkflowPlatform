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
    return {
        /**
         * Toggle theme: light -> dark -> system -> light
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

            this.setTheme(nextTheme);
        },

        /**
         * Set theme preference
         * @param {'light'|'dark'|'system'} theme - Theme to set
         */
        setTheme(theme) {
            this.theme = theme;
            localStorage.setItem('theme', theme);
            this.__applyTheme(theme);

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
            this.showToast(`${themeLabels[theme]} enabled`, 'info', 2000);
        },

        /**
         * Apply theme to document
         * @param {'light'|'dark'|'system'} theme - Theme to apply
         */
        __applyTheme(theme) {
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
        },

        /**
         * Initialize theme from localStorage or system preference
         */
        __initTheme() {
            const savedTheme = localStorage.getItem('theme') || 'system';
            this.theme = savedTheme;
            this.__applyTheme(savedTheme);

            // Listen for system theme changes when in system mode
            const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
            mediaQuery.addEventListener('change', () => {
                if (this.theme === 'system') {
                    this.__applyTheme('system');
                }
            });
        }
    };
}
