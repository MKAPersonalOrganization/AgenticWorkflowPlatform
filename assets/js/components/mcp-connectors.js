/**
 * MCP Connectors Component
 * Manages user-registered MCP (Model Context Protocol) plugin connectors.
 * Handles connection lifecycle, OAuth authorization, and tool discovery.
 */
export const mcpConnectors = () => ({
    mcpConnectors: [],
    mcpLoading: false,
    mcpError: '',

    // "Add MCP Server" dialog state
    mcpNewUrl: '',
    mcpNewTitle: '',
    mcpNewClientID: '',
    mcpNewClientSecret: '',

    // Detail dialog state
    mcpDetailConnector: null,
    mcpDetailTools: [],

    // OAuth popup reference
    _oauthWindow: null,

    init() {
        this.mcpFetchConnectors();

        // Listen for OAuth callback messages from popup
        window.addEventListener('message', (event) => {
            if (event.data?.type === 'mcp-oauth-callback') {
                this._handleOAuthMessage(event.data);
            }
        });
    },

    // ── CRUD ────────────────────────────────────────────────────────────

    async mcpFetchConnectors() {
        try {
            const res = await fetch('/v1/mcp/connectors');
            if (res.ok) {
                const data = await res.json();
                this.mcpConnectors = data.connectors || [];
            }
        } catch (err) {
            console.error('[MCP] Failed to fetch connectors:', err);
        }
    },

    async mcpAddConnector() {
        if (!this.mcpNewUrl) return;

        this.mcpLoading = true;
        this.mcpError = '';

        try {
            // 1. Create the connector record
            const createRes = await fetch('/v1/mcp/connectors', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    title: this.mcpNewTitle || this._titleFromUrl(this.mcpNewUrl),
                    server_url: this.mcpNewUrl,
                }),
            });

            if (!createRes.ok) {
                const errBody = await createRes.json().catch(() => ({}));
                throw new Error(errBody.error || `Server returned ${createRes.status}`);
            }

            const connector = await createRes.json();

            // 2. Attempt to connect
            const connectRes = await fetch(`/v1/mcp/connectors/${connector.id}/connect`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    oauth_client_id: this.mcpNewClientID || undefined,
                    oauth_client_secret: this.mcpNewClientSecret || undefined,
                }),
            });

            if (connectRes.ok) {
                const connectData = await connectRes.json();

                // If OAuth is required, open popup
                if (connectData.auth_url) {
                    this._openOAuthPopup(connectData.auth_url);
                }
            }

            // 3. Refresh list & close dialog
            await this.mcpFetchConnectors();
            window.dispatchEvent(new CustomEvent('mcp-state-changed'));
            this._resetAddForm();
            if (window.tui?.dialog) {
                window.tui.dialog.close('mcp-add-dialog');
            }
        } catch (err) {
            console.error('[MCP] Failed to add connector:', err);
            this.mcpError = err.message || 'Failed to add MCP server';
        } finally {
            this.mcpLoading = false;
        }
    },

    async mcpRemove(connectorId) {
        try {
            const res = await fetch(`/v1/mcp/connectors/${connectorId}`, {
                method: 'DELETE',
            });
            if (res.ok) {
                this.mcpConnectors = this.mcpConnectors.filter(c => c.id !== connectorId);
                // Close detail dialog if showing this connector
                if (this.mcpDetailConnector?.id === connectorId) {
                    this.mcpCloseDetail();
                }
                window.dispatchEvent(new CustomEvent('mcp-state-changed'));
            }
        } catch (err) {
            console.error('[MCP] Failed to remove connector:', err);
        }
    },

    // ── Connection lifecycle ────────────────────────────────────────────

    async mcpConnect(connectorId) {
        const conn = this.mcpConnectors.find(c => c.id === connectorId);
        if (conn) conn.status = 'connecting';

        try {
            const res = await fetch(`/v1/mcp/connectors/${connectorId}/connect`, {
                method: 'POST',
            });
            if (res.ok) {
                const data = await res.json();
                if (data.auth_url) {
                    this._openOAuthPopup(data.auth_url);
                }
            }
            await this.mcpFetchConnectors();
            window.dispatchEvent(new CustomEvent('mcp-state-changed'));
        } catch (err) {
            console.error('[MCP] Failed to connect:', err);
            if (conn) conn.status = 'error';
        }
    },

    async mcpDisconnect(connectorId) {
        try {
            await fetch(`/v1/mcp/connectors/${connectorId}/disconnect`, {
                method: 'POST',
            });
            await this.mcpFetchConnectors();
            window.dispatchEvent(new CustomEvent('mcp-state-changed'));
        } catch (err) {
            console.error('[MCP] Failed to disconnect:', err);
        }
    },

    async mcpAuthorize(connector) {
        // Re-trigger connection to get a fresh auth_url
        await this.mcpConnect(connector.id);
    },

    // ── Detail dialog ───────────────────────────────────────────────────

    async mcpOpenDetail(connector) {
        this.mcpDetailConnector = connector;
        this.mcpDetailTools = [];

        // Fetch tools for this connector
        try {
            const res = await fetch(`/v1/mcp/connectors/${connector.id}/tools`);
            if (res.ok) {
                const data = await res.json();
                this.mcpDetailTools = data.tools || [];
            }
        } catch (err) {
            console.error('[MCP] Failed to fetch tools:', err);
        }

        this.$nextTick(() => {
            if (window.tui?.dialog) {
                window.tui.dialog.open('mcp-detail-dialog');
            }
        });
    },

    mcpCloseDetail() {
        if (window.tui?.dialog) {
            window.tui.dialog.close('mcp-detail-dialog');
        }
        this.mcpDetailConnector = null;
        this.mcpDetailTools = [];
    },

    // ── OAuth popup ─────────────────────────────────────────────────────

    _openOAuthPopup(authUrl) {
        const w = 600, h = 700;
        const left = window.screenX + (window.outerWidth - w) / 2;
        const top = window.screenY + (window.outerHeight - h) / 2;

        this._oauthWindow = window.open(
            authUrl,
            'mcp-oauth',
            `width=${w},height=${h},left=${left},top=${top},popup=1`
        );
    },

    async _handleOAuthMessage(data) {
        if (this._oauthWindow) {
            this._oauthWindow.close();
            this._oauthWindow = null;
        }

        if (data.success) {
            // OAuth succeeded — refresh connectors to pick up new status
            await this.mcpFetchConnectors();
            window.dispatchEvent(new CustomEvent('mcp-state-changed'));
            // Also update the detail dialog if it's showing this connector
            if (this.mcpDetailConnector?.id === data.connector_id) {
                const conn = this.mcpConnectors.find(c => c.id === data.connector_id);
                if (conn) this.mcpDetailConnector = conn;
            }
        } else {
            console.error('[MCP] OAuth failed:', data.error);
        }
    },

    // ── Helpers ─────────────────────────────────────────────────────────

    _resetAddForm() {
        this.mcpNewUrl = '';
        this.mcpNewTitle = '';
        this.mcpNewClientID = '';
        this.mcpNewClientSecret = '';
        this.mcpError = '';
    },

    _titleFromUrl(url) {
        try {
            const u = new URL(url);
            return u.hostname;
        } catch {
            return 'MCP Server';
        }
    },

    mcpOpenAddDialog() {
        this._resetAddForm();
        this.$nextTick(() => {
            if (window.tui?.dialog) {
                window.tui.dialog.open('mcp-add-dialog');
            }
        });
    },
});
