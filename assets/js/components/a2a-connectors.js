/**
 * A2A Connectors Component
 * Manages user-registered A2A (Agent-to-Agent) protocol connectors.
 * Handles connection lifecycle, agent card discovery, and skill resolution.
 */
export const a2aConnectors = () => ({
    connectors: [],
    loading: false,
    search: '',
    _loaded: false,

    // Detail view state
    detailItem: null,
    detailSkills: [],

    // "Add A2A Agent" dialog state
    newUrl: '',
    newTitle: '',
    newAuthType: 'none',
    newAuthToken: '',
    newAuthHeader: '',
    addLoading: false,
    error: '',

    init() {
        // Listen for connect/disconnect requests from the detail view
        window.addEventListener('a2a-action-connect', (e) => {
            this.connectAgent(e.detail);
        });
        window.addEventListener('a2a-action-disconnect', (e) => {
            this.disconnect(e.detail);
        });

        // Close detail if the connector was removed
        window.addEventListener('a2a-connector-removed', (e) => {
            if (this.detailItem?.id === e.detail) {
                this.closeDetail();
            }
        });

        // Update detail connector data when state changes
        window.addEventListener('a2a-detail-updated', (e) => {
            if (this.detailItem?.id === e.detail?.id) {
                Object.assign(this.detailItem, e.detail);
            }
        });
    },

    /** Called when the dialog opens */
    async open() {
        if (!this._loaded) {
            await this.fetchConnectors();
            this._loaded = true;
        }
    },

    // ── Computed ─────────────────────────────────────────────────────────

    get filteredConnectors() {
        if (!this.search) return this.connectors;
        const q = this.search.toLowerCase();
        return this.connectors.filter(c =>
            (c.title || '').toLowerCase().includes(q) ||
            (c.agent_name || '').toLowerCase().includes(q) ||
            (c.agent_description || '').toLowerCase().includes(q) ||
            (c.agent_url || '').toLowerCase().includes(q)
        );
    },

    get totalSkills() {
        return this.connectors.reduce((sum, c) => sum + (c.skill_count || 0), 0);
    },

    // ── CRUD ────────────────────────────────────────────────────────────

    async fetchConnectors() {
        this.loading = true;
        try {
            const res = await fetch('/v1/a2a/connectors');
            if (res.ok) {
                const data = await res.json();
                this.connectors = data.connectors || [];
            }
        } catch (err) {
            console.error('[A2A] Failed to fetch connectors:', err);
        } finally {
            this.loading = false;
        }
    },

    async addConnector() {
        if (!this.newUrl) return;

        this.addLoading = true;
        this.error = '';

        try {
            // 1. Create the connector record
            const body = {
                agent_url: this.newUrl,
                title: this.newTitle || this._titleFromUrl(this.newUrl),
            };

            // Include auth if set
            if (this.newAuthType !== 'none') {
                body.auth_type = this.newAuthType;
                body.auth_token = this.newAuthToken;
                if (this.newAuthHeader) {
                    body.auth_header = this.newAuthHeader;
                }
            }

            const createRes = await fetch('/v1/a2a/connectors', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body),
            });

            if (!createRes.ok) {
                const errBody = await createRes.json().catch(() => ({}));
                throw new Error(errBody.error || `Server returned ${createRes.status}`);
            }

            const connector = await createRes.json();

            // 2. Attempt to connect (discover agent card + skills)
            const connectRes = await fetch(`/v1/a2a/connectors/${connector.id}/connect`, {
                method: 'POST',
            });

            if (!connectRes.ok) {
                const errBody = await connectRes.json().catch(() => ({}));
                console.warn('[A2A] Connect failed after create:', errBody.error);
            }

            // 3. Refresh list & close dialog
            await this.fetchConnectors();
            window.dispatchEvent(new CustomEvent('a2a-state-changed'));
            this._resetAddForm();
            if (window.tui?.dialog) {
                window.tui.dialog.close('a2a-add-dialog');
            }
        } catch (err) {
            console.error('[A2A] Failed to add connector:', err);
            this.error = err.message || 'Failed to add A2A agent';
        } finally {
            this.addLoading = false;
        }
    },

    async remove(connectorId) {
        try {
            const res = await fetch(`/v1/a2a/connectors/${connectorId}`, {
                method: 'DELETE',
            });
            if (res.ok) {
                this.connectors = this.connectors.filter(c => c.id !== connectorId);
                window.dispatchEvent(new CustomEvent('a2a-connector-removed', { detail: connectorId }));
                window.dispatchEvent(new CustomEvent('a2a-state-changed'));
            }
        } catch (err) {
            console.error('[A2A] Failed to remove connector:', err);
        }
    },

    // ── Connection lifecycle ────────────────────────────────────────────

    async connectAgent(connectorId) {
        const conn = this.connectors.find(c => c.id === connectorId);
        if (conn) {
            conn.status = 'discovering';
        }

        try {
            const res = await fetch(`/v1/a2a/connectors/${connectorId}/connect`, {
                method: 'POST',
            });
            if (!res.ok) {
                const errBody = await res.json().catch(() => ({}));
                console.error('[A2A] Connect failed:', errBody.error);
            }
            await this.fetchConnectors();
            window.dispatchEvent(new CustomEvent('a2a-state-changed'));
            // Sync detail if viewing this connector
            if (this.detailItem?.id === connectorId) {
                const updated = this.connectors.find(c => c.id === connectorId);
                if (updated) {
                    Object.assign(this.detailItem, updated);
                    this.detailSkills = await this._fetchSkills(connectorId);
                }
            }
        } catch (err) {
            console.error('[A2A] Failed to connect:', err);
            if (conn) conn.status = 'error';
        }
    },

    async disconnect(connectorId) {
        const conn = this.connectors.find(c => c.id === connectorId);
        if (conn) {
            conn.status = 'disconnected';
        }

        try {
            await fetch(`/v1/a2a/connectors/${connectorId}/disconnect`, {
                method: 'POST',
            });
            await this.fetchConnectors();
            window.dispatchEvent(new CustomEvent('a2a-state-changed'));
            // Sync detail
            if (this.detailItem?.id === connectorId) {
                const updated = this.connectors.find(c => c.id === connectorId);
                if (updated) Object.assign(this.detailItem, updated);
                this.detailSkills = [];
            }
        } catch (err) {
            console.error('[A2A] Failed to disconnect:', err);
        }
    },

    // ── Detail view ─────────────────────────────────────────────────────

    async openDetail(connector) {
        this.detailItem = { ...connector };
        this.detailSkills = [];
        // Fetch skills for this connector
        this.detailSkills = await this._fetchSkills(connector.id);
    },

    closeDetail() {
        this.detailItem = null;
        this.detailSkills = [];
    },

    // ── Add dialog ──────────────────────────────────────────────────────

    openAddDialog() {
        this._resetAddForm();
        this.$nextTick(() => {
            if (window.tui?.dialog) {
                window.tui.dialog.open('a2a-add-dialog');
            }
        });
    },

    // ── Helpers ─────────────────────────────────────────────────────────

    async _fetchSkills(connectorId) {
        try {
            const res = await fetch(`/v1/a2a/connectors/${connectorId}/skills`);
            if (res.ok) {
                const data = await res.json();
                return data.skills || [];
            }
        } catch (err) {
            console.error('[A2A] Failed to fetch skills:', err);
        }
        return [];
    },

    _resetAddForm() {
        this.newUrl = '';
        this.newTitle = '';
        this.newAuthType = 'none';
        this.newAuthToken = '';
        this.newAuthHeader = '';
        this.error = '';
    },

    _titleFromUrl(url) {
        try {
            const u = new URL(url);
            return u.hostname;
        } catch {
            return 'A2A Agent';
        }
    },
});
