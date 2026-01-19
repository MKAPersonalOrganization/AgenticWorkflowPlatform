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

    // Custom Render Rules for better Styling
    // Table: Wrap in div for overflow, style table
    md.renderer.rules.table_open = () => '<div class="overflow-x-auto my-6 rounded-lg border border-border bg-card/50 shadow-sm"><table class="w-full text-sm text-left">';
    md.renderer.rules.table_close = () => '</table></div>';

    // Table Header
    md.renderer.rules.thead_open = () => '<thead class="bg-muted/50 text-xs uppercase font-medium text-muted-foreground">';
    md.renderer.rules.th_open = () => '<th scope="col" class="px-4 py-3 border-b border-border/50">';
    md.renderer.rules.td_open = () => '<td class="px-4 py-3 border-b border-border/10">';

    // Blockquote
    md.renderer.rules.blockquote_open = () => '<blockquote class="border-l-4 border-primary/40 pl-4 py-2 my-4 italic bg-muted/20 rounded-r text-muted-foreground">';

    // Links (External links open in new tab)
    const defaultLinkOpen = md.renderer.rules.link_open || function (tokens, idx, options, env, self) {
        return self.renderToken(tokens, idx, options);
    };
    md.renderer.rules.link_open = function (tokens, idx, options, env, self) {
        tokens[idx].attrPush(['target', '_blank']);
        tokens[idx].attrPush(['rel', 'noopener noreferrer']);
        tokens[idx].attrJoin('class', 'text-primary hover:underline font-medium decoration-primary/30 underline-offset-4');
        return defaultLinkOpen(tokens, idx, options, env, self);
    };
}

/**
 * Enhance code blocks with copy buttons
 * @param {string} html - Rendered HTML
 * @returns {string} - Enhanced HTML with copy buttons
 */
function enhanceCodeBlocks(html) {
    if (!html) return html;

    // Wrap code blocks in a structured container with header
    return html.replace(
        /<pre><code([^>]*)>([\s\S]*?)<\/code><\/pre>/g,
        (match, attrs, code) => {
            // Extract language from class attribute (e.g., class="language-js")
            const langMatch = attrs.match(/class="[^"]*language-([a-zA-Z0-9_-]+)[^"]*"/);
            const language = langMatch ? langMatch[1] : 'text';

            return `<div class="code-block-wrapper my-4 rounded-lg border border-border bg-card overflow-hidden shadow-sm group/codeblock">
                <div class="flex items-center justify-between px-3 py-2 bg-muted/50 border-b border-border/50">
                    <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider select-none">${language}</span>
                    <button
                        onclick="copyCodeBlock(event)"
                        class="flex items-center gap-1.5 px-2 py-1 rounded-md text-xs font-medium text-muted-foreground hover:bg-background hover:text-foreground transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-ring focus:ring-offset-1"
                        type="button"
                        aria-label="Copy code"
                        title="Copy code"
                    >
                        <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"></path>
                        </svg>
                        <span>Copy code</span>
                    </button>
                </div>
                <div class="p-0 overflow-x-auto">
                    <pre class="!m-0 !p-4 !bg-transparent"><code${attrs} class="!bg-transparent !p-0 font-mono text-sm">${code}</code></pre>
                </div>
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
