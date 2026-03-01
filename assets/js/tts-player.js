/**
 * TTS Player — Alpine.js component for synchronized word highlighting
 * within a Markdown-rendered transcript.
 *
 * Alignment approach: Needleman-Wunsch global sequence alignment
 * ─────────────────────────────────────────────────────────────
 * The same algorithm used in:
 *   - Bioinformatics (BLAST, ClustalW) for DNA/protein alignment
 *   - Subtitle synchronization tools (Aeneas, Gentle, WhisperX)
 *   - Forced alignment in ASR (Montreal Forced Aligner)
 *
 * Why not greedy sequential matching?
 *   If Whisper adds, drops, or merges even ONE word, a greedy pointer
 *   drifts and all subsequent matches are wrong. NW handles insertions,
 *   deletions, and substitutions optimally via dynamic programming.
 *
 * Pipeline:
 * 1. Render Markdown → HTML (preserves headings, bold, lists, etc.)
 * 2. Walk DOM text nodes, wrap every word in a <span>
 * 3. Normalize both DOM words and Whisper words (lowercase, strip
 *    punctuation, Unicode-aware via \p{L}\p{N})
 * 4. Run Needleman-Wunsch with Levenshtein-weighted substitution
 *    scoring to find optimal global alignment
 * 5. Traceback → emit only high-confidence (score > 0) pairs
 * 6. Attach timestamp indices + click-to-seek to aligned spans
 * 7. requestAnimationFrame loop with binary search for 60fps tracking
 *
 * Complexity: O(n·m) time and space where n,m are word counts.
 * For max 4096 chars (~600 words): ~360K cells — sub-millisecond.
 */
