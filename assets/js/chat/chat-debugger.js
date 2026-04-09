/**
 * Chat Debugger
 *
 * Bottom-docked debug panel with Event Log and Workflow Tree tabs.
 * Provides panel toggle, tab switching, and vertical resize via pointer drag.
 *
 * State properties live here (spread into the Alpine component) so that
 * the panel height and active tab survive page reloads via localStorage.
 */

/**
 * Create debugger panel methods mixin
 * @returns {Object} State + methods for the debugger panel
 */
export function createDebuggerMethods() {
    return {
        // ── Panel state ────────────────────────────────────────────────
        debugActiveTab: localStorage.getItem('debugActiveTab') || 'events',
        debugPanelHeight: parseInt(localStorage.getItem('debugPanelHeight')) || 280,

        // Internal resize tracking (not persisted)
        _debugResizing: false,
        _debugResizeStartY: 0,
        _debugResizeStartHeight: 0,

        // ── Panel toggle ───────────────────────────────────────────────

        /**
         * Toggle the debug panel open/closed.
         * On open, auto-scrolls the event log to the latest entry.
         */
        toggleDebugPanel() {
            this.showDebugPanel = !this.showDebugPanel;
            if (this.showDebugPanel && this.debugActiveTab === 'events') {
                this.$nextTick(() => {
                    const container = this.$refs?.debugEventLogContainer;
                    if (container?.isConnected) {
                        container.scrollTop = container.scrollHeight;
                    }
                });
            }
        },

        // ── Tab switching ──────────────────────────────────────────────

        /**
         * Switch the active debugger tab.
         * @param {string} tab - 'events' or 'workflow'
         */
        setDebugTab(tab) {
            this.debugActiveTab = tab;
            localStorage.setItem('debugActiveTab', tab);
            if (tab === 'events') {
                this.$nextTick(() => {
                    const container = this.$refs?.debugEventLogContainer;
                    if (container?.isConnected) {
                        container.scrollTop = container.scrollHeight;
                    }
                });
            }
        },

        // ── Vertical resize (pointer-capture drag) ────────────────────

        /**
         * Begin resizing the debug panel.
         * Uses pointer capture so the drag works even if the cursor
         * leaves the handle element.
         */
        startDebugResize(e) {
            this._debugResizing = true;
            this._debugResizeStartY = e.clientY;
            this._debugResizeStartHeight = this.debugPanelHeight;
            e.currentTarget.setPointerCapture(e.pointerId);
        },

        /**
         * Update panel height while dragging. Clamped to 120–600 px.
         */
        onDebugResize(e) {
            if (!this._debugResizing) return;
            const delta = this._debugResizeStartY - e.clientY;
            this.debugPanelHeight = Math.max(120, Math.min(600, this._debugResizeStartHeight + delta));
        },

        /**
         * End resize and persist the height.
         */
        stopDebugResize() {
            if (!this._debugResizing) return;
            this._debugResizing = false;
            localStorage.setItem('debugPanelHeight', this.debugPanelHeight);
        },

        // ── Workflow graph resolution ──────────────────────────────────

        /**
         * Resolve the workflow graph to display in the debugger.
         * During execution, prefers the orchestrator's graph (resolved via
         * activeWorkflowContext). Falls back to the selected agent's graph.
         * @returns {Object|null} The workflow graph tree, or null
         */
        getDebugGraph() {
            // During execution, prefer the orchestrator agent's graph
            const orchId = this.activeWorkflowContext?.orchestratorId;
            if (orchId && this.AVAILABLE_AGENTS?.length) {
                const orch = this.AVAILABLE_AGENTS.find(
                    a => a.id === orchId || a.name === orchId
                );
                if (orch?.graph) return orch.graph;
            }
            // Fall back to the selected agent's graph
            return this.currentAgentData?.graph || null;
        },
    };
}
