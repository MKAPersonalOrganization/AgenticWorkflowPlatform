/**
 * MCP Connectors Component
 * Manages user-registered MCP (Model Context Protocol) plugin connectors.
 * Handles connection lifecycle, OAuth authorization, and tool discovery.
 */

// Module-level variable — MUST NOT live inside Alpine's reactive proxy.
// window.open() returns a cross-origin Window reference once the popup
// navigates to the external OAuth provider. Alpine's reactivity system
// (backed by @vue/reactivity Proxy) tries to check __v_raw on every
// proxied property, which throws "Permission denied to access property
// '__v_raw' on cross-origin object" on cross-origin Window objects.
let _oauthPopup = null;

export const mcpConnectors = () => ({
    mcpConnectors: [],
    mcpLoading: false,
    mcpError: '',

    // "Add MCP Server" dialog state
    mcpNewUrl: '',
    mcpNewTitle: '',
    mcpNewClientID: '',
    mcpNewClientSecret: '',

    init() {
        this.mcpFetchConnectors();

        // Listen for OAuth callback messages from popup.
        // Guard with origin check to ignore cross-origin messages (browser
        // extensions, ads, etc.) that would trigger Alpine's proxy to
        // introspect the cross-origin MessageEvent.source.
        window.addEventListener('message', (event) => {
            if (event.origin !== window.location.origin) return;

            // Shallow-copy event.data into a plain object before Alpine's
            // reactive proxy can touch it. This detaches from the
            // MessageEvent and its cross-origin .source property.
            const data = Object.assign({}, event.data);
            if (data.type === 'mcp-oauth-callback') {
                this._handleOAuthMessage(data);
            }
        });

        // Listen for connect/disconnect requests from the detail view (parent scope)
        window.addEventListener('mcp-action-connect', (e) => {
            this.mcpConnect(e.detail);
        });
        window.addEventListener('mcp-action-disconnect', (e) => {
            this.mcpDisconnect(e.detail);
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
                // Notify parent to close detail view if showing this connector
                window.dispatchEvent(new CustomEvent('mcp-connector-removed', { detail: connectorId }));
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

    mcpOpenDetail(connector) {
        window.dispatchEvent(new CustomEvent('show-mcp-detail', { detail: connector }));
    },

    mcpCloseDetail() {
        // No-op — detail view is managed by the parent toolsDialog
    },

    // ── OAuth popup ─────────────────────────────────────────────────────

    _openOAuthPopup(authUrl) {
        const w = 600, h = 700;
        const left = window.screenX + (window.outerWidth - w) / 2;
        const top = window.screenY + (window.outerHeight - h) / 2;

        _oauthPopup = window.open(
            authUrl,
            'mcp-oauth',
            `width=${w},height=${h},left=${left},top=${top},popup=1`
        );
    },

    async _handleOAuthMessage(data) {
        if (_oauthPopup) {
            _oauthPopup.close();
            _oauthPopup = null;
        }

        if (data.success) {
            // OAuth succeeded — refresh connectors to pick up new status
            await this.mcpFetchConnectors();
            window.dispatchEvent(new CustomEvent('mcp-state-changed'));
            // Notify parent to update MCP detail view if showing this connector
            const conn = this.mcpConnectors.find(c => c.id === data.connector_id);
            if (conn) {
                window.dispatchEvent(new CustomEvent('mcp-detail-updated', { detail: conn }));
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
