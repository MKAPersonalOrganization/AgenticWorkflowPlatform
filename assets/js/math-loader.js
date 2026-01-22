/**
 * Math Loader - LaTeX Rendering with i18n Support
 * 
 * Configures MathJax/Mathpix to properly render mathematics
 * in different languages, including RTL languages like Arabic.
 */

/**
 * Load and configure MathJax with language-aware settings
 * @returns {boolean} - Whether MathJax was loaded successfully
 */
window.loadMathJax = function () {
    try {
        // Get current language from document
        const lang = document.documentElement.lang || 'en';
        const dir = document.documentElement.dir || 'ltr';
        const isRTL = dir === 'rtl';
        const isArabic = lang === 'ar';

        // Check if Mathpix is loaded
        if (typeof window.loadMathJax !== 'undefined' && typeof window.mathpixMarkdownItOptions !== 'undefined') {
            console.log('Mathpix already loaded, reconfiguring for language:', lang);
        }

        // Configure MathJax before it loads
        // We avoid defining 'loader' as Mathpix's bundled MathJax might handle loading differently or have packages pre-loaded
        window.MathJax = {
            // TeX input configuration
            tex: {
                inlineMath: [['$', '$'], ['\\(', '\\)']],
                displayMath: [['$$', '$$'], ['\\[', '\\]'], ['[', ']']], // Support [ ] as well for non-standard input
                processEscapes: true,
                processEnvironments: true,

                // Macros for better Arabic support
                macros: {
                    // Standard macros
                    RR: "{\\bf R}",
                    bold: ["{\\bf #1}", 1],

                    // Arabic-specific macros if needed
                    ...(isArabic ? {
                        // Arabic mathematical notation helpers
                        artext: ["{\\text{#1}}", 1]
                    } : {})
                }
            },

            // Output configuration (chtml is usually default for Mathpix)
            chtml: {
                scale: 1,
                minScale: 0.5,
                mtextInheritFont: true,
                merrorInheritFont: true,
                displayAlign: isRTL ? 'right' : 'left',
                displayIndent: '0'
            },

            svg: {
                scale: 1,
                minScale: 0.5,
                mtextInheritFont: true,
                merrorInheritFont: true,
                displayAlign: isRTL ? 'right' : 'left',
                displayIndent: '0'
            },

            // Options
            options: {
                enableMenu: true,
                renderActions: {
                    addMenu: [0, '', ''],
                    checkLoading: [1, '', '']
                },
                a11y: {
                    speech: true,
                    locale: lang
                }
            },

            // Startup configuration
            startup: {
                pageReady: () => {
                    // Safety check for startup
                    if (window.MathJax.startup && window.MathJax.startup.defaultPageReady) {
                        return MathJax.startup.defaultPageReady().then(() => {
                            console.log('MathJax loaded and configured for language:', lang);
                        });
                    }
                    return Promise.resolve();
                }
            }
        };

        // Apply RTL fixes immediately if needed
        if (isRTL) {
            applyRTLMathFixes();
        }

        // Load Mathpix styles if available
        if (typeof window.loadMathJaxCss === 'function') {
            try {
                window.loadMathJaxCss();
                console.log('Mathpix styles loaded');
            } catch (e) {
                console.warn('Could not load Mathpix styles:', e);
            }
        }

        return true;
    } catch (error) {
        console.error('Error configuring MathJax:', error);
        return false;
    }
};

/**
 * Apply RTL-specific fixes to math rendering
 */
function applyRTLMathFixes() {
    // Add CSS rules for RTL math rendering
    const style = document.createElement('style');
    style.id = 'mathjax-rtl-fixes';
    style.textContent = `
        /* RTL Math Rendering Fixes */
        /* Math itself is usually LTR (variables, operators), but we want it right-aligned in usage */
        [dir="rtl"] .MathJax {
            direction: ltr; 
            text-align: right;
            display: inline-block; /* Helps prevent full-width LTR blocks messing up flow */
        }
        
        [dir="rtl"] mjx-container[display="true"] {
            text-align: right !important;
            margin: 1em 0 !important;
        }
        
        /* Ensure display math is properly aligned */
        [dir="rtl"] .MathJax_Display {
            text-align: right !important;
            direction: ltr !important; /* Internal math structure stays LTR */
        }
        
        /* Fix indentation and margins for block math in RTL */
        [dir="rtl"] .MathJax_Display {
            margin-right: 0 !important;
            margin-left: auto !important;
        }
        
        /* Arabic text within math should use Arabic fonts */
        .MathJax [class*="mjx-ar"] {
            font-family: 'Noto Sans Arabic', 'Arial Unicode MS', sans-serif;
        }
        
        /* Improve spacing in RTL */
        [dir="rtl"] .math-inline {
            margin: 0 0.2em;
        }
    `;

    // Remove existing style if present
    const existing = document.getElementById('mathjax-rtl-fixes');
    if (existing) {
        existing.remove();
    }

    document.head.appendChild(style);
}

/**
 * Reinitialize MathJax when language changes
 * Call this when the user switches languages
 */
window.reinitializeMath = function () {
    // Clear the configuration
    if (window.MathJax) {
        window.MathJax = undefined;
    }

    // Reload the configuration
    window.loadMathJax();

    // Re-typeset the page if MathJax is available
    if (window.MathJax && window.MathJax.typesetPromise) {
        window.MathJax.typesetPromise().catch((err) => {
            console.error('Error re-typesetting math:', err);
        });
    }
};

/**
 * Utility: Typeset a specific element
 * @param {HTMLElement} element - The element to typeset
 */
window.typesetMath = function (element) {
    if (!element) return;

    if (window.MathJax && window.MathJax.typesetPromise) {
        window.MathJax.typesetPromise([element]).catch((err) => {
            console.error('Error typesetting element:', err);
        });
    }
};

// Export for module usage if supported
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { loadMathJax, reinitializeMath, typesetMath };
}
