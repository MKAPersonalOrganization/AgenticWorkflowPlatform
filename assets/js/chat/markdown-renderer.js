/**
 * Markdown Renderer
 *
 * Handles markdown rendering with syntax highlighting and code block enhancements.
 * Supports Mathpix for LaTeX math rendering.
 */

// Initialize markdown-it with highlight.js and Mathpix integration
let md = null;

if (window.markdownit) {
    md = window.markdownit({
        html: true,
        linkify: true,
        typographer: true,
        breaks: true,
        highlight: function (str, lang) {
            if (window.hljs && lang && window.hljs.getLanguage(lang)) {
                try {
                    return window.hljs.highlight(str, { language: lang }).value;
                } catch (__) { }
            }
            return ''; // use external default escaping
        }
    });
}

/**
 * Enhance code blocks with copy buttons
 * @param {string} html - Rendered HTML
 * @returns {string} - Enhanced HTML with copy buttons
 */
function enhanceCodeBlocks(html) {
    if (!html) return html;

    // Wrap code blocks in a relative container and add copy button
    return html.replace(
        /<pre><code([^>]*)>([\s\S]*?)<\/code><\/pre>/g,
        (match, attrs, code) => {
            return `<div class="relative group/codeblock">
                <button
                    onclick="copyCodeBlock(event)"
                    class="absolute top-2 right-2 opacity-0 group-hover/codeblock:opacity-100 transition-opacity inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded-md text-xs font-medium ring-offset-background bg-muted hover:bg-accent hover:text-accent-foreground border border-border h-7 px-2 shadow-sm touch-target"
                    type="button"
                    aria-label="Copy code"
                    title="Copy code"
                >
                    <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
                    </svg>
                    <span>Copy</span>
                </button>
                <pre><code${attrs}>${code}</code></pre>
            </div>`;
        }
    );
}

/**
 * Render markdown content safely
 * @param {string} content - The markdown content to render
 * @returns {string} - Rendered HTML
 */
export function renderMarkdown(content) {
    if (!content) return '';

    // Use Mathpix renderer if available (supports LaTeX math with $$, \[\], etc.)
    if (typeof window.markdownToHTML === 'function') {
        try {
            const rendered = window.markdownToHTML(content, {
                htmlTags: true
            });
            return enhanceCodeBlocks(rendered);
        } catch (e) {
            console.error('Error rendering with Mathpix:', e);
            // Fallback to regular markdown-it
            if (md) {
                return enhanceCodeBlocks(md.render(content));
            }
        }
    }

    // Fallback to regular markdown-it
    if (md) {
        return enhanceCodeBlocks(md.render(content));
    }

    return content;
}

/**
 * Global function to copy code blocks (called from onclick)
 * @param {Event} event - Click event
 */
window.copyCodeBlock = async function (event) {
    const button = event.currentTarget;
    const codeBlock = button.closest('.group\\/codeblock')?.querySelector('code');
    if (!codeBlock) return;

    try {
        await navigator.clipboard.writeText(codeBlock.textContent);

        // Visual feedback
        const originalHTML = button.innerHTML;
        button.innerHTML = `
            <svg class="w-3 h-3 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
            </svg>
            <span class="text-primary">Copied!</span>
        `;

        setTimeout(() => {
            button.innerHTML = originalHTML;
        }, 2000);

        // Show toast if available
        const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
        if (chatComponent?.showToast) {
            chatComponent.showToast('Code copied to clipboard', 'success', 2000);
        }
    } catch (error) {
        console.error('Failed to copy code:', error);

        // Show error toast if available
        const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
        if (chatComponent?.showToast) {
            chatComponent.showToast('Failed to copy code', 'error');
        }
    }
}
