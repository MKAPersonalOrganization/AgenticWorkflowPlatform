/**
 * Markdown Renderer
 *
 * Handles markdown rendering with syntax highlighting and code block enhancements.
 * Uses KaTeX for LaTeX math rendering.
 *
 * LaTeX preprocessing is a 1:1 port of LibreChat's implementation:
 * Source: LibreChat/client/src/utils/latex.ts
 */

// ============================================================================
// markdown-it initialization
// ============================================================================

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
  md.renderer.rules.table_open = () => '<div class="overflow-x-auto my-6 rounded-lg border border-border bg-card/50 shadow-sm"><table class="w-full text-sm text-left">';
  md.renderer.rules.table_close = () => '</table></div>';
  md.renderer.rules.thead_open = () => '<thead class="bg-muted/50 text-xs uppercase font-medium text-muted-foreground">';
  md.renderer.rules.th_open = () => '<th scope="col" class="px-4 py-3 border-b border-border/50">';
  md.renderer.rules.td_open = () => '<td class="px-4 py-3 border-b border-border/10">';
  md.renderer.rules.blockquote_open = () => '<blockquote class="border-l-4 border-primary/40 pl-4 py-2 my-4 italic bg-muted/20 rounded-r text-muted-foreground">';

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

// ============================================================================
// LaTeX Preprocessing — 1:1 port of LibreChat/client/src/utils/latex.ts
// ============================================================================

// Pre-compile all regular expressions for better performance
const MHCHEM_CE_REGEX = /\$\\ce\{/g;
const MHCHEM_PU_REGEX = /\$\\pu\{/g;
const MHCHEM_CE_ESCAPED_REGEX = /\$\\\\ce\{[^}]*\}\$/g;
const MHCHEM_PU_ESCAPED_REGEX = /\$\\\\pu\{[^}]*\}\$/g;
const CURRENCY_REGEX =
  /(?<![\\$])\$(?!\$)(?=\d+(?:,\d{3})*(?:\.\d+)?(?:[KMBkmb])?(?:\s|$|[^a-zA-Z\d]))/g;
const SINGLE_DOLLAR_REGEX = /(?<!\\)\$(?!\$)((?:[^$\n]|\\[$])+?)(?<!\\)(?<!`)\$(?!\$)/g;

/**
 * Escapes mhchem package notation in LaTeX by converting single dollar delimiters to double dollars
 * and escaping backslashes in mhchem commands.
 *
 * @param {string} text - The input text containing potential mhchem notation
 * @returns {string} The processed text with properly escaped mhchem notation
 */
function escapeMhchem(text) {
  // First escape the backslashes in mhchem commands
  let result = text.replace(MHCHEM_CE_REGEX, '$\\\\ce{');
  result = result.replace(MHCHEM_PU_REGEX, '$\\\\pu{');

  // Then convert single dollar mhchem to double dollar
  result = result.replace(MHCHEM_CE_ESCAPED_REGEX, (match) => `$${match}$`);
  result = result.replace(MHCHEM_PU_ESCAPED_REGEX, (match) => `$${match}$`);

  return result;
}

/**
 * Efficiently finds all code block regions in the content
 * @param {string} content The content to analyze
 * @returns {Array<[number, number]>} Array of code block regions [start, end]
 */
function findCodeBlockRegions(content) {
  const regions = [];
  let inlineStart = -1;
  let multilineStart = -1;

  for (let i = 0; i < content.length; i++) {
    const char = content[i];

    // Check for multiline code blocks
    if (
      char === '`' &&
      i + 2 < content.length &&
      content[i + 1] === '`' &&
      content[i + 2] === '`'
    ) {
      if (multilineStart === -1) {
        multilineStart = i;
        i += 2; // Skip the next two backticks
      } else {
        regions.push([multilineStart, i + 2]);
        multilineStart = -1;
        i += 2;
      }
    }
    // Check for inline code blocks (only if not in multiline)
    else if (char === '`' && multilineStart === -1) {
      if (inlineStart === -1) {
        inlineStart = i;
      } else {
        regions.push([inlineStart, i]);
        inlineStart = -1;
      }
    }
  }

  return regions;
}

/**
 * Checks if a position is inside any code block region using binary search
 * @param {number} position The position to check
 * @param {Array<[number, number]>} codeRegions Array of code block regions
 * @returns {boolean} True if position is inside a code block
 */
function isInCodeBlock(position, codeRegions) {
  let left = 0;
  let right = codeRegions.length - 1;

  while (left <= right) {
    const mid = Math.floor((left + right) / 2);
    const [start, end] = codeRegions[mid];

    if (position >= start && position <= end) {
      return true;
    } else if (position < start) {
      right = mid - 1;
    } else {
      left = mid + 1;
    }
  }

  return false;
}

/**
 * Preprocesses LaTeX content by escaping currency indicators and converting single dollar math delimiters.
 * Optimized for high-frequency execution.
 * @param {string} content The input string containing LaTeX expressions.
 * @returns {string} The processed string with escaped currency indicators and converted math delimiters.
 */
