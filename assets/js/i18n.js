/**
 * i18n JavaScript Module
 * Provides client-side language switching and direction management
 */

// Language configuration matching backend
const LANGUAGES = {
    en: {
        code: 'en',
        name: 'English',
        nativeName: 'English',
        dir: 'ltr'
    },
    ar: {
        code: 'ar',
        name: 'Arabic',
        nativeName: 'العربية',
        dir: 'rtl'
    }
};

/**
 * Get current language from cookie or default to English
 * @returns {string} Language code (e.g., 'en', 'ar')
 */
function getCurrentLanguage() {
    const cookie = document.cookie.match(/lang=([^;]+)/);
    return cookie ? cookie[1] : 'en';
}

/**
 * Get language object for a given code
 * @param {string} langCode - Language code
 * @returns {object|null} Language object or null if not found
 */
function getLanguage(langCode) {
    return LANGUAGES[langCode] || null;
}

/**
 * Set language and persist preference
 * Updates cookie, HTML attributes, and reloads page for server-side translations
 * @param {string} langCode - Language code to switch to
 */
function setLanguage(langCode) {
    const lang = LANGUAGES[langCode];
    if (!lang) {
        console.warn(`Language '${langCode}' not supported`);
        return;
    }

    // Update cookie with 1 year expiration
    document.cookie = `lang=${langCode}; path=/; max-age=31536000; SameSite=Lax`;

    // Update HTML attributes immediately for smooth transition
    document.documentElement.lang = langCode;
    document.documentElement.dir = lang.dir;

    // Dispatch custom event for components to react before reload
    window.dispatchEvent(new CustomEvent('languagechange', {
        detail: {
            language: langCode,
            direction: lang.dir,
            languageData: lang
        }
    }));

    // Reload page to get new server-side translations
    // Use query param to ensure middleware picks it up
    window.location.href = `${window.location.pathname}?lang=${langCode}`;
}

/**
 * Initialize i18n on page load
 * Ensures HTML attributes match the current language
 */
function initI18n() {
    const currentLang = getCurrentLanguage();
    const lang = LANGUAGES[currentLang];

    if (lang) {
        // Ensure HTML attributes are set correctly
        document.documentElement.lang = currentLang;
        document.documentElement.dir = lang.dir;

        // Log initialization for debugging
        console.log(`i18n initialized: ${lang.nativeName} (${lang.dir})`);
    }
}

/**
 * Create Alpine.js data component for language switching
 * Use this in components that need language switching functionality
 */
function createLanguageSwitcher() {
    return {
        currentLang: getCurrentLanguage(),
        languages: LANGUAGES,

        switchLanguage(langCode) {
            setLanguage(langCode);
        },

        getCurrentLanguageData() {
            return LANGUAGES[this.currentLang];
        },

        isRTL() {
            return LANGUAGES[this.currentLang]?.dir === 'rtl';
        }
    };
}

// Auto-initialize on DOM ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initI18n);
} else {
    initI18n();
}

// Export for module usage
export {
    LANGUAGES,
    getCurrentLanguage,
    getLanguage,
    setLanguage,
    initI18n,
    createLanguageSwitcher
};

// Also expose globally for non-module usage
window.i18n = {
    LANGUAGES,
    getCurrentLanguage,
    getLanguage,
    setLanguage,
    initI18n,
    createLanguageSwitcher
};
