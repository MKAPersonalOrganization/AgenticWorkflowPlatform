/**
 * Theme Plugin
 * Handles dark/light/system theme management
 * 
 * @param {Object} Alpine - Alpine.js instance
 */
export const themePlugin = (Alpine) => {
    // Component data
    Alpine.data('themeManager', () => ({
        theme: localStorage.getItem('theme') || 'system',

        toggleTheme() {
            const themes = ['light', 'dark', 'system'];
            const currentIndex = themes.indexOf(this.theme);
            const nextTheme = themes[(currentIndex + 1) % themes.length];
            this.setTheme(nextTheme);
        },

        setTheme(theme) {
            this.theme = theme;
            localStorage.setItem('theme', theme);
            this.applyTheme(theme);
        },

        applyTheme(theme) {
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
        }
    }));
};
