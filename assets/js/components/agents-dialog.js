/**
 * Agents Manager Component
 * Unified agent management — both simple and composed (multi-agent) agents
 * use the same CRUD API. An agent with a composition graph is a "composed agent"
 * that orchestrates sub-agents via sequential, parallel, or loop patterns.
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

    /**
     * Whether the currently edited agent is a root (no parent workflow).
     * Gates the Global Instruction editor — ADK only honours the root's
     * GlobalInstruction at runtime (see .READ/adk-go/agent/llmagent/
     * llmagent.go — "ONLY the GlobalInstruction in the root agent will
     * take effect"), so we hide it on sub-agents to avoid ignored input.
     * New agents (no editingAgent) are treated as root — they start as
     * top-level entries in the user's library.
     */
    isRootNode() {
        if (!this.editingAgent) return true;
        return !this.editingAgent.parentId;
    },

    // Unified form state — a single form for both simple and composed agents
    editingAgent: null, // null = create mode, object = edit mode
    form: {
        name: '',
        description: '',
        systemPrompt: '',
        graph: null,
        // Multi-agent orchestration fields
        disallowTransferToParent: false,
        disallowTransferToPeers: false,
        outputKey: '',
        includeContents: '',
        globalInstruction: '',
        outputSchema: '',
    },

    // UI state
    showOrchestration: false,
    showComposition: false,
    expanded: false,

    // Tools & KB state (for edit mode)
    showTools: false,
    showKnowledgeBases: false,
    showFilesystems: false,
    agentPlugins: [],
    agentWorkspaces: [],
    agentFilesystems: [],
    loadingTools: false,
    loadingKBs: false,
    loadingFilesystems: false,

    // Delete state
    deletingAgent: null,

    // ── Agent Templates state ──
    storeEntries: [],
    storeCategories: [],
    storeSearchQuery: '',
    selectedCategory: '',
    storeLoading: false,
    storeAddingIds: [],
    addedCatalogIds: [],
    storeSuccessId: null,
    storeTotal: 0,

    // ── Composition tree state ──
    _nodeCounter: 0,

    async open() {
        this.view = 'list';
        this.searchQuery = '';
        this.storeSuccessId = null;
        this.selectedAgent = localStorage.getItem('selectedAgent') || '';
        await this.fetchAgents();
        if (this.storeEntries.length === 0) {
            await this.fetchCatalog();
        }
    },

    toggleExpand() {
        this.expanded = !this.expanded;
    },

    // ============================================
    // Data fetching — single unified API
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
        let results = [...this.agents];
        if (this.searchQuery.trim()) {
            const q = this.searchQuery.toLowerCase();
            results = results.filter(a =>
                a.name.toLowerCase().includes(q) ||
                (a.description && a.description.toLowerCase().includes(q))
            );
        }
        // Pin the active agent to the top
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
    // Unified form operations
    // ============================================

    showCreateForm() {
        // Redirect to the Agent Designer dialog (node design mode)
        if (window.tui?.dialog?.close) {
            window.tui.dialog.close('agents-manager-dialog');
        }
        setTimeout(() => {
            window.dispatchEvent(new CustomEvent('open-agent-designer'));
            if (window.tui?.dialog?.open) {
                window.tui.dialog.open('agent-designer-dialog');
            }
        }, 150);
    },

    showEditForm(agent) {
        // Redirect to the Agent Designer dialog with agent data
        if (window.tui?.dialog?.close) {
            window.tui.dialog.close('agents-manager-dialog');
        }
        setTimeout(() => {
            const eventName = agent.isComposed ? 'open-agent-designer-pipeline' : 'open-agent-designer-node';
            window.dispatchEvent(new CustomEvent(eventName, { detail: agent }));
            if (window.tui?.dialog?.open) {
                window.tui.dialog.open('agent-designer-dialog');
            }
        }, 150);
    },

    /** Enable composition mode — transforms a simple agent into a composed one. */
    enableComposition() {
        this._nodeCounter = 1;
        this.form.graph = {
            id: 'node-1',
            type: 'sequential',
            label: '',
            children: [this.createNode('agent')],
        };
        this.showComposition = true;
    },

    /** Disable composition mode — removes the graph. */
    disableComposition() {
        this.form.graph = null;
        this.showComposition = false;
    },

    async saveAgent() {
        if (!this.form.name.trim()) return;
        this.saving = true;
        try {
            const payload = {
                name: this.form.name.trim(),
                description: this.form.description.trim(),
                systemPrompt: this.form.systemPrompt.trim(),
                graph: this.form.graph || undefined,
                disallowTransferToParent: this.form.disallowTransferToParent || false,
                disallowTransferToPeers: this.form.disallowTransferToPeers || false,
                outputKey: this.form.outputKey.trim(),
                includeContents: this.form.includeContents || '',
                globalInstruction: this.form.globalInstruction.trim(),
                outputSchema: this.form.outputSchema.trim(),
            };

            // Preserve existing icon when editing
            if (this.editingAgent && this.editingAgent.icon) {
                payload.icon = this.editingAgent.icon;
            }

            let response;
            if (this.editingAgent) {
                response = await fetch(`/v1/agent-definitions/${this.editingAgent.id}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload),
                });
            } else {
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
        if (window.tui?.dialog?.close) {
            window.tui.dialog.close('agents-manager-dialog');
        }
    },

    // ============================================
    // Sharing
    // ============================================

    async shareAgent(agentId) {
        try {
            const response = await fetch(`/v1/agent-definitions/${agentId}/share`, {
                method: 'POST',
            });
            if (response.ok) {
                const data = await response.json();
                const shareUrl = window.location.origin + data.shareUrl;
                await navigator.clipboard.writeText(shareUrl);
                // Update the agent in the list
                const ag = this.agents.find(a => a.id === agentId);
                if (ag) {
                    ag.shareToken = data.shareToken;
                    ag.visibility = 'shared';
                }
                alert('Share link copied to clipboard!');
            } else {
                alert('Failed to share agent');
            }
        } catch (err) {
            console.error('Failed to share agent:', err);
        }
    },

    async unshareAgent(agentId) {
        try {
            const response = await fetch(`/v1/agent-definitions/${agentId}/share`, {
                method: 'DELETE',
            });
            if (response.ok) {
                const ag = this.agents.find(a => a.id === agentId);
                if (ag) {
                    ag.shareToken = '';
                    ag.visibility = 'private';
                }
            }
        } catch (err) {
            console.error('Failed to unshare agent:', err);
        }
    },

    // ============================================
    // Agent Tools & Knowledge Bases
    // ============================================

    get toolsSummary() {
        if (this.loadingTools) return '';
        const blocked = this.agentPlugins.filter(p => p.requirement_config?.action === 'block').length;
        const active = this.agentPlugins.length - blocked;
        return `${active}/${this.agentPlugins.length}`;
    },

    get kbSummary() {
        if (this.loadingKBs) return '';
        const enabled = this.agentWorkspaces.filter(w => w.enabled).length;
        return `${enabled}/${this.agentWorkspaces.length}`;
    },

    get fsSummary() {
        if (this.loadingFilesystems) return '';
        const enabled = this.agentFilesystems.filter(fs => fs.enabled).length;
        return `${enabled}/${this.agentFilesystems.length}`;
    },

    get orchestrationSummary() {
        const flags = [];
        if (this.form.disallowTransferToParent) flags.push('no↑parent');
        if (this.form.disallowTransferToPeers) flags.push('no↔peers');
        if (this.form.outputKey) flags.push('key:' + this.form.outputKey);
        if (this.form.includeContents === 'none') flags.push('fresh ctx');
        if (this.form.globalInstruction) flags.push('global');
        if (this.form.outputSchema) flags.push('schema');
        return flags.length ? flags.join(' · ') : '';
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

    async fetchAgentFilesystems(agentId) {
        this.loadingFilesystems = true;
        try {
            const response = await fetch(`/v1/agents/${encodeURIComponent(agentId)}/filesystems`);
            if (response.ok) {
                const data = await response.json();
                this.agentFilesystems = data || [];
            }
        } catch (err) {
            console.error('Failed to fetch agent filesystems:', err);
        } finally {
            this.loadingFilesystems = false;
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
            if (response.ok) { ws.enabled = newState; }
        } catch (err) { console.error('Failed to toggle agent workspace:', err); }
    },

    async toggleAgentFilesystem(fs) {
        if (!this.editingAgent) return;
        const newState = !fs.enabled;
        try {
            const response = await fetch(`/v1/agents/${encodeURIComponent(this.editingAgent.id)}/filesystems/${fs.id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled: newState }),
            });
            if (response.ok) { fs.enabled = newState; }
        } catch (err) { console.error('Failed to toggle agent filesystem:', err); }
    },

    // ============================================
    // Agent Templates (shared catalog)
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
        if (this.addedCatalogIds.includes(entry.id)) return;
        if (this.storeAddingIds.includes(entry.id)) return;
        this.storeAddingIds.push(entry.id);
        try {
            const response = await fetch(`/v1/agent-catalog/${entry.id}/add`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
            });
            if (response.ok) {
                if (!this.addedCatalogIds.includes(entry.id)) {
                    this.addedCatalogIds.push(entry.id);
                }
                this.fetchAgents();
                window.dispatchEvent(new CustomEvent('agents-updated'));
                this.storeSuccessId = entry.id;
                setTimeout(() => {
                    if (this.storeSuccessId === entry.id) this.storeSuccessId = null;
                }, 2500);
            } else if (response.status === 409) {
                if (!this.addedCatalogIds.includes(entry.id)) {
                    this.addedCatalogIds.push(entry.id);
                }
            } else {
                const errData = await response.text();
                console.error('Failed to add agent from store:', response.status, errData);
                try {
                    const errJson = JSON.parse(errData);
                    alert(errJson.error || 'Failed to add agent');
                } catch { alert('Failed to add agent'); }
            }
        } catch (err) {
            console.error('Failed to add agent from store:', err);
            alert('Failed to add agent');
        } finally {
            this.storeAddingIds = this.storeAddingIds.filter(id => id !== entry.id);
        }
    },

    // ============================================
    // Composition tree helpers
    // ============================================

    _nextNodeId() {
        this._nodeCounter++;
        return `node-${this._nodeCounter}`;
    },

    createNode(type) {
        const id = this._nextNodeId();
        if (type === 'agent') {
            return {
                id, type: 'agent', label: '', agent_id: '', output_key: '',
                disallow_transfer_to_parent: null,
                disallow_transfer_to_peers: null,
                include_contents: '',
            };
        }
        const node = { id, type, label: '', children: [] };
        if (type === 'loop') { node.max_iterations = 3; }
        return node;
    },

    countNodes(node) {
        if (!node) return 0;
        let count = 1;
        if (node.children) {
            for (const child of node.children) { count += this.countNodes(child); }
        }
        return count;
    },

    nodeDepth(node) {
        if (!node || !node.children || node.children.length === 0) return 1;
        let maxD = 0;
        for (const child of node.children) {
            const d = this.nodeDepth(child);
            if (d > maxD) maxD = d;
        }
        return 1 + maxD;
    },

    canAddNode() {
        return this.form.graph && this.countNodes(this.form.graph) < 50;
    },

    addChildToNode(parentPath, childType) {
        const parent = this._getNodeByPath(parentPath);
        if (!parent || !parent.children) return;
        if (!this.canAddNode()) return;
        parent.children.push(this.createNode(childType));
    },

    removeChildFromNode(parentPath, index) {
        const parent = this._getNodeByPath(parentPath);
        if (!parent || !parent.children) return;
        if (parent.children.length <= 1) return;
        parent.children.splice(index, 1);
    },

    changeNodeType(path, newType) {
        const node = this._getNodeByPath(path);
        if (!node) return;
        node.type = newType;
        if (newType === 'agent') {
            delete node.children;
            delete node.max_iterations;
            node.agent_id = '';
            node.output_key = '';
            node.disallow_transfer_to_parent = null;
            node.disallow_transfer_to_peers = null;
            node.include_contents = '';
        } else {
            delete node.agent_id;
            delete node.output_key;
            delete node.disallow_transfer_to_parent;
            delete node.disallow_transfer_to_peers;
            delete node.include_contents;
            if (!node.children || node.children.length === 0) {
                node.children = [this.createNode('agent')];
            }
            if (newType === 'loop' && !node.max_iterations) { node.max_iterations = 3; }
            if (newType !== 'loop') { delete node.max_iterations; }
        }
    },

    _getNodeByPath(path) {
        const parts = path.split('.');
        if (parts[0] !== 'graph') return null;
        let node = this.form.graph;
        for (let i = 1; i < parts.length; i++) {
            const idx = parseInt(parts[i], 10);
            if (!node.children || idx >= node.children.length) return null;
            node = node.children[idx];
        }
        return node;
    },

    nodeTypeLabel(type) {
        const labels = { sequential: 'Sequential', parallel: 'Parallel', loop: 'Loop', agent: 'Agent' };
        return labels[type] || type;
    },

    nodeTypeBadgeClass(type) {
        const classes = {
            sequential: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20',
            parallel: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20',
            loop: 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20',
            agent: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20',
        };
        return classes[type] || '';
    },

    collectAgentIds(node) {
        const ids = [];
        const walk = (n) => {
            if (n.type === 'agent' && n.agent_id) ids.push(n.agent_id);
            if (n.children) n.children.forEach(walk);
        };
        if (node) walk(node);
        return ids;
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
