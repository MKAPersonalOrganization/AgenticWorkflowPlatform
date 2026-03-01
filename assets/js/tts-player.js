/**
 * TTS Player — Alpine.js component for an immersive language-learning
 * listening session with synchronized word highlighting.
 *
 * ─── Features ──────────────────────────────────────────────────────
 *  ✦ Fullscreen Focus Mode  — native Fullscreen API, dark backdrop
 *  ✦ Real-time word highlight — Whisper timestamps + jsdiff alignment
 *  ✦ Custom transport controls — play/pause, seek bar, time display
 *  ✦ Playback speed — 0.5× to 2.0× in 0.25 steps
 *  ✦ Repeat / Loop — auto-replay on end
 *  ✦ A-B Loop — set markers to loop a specific section
 *  ✦ Sentence navigation — jump between sentence boundaries
 *  ✦ Keyboard shortcuts — Space, ←→, ↑↓, R, F, A, B, Escape
 *
 * ─── Alignment ─────────────────────────────────────────────────────
 *  jsdiff (Diff.diffArrays) — Myers O(ND) algorithm
 *  40M+ weekly npm downloads, powers Jest & Mocha.
 */
function ttsPlayer() {
    return {
        // ═══════════════════════════════════════════════════════════════
        //  STATE
        // ═══════════════════════════════════════════════════════════════

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

        /** @private Sentence boundary indices (start tsIdx of each sentence) */
        _sentences: [],

        /** @private Bound keyboard handler reference for cleanup */
        _keyHandler: null,

        // ── Public reactive state (Alpine) ────────────────────────────

        /** Fullscreen focus mode active */
        isFullscreen: false,

        /** Is audio currently playing */
        isPlaying: false,

        /** Current playback time (seconds) */
        currentTime: 0,

        /** Total duration (seconds) */
        duration: 0,

        /** Playback speed multiplier */
        speed: 1.0,

        /** Auto-repeat on end */
        repeat: false,

        /** A-B loop: start time (null = not set) */
        loopA: null,

        /** A-B loop: end time (null = not set) */
        loopB: null,

        /** Progress percentage (0-100) */
        progress: 0,

        /** Show keyboard shortcut hints overlay */
        showShortcuts: false,

        // ── Reading Mode Toggle ──────────────────────────────────────

        /** Transcript visibility mode: 'visible' | 'blurred' | 'hidden' */
        transcriptMode: 'visible',

        // ═══════════════════════════════════════════════════════════════
        //  LIFECYCLE
        // ═══════════════════════════════════════════════════════════════

        destroy() {
            this._stopTracking();
            this._removeKeyboardShortcuts();
            if (this._onFullscreenChange) {
                document.removeEventListener('fullscreenchange', this._onFullscreenChange);
            }
        },

        // ═══════════════════════════════════════════════════════════════
        //  TRANSCRIPT INITIALIZATION
        // ═══════════════════════════════════════════════════════════════

        /**
         * Render Markdown and optionally enhance with word-level highlighting.
         * Called via x-effect — re-runs when parsedResult arrives.
         */
        initTranscript(text, timestamps) {
            const box = this.$refs.transcriptBox;
            if (!box || !text) return;

            const mode = timestamps?.length ? 'ts' : 'md';
            if (this._initMode === mode) return;
            this._initMode = mode;

            // Step 1: Render Markdown → HTML
            box.innerHTML = window.renderMarkdown
                ? window.renderMarkdown(text)
                : text;

            // Step 2: If no timestamps, static Markdown is enough
            if (!timestamps?.length) {
                this._timestamps = [];
                this._wordSpans = [];
                this._sentences = [];
                return;
            }

            // Step 3: Wrap DOM words + align via jsdiff
            this._timestamps = timestamps;
            this._wordSpans = this._wrapAndAlign(box, timestamps);

            // Step 4: Detect sentence boundaries from timestamps
            this._sentences = this._detectSentences(timestamps);
        },

        // ═══════════════════════════════════════════════════════════════
        //  AUDIO INITIALIZATION
        // ═══════════════════════════════════════════════════════════════

        /**
         * Called when the audio element is ready.
         * Sets up all audio event listeners and fullscreen change listener.
         */
        initAudio() {
            if (this._audio) return; // Already initialized — single DOM, no re-init needed
            const audio = this.$refs.ttsAudio;
            if (!audio) return;
            this._audio = audio;

            audio.addEventListener('loadedmetadata', () => {
                this.duration = audio.duration || 0;
            });

            audio.addEventListener('timeupdate', () => {
                this.currentTime = audio.currentTime;
                this.duration = audio.duration || 0;
                this.progress = this.duration > 0
                    ? (audio.currentTime / this.duration) * 100
                    : 0;

                // A-B loop enforcement
                if (this.loopA !== null && this.loopB !== null) {
                    if (audio.currentTime >= this.loopB) {
                        audio.currentTime = this.loopA;
                    }
                }
            });

            audio.addEventListener('play', () => {
                this.isPlaying = true;
                this.startTracking();
            });

            audio.addEventListener('pause', () => {
                this.isPlaying = false;
                this.stopTracking();
            });

            audio.addEventListener('ended', () => {
                this.isPlaying = false;
                this.stopTracking();
                if (this.repeat) {
                    audio.currentTime = this.loopA ?? 0;
                    audio.play();
                } else {
                    this.resetPlayer();
                }
            });

            audio.addEventListener('seeked', () => {
                if (!audio.paused) this.startTracking();
            });

            // Fullscreen change listener
            this._onFullscreenChange = () => {
                this.isFullscreen = !!document.fullscreenElement;
                if (!this.isFullscreen) {
                    this._removeKeyboardShortcuts();
                    this.showShortcuts = false;
                }
            };
            document.addEventListener('fullscreenchange', this._onFullscreenChange);
        },

        // ═══════════════════════════════════════════════════════════════
        //  FULLSCREEN FOCUS MODE
        // ═══════════════════════════════════════════════════════════════

        toggleFullscreen() {
            const wrapper = this.$refs.ttsWrapper;
            if (!wrapper) return;

            if (!document.fullscreenElement) {
                wrapper.requestFullscreen().then(() => {
                    this.isFullscreen = true;
                    this._installKeyboardShortcuts();
                }).catch(() => { });
            } else {
                document.exitFullscreen().then(() => {
                    this.isFullscreen = false;
                }).catch(() => { });
            }
        },

        // ═══════════════════════════════════════════════════════════════
        //  TRANSPORT CONTROLS
        // ═══════════════════════════════════════════════════════════════

        togglePlay() {
            const audio = this._audio || this.$refs.ttsAudio;
            if (!audio) return;
            if (audio.paused) {
                audio.play();
            } else {
                audio.pause();
            }
        },

        seekRelative(delta) {
            const audio = this._audio || this.$refs.ttsAudio;
            if (!audio) return;
            audio.currentTime = Math.max(0, Math.min(audio.duration || 0, audio.currentTime + delta));
        },

        seekTo(pct) {
            const audio = this._audio || this.$refs.ttsAudio;
            if (!audio || !audio.duration) return;
            audio.currentTime = (pct / 100) * audio.duration;
        },

        seekFromClick(event) {
            const bar = event.currentTarget;
            const rect = bar.getBoundingClientRect();
            const pct = ((event.clientX - rect.left) / rect.width) * 100;
            this.seekTo(Math.max(0, Math.min(100, pct)));
        },

        // ── Playback Speed ────────────────────────────────────────────

        setSpeed(newSpeed) {
            this.speed = Math.round(Math.max(0.25, Math.min(3.0, newSpeed)) * 100) / 100;
            const audio = this._audio || this.$refs.ttsAudio;
            if (audio) audio.playbackRate = this.speed;
        },

        cycleSpeed() {
            const speeds = [0.5, 0.75, 1.0, 1.25, 1.5, 1.75, 2.0];
            const idx = speeds.indexOf(this.speed);
            const next = idx >= 0 ? (idx + 1) % speeds.length : 2;
            this.setSpeed(speeds[next]);
        },

        // ── Repeat / Loop ─────────────────────────────────────────────

        toggleRepeat() {
            this.repeat = !this.repeat;
        },

        setLoopA() {
            const audio = this._audio;
            if (!audio) return;
            this.loopA = audio.currentTime;
            if (this.loopB !== null && this.loopB <= this.loopA) {
                this.loopB = null;
            }
        },

        setLoopB() {
            const audio = this._audio;
            if (!audio) return;
            if (this.loopA === null) return;
            this.loopB = audio.currentTime;
            if (this.loopB <= this.loopA) {
                this.loopB = null;
            }
        },

        clearLoop() {
            this.loopA = null;
            this.loopB = null;
        },

        get hasLoop() {
            return this.loopA !== null && this.loopB !== null;
        },

        // ── Reading Mode ──────────────────────────────────────────────

        cycleTranscriptMode() {
            const modes = ['visible', 'blurred', 'hidden'];
            const idx = modes.indexOf(this.transcriptMode);
            this.transcriptMode = modes[(idx + 1) % modes.length];
        },

        toggleTranscriptVisibility() {
            this.transcriptMode = this.transcriptMode === 'hidden' ? 'visible' : 'hidden';
        },

        peekSentence(event) {
            if (this.transcriptMode !== 'blurred') return;
            const box = this.$refs.transcriptBox;
            if (!box) return;
            // Find the closest block-level child of the transcript box
            let target = event.target;
            while (target && target.parentNode !== box) {
                target = target.parentNode;
            }
            if (target && target !== box) {
                target.classList.add('tts-sentence-peek');
                setTimeout(() => target.classList.remove('tts-sentence-peek'), 3000);
            }
        },

        // ── Sentence Navigation ───────────────────────────────────────────

        _detectSentences(timestamps) {
            if (!timestamps?.length) return [];
            const boundaries = [0];
            for (let i = 0; i < timestamps.length - 1; i++) {
                const word = timestamps[i].word;
                const gap = timestamps[i + 1].start - timestamps[i].end;
                if (/[.!?。？！]\s*$/.test(word) || gap > 0.4) {
                    boundaries.push(i + 1);
                }
            }
            return boundaries;
        },

        prevSentence() {
            const audio = this._audio;
            if (!audio || !this._sentences.length) return;
            const t = audio.currentTime;
            const ts = this._timestamps;

            let currentSent = 0;
            for (let i = this._sentences.length - 1; i >= 0; i--) {
                if (ts[this._sentences[i]].start <= t + 0.1) {
                    currentSent = i;
                    break;
                }
            }

            const sentStart = ts[this._sentences[currentSent]].start;
            if (t - sentStart > 2) {
                audio.currentTime = sentStart;
            } else {
                const prevIdx = Math.max(0, currentSent - 1);
                audio.currentTime = ts[this._sentences[prevIdx]].start;
            }
            if (audio.paused) audio.play();
        },

        nextSentence() {
            const audio = this._audio;
            if (!audio || !this._sentences.length) return;
            const t = audio.currentTime;
            const ts = this._timestamps;

            for (let i = 0; i < this._sentences.length; i++) {
                if (ts[this._sentences[i]].start > t + 0.1) {
                    audio.currentTime = ts[this._sentences[i]].start;
                    if (audio.paused) audio.play();
                    return;
                }
            }
            audio.currentTime = audio.duration || 0;
        },

        // ═══════════════════════════════════════════════════════════════
        //  KEYBOARD SHORTCUTS (active in fullscreen)
        // ═══════════════════════════════════════════════════════════════

        _installKeyboardShortcuts() {
            this._removeKeyboardShortcuts();
            this._keyHandler = (e) => {
                if (['INPUT', 'TEXTAREA', 'SELECT'].includes(e.target.tagName)) return;

                switch (e.code) {
                    case 'Space':
                        e.preventDefault();
                        this.togglePlay();
                        break;
                    case 'ArrowLeft':
                        e.preventDefault();
                        e.shiftKey ? this.prevSentence() : this.seekRelative(-5);
                        break;
                    case 'ArrowRight':
                        e.preventDefault();
                        e.shiftKey ? this.nextSentence() : this.seekRelative(5);
                        break;
                    case 'ArrowUp':
                        e.preventDefault();
                        this.setSpeed(this.speed + 0.25);
                        break;
                    case 'ArrowDown':
                        e.preventDefault();
                        this.setSpeed(this.speed - 0.25);
                        break;
                    case 'KeyR':
                        e.preventDefault();
                        this.toggleRepeat();
                        break;
                    case 'KeyF':
                        e.preventDefault();
                        this.toggleFullscreen();
                        break;
                    case 'KeyA':
                        e.preventDefault();
                        this.setLoopA();
                        break;
                    case 'KeyB':
                        e.preventDefault();
                        this.setLoopB();
                        break;
                    case 'KeyC':
                        e.preventDefault();
                        this.clearLoop();
                        break;
                    case 'KeyT':
                        e.preventDefault();
                        this.cycleTranscriptMode();
                        break;
                    case 'KeyH':
                        e.preventDefault();
                        this.toggleTranscriptVisibility();
                        break;
                    case 'Escape':
                        e.preventDefault();
                        if (this.showShortcuts) {
                            this.showShortcuts = false;
                        } else {
                            this.toggleFullscreen();
                        }
                        break;
                    case 'Slash':
                        if (e.shiftKey) {
                            e.preventDefault();
                            this.showShortcuts = !this.showShortcuts;
                        }
                        break;
                }
            };
            document.addEventListener('keydown', this._keyHandler);
        },

        _removeKeyboardShortcuts() {
            if (this._keyHandler) {
                document.removeEventListener('keydown', this._keyHandler);
                this._keyHandler = null;
            }
        },

        // ═══════════════════════════════════════════════════════════════
        //  ALIGNMENT ENGINE (jsdiff)
        // ═══════════════════════════════════════════════════════════════

        _wrapAndAlign(container, timestamps) {
            const domWords = this._wrapDOMWords(container);
            if (!domWords.length) return [];

            const norm = s => s.toLowerCase().replace(/[^\p{L}\p{N}]/gu, '');
            const domNorm = domWords.map(d => norm(d.text));
            const tsNorm = timestamps.map(t => norm(t.word));

            const pairs = this._diffAlign(domNorm, tsNorm);

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

        _diffAlign(domNorm, tsNorm) {
            if (typeof Diff === 'undefined' || !Diff.diffArrays) {
                return this._fallbackAlign(domNorm, tsNorm);
            }

            const changes = Diff.diffArrays(tsNorm, domNorm, {
                comparator: (a, b) => {
                    if (a === b) return true;
                    if (!a || !b) return false;
                    return a.startsWith(b) || b.startsWith(a);
                },
            });

            const pairs = [];
            let tsPtr = 0;
            let domPtr = 0;

            for (const change of changes) {
                const count = change.value.length;
                if (!change.added && !change.removed) {
                    for (let k = 0; k < count; k++) {
                        pairs.push([domPtr + k, tsPtr + k]);
                    }
                    tsPtr += count;
                    domPtr += count;
                } else if (change.removed) {
                    tsPtr += count;
                } else {
                    domPtr += count;
                }
            }

            return pairs;
        },

        _fallbackAlign(domNorm, tsNorm) {
            const pairs = [];
            let tsPtr = 0;
            for (let d = 0; d < domNorm.length && tsPtr < tsNorm.length; d++) {
                const dw = domNorm[d];
                const tw = tsNorm[tsPtr];
                if (dw && tw && (dw === tw || dw.startsWith(tw) || tw.startsWith(dw))) {
                    pairs.push([d, tsPtr]);
                    tsPtr++;
                }
            }
            return pairs;
        },

        _wrapDOMWords(container) {
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
                        frag.appendChild(document.createTextNode(match[1]));
                    } else {
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

        // ═══════════════════════════════════════════════════════════════
        //  TRACKING
        // ═══════════════════════════════════════════════════════════════

        startTracking() {
            this._stopTracking();
            this._audio = this.$refs.ttsAudio;
            if (!this._audio || this._audio.paused) return;
            if (!this._timestamps.length || !this._wordSpans.length) return;
            this._scheduleTrack();
        },

        stopTracking() {
            this._stopTracking();
        },

        resetPlayer() {
            this._stopTracking();
            this._currentIdx = -1;
            for (const { span } of this._wordSpans) {
                span.classList.remove('tts-word-active', 'tts-word-past');
            }
        },

        _scheduleTrack() {
            const audio = this._audio;
            const ts = this._timestamps;
            const spans = this._wordSpans;

            const tick = () => {
                if (!audio || audio.paused || audio.ended) return;

                const t = audio.currentTime;

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
                        // Use the scroll container ref (wrapper in both normal + fullscreen)
                        const scrollBox = this.$refs.transcriptScroll || this.$refs.transcriptBox;
                        if (scrollBox) {
                            const boxRect = scrollBox.getBoundingClientRect();
                            const spanRect = active.getBoundingClientRect();
                            if (spanRect.top < boxRect.top || spanRect.bottom > boxRect.bottom) {
                                active.scrollIntoView({ behavior: 'smooth', block: 'center' });
                            }
                        }
                    }
                }

                this._raf = requestAnimationFrame(tick);
            };

            this._raf = requestAnimationFrame(tick);
        },

        _seekTo(time) {
            const audio = this.$refs.ttsAudio;
            if (!audio) return;
            audio.currentTime = time;
            if (audio.paused) audio.play();
        },

        _stopTracking() {
            if (this._raf) {
                cancelAnimationFrame(this._raf);
                this._raf = null;
            }
        },

        // ═══════════════════════════════════════════════════════════════
        //  UTILITY
        // ═══════════════════════════════════════════════════════════════

        formatTime(seconds) {
            if (!seconds || isNaN(seconds)) return '0:00';
            const m = Math.floor(seconds / 60);
            const s = Math.floor(seconds % 60);
            return `${m}:${s.toString().padStart(2, '0')}`;
        },
    };
}
