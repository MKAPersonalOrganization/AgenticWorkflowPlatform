/**
 * Floating Activity Panel
 *
 * The multi-agent activity surface lives in its own draggable, minimizable
 * floating window — decoupled from the right side panel. Position and
 * minimized/expanded state persist to localStorage so the user's chosen
 * layout sticks across reloads.
 *
 * Pointer-based drag matches the established eventlog pattern: the header
 * is the drag handle, the panel body is independently scrollable.
 *
 * Premium-feel defaults:
 *   • Opens at the top-right of the viewport on first run.
 *   • Stays out of the way (clamped to viewport bounds on every move so
 *     it can't be dragged off-screen).
 *   • Minimized state collapses to a compact pill that still shows live
 *     workflow status; click the pill to restore.
 */

const PANEL_WIDTH = 460;
const PANEL_HEIGHT = 600;
const MIN_WIDTH = 380;
const MIN_HEIGHT = 240;
const MARGIN = 16;

function clamp(value, min, max) {
    return Math.max(min, Math.min(max, value));
}

function defaultX() {
    if (typeof window === 'undefined') return MARGIN;
    return clamp(window.innerWidth - PANEL_WIDTH - MARGIN, MARGIN, window.innerWidth - MIN_WIDTH);
}

function defaultY() {
    return MARGIN + 56; // leave room below the top nav bar
}

