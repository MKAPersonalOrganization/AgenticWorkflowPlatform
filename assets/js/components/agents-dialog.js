/**
 * Agents Manager Component
 * Handles agent CRUD operations, search, selection, and per-agent tool/KB wiring
 * Now includes Agent Store (shared catalog) tab for browsing and adding pre-built agents.
 */
export const agentsManager = () => ({
    // State
    view: 'list', // 'list' | 'form' | 'delete'
    activeTab: 'my-agents', // 'my-agents' | 'store'
    formMode: 'agent', // 'agent' | 'workflow' — toggle inside the create/edit form
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
        // Multi-agent orchestration fields
        disallowTransferToParent: false,
        disallowTransferToPeers: false,
        outputKey: '',
        includeContents: '',
        globalInstruction: '',
        outputSchema: '',
    },

    // Orchestration section
    showOrchestration: false,

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

    // ── Workflow state ──
    workflows: [],
    filteredWorkflows: [],
    workflowsLoading: false,
    workflowSearchQuery: '',
    editingWorkflow: null, // null = create mode, object = edit mode
    deletingWorkflow: null,
    _nodeCounter: 0, // auto-increment for unique node IDs
    workflowForm: {
        name: '',
        description: '',
        graph: { id: 'node-1', type: 'sequential', label: '', children: [] },
    },

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
        // Fetch workflows to merge into agent list
        await this.fetchWorkflows();
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
                this.mergeAndFilter();
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
            this.mergeAndFilter();
        }
    },

    /** Merge single agents + workflows into one unified list, then apply the search filter. */
    mergeAndFilter() {
        // Build a combined list: regular agents + workflow entries (marked with _isWorkflow)
        const agentList = [...this.agents];
        const workflowItems = this.workflows.map(wf => ({
            ...wf,
            _isWorkflow: true,
            createdAt: wf.created_at || wf.createdAt,
        }));
        const combined = [...agentList, ...workflowItems];

        let results;
        if (!this.searchQuery.trim()) {
            results = combined;
        } else {
            const q = this.searchQuery.toLowerCase();
            results = combined.filter(a =>
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

    applyFilter() {
        this.mergeAndFilter();
    },

    // ============================================
    // Form operations
    // ============================================

    showCreateForm() {
        this.editingAgent = null;
        this.editingWorkflow = null;
        this.formMode = 'agent';
        this.form = {
            name: '',
            description: '',
            systemPrompt: '',
            disallowTransferToParent: false,
            disallowTransferToPeers: false,
            outputKey: '',
            includeContents: '',
            globalInstruction: '',
            outputSchema: '',
        };
        this._nodeCounter = 1;
        this.workflowForm = {
            name: '',
            description: '',
            graph: {
                id: 'node-1',
                type: 'sequential',
                label: '',
                children: [this.createNode('agent')],
            },
        };
        this.showTools = false;
        this.showKnowledgeBases = false;
        this.showFilesystems = false;
        this.showOrchestration = false;
        this.agentPlugins = [];
        this.agentWorkspaces = [];
        this.agentFilesystems = [];
        this.view = 'form';
    },

    showEditForm(agent) {
        this.editingAgent = agent;
        this.editingWorkflow = null;
        this.formMode = 'agent';
        this.form = {
            name: agent.name || '',
            description: agent.description || '',
            systemPrompt: agent.systemPrompt || '',
            disallowTransferToParent: agent.disallowTransferToParent || false,
            disallowTransferToPeers: agent.disallowTransferToPeers || false,
            outputKey: agent.outputKey || '',
            includeContents: agent.includeContents || '',
            globalInstruction: agent.globalInstruction || '',
            outputSchema: agent.outputSchema || '',
        };
        this.showTools = false;
        this.showKnowledgeBases = false;
        this.showFilesystems = false;
        this.showOrchestration = false;
        this.agentPlugins = [];
        this.agentWorkspaces = [];
        this.view = 'form';

        // Load tool/KB/Filesystem data for this agent
        this.fetchAgentPlugins(agent.id);
        this.fetchAgentWorkspaces(agent.id);
        this.fetchAgentFilesystems(agent.id);
    },

    async saveAgent() {
        if (!this.form.name.trim()) return;
        this.saving = true;
        try {
            const payload = {
                name: this.form.name.trim(),
                description: this.form.description.trim(),
                systemPrompt: this.form.systemPrompt.trim(),
                disallowTransferToParent: this.form.disallowTransferToParent || false,
                disallowTransferToPeers: this.form.disallowTransferToPeers || false,
                outputKey: this.form.outputKey.trim(),
                includeContents: this.form.includeContents || '',
                globalInstruction: this.form.globalInstruction.trim(),
                outputSchema: this.form.outputSchema.trim(),
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
        this.deletingWorkflow = null;
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

    async toggleAgentFilesystem(fs) {
        if (!this.editingAgent) return;
        const newState = !fs.enabled;
        try {
            const response = await fetch(`/v1/agents/${encodeURIComponent(this.editingAgent.id)}/filesystems/${fs.id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled: newState }),
            });
            if (response.ok) {
                fs.enabled = newState;
            }
        } catch (err) {
            console.error('Failed to toggle agent filesystem:', err);
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
    // Workflows (multi-agent composition)
    // ============================================

    async fetchWorkflows() {
        this.workflowsLoading = true;
        try {
            const response = await fetch('/v1/workflows');
            if (response.ok) {
                const data = await response.json();
                this.workflows = data.workflows || [];
                this.filterWorkflows();
                this.mergeAndFilter(); // re-merge into the unified agent list
            }
        } catch (err) {
            console.error('Failed to fetch workflows:', err);
        } finally {
            this.workflowsLoading = false;
        }
    },

    filterWorkflows() {
        if (!this.workflowSearchQuery.trim()) {
            this.filteredWorkflows = [...this.workflows];
        } else {
            const q = this.workflowSearchQuery.toLowerCase();
            this.filteredWorkflows = this.workflows.filter(wf =>
                wf.name.toLowerCase().includes(q) ||
                (wf.description && wf.description.toLowerCase().includes(q))
            );
        }
    },

    // ── Node ID generator ──
    _nextNodeId() {
        this._nodeCounter++;
        return `node-${this._nodeCounter}`;
    },

    // ── Tree helpers ──

    /** Create a new node of the given type. */
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
        if (type === 'loop') {
            node.max_iterations = 3;
        }
        return node;
    },

    /** Count total nodes in a subtree. */
    countNodes(node) {
        if (!node) return 0;
        let count = 1;
        if (node.children) {
            for (const child of node.children) {
                count += this.countNodes(child);
            }
        }
        return count;
    },

    /** Get max depth of a subtree. */
    nodeDepth(node) {
        if (!node || !node.children || node.children.length === 0) return 1;
        let maxD = 0;
        for (const child of node.children) {
            const d = this.nodeDepth(child);
            if (d > maxD) maxD = d;
        }
        return 1 + maxD;
    },

    /** Check if we can add more nodes (max 50 total). */
    canAddNode() {
        return this.countNodes(this.workflowForm.graph) < 50;
    },

    /** Add a child to a composite node by path. */
    addChildToNode(parentPath, childType) {
        const parent = this._getNodeByPath(parentPath);
        if (!parent || !parent.children) return;
        if (!this.canAddNode()) return;
        parent.children.push(this.createNode(childType));
    },

    /** Remove a child from a composite node by path and index. */
    removeChildFromNode(parentPath, index) {
        const parent = this._getNodeByPath(parentPath);
        if (!parent || !parent.children) return;
        if (parent.children.length <= 1) return; // keep at least one
        parent.children.splice(index, 1);
    },

    /** Change a node's type (resets children/agent fields). */
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
            if (newType === 'loop' && !node.max_iterations) {
                node.max_iterations = 3;
            }
            if (newType !== 'loop') {
                delete node.max_iterations;
            }
        }
    },

    /** Resolve a node by dot-separated path (e.g., "graph", "graph.0", "graph.0.2"). */
    _getNodeByPath(path) {
        const parts = path.split('.');
        if (parts[0] !== 'graph') return null;
        let node = this.workflowForm.graph;
        for (let i = 1; i < parts.length; i++) {
            const idx = parseInt(parts[i], 10);
            if (!node.children || idx >= node.children.length) return null;
            node = node.children[idx];
        }
        return node;
    },

    /** Get the type label for display. */
    nodeTypeLabel(type) {
        const labels = { sequential: 'Sequential', parallel: 'Parallel', loop: 'Loop', agent: 'Agent' };
        return labels[type] || type;
    },

    /** Get CSS classes for node type badge. */
    nodeTypeBadgeClass(type) {
        const classes = {
            sequential: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20',
            parallel: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20',
            loop: 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20',
            agent: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20',
        };
        return classes[type] || '';
    },

    /** Collect all agent IDs from a node tree (for summary). */
    collectAgentIds(node) {
        const ids = [];
        const walk = (n) => {
            if (n.type === 'agent' && n.agent_id) ids.push(n.agent_id);
            if (n.children) n.children.forEach(walk);
        };
        walk(node);
        return ids;
    },

    showCreateWorkflowForm() {
        this.editingAgent = null;
        this.editingWorkflow = null;
        this.formMode = 'workflow';
        this._nodeCounter = 1;
        this.workflowForm = {
            name: '',
            description: '',
            graph: {
                id: 'node-1',
                type: 'sequential',
                label: '',
                children: [this.createNode('agent')],
            },
        };
        this.form = {
            name: '',
            description: '',
            systemPrompt: '',
            disallowTransferToParent: false,
            disallowTransferToPeers: false,
            outputKey: '',
            includeContents: '',
            globalInstruction: '',
            outputSchema: '',
        };
        this.view = 'form';
    },

    showEditWorkflowForm(wf) {
        this.editingWorkflow = wf;
        this.editingAgent = wf; // treat as editing so toggle is hidden
        this.formMode = 'workflow';
        // Find the max node counter to avoid ID collisions.
        let maxId = 0;
        const walk = (n) => {
            if (n.id) {
                const num = parseInt(n.id.replace('node-', ''), 10);
                if (num > maxId) maxId = num;
            }
            if (n.children) n.children.forEach(walk);
        };
        if (wf.graph) walk(wf.graph);
        this._nodeCounter = maxId;

        this.workflowForm = {
            name: wf.name || '',
            description: wf.description || '',
            graph: wf.graph ? JSON.parse(JSON.stringify(wf.graph)) : {
                id: this._nextNodeId(),
                type: 'sequential',
                label: '',
                children: [this.createNode('agent')],
            },
        };
        this.view = 'form';
    },

    async saveWorkflow() {
        if (!this.workflowForm.name.trim()) return;

        this.saving = true;
        try {
            const payload = {
                name: this.workflowForm.name.trim(),
                description: this.workflowForm.description.trim(),
                graph: this.workflowForm.graph,
            };

            let response;
            if (this.editingWorkflow) {
                response = await fetch(`/v1/workflows/${this.editingWorkflow.id}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload),
                });
            } else {
                response = await fetch('/v1/workflows', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload),
                });
            }

            if (response.ok) {
                await this.fetchWorkflows();
                await this.fetchAgents();
                this.view = 'list';
            } else {
                const err = await response.json().catch(() => ({}));
                console.error('Failed to save workflow:', err);
                alert(err.error || 'Failed to save workflow');
            }
        } catch (err) {
            console.error('Failed to save workflow:', err);
            alert('Failed to save workflow');
        } finally {
            this.saving = false;
        }
    },

    confirmDeleteWorkflow(wf) {
        this.deletingWorkflow = wf;
        this.deletingAgent = null;
        this.view = 'delete';
    },

    async deleteWorkflow() {
        if (!this.deletingWorkflow) return;
        this.saving = true;
        try {
            const response = await fetch(`/v1/workflows/${this.deletingWorkflow.id}`, {
                method: 'DELETE',
            });
            if (response.ok || response.status === 204) {
                await this.fetchWorkflows();
                await this.fetchAgents();
                this.view = 'list';
            } else {
                const err = await response.json().catch(() => ({}));
                console.error('Failed to delete workflow:', err);
                alert(err.error || 'Failed to delete workflow');
            }
        } catch (err) {
            console.error('Failed to delete workflow:', err);
            alert('Failed to delete workflow');
        } finally {
            this.saving = false;
        }
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
