/**
 * Agents Manager Component
 * Handles agent CRUD operations, search, selection, and per-agent tool/KB wiring
 * Now includes Agent Store (shared catalog) tab for browsing and adding pre-built agents.
 */
export const agentsManager = () => ({
    // State
    view: 'list', // 'list' | 'form' | 'delete'
    activeTab: 'my-agents', // 'my-agents' | 'store'
    agents: [],
    filteredAgents: [],
    loading: false,
    saving: false,
    searchQuery: '',

    // Currently active agent (synced from localStorage / chat component)
    selectedAgent: localStorage.getItem('selectedAgent') || '',

    // Form state
    editingAgent: null, // null = create mode, object = edit mode
    form: {
        name: '',
        description: '',
        systemPrompt: '',
    },

    // Tools & KB state (for edit mode)
    showTools: false,
    showKnowledgeBases: false,
    agentPlugins: [],
    agentWorkspaces: [],
    loadingTools: false,
    loadingKBs: false,

    // Delete state
    deletingAgent: null,

    // ── Agent Store state ──
    storeEntries: [],
    storeCategories: [],
    storeSearchQuery: '',
    selectedCategory: '',
    storeLoading: false,
    storeAddingIds: [], // catalog IDs currently being added
    addedCatalogIds: [], // catalog IDs that the user has already added
    storeSuccessId: null, // catalog ID that was just successfully added (for inline toast)
    storeTotal: 0,

    async open() {
        this.view = 'list';
        this.searchQuery = '';
        this.storeSuccessId = null;
        // Refresh the active agent from localStorage (set by the chat component)
        this.selectedAgent = localStorage.getItem('selectedAgent') || '';
        await this.fetchAgents();
        // Pre-fetch store data in background
        if (this.storeEntries.length === 0) {
            await this.fetchCatalog();
        }
    },

    // ============================================
    // Data fetching
    // ============================================

    async fetchAgents() {
        this.loading = true;
        try {
            const response = await fetch('/v1/agent-definitions');
            if (response.ok) {
                const data = await response.json();
                this.agents = data.agents || [];
                this.applyFilter();
            }
        } catch (err) {
            console.error('Failed to fetch agents:', err);
        } finally {
            this.loading = false;
        }
    },

    async searchAgents() {
        if (!this.searchQuery.trim()) {
            this.applyFilter();
            return;
        }
        try {
            const response = await fetch(`/v1/agent-definitions/search?q=${encodeURIComponent(this.searchQuery)}`);
            if (response.ok) {
                const data = await response.json();
                this.filteredAgents = data.agents || [];
            }
        } catch (err) {
            console.error('Failed to search agents:', err);
            this.applyFilter();
        }
    },

    applyFilter() {
        let results;
        if (!this.searchQuery.trim()) {
            results = [...this.agents];
        } else {
            const q = this.searchQuery.toLowerCase();
            results = this.agents.filter(a =>
                a.name.toLowerCase().includes(q) ||
                (a.description && a.description.toLowerCase().includes(q))
            );
        }
        // Pin the active agent to the top of the list
        const activeId = this.selectedAgent;
        if (activeId) {
            results.sort((a, b) => {
                if (a.id === activeId) return -1;
                if (b.id === activeId) return 1;
                return 0;
            });
        }
        this.filteredAgents = results;
    },

    // ============================================
    // Form operations
    // ============================================

    showCreateForm() {
        this.editingAgent = null;
        this.form = {
            name: '',
            description: '',
            systemPrompt: '',
        };
        this.showTools = false;
        this.showKnowledgeBases = false;
        this.agentPlugins = [];
        this.agentWorkspaces = [];
        this.view = 'form';
    },

    showEditForm(agent) {
        this.editingAgent = agent;
        this.form = {
            name: agent.name || '',
            description: agent.description || '',
            systemPrompt: agent.systemPrompt || '',
        };
        this.showTools = false;
        this.showKnowledgeBases = false;
        this.agentPlugins = [];
        this.agentWorkspaces = [];
        this.view = 'form';

        // Load tool/KB data for this agent
        this.fetchAgentPlugins(agent.id);
        this.fetchAgentWorkspaces(agent.id);
    },

    async saveAgent() {
        if (!this.form.name.trim()) return;
        this.saving = true;
        try {
            const payload = {
                name: this.form.name.trim(),
                description: this.form.description.trim(),
                systemPrompt: this.form.systemPrompt.trim(),
            };

            // Preserve the existing icon when editing (the backend handles fallback)
            if (this.editingAgent && this.editingAgent.icon) {
                payload.icon = this.editingAgent.icon;
            }

            let response;
            if (this.editingAgent) {
                // Update
                response = await fetch(`/v1/agent-definitions/${this.editingAgent.id}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload),
                });
            } else {
                // Create
                response = await fetch('/v1/agent-definitions', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload),
                });
            }

            if (response.ok) {
                await this.fetchAgents();
                this.view = 'list';
            } else {
                const err = await response.json().catch(() => ({}));
                console.error('Failed to save agent:', err);
                alert(err.error || 'Failed to save agent');
            }
        } catch (err) {
            console.error('Failed to save agent:', err);
            alert('Failed to save agent');
        } finally {
            this.saving = false;
        }
    },

    // ============================================
    // Delete operations
    // ============================================

    confirmDelete(agent) {
        this.deletingAgent = agent;
        this.view = 'delete';
    },

    async deleteAgent() {
        if (!this.deletingAgent) return;
        this.saving = true;
        try {
            const response = await fetch(`/v1/agent-definitions/${this.deletingAgent.id}`, {
                method: 'DELETE',
            });
            if (response.ok) {
                await this.fetchAgents();
                this.view = 'list';
            } else {
                const err = await response.json().catch(() => ({}));
                console.error('Failed to delete agent:', err);
                alert(err.error || 'Failed to delete agent');
            }
        } catch (err) {
            console.error('Failed to delete agent:', err);
            alert('Failed to delete agent');
        } finally {
            this.saving = false;
        }
    },

    // ============================================
    // Agent selection (for chat)
    // ============================================

    selectAgentForChat(agentId) {
        this.selectedAgent = agentId;
        window.dispatchEvent(new CustomEvent('agent-selected', {
            detail: { id: agentId }
        }));
        // Close the dialog using TUI dialog API
        if (window.tui?.dialog?.close) {
            window.tui.dialog.close('agents-manager-dialog');
        }
    },

    // ============================================
    // Agent Tools & Knowledge Bases
    // ============================================

    get toolsSummary() {
        if (this.loadingTools) return '';
        const enabled = this.agentPlugins.filter(p => p.enabled).length;
        return `${enabled}/${this.agentPlugins.length}`;
    },

    get kbSummary() {
        if (this.loadingKBs) return '';
        const enabled = this.agentWorkspaces.filter(w => w.enabled).length;
        return `${enabled}/${this.agentWorkspaces.length}`;
    },

    async fetchAgentPlugins(agentId) {
        this.loadingTools = true;
        try {
            const response = await fetch(`/v1/plugins?agent_id=${encodeURIComponent(agentId)}`);
            if (response.ok) {
                const data = await response.json();
                this.agentPlugins = data.plugins || [];
            }
        } catch (err) {
            console.error('Failed to fetch agent plugins:', err);
        } finally {
            this.loadingTools = false;
        }
    },

    async fetchAgentWorkspaces(agentId) {
        this.loadingKBs = true;
        try {
            const response = await fetch(`/v1/agents/${encodeURIComponent(agentId)}/workspaces`);
            if (response.ok) {
                const data = await response.json();
                this.agentWorkspaces = data.workspaces || [];
            }
        } catch (err) {
            console.error('Failed to fetch agent workspaces:', err);
        } finally {
            this.loadingKBs = false;
        }
    },

    async toggleAgentPlugin(plugin) {
        if (!this.editingAgent) return;
        const newState = !plugin.enabled;
        try {
            const response = await fetch(`/v1/plugins/${plugin.id}?agent_id=${encodeURIComponent(this.editingAgent.id)}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled: newState }),
            });
            if (response.ok) {
                plugin.enabled = newState;
                // When toggling a plugin set, also toggle all its tools
                plugin.tools.forEach(t => t.enabled = newState);
            }
        } catch (err) {
            console.error('Failed to toggle agent plugin:', err);
        }
    },

    async toggleAgentTool(plugin, tool) {
        if (!this.editingAgent) return;
        const newState = !tool.enabled;
        try {
            const response = await fetch(`/v1/plugins/${plugin.id}/tools/${tool.id}?agent_id=${encodeURIComponent(this.editingAgent.id)}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled: newState }),
            });
            if (response.ok) {
                tool.enabled = newState;
                // Update plugin enabled state based on whether any tool is enabled
                plugin.enabled = plugin.tools.some(t => t.enabled);
            }
        } catch (err) {
            console.error('Failed to toggle agent tool:', err);
        }
    },

    async toggleAgentWorkspace(ws) {
        if (!this.editingAgent) return;
        const newState = !ws.enabled;
        try {
            const response = await fetch(`/v1/agents/${encodeURIComponent(this.editingAgent.id)}/workspaces/${ws.id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled: newState }),
            });
            if (response.ok) {
                ws.enabled = newState;
            }
        } catch (err) {
            console.error('Failed to toggle agent workspace:', err);
        }
    },

    // ============================================
    // Agent Store (shared catalog)
    // ============================================

    switchTab(tab) {
        this.activeTab = tab;
        if (tab === 'store' && this.storeEntries.length === 0) {
            this.fetchCatalog();
        }
    },

    async fetchCatalog() {
        this.storeLoading = true;
        try {
            let url = '/v1/agent-catalog?limit=100';
            if (this.selectedCategory) {
                url += `&category=${encodeURIComponent(this.selectedCategory)}`;
            }
            const response = await fetch(url);
            if (response.ok) {
                const data = await response.json();
                this.storeEntries = data.entries || [];
                this.storeTotal = data.total || 0;
                if (data.categories && data.categories.length > 0) {
                    this.storeCategories = data.categories;
                }
                // Server provides the authoritative set of already-added catalog IDs
                this.addedCatalogIds = data.addedCatalogIds || [];
            }
        } catch (err) {
            console.error('Failed to fetch agent catalog:', err);
        } finally {
            this.storeLoading = false;
        }
    },

    async searchCatalog() {
        if (!this.storeSearchQuery.trim()) {
            this.fetchCatalog();
            return;
        }
        this.storeLoading = true;
        try {
            const response = await fetch(`/v1/agent-catalog/search?q=${encodeURIComponent(this.storeSearchQuery)}`);
            if (response.ok) {
                const data = await response.json();
                this.storeEntries = data.entries || [];
                this.storeTotal = data.total || 0;
                // Server provides the authoritative set
                if (data.addedCatalogIds) {
                    this.addedCatalogIds = data.addedCatalogIds;
                }
            }
        } catch (err) {
            console.error('Failed to search agent catalog:', err);
        } finally {
            this.storeLoading = false;
        }
    },

    filterByCategory(category) {
        this.selectedCategory = this.selectedCategory === category ? '' : category;
        this.storeSearchQuery = '';
        this.fetchCatalog();
    },

    async addFromStore(entry) {
        // Prevent adding if already added or currently adding
        if (this.addedCatalogIds.includes(entry.id)) return;
        if (this.storeAddingIds.includes(entry.id)) return;
        this.storeAddingIds.push(entry.id);
        try {
            const response = await fetch(`/v1/agent-catalog/${entry.id}/add`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
            });
            if (response.ok) {
                // Mark as added — stay on the store tab so user can keep browsing
                if (!this.addedCatalogIds.includes(entry.id)) {
                    this.addedCatalogIds.push(entry.id);
                }
                // Refresh agents list in the background
                this.fetchAgents();
                // Show brief inline success toast
                this.storeSuccessId = entry.id;
                setTimeout(() => {
                    if (this.storeSuccessId === entry.id) this.storeSuccessId = null;
                }, 2500);
            } else if (response.status === 409) {
                // Already added (race condition or stale UI) — just mark it
                if (!this.addedCatalogIds.includes(entry.id)) {
                    this.addedCatalogIds.push(entry.id);
                }
            } else {
                const errData = await response.text();
                console.error('Failed to add agent from store:', response.status, errData);
                try {
                    const errJson = JSON.parse(errData);
                    alert(errJson.error || 'Failed to add agent');
                } catch {
                    alert('Failed to add agent');
                }
            }
        } catch (err) {
            console.error('Failed to add agent from store:', err);
            alert('Failed to add agent');
        } finally {
            this.storeAddingIds = this.storeAddingIds.filter(id => id !== entry.id);
        }
    },

    isStoreAdding(id) {
        return this.storeAddingIds.includes(id);
    },

    get filteredStoreEntries() {
        return this.storeEntries;
    },

    // ============================================
    // Utilities
    // ============================================

    formatDate(dateStr) {
        if (!dateStr) return '';
        try {
            const d = new Date(dateStr);
            const now = new Date();
            const diffMs = now - d;
            const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

            if (diffDays === 0) return 'today';
            if (diffDays === 1) return 'yesterday';
            if (diffDays < 7) return `${diffDays}d ago`;
            if (diffDays < 30) return `${Math.floor(diffDays / 7)}w ago`;
            return d.toLocaleDateString();
        } catch {
            return '';
        }
    },
});