function preprocessLaTeX(content) {
  // Early return for most common case
  if (!content.includes('$')) return content;

  // Process mhchem first (usually rare, so check if needed)
  let processed = content;
  if (content.includes('\\ce{') || content.includes('\\pu{')) {
    processed = escapeMhchem(content);
  }

  // Find all code block regions once
  const codeRegions = findCodeBlockRegions(processed);

  // First pass: escape currency dollar signs
  const parts = [];
  let lastIndex = 0;

  // Reset regex for reuse
  CURRENCY_REGEX.lastIndex = 0;

  let match;
  while ((match = CURRENCY_REGEX.exec(processed)) !== null) {
    if (!isInCodeBlock(match.index, codeRegions)) {
      parts.push(processed.substring(lastIndex, match.index));
      parts.push('\\$');
      lastIndex = match.index + 1;
    }
  }
  parts.push(processed.substring(lastIndex));
  processed = parts.join('');

  // Second pass: convert single dollar delimiters to double dollars
  const result = [];
  lastIndex = 0;

  // Reset regex for reuse
  SINGLE_DOLLAR_REGEX.lastIndex = 0;

  while ((match = SINGLE_DOLLAR_REGEX.exec(processed)) !== null) {
    if (!isInCodeBlock(match.index, codeRegions)) {
      result.push(processed.substring(lastIndex, match.index));
      result.push(`$$${match[1]}$$`);
      lastIndex = match.index + match[0].length;
    }
  }
  result.push(processed.substring(lastIndex));

  return result.join('');
}

// ============================================================================
// Math Extraction & KaTeX Rendering
// (replaces remark-math + rehype-katex from LibreChat's React pipeline)
// ============================================================================

/**
 * Render a TeX string to HTML using KaTeX.
 * @param {string} tex - The TeX source
 * @param {boolean} displayMode - true for block/display math, false for inline
 * @returns {string} Rendered HTML or fallback
 */
function renderKatex(tex, displayMode) {
  if (typeof window.katex === 'undefined') return tex;
  try {
    return window.katex.renderToString(tex.trim(), {
      displayMode,
      throwOnError: false,
      strict: false,
      output: 'html'
    });
  } catch (e) {
    const escaped = tex.replace(/</g, '&lt;').replace(/>/g, '&gt;');
    return displayMode
      ? `<pre class="math-error">${escaped}</pre>`
      : `<code class="math-error">${escaped}</code>`;
  }
}

/**
 * Extract math expressions from content, render with KaTeX, and replace
 * with unique placeholders so markdown-it won't mangle the LaTeX.
 *
 * After preprocessLaTeX, all single-dollar math is now double-dollar.
 * Delimiter handling (mirrors remark-math behavior):
 *   \[...\]           → display math
 *   $$...$$ multiline → display math
 *   \(...\)           → inline math
 *   $$...$$ one-line  → inline math
 *
 * @param {string} content - Preprocessed content
 * @returns {{ processed: string, mathMap: Object<string,string> }}
 */
function extractAndRenderMath(content) {
  const mathMap = {};
  let counter = 0;

  if (typeof window.katex === 'undefined') {
    return { processed: content, mathMap };
  }

  let processed = content;

  const placeholder = (tex, display, block) => {
    const tag = block ? 'MBLK' : 'MINL';
    const key = `KATEX${tag}${counter++}XETAK`;
    mathMap[key] = renderKatex(tex, display);
    return block ? `\n\n${key}\n\n` : key;
  };

  // 1. Display math: \[...\]
  processed = processed.replace(/\\\[([\s\S]+?)\\\]/g, (_m, tex) => placeholder(tex, true, true));

  // 2. Display math: $$...$$ that contains newlines (multiline block math)
  processed = processed.replace(/\$\$([\s\S]*?\n[\s\S]*?)\$\$/g, (_m, tex) => placeholder(tex, true, true));

  // 3. Inline math: \(...\)
  processed = processed.replace(/\\\((.+?)\\\)/g, (_m, tex) => placeholder(tex, false, false));

  // 4. Inline math: remaining $$...$$ (single-line, within text)
  processed = processed.replace(/\$\$([^\n]+?)\$\$/g, (_m, tex) => placeholder(tex, false, false));

  return { processed, mathMap };
}

/**
 * Restore rendered math from placeholders after markdown-it processing.
 * @param {string} html - Rendered HTML containing placeholders
 * @param {Object<string,string>} mathMap - placeholder → KaTeX HTML
 * @returns {string}
 */
function restoreMath(html, mathMap) {
  for (const [key, rendered] of Object.entries(mathMap)) {
    // Block placeholders may be wrapped in <p> tags by markdown-it — strip those
    const wrappedRe = new RegExp(`<p>\\s*${key}\\s*</p>`, 'g');
    html = html.replace(wrappedRe, rendered);
    // Handle any remaining unwrapped occurrences
    html = html.replaceAll(key, rendered);
  }
  return html;
}

// ============================================================================
// Code Block Enhancement
// ============================================================================

/**
 * Enhance code blocks with copy buttons
 * @param {string} html - Rendered HTML
 * @returns {string} - Enhanced HTML with copy buttons
 */
