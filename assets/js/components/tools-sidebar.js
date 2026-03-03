/**
 * Tools Dialog Component
 * Handles plugin browsing and MCP server management in a dialog.
 * Replaces the old sidebar approach with a centered dialog
 * matching the agent store and resources dialog patterns.
 */

export const toolsDialog = () => ({
    plugins: [],
    loading: false,
    search: '',
    mcpSearch: '',
    activeTab: 'plugins',
    _loaded: false,

    /** The plugin whose detail panel is open (null = closed) */
    profilePlugin: null,

    /** MCP detail view state (lifted from mcpConnectors for in-dialog view) */
    mcpDetailConnector: null,
    mcpDetailTools: [],

    init() {
        // Refresh plugins when MCP connection state changes
        window.addEventListener('mcp-state-changed', () => {
            this.fetchPlugins();
        });

        // Listen for MCP detail requests from the nested mcpConnectors component
        window.addEventListener('show-mcp-detail', (e) => {
            this.openMcpDetail(e.detail);
        });

        // Close MCP detail if the connector was removed
        window.addEventListener('mcp-connector-removed', (e) => {
            if (this.mcpDetailConnector?.id === e.detail) {
                this.closeMcpDetail();
            }
        });

        // Update MCP detail connector when state changes (e.g. after OAuth)
        window.addEventListener('mcp-detail-updated', (e) => {
            if (this.mcpDetailConnector?.id === e.detail?.id) {
                this.mcpDetailConnector = e.detail;
            }
        });
    },

    /** Called when the dialog opens */
    async open() {
        if (!this._loaded) {
            await this.fetchPlugins();
            this._loaded = true;
        }
    },

    /** Refresh all data */
    async fetchAll() {
        await this.fetchPlugins();
    },

    async fetchPlugins() {
        this.loading = true;
        try {
            const response = await fetch('/v1/plugins');
            if (response.ok) {
                const data = await response.json();
                if (data.plugins && Array.isArray(data.plugins)) {
                    this.plugins = data.plugins;
                }
            } else {
                this.plugins = [];
            }
        } catch (err) {
            console.error('Failed to fetch plugins:', err);
            this.plugins = [];
        } finally {
            this.loading = false;
        }
    },

    get filteredPlugins() {
        if (!this.search) return this.plugins;
        const q = this.search.toLowerCase();
        return this.plugins.filter(plugin =>
            plugin.name.toLowerCase().includes(q) ||
            plugin.description.toLowerCase().includes(q) ||
            plugin.category.toLowerCase().includes(q) ||
            plugin.tools.some(tool =>
                tool.name.toLowerCase().includes(q) ||
                tool.description.toLowerCase().includes(q)
            )
        );
    },

    get pluginSummary() {
        let totalTools = 0;
        this.plugins.forEach(p => {
            totalTools += (p.tools || []).length;
        });
        return {
            plugins: this.plugins.length,
            tools: totalTools,
        };
    },

    // ── Navigation ──────────────────────────────────────────────────────

    /** Show the plugin detail view (in-dialog, same as agents pattern) */
    openProfile(plugin) {
        this.profilePlugin = plugin;
    },

    /** Close the detail view and return to list */
    closeProfile() {
        this.profilePlugin = null;
    },

    // ── MCP Detail View ─────────────────────────────────────────────────

    /** Show the MCP connector detail view (in-dialog, replaces nested dialog) */
    async openMcpDetail(connector) {
        this.mcpDetailConnector = connector;
        this.mcpDetailTools = [];

        try {
            const res = await fetch(`/v1/mcp/connectors/${connector.id}/tools`);
            if (res.ok) {
                const data = await res.json();
                this.mcpDetailTools = data.tools || [];
            }
        } catch (err) {
            console.error('[MCP] Failed to fetch tools:', err);
        }
    },

    /** Close MCP detail view and return to list */
    closeMcpDetail() {
        this.mcpDetailConnector = null;
        this.mcpDetailTools = [];
    },

});
