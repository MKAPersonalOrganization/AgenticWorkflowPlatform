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
        dir: 'ltr',
        useEasternDigits: false
    },
    ar: {
        code: 'ar',
        name: 'Arabic',
        nativeName: 'العربية',
        dir: 'rtl',
        useEasternDigits: true
    }
};

// Translation dictionary
let translations = {};

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
 * Set the translation dictionary
 * @param {object} dict - Key-value pair translations
 */
function setTranslations(dict) {
    translations = dict || {};
}

/**
 * Convert Western Arabic numerals (0-9) to Eastern Arabic numerals (٠-٩)
 * @param {string|number} value - Value to convert
 * @returns {string} Converted value
 */
function toEasternArabicDigits(value) {
    if (value === null || value === undefined) return '';
    const charMap = {
        '0': '٠', '1': '١', '2': '٢', '3': '٣', '4': '٤',
        '5': '٥', '6': '٦', '7': '٧', '8': '٨', '9': '٩'
    };
    return String(value).replace(/[0-9]/g, (char) => charMap[char]);
}

/**
 * Translate a key
 * @param {string} key - Translation key
 * @param {object} [args] - Optional arguments to replace in string
 * @returns {string} Translated string or key if not found
 */
function t(key, args = {}) {
    let translated = translations[key] || key;
    const currentLang = getCurrentLanguage();
    const langData = LANGUAGES[currentLang] || LANGUAGES['en'];

    // Handle string replacements if args are provided
    // Example: t("Hello {name}", { name: "World" })
    if (args && typeof args === 'object') {
        Object.entries(args).forEach(([k, v]) => {
            let val = v;
            // Convert numbers to Eastern Arabic if configured
            if (langData.useEasternDigits && (typeof v === 'number' || (!isNaN(v) && !String(v).startsWith('0x')))) {
                val = toEasternArabicDigits(v);
            }
            translated = translated.replace(`{${k}}`, val);
        });
    }

    // Also handle numbers embedded in the translated string itself if configured
    if (langData.useEasternDigits) {
        translated = toEasternArabicDigits(translated);
    }

    return translated;
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
        },

        t(key, args) {
            return t(key, args);
        }
    };
}

// Auto-initialize on DOM ready
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initI18n);
} else {
    initI18n();
}

// Expose globally for non-module usage
window.i18n = {
    LANGUAGES,
    getCurrentLanguage,
    getLanguage,
    setLanguage,
    initI18n,
    setTranslations,
    t,
    createLanguageSwitcher
};