function enhanceCodeBlocks(html) {
  if (!html) return html;

  return html.replace(
    /<pre><code([^>]*)>([\s\S]*?)<\/code><\/pre>/g,
    (match, attrs, code) => {
      const langMatch = attrs.match(/class="[^"]*language-([a-zA-Z0-9_-]+)[^"]*"/);
      const language = langMatch ? langMatch[1] : 'text';

      return `<div class="code-block-wrapper my-4 rounded-lg border border-border bg-card overflow-hidden shadow-sm group/codeblock">
                <div class="flex items-center justify-between px-3 py-2 bg-muted/50 border-b border-border/50">
                    <span class="text-xs font-semibold text-muted-foreground uppercase tracking-wider select-none">${language}</span>
                    <button
                        onclick="copyCodeBlock(event)"
                        class="flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-[11px] font-semibold text-muted-foreground hover:bg-background hover:text-primary hover:shadow-sm transition-all duration-200 focus:outline-none focus:ring-2 focus:ring-primary/20 group/copybtn"
                        type="button"
                        aria-label="${window.i18n.t('Copy code')}"
                        title="${window.i18n.t('Copy code')}"
                    >
                        <svg class="w-3.5 h-3.5 transition-transform group-hover/copybtn:scale-110" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 5H6a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-1M8 5a2 2 0 002 2h2a2 2 0 002-2M8 5a2 2 0 012-2h2a2 2 0 012 2m0 0h2a2 2 0 012 2v3m2 4H10m0 0l3-3m-3 3l3 3"></path>
                        </svg>
                        <span class="uppercase tracking-wider">${window.i18n.t('Copy')}</span>
                    </button>
                </div>
                <div class="p-0 overflow-x-auto" dir="ltr">
                    <pre class="!m-0 !p-4 !bg-transparent"><code${attrs} class="!bg-transparent !p-0 font-mono text-sm">${code}</code></pre>
                </div>
            </div>`;
    }
  );
}

// ============================================================================
// Public API
// ============================================================================

/**
 * Render markdown content safely with math support.
 * Pipeline: preprocessLaTeX → extractMath → markdown-it → restoreMath → enhanceCodeBlocks
 * @param {string} content - The markdown content to render
 * @returns {string} - Rendered HTML
 */
export function renderMarkdown(content) {
  if (!content) return '';

  const langData = window.i18n?.getLanguage(window.i18n.getCurrentLanguage());
  let processedContent = content;

  if (langData?.useEasternDigits) {
    processedContent = processedContent.replace(
      /\$\$([\s\S]*?)\$\$|\\\[([\s\S]*?)\\\]|\\\(([\s\S]*?)\\\)|\$([\s\S]*?)\$/g,
      (match) => convertDigits(match)
    );
  }

  // Step 1: Preprocess LaTeX (LibreChat port — escape currency, mhchem, convert $→$$)
  processedContent = preprocessLaTeX(processedContent);

  // Step 2: Extract math, render with KaTeX, replace with placeholders
  const { processed, mathMap } = extractAndRenderMath(processedContent);

  // Step 3: Render markdown (with error boundary for stability)
  if (md) {
    let html;
    try {
      html = md.render(processed);
    } catch (error) {
      console.error('Markdown rendering failed:', error);
      // Fallback to basic text escaping if renderer crashes
      html = processed.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
    }

    // Step 4: Restore rendered math from placeholders
    html = restoreMath(html, mathMap);

    // Step 5: Enhance code blocks
    return enhanceCodeBlocks(html);
  }

  return processedContent;
}

/**
 * Convert Western digits to Arabic-Indic digits
 * @param {string} str
 * @returns {string}
 */
function convertDigits(str) {
  const arabicDigits = ['٠', '١', '٢', '٣', '٤', '٥', '٦', '٧', '٨', '٩'];
  return str.replace(/[0-9]/g, (w) => arabicDigits[+w]);
}

/**
 * Global function to copy code blocks (called from onclick)
 * @param {Event} event
 */
window.copyCodeBlock = async function (event) {
  const button = event.currentTarget;
  const codeBlock = button.closest('.group\\/codeblock')?.querySelector('code');
  if (!codeBlock) return;

  try {
    await navigator.clipboard.writeText(codeBlock.textContent);

    const originalHTML = button.innerHTML;
    button.innerHTML = `
            <svg class="w-3.5 h-3.5 text-primary animate-in zoom-in duration-200" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path>
            </svg>
            <span class="text-primary uppercase tracking-wider">${window.i18n.t('Copied!')}</span>
        `;

    setTimeout(() => {
      button.innerHTML = originalHTML;
    }, 2000);

    const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
    if (chatComponent?.showToast) {
      chatComponent.showToast(window.i18n.t('Code copied to clipboard'), 'success', 2000);
    }
  } catch (error) {
    console.error('Failed to copy code:', error);

    const chatComponent = Alpine.$data(document.querySelector('[x-data*="chat"]'));
    if (chatComponent?.showToast) {
      chatComponent.showToast(window.i18n.t('Failed to copy code'), 'error');
    }
  }
}
