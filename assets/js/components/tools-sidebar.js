/**
 * Tools Dialog Component
 * Handles plugin browsing and MCP server management in a dialog.
 * Uses a unified detail view for both plugins and MCP connectors.
 */

export const toolsDialog = () => ({
    plugins: [],
    loading: false,
    search: '',
    mcpSearch: '',
    activeTab: 'plugins',
    _loaded: false,

    /**
     * Unified detail item (null = list view).
     * Normalised shape:
     *   _type: 'plugin' | 'mcp'
     *   _name, _description, _subtitle, _icon: string
     *   _tools: [{id, name, description, category?}]
     * Plus the raw original fields (id, category, status, server_url, …)
     */
    detailItem: null,

    init() {
        // Refresh plugins when MCP connection state changes
        window.addEventListener('mcp-state-changed', () => {
            this.fetchPlugins();
        });

        // Listen for MCP detail requests from the nested mcpConnectors component
        window.addEventListener('show-mcp-detail', (e) => {
            this.openMcpDetail(e.detail);
        });

        // Close detail if the MCP connector was removed
        window.addEventListener('mcp-connector-removed', (e) => {
            if (this.detailItem?._type === 'mcp' && this.detailItem?.id === e.detail) {
                this.closeDetail();
            }
        });

        // Update MCP detail connector data when state changes (e.g. after OAuth)
        window.addEventListener('mcp-detail-updated', (e) => {
            if (this.detailItem?._type === 'mcp' && this.detailItem?.id === e.detail?.id) {
                // Re-normalise with fresh connector data, keep existing tools
                const tools = this.detailItem._tools;
                this.detailItem = this._normaliseMcp(e.detail, tools);
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

    // ── Normalisation helpers ───────────────────────────────────────────

    _normalisePlugin(plugin) {
        return {
            ...plugin,
            _type: 'plugin',
            _name: plugin.name,
            _description: plugin.description,
            _subtitle: plugin.category,
            _icon: plugin.icon,
            _tools: plugin.tools || [],
        };
    },

    _normaliseMcp(connector, tools) {
        // Build a globe SVG as the icon (matches the templ icon.Globe output)
        const globeSvg = '<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M12 2a14.5 14.5 0 0 0 0 20 14.5 14.5 0 0 0 0-20"/><path d="M2 12h20"/></svg>';
        return {
            ...connector,
            _type: 'mcp',
            _name: connector.title,
            _description: connector.server_url || '',
            _subtitle: connector.server_url || '',
            _icon: globeSvg,
            _tools: tools || [],
        };
    },

    // ── Navigation (unified) ────────────────────────────────────────────

    /** Open the detail view for a plugin */
    openProfile(plugin) {
        this.detailItem = this._normalisePlugin(plugin);
    },

    /** Open the detail view for an MCP connector */
    async openMcpDetail(connector) {
        this.detailItem = this._normaliseMcp(connector, []);

        try {
            const res = await fetch(`/v1/mcp/connectors/${connector.id}/tools`);
            if (res.ok) {
                const data = await res.json();
                this.detailItem = this._normaliseMcp(connector, data.tools || []);
            }
        } catch (err) {
            console.error('[MCP] Failed to fetch tools:', err);
        }
    },

    /** Close whatever detail view is open and return to the list */
    closeDetail() {
        this.detailItem = null;
    },

    // Keep old names as aliases for backward compat with any inline handlers
    closeProfile() { this.closeDetail(); },
    closeMcpDetail() { this.closeDetail(); },
});
