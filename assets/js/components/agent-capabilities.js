/**
 * Agent Capabilities Component
 *
 * Displays a read-only overview of the current agent's enabled tools,
 * knowledge bases, and filesystems in an Alpine-driven panel.  Data is
 * fetched from the same API endpoints used by the agents-dialog editor,
 * but only on demand (when the panel is opened) and cached per agent ID
 * to avoid redundant network requests.
 *
 * The panel uses Alpine-native x-show positioning rather than the TUI
 * popover component because the popover portals its content to
 * document.body, which breaks Alpine scope merging.
 */
export const agentCapabilities = () => ({
    // Panel open state
    open: false,

    // Data
    plugins: [],
    workspaces: [],
    filesystems: [],
    loading: false,

    // UI toggles for collapsible sections
    showToolsSection: true,
    showKBSection: true,
    showFSSection: true,

    // Cache key — skip re-fetch if the agent hasn't changed
    _cachedAgentId: null,

    // ── Computed properties ──
    // This component is nested inside x-data="chat", so Alpine's scope
    // merging gives us direct access to `selectedAgent` and
    // `currentAgentData` from the parent chat component.

    get agentName() {
        return this.currentAgentData?.name || 'Agent';
    },

    get agentDescription() {
        return this.currentAgentData?.description || '';
    },

    get agentIcon() {
        return this.currentAgentData?.icon || '';
    },

    get _agentId() {
        return this.selectedAgent || '';
    },

    get enabledTools() {
        let count = 0;
        for (const p of this.plugins) {
            if (!p.enabled) continue;
            for (const t of p.tools) {
                if (t.enabled) count++;
            }
        }
        return count;
    },

    get enabledKBs() {
        return this.workspaces.filter(w => w.enabled).length;
    },

    get enabledFS() {
        return this.filesystems.filter(f => f.enabled).length;
    },

    get totalEnabled() {
        return this.enabledTools + this.enabledKBs + this.enabledFS;
    },

    // ── Methods ──

    /**
     * Count enabled tools within a single plugin (used in template).
     * @param {Object} plugin
     * @returns {number}
     */
    enabledToolCount(plugin) {
        return plugin.tools.filter(t => t.enabled).length;
    },

    /**
     * Toggle the panel open/closed.  Loads data lazily on first open.
     */
    toggle() {
        this.open = !this.open;
        if (this.open) {
            this._loadIfNeeded();
        }
    },

    /**
     * Initialise the component.
     */
    init() {
        // Listen for agent changes to invalidate the cache and close panel.
        this._onAgentSelected = () => {
            this._cachedAgentId = null;
            this.open = false;
        };
        window.addEventListener('agent-selected', this._onAgentSelected);
    },

    destroy() {
        if (this._onAgentSelected) {
            window.removeEventListener('agent-selected', this._onAgentSelected);
        }
    },

    /**
     * Load capabilities if the current agent differs from the cached one.
     */
    async _loadIfNeeded() {
        const agentId = this._agentId;
        if (!agentId) return;
        if (agentId === this._cachedAgentId) return;

        this.loading = true;
        this._cachedAgentId = agentId;

        try {
            const [pluginsResp, workspacesResp, fsResp] = await Promise.all([
                fetch(`/v1/plugins?agent_id=${encodeURIComponent(agentId)}`),
                fetch(`/v1/agents/${encodeURIComponent(agentId)}/workspaces`),
                fetch(`/v1/agents/${encodeURIComponent(agentId)}/filesystems`),
            ]);

            if (pluginsResp.ok) {
                const data = await pluginsResp.json();
                this.plugins = data.plugins || [];
            } else {
                this.plugins = [];
            }

            if (workspacesResp.ok) {
                const data = await workspacesResp.json();
                this.workspaces = data.workspaces || [];
            } else {
                this.workspaces = [];
            }

            if (fsResp.ok) {
                const data = await fsResp.json();
                this.filesystems = data || [];
            } else {
                this.filesystems = [];
            }
        } catch (err) {
            console.error('Failed to load agent capabilities:', err);
            this.plugins = [];
            this.workspaces = [];
            this.filesystems = [];
        } finally {
            this.loading = false;
        }
    },
});
