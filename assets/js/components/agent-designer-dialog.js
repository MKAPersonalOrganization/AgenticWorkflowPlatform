/**
 * Agent Designer Component
 *
 * Unified dialog for designing all ADK-Go agent types:
 *   - LLM Agent     → llmagent.New(...)
 *   - Sequential    → sequentialagent.New({ SubAgents })
 *   - Parallel      → parallelagent.New({ SubAgents })
 *   - Loop          → loopagent.New({ SubAgents, MaxIterations })
 *
 * All agents share the same Agent interface in ADK-Go.
 * Transfer policies, output keys, include_contents map to agent config.
 * The last child of a loop gets exit_loop tool automatically (backend).
 */
export const agentDesigner = () => ({
    // ══════════════════════════════════════════
    // State
    // ══════════════════════════════════════════

    /** @type {'list' | 'agentForm' | 'flow' | 'delete'} */
    view: 'list',

    // Agent lists
    allAgents: [],      // all agent definitions (both LLM and workflow)
    agents: [],         // non-composed only (for palette/tree builder leaf references)
    filteredAgents: [],  // filtered allAgents for the list view
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
        modelConfig: {
            temperature: null,
            topP: null,
            topK: null,
            maxOutputTokens: null,
            stopSequences: [],
        },
        modelOverride: { provider: '', modelName: '' },
    },

    // UI state
    expanded: false,
    showOrchestration: false,
    showModelSettings: false,
    showModelOverride: false,

    providerModelOptions: [],  // populated dynamically when a provider is selected
    providerModelsLoading: false,
    providerModelsError: '',

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
        max_invocations: 0,
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

    // Canvas interaction
    selectedNodePath: null,
    dragState: null,

    // Sidebar visibility (for property panel)
    propertyPanelVisible: true,

    // ══════════════════════════════════════════
    // Lifecycle
    // ══════════════════════════════════════════

    async open() {
        this.view = 'list';
        this.searchQuery = '';
        this.selectedNodePath = null;
        await Promise.all([this.fetchAgents(), this.fetchConnectors()]);
    },

    toggleExpand() {
        this.expanded = !this.expanded;
    },

    togglePropertyPanel() {
        this.propertyPanelVisible = !this.propertyPanelVisible;
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
                this.allAgents = all;
                this.agents = all.filter(a => !a.isComposed); // leaf agents for palette/tree builder
                this.applyFilter();
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
                this.filteredAgents = data.agents || [];
            }
        } catch (err) {
            console.error('Failed to search agents:', err);
            this.applyFilter();
        }
    },

    applyFilter() {
        let results = [...this.allAgents];
        if (this.searchQuery.trim()) {
            const q = this.searchQuery.toLowerCase();
            results = results.filter(a =>
                a.name.toLowerCase().includes(q) ||
                (a.description && a.description.toLowerCase().includes(q))
            );
        }
        this.filteredAgents = results;
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
            modelConfig: {
                temperature: null,
                topP: null,
                topK: null,
                maxOutputTokens: null,
                stopSequences: [],
            },
            modelOverride: { provider: '', modelName: '' },
        };
        this.showTools = false;
        this.showKnowledgeBases = false;
        this.showFilesystems = false;
        this.showOrchestration = false;
        this.showModelSettings = false;
        this.showModelOverride = false;
        this.showRequirementsPanel = false;
        this.agentPlugins = [];
        this.agentWorkspaces = [];
        this.agentFilesystems = [];
        this.providerModelOptions = [];
        this.providerModelsError = '';
        this.view = 'agentForm';
    },

    showEditNodeForm(agent) {
        this.editingAgent = agent;
        const mc = agent.modelConfig || {};
        const mo = agent.modelOverride || {};
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
            modelConfig: {
                temperature: mc.temperature ?? null,
                topP: mc.topP ?? null,
                topK: mc.topK ?? null,
                maxOutputTokens: mc.maxOutputTokens ?? null,
                stopSequences: mc.stopSequences ? [...mc.stopSequences] : [],
            },
            modelOverride: {
                provider: mo.provider || '',
                modelName: mo.modelName || '',
            },
        };
        this.showTools = false;
        this.showKnowledgeBases = false;
        this.showFilesystems = false;
        this.showOrchestration = false;
        this.showModelSettings = false;
        this.showModelOverride = false;
        this.showRequirementsPanel = false;
        this.providerModelOptions = [];
        this.providerModelsError = '';
        this.view = 'agentForm';

        // Load tool/KB/FS data
        this.fetchAgentPlugins(agent.id);
        this.fetchAgentWorkspaces(agent.id);
        this.fetchAgentFilesystems(agent.id);

        // Pre-load model list if this agent already has a provider override saved
        if (mo.provider) {
            this.fetchModelsForProvider(mo.provider);
        }
    },

    async saveNode() {
        if (!this.form.name.trim()) return;
        this.saving = true;
        try {
            const mc = this.form.modelConfig;
            const modelConfig = {};
            if (mc.temperature !== null && mc.temperature !== '') modelConfig.temperature = Number(mc.temperature);
            if (mc.topP !== null && mc.topP !== '') modelConfig.topP = Number(mc.topP);
            if (mc.topK !== null && mc.topK !== '') modelConfig.topK = Number(mc.topK);
            if (mc.maxOutputTokens !== null && mc.maxOutputTokens !== '') modelConfig.maxOutputTokens = Number(mc.maxOutputTokens);
            if (mc.stopSequences && mc.stopSequences.length > 0) modelConfig.stopSequences = mc.stopSequences;

            const mo = this.form.modelOverride;
            const modelOverride = (mo.provider && mo.modelName)
                ? { provider: mo.provider, modelName: mo.modelName }
                : null;

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
                modelConfig: Object.keys(modelConfig).length > 0 ? modelConfig : null,
                modelOverride,
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
                this.view = 'list';
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
        this.view = 'delete';
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
                this.view = 'list';
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

    async fetchModelsForProvider(provider) {
        if (!provider) {
            this.providerModelOptions = [];
            this.providerModelsError = '';
            return;
        }
        this.providerModelsLoading = true;
        this.providerModelsError = '';
        this.providerModelOptions = [];
        try {
            const resp = await fetch(`/api/ai-settings/providers/${encodeURIComponent(provider)}/models`);
            if (!resp.ok) {
                const text = await resp.text();
                throw new Error(text || `Failed to load models (${resp.status})`);
            }
            this.providerModelOptions = await resp.json();
        } catch (e) {
            this.providerModelsError = e.message;
        } finally {
            this.providerModelsLoading = false;
        }
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
            max_invocations: plugin.requirement_config?.max_invocations || 0,
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
            max_invocations: tool.requirement_config?.max_invocations || 0,
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
        this.$nextTick(() => this.openFlowView());
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
        this.$nextTick(() => this.openFlowView());
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
                this.view = 'list';
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
        this.view = 'delete';
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
                this.view = 'list';
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
    // AlpineFlow Canvas Integration
    // ══════════════════════════════════════════

    /**
     * Switch the pipeline editor to the visual flow canvas.
     * Loads the current pipelineGraph into Alpine.store('flow') and switches view.
     */
    async openFlowView() {
        const store = Alpine.store('flow');
        if (!store) return;

        // The /v1/agent-definitions list excludes sub-agents that are already
        // embedded inside a pipeline graph (via excludeSubAgents on the backend).
        // We must fetch those missing agents individually so the canvas can display
        // their names, system prompts, and other config.
        const graphAgentIds = [];
        const collectIds = (node) => {
            if (!node) return;
            if (node.type === 'agent' && node.agent_id) graphAgentIds.push(node.agent_id);
            (node.children || []).forEach(collectIds);
        };
        collectIds(this.pipelineGraph);

        const knownIds = new Set((this.allAgents || []).map(a => a.id));
        const missingIds = [...new Set(graphAgentIds.filter(id => id && !knownIds.has(id)))];

        const fetched = await Promise.all(
            missingIds.map(id =>
                fetch(`/v1/agent-definitions/${id}`)
                    .then(r => r.ok ? r.json() : null)
                    .catch(() => null)
            )
        );

        // Merge: palette agents (non-composed) + sub-agents fetched on demand
        const allAgents = [...this.agents, ...fetched.filter(Boolean)];

        store.load(this.pipelineGraph, allAgents, this.connectors, this.pipelineSystemPrompt);
        this.view = 'flow';

        // Initialize the editable workflow tree component
        this.$nextTick(() => {
            const treeEl = this.$el.querySelector('[x-data="editableWorkflowTree()"]');
            if (treeEl && treeEl.__x) {
                treeEl.__x.$data.setGraph(this.pipelineGraph);
                treeEl.__x.$data.setAgents(allAgents);
                treeEl.__x.$data.setConnectors(this.connectors);
                treeEl.__x.$data.onChange((graph) => {
                    this.pipelineGraph = graph;
                    store.load(graph, allAgents, this.connectors, this.pipelineSystemPrompt);
                });
            }
        });

        // Keyboard shortcut for property panel toggle
        this._sidebarKeyHandler = (e) => {
            if ((e.metaKey || e.ctrlKey) && e.key === ']') {
                e.preventDefault();
                this.propertyPanelVisible = !this.propertyPanelVisible;
            }
        };
        document.addEventListener('keydown', this._sidebarKeyHandler);
    },

    /**
     * Save from the flow view — syncs graph from store then calls savePipeline().
     */
    async saveFromFlowView() {
        this._syncGraphFromStore();
        await this.savePipeline();
    },

    /** Copy graph and systemPrompt from Alpine.store('flow') back into component state. */
    _syncGraphFromStore() {
        const store = Alpine.store('flow');
        if (!store) return;
        const g = store.export();
        if (g) this.pipelineGraph = g;
        this.pipelineSystemPrompt = store.systemPrompt || this.pipelineSystemPrompt;
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
