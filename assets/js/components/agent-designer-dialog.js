/**
 * Agent Designer Component
 *
 * A dedicated dialog for designing agents. Two modes:
 *
 *   1. **Node Design Mode** — Create or edit a single agent definition
 *      (name, system prompt, tools, KBs, orchestration settings).
 *
 *   2. **Pipeline Compose Mode** — Visually compose multiple agents into
 *      a workflow pipeline using sequential, parallel, and loop patterns.
 *      Maps 1:1 to ADK-Go's composition model (sequentialagent, parallelagent, loopagent).
 *
 * The designer dialog is opened from the navbar via a dedicated icon,
 * or via events dispatched from the agents listing.
 *
 * ADK-Go alignment:
 *   - Each leaf node in a pipeline maps to llmagent.New(...)
 *   - Sequential → sequentialagent.New({ SubAgents })
 *   - Parallel   → parallelagent.New({ SubAgents })
 *   - Loop       → loopagent.New({ SubAgents, MaxIterations })
 *   - Transfer policies, output keys, include_contents map to LLM agent config
 *   - The last child of a loop gets exit_loop tool automatically (backend)
 */
export const agentDesigner = () => ({
    // ══════════════════════════════════════════
    // State
    // ══════════════════════════════════════════

    /** @type {'node' | 'pipeline'} Active mode tab */
    mode: 'node',

    /** @type {'list' | 'form' | 'delete'} Node design sub-view */
    nodeView: 'list',

    /** @type {'canvas' | 'form'} Pipeline sub-view */
    pipelineView: 'canvas',

    // Agent list
    agents: [],
    filteredAgents: [],
    loading: false,
    saving: false,
    searchQuery: '',

    // Editing state
    editingAgent: null,
    form: {
        name: '',
        description: '',
        systemPrompt: '',
        graph: null,
        disallowTransferToParent: false,
        disallowTransferToPeers: false,
        outputKey: '',
        includeContents: '',
        globalInstruction: '',
        inputSchema: '',
        outputSchema: '',
    },

    // UI state
    expanded: false,
    showOrchestration: false,

    // Tools & KB state
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

    // Requirement config state
    showRequirementsPanel: false,
    reqConfigTarget: null,      // { type: 'tool'|'toolset', pluginId, toolId?, name }
    reqConfig: {
        action: 'allow',
        require_confirmation: false,
        confirmation_hint: '',
    },
    savingReqConfig: false,

    // Pipeline composition state
    _nodeCounter: 0,
    showComposition: false,
    pipelineName: '',
    pipelineDescription: '',
    pipelineSystemPrompt: '',
    pipelineGraph: null,
    pipelineOrchestration: {
        disallowTransferToParent: false,
        disallowTransferToPeers: false,
        outputKey: '',
        includeContents: '',
        globalInstruction: '',
        outputSchema: '',
    },
    editingPipeline: null,
    pipelineSaving: false,

    // A2A connectors (for remote nodes)
    connectors: [],

    // Pipeline list state
    pipelines: [],
    filteredPipelines: [],
    pipelineSearchQuery: '',

    // Canvas interaction
    selectedNodePath: null,
    dragState: null,

    // ══════════════════════════════════════════
    // Lifecycle
    // ══════════════════════════════════════════

    async open(startMode) {
        this.mode = startMode || 'node';
        this.nodeView = 'list';
        this.pipelineView = 'canvas';
        this.searchQuery = '';
        this.pipelineSearchQuery = '';
        this.selectedNodePath = null;
        await Promise.all([this.fetchAgents(), this.fetchConnectors()]);
        this.filterPipelines();
    },

    toggleExpand() {
        this.expanded = !this.expanded;
    },

    switchMode(mode) {
        this.mode = mode;
        if (mode === 'pipeline' && this.pipelines.length === 0) {
            this.filterPipelines();
        }
    },

    // ══════════════════════════════════════════
    // Data Fetching
    // ══════════════════════════════════════════

    async fetchAgents() {
        this.loading = true;
        try {
            const response = await fetch('/v1/agent-definitions');
            if (response.ok) {
                const data = await response.json();
                const all = data.agents || [];
                // Separate simple agents from composed pipelines
                this.agents = all.filter(a => !a.isComposed);
                this.pipelines = all.filter(a => a.isComposed);
                this.applyFilter();
                this.filterPipelines();
            }
        } catch (err) {
            console.error('Failed to fetch agents:', err);
        } finally {
            this.loading = false;
        }
    },

    async fetchConnectors() {
        try {
            const response = await fetch('/v1/a2a/connectors');
            if (response.ok) {
                const data = await response.json();
                this.connectors = data.connectors || [];
            }
        } catch (err) {
            console.error('Failed to fetch A2A connectors:', err);
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
                const all = data.agents || [];
                this.filteredAgents = all.filter(a => !a.isComposed);
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
        this.filteredAgents = results;
    },

    filterPipelines() {
        let results = [...this.pipelines];
        if (this.pipelineSearchQuery.trim()) {
            const q = this.pipelineSearchQuery.toLowerCase();
            results = results.filter(a =>
                a.name.toLowerCase().includes(q) ||
                (a.description && a.description.toLowerCase().includes(q))
            );
        }
        this.filteredPipelines = results;
    },

    // ══════════════════════════════════════════
    // Node Design Mode — CRUD
    // ══════════════════════════════════════════

    showCreateNodeForm() {
        this.editingAgent = null;
        this.form = {
            name: '',
            description: '',
            systemPrompt: '',
            graph: null,
            disallowTransferToParent: false,
            disallowTransferToPeers: false,
            outputKey: '',
            includeContents: '',
            globalInstruction: '',
            inputSchema: '',
            outputSchema: '',
        };
        this.showTools = false;
        this.showKnowledgeBases = false;
        this.showFilesystems = false;
        this.showOrchestration = false;
        this.showRequirementsPanel = false;
        this.agentPlugins = [];
        this.agentWorkspaces = [];
        this.agentFilesystems = [];
        this.nodeView = 'form';
    },

    showEditNodeForm(agent) {
        this.editingAgent = agent;
        this.form = {
            name: agent.name || '',
            description: agent.description || '',
            systemPrompt: agent.systemPrompt || '',
            graph: null, // Node mode = simple LLM agent, no composition graph
            disallowTransferToParent: agent.disallowTransferToParent || false,
            disallowTransferToPeers: agent.disallowTransferToPeers || false,
            outputKey: agent.outputKey || '',
            includeContents: agent.includeContents || '',
            globalInstruction: agent.globalInstruction || '',
            inputSchema: agent.inputSchema || '',
            outputSchema: agent.outputSchema || '',
        };
        this.showTools = false;
        this.showKnowledgeBases = false;
        this.showFilesystems = false;
        this.showOrchestration = false;
        this.showRequirementsPanel = false;
        this.nodeView = 'form';

        // Load tool/KB/FS data
        this.fetchAgentPlugins(agent.id);
        this.fetchAgentWorkspaces(agent.id);
        this.fetchAgentFilesystems(agent.id);
    },

    async saveNode() {
        if (!this.form.name.trim()) return;
        this.saving = true;
        try {
            const payload = {
                name: this.form.name.trim(),
                description: this.form.description.trim(),
                systemPrompt: this.form.systemPrompt.trim(),
                // Node mode → no graph (simple LLM agent → llmagent.New)
                disallowTransferToParent: this.form.disallowTransferToParent || false,
                disallowTransferToPeers: this.form.disallowTransferToPeers || false,
                outputKey: this.form.outputKey.trim(),
                includeContents: this.form.includeContents || '',
                globalInstruction: this.form.globalInstruction.trim(),
                inputSchema: this.form.inputSchema.trim(),
                outputSchema: this.form.outputSchema.trim(),
            };

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
                this.nodeView = 'list';
                // Notify other components that agent list changed
                window.dispatchEvent(new CustomEvent('agents-updated'));
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

    confirmDeleteNode(agent) {
        this.deletingAgent = agent;
        this.nodeView = 'delete';
    },

    async deleteNode() {
        if (!this.deletingAgent) return;
        this.saving = true;
        try {
            const response = await fetch(`/v1/agent-definitions/${this.deletingAgent.id}`, {
                method: 'DELETE',
            });
            if (response.ok) {
                await this.fetchAgents();
                this.nodeView = 'list';
                window.dispatchEvent(new CustomEvent('agents-updated'));
            } else {
                const err = await response.json().catch(() => ({}));
                alert(err.error || 'Failed to delete agent');
            }
        } catch (err) {
            console.error('Failed to delete agent:', err);
            alert('Failed to delete agent');
        } finally {
            this.saving = false;
        }
    },

    // ══════════════════════════════════════════
    // Tools, Knowledge Bases, Filesystems
    // ══════════════════════════════════════════

    get toolsSummary() {
        if (this.loadingTools) return '';
        const blocked = this.agentPlugins.filter(p => p.requirement_config && p.requirement_config.action === 'block').length;
        if (blocked > 0) return `${this.agentPlugins.length - blocked}/${this.agentPlugins.length}`;
        return `${this.agentPlugins.length}`;
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
        if (this.form.inputSchema) flags.push('in-schema');
        if (this.form.outputSchema) flags.push('out-schema');
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

    // ══════════════════════════════════════════
    // Tool Requirement Configuration
    // (per-user, per-agent, per-tool/toolset)
    //
    // Maps to:
    //   - ADK-Go: RequireConfirmation, BeforeToolCallback
    //   - AgentStack: approval extension, demand/fulfillment
    //   - Backend: toolreqs.ToolRequirementConfig
    // ══════════════════════════════════════════

    /**
     * Open the requirements configuration panel for a tool set (plugin).
     */
    openToolSetRequirements(plugin) {
        this.reqConfigTarget = {
            type: 'toolset',
            pluginId: plugin.id,
            name: plugin.name,
        };
        // Pre-fill from existing config
        this.reqConfig = {
            action: plugin.requirement_config?.action || 'allow',
            require_confirmation: plugin.requirement_config?.require_confirmation || false,
            confirmation_hint: plugin.requirement_config?.confirmation_hint || '',
        };
        this.showRequirementsPanel = true;
    },

    /**
     * Open the requirements configuration panel for an individual tool.
     */
    openToolRequirements(plugin, tool) {
        this.reqConfigTarget = {
            type: 'tool',
            pluginId: plugin.id,
            toolId: tool.id,
            name: tool.name,
        };
        // Pre-fill from existing config
        this.reqConfig = {
            action: tool.requirement_config?.action || 'allow',
            require_confirmation: tool.requirement_config?.require_confirmation || false,
            confirmation_hint: tool.requirement_config?.confirmation_hint || '',
        };
        this.showRequirementsPanel = true;
    },

    /**
     * Close the requirements panel without saving.
     */
    closeRequirementsPanel() {
        this.showRequirementsPanel = false;
        this.reqConfigTarget = null;
    },

    /**
     * Update the action in the current requirement config (used by the advanced settings dialog).
     * Syncs require_confirmation flag with the action.
     */
    setReqConfigAction(action) {
        this.reqConfig.action = action;
        this.reqConfig.require_confirmation = (action === 'require_confirmation');
    },

    /**
     * Set the access action for a tool set (plugin) immediately.
     * Saves to the backend without opening a modal — the inline segmented control calls this.
     * @param {Object} plugin
     * @param {'allow'|'block'|'require_confirmation'} action
     */
    async setToolSetAction(plugin, action) {
        if (!this.editingAgent) return;
        const agentId = this.editingAgent.id;

        // Optimistic update
        if (!plugin.requirement_config) plugin.requirement_config = {};
        plugin.requirement_config.action = action;
        plugin.requirement_config.require_confirmation = (action === 'require_confirmation');

        const payload = {
            action,
            require_confirmation: action === 'require_confirmation',
            confirmation_hint: plugin.requirement_config.confirmation_hint || '',
        };

        try {
            const url = `/v1/agents/${encodeURIComponent(agentId)}/toolsets/${encodeURIComponent(plugin.id)}/requirements`;
            await fetch(url, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });
        } catch (err) {
            console.error('Failed to set toolset action:', err);
        }
    },

    /**
     * Set the access action for an individual tool immediately.
     * Saves to the backend without opening a modal — the inline segmented control calls this.
     * @param {Object} plugin
     * @param {Object} tool
     * @param {'allow'|'block'|'require_confirmation'} action
     */
    async setToolAction(plugin, tool, action) {
        if (!this.editingAgent) return;
        const agentId = this.editingAgent.id;

        // Optimistic update
        if (!tool.requirement_config) tool.requirement_config = {};
        tool.requirement_config.action = action;
        tool.requirement_config.require_confirmation = (action === 'require_confirmation');

        const payload = {
            action,
            require_confirmation: action === 'require_confirmation',
            confirmation_hint: tool.requirement_config.confirmation_hint || '',
        };

        try {
            const url = `/v1/agents/${encodeURIComponent(agentId)}/toolsets/${encodeURIComponent(plugin.id)}/tools/${encodeURIComponent(tool.id)}/requirements`;
            await fetch(url, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });
        } catch (err) {
            console.error('Failed to set tool action:', err);
        }
    },

    /**
     * Save the current requirement config to the backend.
     * Updates the local plugin/tool state optimistically.
     */
    async saveRequirementConfig() {
        if (!this.editingAgent || !this.reqConfigTarget) return;
        this.savingReqConfig = true;

        try {
            const agentId = this.editingAgent.id;
            const target = this.reqConfigTarget;
            const payload = { ...this.reqConfig };

            let url;
            if (target.type === 'toolset') {
                url = `/v1/agents/${encodeURIComponent(agentId)}/toolsets/${encodeURIComponent(target.pluginId)}/requirements`;
            } else {
                url = `/v1/agents/${encodeURIComponent(agentId)}/toolsets/${encodeURIComponent(target.pluginId)}/tools/${encodeURIComponent(target.toolId)}/requirements`;
            }

            const response = await fetch(url, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload),
            });

            if (response.ok) {
                // Update local state optimistically
                if (target.type === 'toolset') {
                    const plugin = this.agentPlugins.find(p => p.id === target.pluginId);
                    if (plugin) {
                        plugin.requirement_config = { ...payload };
                    }
                } else {
                    const plugin = this.agentPlugins.find(p => p.id === target.pluginId);
                    if (plugin) {
                        const tool = plugin.tools.find(t => t.id === target.toolId);
                        if (tool) {
                            tool.requirement_config = { ...payload };
                        }
                    }
                }
                this.closeRequirementsPanel();
            } else {
                const err = await response.json().catch(() => ({}));
                console.error('Failed to save requirement config:', err);
                alert(err.error || 'Failed to save requirement config');
            }
        } catch (err) {
            console.error('Failed to save requirement config:', err);
            alert('Failed to save requirement config');
        } finally {
            this.savingReqConfig = false;
        }
    },

    // ══════════════════════════════════════════
    // Pipeline Compose Mode — CRUD
    // ══════════════════════════════════════════

    showCreatePipelineForm() {
        this.editingPipeline = null;
        this._nodeCounter = 1;
        this.pipelineName = '';
        this.pipelineDescription = '';
        this.pipelineSystemPrompt = '';
        this.pipelineOrchestration = {
            disallowTransferToParent: false,
            disallowTransferToPeers: false,
            outputKey: '',
            includeContents: '',
            globalInstruction: '',
            outputSchema: '',
        };
        // Default: sequential root with one agent child
        this.pipelineGraph = {
            id: 'node-1',
            type: 'sequential',
            label: '',
            children: [this.createNode('agent')],
        };
        this.selectedNodePath = null;
        this.pipelineView = 'form';
    },

    showEditPipelineForm(pipeline) {
        this.editingPipeline = pipeline;
        this._nodeCounter = 0;

        if (pipeline.graph) {
            const walk = (n) => {
                if (n.id) {
                    const num = parseInt(n.id.replace('node-', ''), 10);
                    if (num > this._nodeCounter) this._nodeCounter = num;
                }
                if (n.children) n.children.forEach(walk);
            };
            walk(pipeline.graph);
        }

        this.pipelineName = pipeline.name || '';
        this.pipelineDescription = pipeline.description || '';
        this.pipelineSystemPrompt = pipeline.systemPrompt || '';
        this.pipelineGraph = pipeline.graph ? JSON.parse(JSON.stringify(pipeline.graph)) : {
            id: 'node-1',
            type: 'sequential',
            label: '',
            children: [this.createNode('agent')],
        };
        this.pipelineOrchestration = {
            disallowTransferToParent: pipeline.disallowTransferToParent || false,
            disallowTransferToPeers: pipeline.disallowTransferToPeers || false,
            outputKey: pipeline.outputKey || '',
            includeContents: pipeline.includeContents || '',
            globalInstruction: pipeline.globalInstruction || '',
            outputSchema: pipeline.outputSchema || '',
        };
        this.selectedNodePath = null;
        this.pipelineView = 'form';
    },

    async savePipeline() {
        if (!this.pipelineName.trim()) return;
        this.pipelineSaving = true;
        try {
            const payload = {
                name: this.pipelineName.trim(),
                description: this.pipelineDescription.trim(),
                systemPrompt: this.pipelineSystemPrompt.trim(),
                graph: this.pipelineGraph,
                disallowTransferToParent: this.pipelineOrchestration.disallowTransferToParent || false,
                disallowTransferToPeers: this.pipelineOrchestration.disallowTransferToPeers || false,
                outputKey: this.pipelineOrchestration.outputKey.trim(),
                includeContents: this.pipelineOrchestration.includeContents || '',
                globalInstruction: this.pipelineOrchestration.globalInstruction.trim(),
                outputSchema: this.pipelineOrchestration.outputSchema.trim(),
            };

            if (this.editingPipeline && this.editingPipeline.icon) {
                payload.icon = this.editingPipeline.icon;
            }

            let response;
            if (this.editingPipeline) {
                response = await fetch(`/v1/agent-definitions/${this.editingPipeline.id}`, {
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
                this.pipelineView = 'canvas';
                window.dispatchEvent(new CustomEvent('agents-updated'));
            } else {
                const err = await response.json().catch(() => ({}));
                alert(err.error || 'Failed to save pipeline');
            }
        } catch (err) {
            console.error('Failed to save pipeline:', err);
            alert('Failed to save pipeline');
        } finally {
            this.pipelineSaving = false;
        }
    },

    confirmDeletePipeline(pipeline) {
        this.deletingAgent = pipeline;
        this.pipelineView = 'delete';
    },

    async deletePipeline() {
        if (!this.deletingAgent) return;
        this.pipelineSaving = true;
        try {
            const response = await fetch(`/v1/agent-definitions/${this.deletingAgent.id}`, {
                method: 'DELETE',
            });
            if (response.ok) {
                await this.fetchAgents();
                this.pipelineView = 'canvas';
                window.dispatchEvent(new CustomEvent('agents-updated'));
            } else {
                const err = await response.json().catch(() => ({}));
                alert(err.error || 'Failed to delete pipeline');
            }
        } catch (err) {
            console.error('Failed to delete pipeline:', err);
            alert('Failed to delete pipeline');
        } finally {
            this.pipelineSaving = false;
        }
    },

    // ══════════════════════════════════════════
    // Composition Tree Helpers
    // (maps to ADK-Go: sequential/parallel/loop/agent)
    // ══════════════════════════════════════════

    _nextNodeId() {
        this._nodeCounter++;
        return `node-${this._nodeCounter}`;
    },

    /**
     * Create a new composition node.
     *
     * ADK-Go equivalence:
     *   'agent'      → leaf node → llmagent.New(llmagent.Config{Name, Instruction, Tools, ...})
     *   'sequential' → sequentialagent.New(sequentialagent.Config{AgentConfig: agent.Config{Name, SubAgents}})
     *   'parallel'   → parallelagent.New(parallelagent.Config{AgentConfig: agent.Config{Name, SubAgents}})
     *   'loop'       → loopagent.New(loopagent.Config{AgentConfig: agent.Config{Name, SubAgents}, MaxIterations})
     *
     * label → agent.Config.Name (for composite nodes, auto-generated if empty)
     */
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
        if (type === 'remote') {
            return { id, type: 'remote', label: '', a2a_connector_id: '', output_key: '' };
        }
        // Composite node — label maps to agent.Config.Name in ADK-Go
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
        return this.pipelineGraph && this.countNodes(this.pipelineGraph) < 50;
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
            delete node.a2a_connector_id;
            node.agent_id = '';
            node.output_key = '';
            node.disallow_transfer_to_parent = null;
            node.disallow_transfer_to_peers = null;
            node.include_contents = '';
        } else if (newType === 'remote') {
            delete node.children;
            delete node.max_iterations;
            delete node.agent_id;
            delete node.disallow_transfer_to_parent;
            delete node.disallow_transfer_to_peers;
            delete node.include_contents;
            node.a2a_connector_id = node.a2a_connector_id || '';
            node.output_key = node.output_key || '';
        } else {
            delete node.agent_id;
            delete node.a2a_connector_id;
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
        let node = this.pipelineGraph;
        for (let i = 1; i < parts.length; i++) {
            const idx = parseInt(parts[i], 10);
            if (!node.children || idx >= node.children.length) return null;
            node = node.children[idx];
        }
        return node;
    },

    selectNode(path) {
        this.selectedNodePath = this.selectedNodePath === path ? null : path;
    },

    getSelectedNode() {
        if (!this.selectedNodePath) return null;
        return this._getNodeByPath(this.selectedNodePath);
    },

    nodeTypeLabel(type) {
        const labels = { sequential: 'Sequential', parallel: 'Parallel', loop: 'Loop', agent: 'Agent', remote: 'Remote A2A' };
        return labels[type] || type;
    },

    nodeTypeBadgeClass(type) {
        const classes = {
            sequential: 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20',
            parallel: 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20',
            loop: 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20',
            agent: 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20',
            remote: 'bg-rose-500/10 text-rose-600 dark:text-rose-400 border-rose-500/20',
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

    /** Get agent name by id from the agents list */
    getAgentName(agentId) {
        const ag = this.agents.find(a => a.id === agentId);
        return ag ? ag.name : agentId ? '(unknown)' : '(not set)';
    },

    // ══════════════════════════════════════════
    // Pipeline Validation (ADK-Go alignment)
    // ══════════════════════════════════════════

    /**
     * Validate the workflow graph matches ADK-Go's composition model:
     *
     * - Root must be a composite node (sequential/parallel/loop) — workflow
     *   agents are always orchestrators containing sub-agents.
     * - All agent (leaf) nodes must reference an existing LLM Agent via agent_id.
     * - Composite nodes (sequential/parallel/loop) must have ≥1 child.
     * - Loop nodes must have max_iterations ≥ 1.
     * - Max 50 nodes total (matches backend maxTreeNodes).
     * - Max depth 5 (matches backend maxTreeDepth).
     */
    validatePipeline() {
        const errors = [];
        if (!this.pipelineGraph) {
            errors.push('Workflow Agent must have a root node');
            return errors;
        }

        if (!this.pipelineName.trim()) {
            errors.push('Workflow Agent requires a name (→ maps to the root agent.Config.Name)');
        }

        // Root must be composite — a workflow agent is a sequentialagent/parallelagent/loopagent
        const rootType = this.pipelineGraph.type;
        if (rootType === 'agent' || rootType === 'remote') {
            errors.push('Root node must be Sequential, Parallel, or Loop (a Workflow Agent orchestrates sub-agents — use the LLM Agent tab for a single agent)');
        }

        const walk = (node, path, depth) => {
            if (!node) return;
            const isComposite = node.type === 'sequential' || node.type === 'parallel' || node.type === 'loop';

            if (node.type === 'agent') {
                if (!node.agent_id) {
                    errors.push(`${path}: agent node requires an LLM Agent selection (→ resolves to llmagent.New)`);
                }
            } else if (node.type === 'remote') {
                if (!node.a2a_connector_id) {
                    errors.push(`${path}: remote node requires an A2A Connector selection`);
                }
            } else if (isComposite) {
                if (!node.children || node.children.length === 0) {
                    errors.push(`${path}: ${node.type}agent requires at least one SubAgent`);
                }
                if (node.type === 'loop' && (!node.max_iterations || node.max_iterations < 1)) {
                    errors.push(`${path}: loopagent.Config.MaxIterations must be ≥ 1`);
                }
            }

            if (depth > 5) {
                errors.push(`${path}: tree exceeds maximum depth of 5`);
            }

            if (node.children) {
                node.children.forEach((child, i) => walk(child, `${path}.${i}`, depth + 1));
            }
        };

        walk(this.pipelineGraph, 'root', 1);

        if (this.countNodes(this.pipelineGraph) > 50) {
            errors.push('Workflow tree exceeds maximum of 50 nodes');
        }

        return errors;
    },

    get pipelineErrors() {
        return this.validatePipeline();
    },

    get isPipelineValid() {
        return this.pipelineErrors.length === 0;
    },

    // ══════════════════════════════════════════
    // Utilities
    // ══════════════════════════════════════════

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