function ttsPlayer() {
    return {
        // ── State ────────────────────────────────────────────────────────

        /** @private Current highlighted timestamp index (-1 = none) */
        _currentIdx: -1,

        /** @private RAF handle */
        _raf: null,

        /** @private Cached <audio> element */
        _audio: null,

        /** @private Whisper timestamp array */
        _timestamps: [],

        /** @private Array of { span: HTMLElement, tsIdx: number } */
        _wordSpans: [],

        /** @private Tracks init mode ('md' | 'ts') to allow upgrade */
        _initMode: null,

        // ── Lifecycle ────────────────────────────────────────────────────

        destroy() {
            this._stopTracking();
        },

        // ── Transcript Initialization ────────────────────────────────────

        /**
         * Render Markdown and optionally enhance with word-level highlighting.
         * Called via x-effect — re-runs when parsedResult arrives.
         *
         * @param {string} text - The transcript text (Markdown)
         * @param {Array<{word:string, start:number, end:number}>|undefined} timestamps
         */
        initTranscript(text, timestamps) {
            const box = this.$refs.transcriptBox;
            if (!box || !text) return;

            const mode = timestamps?.length ? 'ts' : 'md';
            if (this._initMode === mode) return;
            this._initMode = mode;

            // Step 1: Render Markdown → HTML (preserves all formatting)
            box.innerHTML = window.renderMarkdown
                ? window.renderMarkdown(text)
                : text;

            // Step 2: If no timestamps, we're done — static Markdown
            if (!timestamps?.length) {
                this._timestamps = [];
                this._wordSpans = [];
                return;
            }

            // Step 3: Wrap + Align using Needleman-Wunsch
            this._timestamps = timestamps;
            this._wordSpans = this._wrapAndAlign(box, timestamps);
        },

        // ═════════════════════════════════════════════════════════════════
        //  ALIGNMENT ENGINE
        // ═════════════════════════════════════════════════════════════════

        /**
         * Master alignment pipeline:
         * 1. Walk DOM → wrap every word in a <span>, collect {text, span}
         * 2. Normalize both sequences
         * 3. Needleman-Wunsch DP alignment
         * 4. Traceback → high-confidence pairs
         * 5. Attach timestamps to matched spans
         *
         * @param {HTMLElement} container
         * @param {Array<{word:string, start:number, end:number}>} timestamps
         * @returns {Array<{span: HTMLElement, tsIdx: number}>}
         */
        _wrapAndAlign(container, timestamps) {
            // ── Phase 1: Wrap every word in the DOM ──────────────────────
            const domWords = this._wrapDOMWords(container);
            if (!domWords.length) return [];

            // ── Phase 2: Normalize both sequences ────────────────────────
            const norm = s => s.toLowerCase().replace(/[^\p{L}\p{N}]/gu, '');
            const domNorm = domWords.map(d => norm(d.text));
            const tsNorm = timestamps.map(t => norm(t.word));

            // ── Phase 3: Needleman-Wunsch alignment ──────────────────────
            const pairs = this._needlemanWunsch(domNorm, tsNorm);

            // ── Phase 4: Apply alignment to DOM ──────────────────────────
            const aligned = [];
            for (const [domIdx, tsIdx] of pairs) {
                const span = domWords[domIdx].span;
                span.classList.add('tts-word');
                span.dataset.tsIdx = String(tsIdx);

                const capturedIdx = tsIdx;
                span.addEventListener('click', () => {
                    this._seekTo(timestamps[capturedIdx].start);
                });

                aligned.push({ span, tsIdx });
            }

            return aligned;
        },

        /**
         * Walk all text nodes in the rendered HTML and wrap each word
         * token in a <span>. Returns an array of {text, span} for
         * every word in DOM order.
         *
         * Preserves all Markdown structure — only text nodes are modified.
         *
         * @param {HTMLElement} container
         * @returns {Array<{text: string, span: HTMLElement}>}
         */
        _wrapDOMWords(container) {
            // Snapshot text nodes (live iteration breaks on DOM mutation)
            const walker = document.createTreeWalker(
                container, NodeFilter.SHOW_TEXT, null
            );
            const textNodes = [];
            let node;
            while ((node = walker.nextNode())) {
                if (node.textContent.trim()) textNodes.push(node);
            }

            const words = [];

            for (const textNode of textNodes) {
                const frag = document.createDocumentFragment();
                const regex = /(\s+)|(\S+)/g;
                let match;

                while ((match = regex.exec(textNode.textContent)) !== null) {
                    if (match[1]) {
                        // Whitespace — preserve as-is
                        frag.appendChild(document.createTextNode(match[1]));
                    } else {
                        // Word — wrap in span
                        const span = document.createElement('span');
                        span.textContent = match[2];
                        frag.appendChild(span);
                        words.push({ text: match[2], span });
                    }
                }

                textNode.parentNode.replaceChild(frag, textNode);
            }

            return words;
        },

        /**
         * Needleman-Wunsch global sequence alignment.
         *
         * Standard DP algorithm from bioinformatics adapted for word-level
         * alignment of DOM text ↔ Whisper timestamps.
         *
         * Substitution scoring uses Levenshtein-based similarity so that
         * near-matches ("dont" ↔ "don't", "naïve" ↔ "naive") are aligned
         * rather than gapped.
         *
         * Scoring parameters (tuned for TTS↔Whisper where most words
         * match exactly and differences are minor):
         *   Match:    +3  (exact normalized match)
         *   Similar:  +3 × similarity  (Levenshtein-weighted)
         *   Mismatch: -1  (similarity below threshold)
         *   Gap:      -2  (insertion/deletion)
         *
         * @param {string[]} seqA - Normalized DOM words
         * @param {string[]} seqB - Normalized Whisper words
         * @returns {Array<[number, number]>} Aligned index pairs [domIdx, tsIdx]
         */
        _needlemanWunsch(seqA, seqB) {
            const n = seqA.length;
            const m = seqB.length;

            // Bail out for degenerate cases
            if (n === 0 || m === 0) return [];

            const MATCH = 3;
            const MISMATCH = -1;
            const GAP = -2;
            const SIM_THRESHOLD = 0.5;

            // Substitution score: Levenshtein-weighted
            const subScore = (a, b) => {
                if (!a || !b) return MISMATCH;
                if (a === b) return MATCH;

                const sim = 1 - this._levenshtein(a, b) / Math.max(a.length, b.length);
                return sim >= SIM_THRESHOLD ? MATCH * sim : MISMATCH;
            };

            // ── DP matrix fill ───────────────────────────────────────────
            // Use Float32Array for cache-friendly, compact storage.
            // Layout: dp[(n+1) × (m+1)] row-major.
            const cols = m + 1;
            const dp = new Float32Array((n + 1) * cols);

            // Initialize first column and row (gap penalties)
            for (let i = 0; i <= n; i++) dp[i * cols] = i * GAP;
            for (let j = 0; j <= m; j++) dp[j] = j * GAP;

            for (let i = 1; i <= n; i++) {
                const row = i * cols;
                const prevRow = (i - 1) * cols;
                const wordA = seqA[i - 1];

                for (let j = 1; j <= m; j++) {
                    dp[row + j] = Math.max(
                        dp[prevRow + j - 1] + subScore(wordA, seqB[j - 1]),
                        dp[prevRow + j] + GAP,
                        dp[row + j - 1] + GAP
                    );
                }
            }

            // ── Traceback ────────────────────────────────────────────────
            const pairs = [];
            let i = n, j = m;

            while (i > 0 && j > 0) {
                const row = i * cols;
                const prevRow = (i - 1) * cols;
                const score = subScore(seqA[i - 1], seqB[j - 1]);
                const diag = dp[prevRow + j - 1] + score;

                if (dp[row + j] === diag) {
                    // Diagonal move — emit pair only if score > 0 (real match)
                    if (score > 0) {
                        pairs.push([i - 1, j - 1]);
                    }
                    i--;
                    j--;
                } else if (dp[row + j] === dp[prevRow + j] + GAP) {
                    // Up — DOM word has no Whisper match (deletion)
                    i--;
                } else {
                    // Left — Whisper word has no DOM match (insertion)
                    j--;
                }
            }

            pairs.reverse();
            return pairs;
        },

        /**
         * Levenshtein edit distance (Wagner-Fischer algorithm).
         *
         * Computes the minimum number of single-character edits (insert,
         * delete, substitute) to transform string a into string b.
         *
         * Optimized: single-row DP, O(min(n,m)) space.
         *
         * @param {string} a
         * @param {string} b
         * @returns {number} Edit distance
         */
        _levenshtein(a, b) {
            // Ensure a is the shorter string (optimize space)
            if (a.length > b.length) { const t = a; a = b; b = t; }

            const m = a.length;
            const n = b.length;

            // Single-row DP
            let prev = new Uint16Array(m + 1);
            let curr = new Uint16Array(m + 1);

            for (let j = 0; j <= m; j++) prev[j] = j;

            for (let i = 1; i <= n; i++) {
                curr[0] = i;
                const bChar = b.charCodeAt(i - 1);

                for (let j = 1; j <= m; j++) {
                    const cost = a.charCodeAt(j - 1) === bChar ? 0 : 1;
                    curr[j] = Math.min(
                        prev[j] + 1,        // deletion
                        curr[j - 1] + 1,    // insertion
                        prev[j - 1] + cost   // substitution
                    );
                }

                // Swap rows
                const tmp = prev;
                prev = curr;
                curr = tmp;
            }

            return prev[m];
        },

        // ═════════════════════════════════════════════════════════════════
        //  AUDIO EVENT HANDLERS
        // ═════════════════════════════════════════════════════════════════

        /** Begin RAF tracking loop. Called on audio play / seeked. */
        startTracking() {
            this._stopTracking();
            this._audio = this.$refs.ttsAudio;
            if (!this._audio || this._audio.paused) return;
            if (!this._timestamps.length || !this._wordSpans.length) return;
            this._scheduleTrack();
        },

        /** Pause tracking. Called on audio pause. */
        stopTracking() {
            this._stopTracking();
        },

        /** Reset to initial state. Called on audio ended. */
        resetPlayer() {
            this._stopTracking();
            this._currentIdx = -1;
            for (const { span } of this._wordSpans) {
                span.classList.remove('tts-word-active', 'tts-word-past');
            }
        },

        // ═════════════════════════════════════════════════════════════════
        //  TRACKING LOOP
        // ═════════════════════════════════════════════════════════════════

        _scheduleTrack() {
            const audio = this._audio;
            const ts = this._timestamps;
            const spans = this._wordSpans;

            const tick = () => {
                if (!audio || audio.paused || audio.ended) return;

                const t = audio.currentTime;

                // Binary search: find last timestamp with start ≤ t
                let lo = 0, hi = ts.length - 1, idx = -1;
                while (lo <= hi) {
                    const mid = (lo + hi) >> 1;
                    if (ts[mid].start <= t) { idx = mid; lo = mid + 1; }
                    else { hi = mid - 1; }
                }

                if (idx !== this._currentIdx) {
                    this._currentIdx = idx;

                    for (const { span, tsIdx } of spans) {
                        span.classList.toggle('tts-word-active', tsIdx === idx);
                        span.classList.toggle('tts-word-past', idx >= 0 && tsIdx < idx);
                    }

                    const active = spans.find(s => s.tsIdx === idx)?.span;
                    if (active) {
                        active.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
                    }
                }

                this._raf = requestAnimationFrame(tick);
            };

            this._raf = requestAnimationFrame(tick);
        },

        /** Seek audio to a specific time and resume playback. */
        _seekTo(time) {
            const audio = this.$refs.ttsAudio;
            if (!audio) return;
            audio.currentTime = time;
            if (audio.paused) audio.play();
        },

        /** Cancel any running RAF loop. */
        _stopTracking() {
            if (this._raf) {
                cancelAnimationFrame(this._raf);
                this._raf = null;
            }
        },
    };
}
