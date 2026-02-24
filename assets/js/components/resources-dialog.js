/**
 * Resources Dialog Component
 * Handles knowledge workspace attachment logic
 */
export const resourcesDialog = () => ({
    workspaces: [],
    agents: [],
    loading: false,
    loadingAgents: false,
    search: '',
    selectedAgent: '',

    async init() {
        const chatEl = document.querySelector('[x-data="chat"]');
        if (chatEl) {
            try {
                const chatData = Alpine.$data(chatEl);
                if (chatData) {
                    this.selectedAgent = chatData.selectedAgent || '';
                }
            } catch (e) {
                /* ignore */
}
        }
        await this.fetchAgents();
        await this.fetchWorkspaces();
    },

    async fetchAgents() {
        this.loadingAgents = true;
        try {
            const response = await fetch('/v1/agents');
            if (response.ok) {
                const data = await response.json();
                if (data.agents && Array.isArray(data.agents)) {
                    this.agents = data.agents;
                    if (!this.selectedAgent && this.agents.length > 0) {
                        this.selectedAgent = this.agents[0].id;
                    }
                }
            }
        } catch (err) {
            console.error('Failed to fetch agents:', err);
        } finally {
            this.loadingAgents = false;
        }
    },

    async fetchWorkspaces() {
        if (!this.selectedAgent) {
            this.workspaces = [];
            return;
        }
        this.loading = true;
        try {
            const response = await fetch('/v1/agents/' + this.selectedAgent + '/workspaces');
            if (response.ok) {
                const data = await response.json();
                if (data.workspaces && Array.isArray(data.workspaces)) {
                    this.workspaces = data.workspaces;
                }
            } else {
                this.workspaces = [];
            }
        } catch (err) {
            console.error('Failed to fetch workspaces:', err);
            this.workspaces = [];
        } finally {
            this.loading = false;
        }
    },

    async onAgentChange() {
        this.workspaces = [];
        await this.fetchWorkspaces();
    },

    get filteredWorkspaces() {
        let filtered = this.workspaces;
        if (this.search) {
            const q = this.search.toLowerCase();
            filtered = filtered.filter(w =>
                w.name.toLowerCase().includes(q) ||
                (w.description && w.description.toLowerCase().includes(q)) ||
                w.slug.toLowerCase().includes(q)
            );
        }
        return filtered;
    },

    get enabledSummary() {
        const enabled = this.workspaces.filter(w => w.enabled).length;
        return {
            enabled,
            total: this.workspaces.length
        };
    },

    async toggleWorkspace(workspace) {
        const newState = !workspace.enabled;
        try {
            const response = await fetch('/v1/agents/' + this.selectedAgent + '/workspaces/' + workspace.id, {
                method: 'PUT',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify({
                    enabled: newState
                })
            });
            if (response.ok) {
                workspace.enabled = newState;
            }
        } catch (err) {
            console.error('Failed to toggle workspace:', err);
        }
    }
});