export function createActivityPanelMethods() {
    return {
        // ── Visibility ────────────────────────────────────────────────
        activityPanelOpen: localStorage.getItem('activityPanelOpen') === 'true',
        activityPanelMinimized: localStorage.getItem('activityPanelMinimized') === 'true',

        // ── Per-leaf state-value expansion ────────────────────────────
        // Tracks which output_keys the user has clicked to expand their
        // full value preview. Keyed by output_key string for stability
        // across rerenders.
        expandedStateKeys: {},

        toggleExpandStateKey(key) {
            if (!key) return;
            this.expandedStateKeys = {
                ...this.expandedStateKeys,
                [key]: !this.expandedStateKeys[key],
            };
        },

        async copyStateValue(value) {
            const text = typeof value === 'string' ? value : JSON.stringify(value, null, 2);
            try {
                await navigator.clipboard.writeText(text);
                this.showToast?.(window.i18n.t('Copied'), 'success', 1200);
            } catch (err) {
                console.error('Failed to copy state value:', err);
            }
        },

        // ── Position (top-left in viewport coords) ────────────────────
        activityPanelX: parseInt(localStorage.getItem('activityPanelX')) || defaultX(),
        activityPanelY: parseInt(localStorage.getItem('activityPanelY')) || defaultY(),

        // ── Size (resizable) ──────────────────────────────────────────
        activityPanelW: parseInt(localStorage.getItem('activityPanelW')) || PANEL_WIDTH,
        activityPanelH: parseInt(localStorage.getItem('activityPanelH')) || PANEL_HEIGHT,

        // ── Transient drag / resize state ─────────────────────────────
        activityPanelDragging: false,
        activityPanelDragMoved: false,
        _apDragOX: 0,
        _apDragOY: 0,

        activityPanelResizing: false,
        _apResizeStartW: 0,
        _apResizeStartH: 0,
        _apResizeStartX: 0,
        _apResizeStartY: 0,

        // ── Open / close / toggle ─────────────────────────────────────
        openActivityPanel() {
            this.activityPanelOpen = true;
            this.activityPanelMinimized = false;
            localStorage.setItem('activityPanelOpen', 'true');
            localStorage.setItem('activityPanelMinimized', 'false');
            this.__clampActivityPanelToViewport();
        },

        closeActivityPanel() {
            this.activityPanelOpen = false;
            localStorage.setItem('activityPanelOpen', 'false');
        },

        toggleActivityPanel() {
            if (this.activityPanelOpen && !this.activityPanelMinimized) {
                // Visible & expanded → minimize first (less destructive than close).
                this.minimizeActivityPanel();
            } else if (this.activityPanelOpen && this.activityPanelMinimized) {
                // Minimized → expand.
                this.restoreActivityPanel();
            } else {
                this.openActivityPanel();
            }
        },

        minimizeActivityPanel() {
            this.activityPanelMinimized = true;
            localStorage.setItem('activityPanelMinimized', 'true');
        },

        restoreActivityPanel() {
            this.activityPanelMinimized = false;
            this.activityPanelOpen = true;
            localStorage.setItem('activityPanelMinimized', 'false');
            localStorage.setItem('activityPanelOpen', 'true');
            this.__clampActivityPanelToViewport();
        },

        // ── Drag handlers (pointer-captured for reliable tracking) ────
        startActivityPanelDrag(e) {
            // Ignore drags that start on interactive controls inside the header.
            if (e.target.closest('[data-no-drag]')) return;
            this.activityPanelDragging = true;
            this.activityPanelDragMoved = false;
            this._apDragOX = e.clientX - this.activityPanelX;
            this._apDragOY = e.clientY - this.activityPanelY;
            try { e.currentTarget.setPointerCapture(e.pointerId); } catch (err) { /* noop */ }
        },

        onActivityPanelMove(e) {
            if (!this.activityPanelDragging) return;
            const nx = e.clientX - this._apDragOX;
            const ny = e.clientY - this._apDragOY;
            if (Math.abs(nx - this.activityPanelX) > 3 || Math.abs(ny - this.activityPanelY) > 3) {
                this.activityPanelDragMoved = true;
            }
            const w = this.activityPanelMinimized ? 260 : this.activityPanelW;
            const h = this.activityPanelMinimized ? 56 : this.activityPanelH;
            this.activityPanelX = clamp(nx, MARGIN - w + 80, window.innerWidth - 80);
            this.activityPanelY = clamp(ny, MARGIN, window.innerHeight - 80);
        },

        stopActivityPanelDrag() {
            if (!this.activityPanelDragging) return;
            this.activityPanelDragging = false;
            localStorage.setItem('activityPanelX', this.activityPanelX);
            localStorage.setItem('activityPanelY', this.activityPanelY);
        },

        // ── Resize handlers (bottom-right corner grip) ────────────────
        startActivityPanelResize(e) {
            this.activityPanelResizing = true;
            this._apResizeStartW = this.activityPanelW;
            this._apResizeStartH = this.activityPanelH;
            this._apResizeStartX = e.clientX;
            this._apResizeStartY = e.clientY;
            try { e.currentTarget.setPointerCapture(e.pointerId); } catch (err) { /* noop */ }
            e.stopPropagation();
        },

        onActivityPanelResize(e) {
            if (!this.activityPanelResizing) return;
            const dx = e.clientX - this._apResizeStartX;
            const dy = e.clientY - this._apResizeStartY;
            this.activityPanelW = clamp(this._apResizeStartW + dx, MIN_WIDTH, window.innerWidth - this.activityPanelX - MARGIN);
            this.activityPanelH = clamp(this._apResizeStartH + dy, MIN_HEIGHT, window.innerHeight - this.activityPanelY - MARGIN);
        },

        stopActivityPanelResize() {
            if (!this.activityPanelResizing) return;
            this.activityPanelResizing = false;
            localStorage.setItem('activityPanelW', this.activityPanelW);
            localStorage.setItem('activityPanelH', this.activityPanelH);
        },

        // ── Viewport guard ────────────────────────────────────────────
        __clampActivityPanelToViewport() {
            if (typeof window === 'undefined') return;
            const w = this.activityPanelMinimized ? 260 : this.activityPanelW;
            const h = this.activityPanelMinimized ? 56 : this.activityPanelH;
            this.activityPanelX = clamp(this.activityPanelX, MARGIN - w + 80, window.innerWidth - 80);
            this.activityPanelY = clamp(this.activityPanelY, MARGIN, window.innerHeight - 80);
        },
    };
}
