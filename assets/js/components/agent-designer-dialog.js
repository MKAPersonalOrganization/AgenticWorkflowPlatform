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

    /** @type {'list' | 'typeSelect' | 'templates' | 'importYaml' | 'agentForm' | 'flow' | 'delete'} */
    view: 'list',

    // ══════════════════════════════════════════
    // Templates / Catalog state
    // ══════════════════════════════════════════
    catalogEntries: [],
    catalogCategories: [],
    catalogSearchQuery: '',
    catalogSelectedCategory: '',
    catalogLoading: false,
    catalogAddingIds: [],
    catalogTotal: 0,

    // Fork name dialog state
    showForkNameDialog: false,
    forkNameEntry: null,
    forkNameInput: '',
    forkNameAdding: false,

    importYamlText: '',
    importYamlURL: '',
    importYamlLoading: false,

    // Agent filter
    agentFilter: 'all', // 'all' | 'agent' | 'workflow'

    // Agent lists
    allAgents: [],      // all agent definitions (both LLM and workflow)
    agents: [],         // all agents (including composed) for tree builder leaf references
    filteredAgents: [],  // filtered allAgents for the list view
    loading: false,
    saving: false,
    searchQuery: '',

    // Toast notifications
    toasts: [],
    __toastIdCounter: 0,

    showToast(message, type = 'info', duration = 3500) {
        const id = ++this.__toastIdCounter;
        this.toasts.push({ id, message, type, visible: true, leaving: false });
        if (duration > 0) {
            setTimeout(() => this.dismissToast(id), duration);
        }
        return id;
    },

    dismissToast(id) {
        const toast = this.toasts.find(t => t.id === id);
        if (toast && !toast.leaving) {
            toast.leaving = true;
            setTimeout(() => { this.toasts = this.toasts.filter(t => t.id !== id); }, 200);
        }
    },

    // Editing state
    editingAgent: null,
    form: {
        name: '',
        description: '',
        systemPrompt: '',
        graph: null,
        subAgents: [],
        agentTools: [],
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
        memoryPolicy: null,
        artifactPolicy: null,
        codeExecutionPolicy: null,
        // Streaming policy. Nil = stream (historical default, also what
        // ADK-Go does when RunConfig.StreamingMode is unset). A non-null
        // { enabled: false } is the only way to explicitly turn streaming
        // off — keeping the shape parallel to the other optional policies.
        streamingPolicy: null,
    },

    // UI state
    expanded: false,
    showSubAgents: false,
    showModelConfig: false,
    showGlobalInstruction: false,
    showMemory: false,
    showArtifacts: false,
    showCodeExecution: false,
    showStreaming: false,

    // ── Instruction template autocomplete ─────────────────────────────
    // We expose the two ADK placeholder forms we officially support:
    //   {key}   — required, resolved from session state
    //   {key?}  — optional, silent empty string when missing
    // Scoped-prefix ({user:key}, {app:key}, {temp:key}) and artifact
    // references ({artifact.name}) still resolve at runtime in ADK but
    // are intentionally hidden from the picker to keep the UI
    // approachable for non-programmers.
    availableStateKeys: [],           // [{key, scope, source}] from backend
    promptAutocomplete: {
        open: false,
        query: '',
        triggerIndex: -1,             // index of the '{' that opened the popover
        items: [],
        activeIndex: 0,
    },
    // A single syntax hint for the optional ?-suffix variant. Users
    // type the bare name; the hint reminds them the suffix is available.
    promptStaticHints: [
        { scope: 'optional', key: 'key?', detail: "Optional — silent when the key isn't in session state" },
    ],

    // Sub-agent selection state
    subAgentDialogOpen: false,

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
    pipelineConsumers: [],

    // A2A connectors (for remote nodes)
    connectors: [],

    // Canvas interaction
    selectedNodePath: null,
    dragState: null,

    // Sidebar visibility (for property panel)
    propertyPanelVisible: true,

    // ══════════════════════════════════════════
    // Tool / Plugin Helpers
    // ══════════════════════════════════════════

    /** Check if a plugin is enabled (has requirement_config.action !== 'block', or no config = disabled) */
    isPluginEnabled(plugin) {
        // No config = disabled by default for new agents
        if (!plugin.requirement_config) return false;
        return plugin.requirement_config.action !== 'block';
    },

    /** Check if a tool is enabled (has requirement_config.action !== 'block', or no config = disabled) */
    isToolEnabled(tool) {
        // No config = disabled by default for new agents
        if (!tool.requirement_config) return false;
        return tool.requirement_config.action !== 'block';
    },

    /** Count enabled plugins */
    get enabledPluginsCount() {
        return this.agentPlugins.filter(p => this.isPluginEnabled(p)).length;
    },
    // ══════════════════════════════════════════
    // Lifecycle
    // ══════════════════════════════════════════

    init() {
    },

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
    // Templates / Catalog
    // ══════════════════════════════════════════
    // Agent Fetching
    // ══════════════════════════════════════════

    async fetchConsumers(agentId) {
        try {
            const response = await fetch(`/v1/agent-definitions/${encodeURIComponent(agentId)}/consumers`);
            if (response.ok) {
                const data = await response.json();
                this.pipelineConsumers = data.agents || [];
            }
        } catch (err) {
            console.error('Failed to fetch consumers:', err);
            this.pipelineConsumers = [];
        }
    },

    async fetchAgents() {
        this.loading = true;
        try {
            const response = await fetch('/v1/agent-definitions');
            if (response.ok) {
                const data = await response.json();
                const all = data.agents || [];
                this.allAgents = all;
                this.agents = all; // all agents (including composed) for tree builder to allow fork references
                this.applyFilter();
            }
        } catch (err) {
            console.error('Failed to fetch agents:', err);
        } finally {
            this.loading = false;
        }
    },

    /**
     * Convenience getters / setters for the Memory card. The form stores
     * memoryPolicy as either null (adapter falls back to the legacy
     * memory-set permission) or { enabled, preload }. Toggling "enabled"
     * from null materialises the object; toggling it back to false keeps
     * the object but flags both switches off so the backend sees an
     * explicit disable rather than a "don't know".
     */
    get memoryEnabled() {
        return !!(this.form.memoryPolicy && this.form.memoryPolicy.enabled);
    },
    set memoryEnabled(v) {
        if (v) {
            if (!this.form.memoryPolicy) this.form.memoryPolicy = { enabled: false, preload: false };
            this.form.memoryPolicy.enabled = true;
        } else {
            this.form.memoryPolicy = { enabled: false, preload: false };
        }
    },
    get memoryPreload() {
        return !!(this.form.memoryPolicy && this.form.memoryPolicy.preload);
    },
    set memoryPreload(v) {
        if (!this.form.memoryPolicy) this.form.memoryPolicy = { enabled: false, preload: false };
        this.form.memoryPolicy.preload = !!v;
    },

    // Artifacts — same shape as Memory: Enabled gates the capability;
    // AutoSaveUserBlobs is a secondary knob that only matters when
    // Enabled is on. Storing null when fully off means the backend
    // receives a clean "no artifacts" and doesn't see a partially-set
    // object.
    get artifactsEnabled() {
        return !!(this.form.artifactPolicy && this.form.artifactPolicy.enabled);
    },
    set artifactsEnabled(v) {
        if (v) {
            if (!this.form.artifactPolicy) this.form.artifactPolicy = { enabled: false, autoSaveUserBlobs: false };
            this.form.artifactPolicy.enabled = true;
        } else {
            this.form.artifactPolicy = { enabled: false, autoSaveUserBlobs: false };
        }
    },
    get artifactsAutoSaveBlobs() {
        return !!(this.form.artifactPolicy && this.form.artifactPolicy.autoSaveUserBlobs);
    },
    set artifactsAutoSaveBlobs(v) {
        if (!this.form.artifactPolicy) this.form.artifactPolicy = { enabled: false, autoSaveUserBlobs: false };
        this.form.artifactPolicy.autoSaveUserBlobs = !!v;
    },

    // Code execution — single Boolean. Toggle materialises / clears the
    // policy object so the backend sees an explicit choice rather than
    // partial state.
    get codeExecutionEnabled() {
        return !!(this.form.codeExecutionPolicy && this.form.codeExecutionPolicy.enabled);
    },
    set codeExecutionEnabled(v) {
        if (v) {
            this.form.codeExecutionPolicy = { enabled: true };
        } else {
            this.form.codeExecutionPolicy = { enabled: false };
        }
    },

    // Streaming — on/off toggle. Nil policy (the default for existing
    // agents) means "stream" since that matches the adapter's historical
    // behaviour. Flipping the toggle off materialises an explicit
    // { enabled: false } so the backend persists the choice and the
    // runner passes StreamingModeNone. Flipping it back on clears the
    // policy to keep the payload tidy — nil round-trips to "stream".
    get streamingEnabled() {
        if (!this.form.streamingPolicy) return true;
        return !!this.form.streamingPolicy.enabled;
    },
    set streamingEnabled(v) {
        if (v) {
            this.form.streamingPolicy = null;
        } else {
            this.form.streamingPolicy = { enabled: false };
        }
    },

    // Helper: is the currently selected model provider one where the
    // built-in code executor is actually wired? Used by the UI to show a
    // hint when another provider is active. Gemini + OpenAI are shipped;
    // Anthropic (via OpenAI-compatible) and any other provider fall
    // through to the default (no code execution wired yet).
    get codeExecutionSupportedByProvider() {
        const p = (this.form.modelOverride && this.form.modelOverride.provider || '').toLowerCase();
        if (!p) return true; // unknown / using global default — assume ok
        return ['google', 'gemini', 'vertex', 'vertexai', 'openai'].includes(p);
    },

    /**
     * Whether the agent currently being edited is a root agent (no parent
     * workflow). Root-only controls — most importantly the Global
     * Instruction editor — read this to hide themselves for sub-agents.
     *
     * Why root-only: ADK enforces that only the root agent's
     * GlobalInstruction takes effect at runtime (see
     * .READ/adk-go/agent/llmagent/llmagent.go — "ONLY the
     * GlobalInstruction in the root agent will take effect"). Exposing
     * it on sub-agents would silently do nothing and mislead the user.
     *
     * A brand-new agent (no editingAgent yet) counts as root — new agents
     * start as top-level entries in the user's library.
     */
    isRootNode() {
        if (!this.editingAgent) return true;
        return !this.editingAgent.parentId;
    },

    /** Fetch a single agent by ID */
    async fetchAgentById(agentId) {
        try {
            const response = await fetch(`/v1/agent-definitions/${agentId}`);
            if (response.ok) {
                const data = await response.json();
                return data;
            } else {
                console.error('Failed to fetch agent:', response.status);
                return null;
            }
        } catch (err) {
            console.error('Failed to fetch agent:', err);
            return null;
        }
    },

    // ── Instruction template autocomplete ─────────────────────────────

    /**
     * Load the list of state keys that can be referenced from this
     * agent's instruction. Backed by GET /v1/agent-definitions/{id}/
     * available-state-keys, which aggregates OutputKey values across the
     * user's agents. Missing-endpoint failures are non-fatal — the user
     * can still type keys by hand.
     */
    async fetchAvailableStateKeys(agentId) {
        if (!agentId) {
            this.availableStateKeys = [];
            return;
        }
        try {
            const response = await fetch(`/v1/agent-definitions/${encodeURIComponent(agentId)}/available-state-keys`);
            if (!response.ok) {
                this.availableStateKeys = [];
                return;
            }
            const data = await response.json();
            this.availableStateKeys = Array.isArray(data.keys) ? data.keys : [];
        } catch (err) {
            console.error('Failed to fetch available state keys:', err);
            this.availableStateKeys = [];
        }
    },

    /**
     * Handle keystrokes inside the system-prompt textarea. Typing '{'
     * opens the picker; typing '}' or whitespace closes it. While open
     * the input narrows the list by prefix. Matches ADK's lexer, which
     * only substitutes keys matching ^[a-zA-Z_][a-zA-Z0-9_]*$ — anything
     * else is treated as literal text.
     */
    handlePromptInput(event) {
        const ta = event.target;
        const caret = ta.selectionStart;
        const text = ta.value;

        // '{' opens the popover and anchors it at the trigger index.
        if (event.data === '{') {
            this.promptAutocomplete.triggerIndex = caret - 1;
            this.promptAutocomplete.query = '';
            this.promptAutocomplete.activeIndex = 0;
            this.refreshPromptAutocompleteItems();
            this.promptAutocomplete.open = true;
            return;
        }

        if (!this.promptAutocomplete.open) return;

        // '}' commits the literal typing and closes the popover.
        if (event.data === '}') {
            this.closePromptAutocomplete();
            return;
        }

        const trigger = this.promptAutocomplete.triggerIndex;
        if (trigger < 0 || caret <= trigger) {
            this.closePromptAutocomplete();
            return;
        }
        const query = text.slice(trigger + 1, caret);
        // ADK's regex starts with [a-zA-Z_]; anything else means the
        // user isn't writing a template key, so close.
        if (!/^[a-zA-Z0-9_:.?]*$/.test(query)) {
            this.closePromptAutocomplete();
            return;
        }
        this.promptAutocomplete.query = query;
        this.promptAutocomplete.activeIndex = 0;
        this.refreshPromptAutocompleteItems();
    },

    /**
     * Keyboard navigation inside the popover. ArrowUp/Down move the
     * active item; Enter/Tab insert it; Escape closes without inserting.
     */
    handlePromptKeydown(event) {
        if (!this.promptAutocomplete.open) return;
        const items = this.promptAutocomplete.items;
        if (items.length === 0) {
            if (event.key === 'Escape') this.closePromptAutocomplete();
            return;
        }
        switch (event.key) {
            case 'ArrowDown':
                event.preventDefault();
                this.promptAutocomplete.activeIndex =
                    (this.promptAutocomplete.activeIndex + 1) % items.length;
                break;
            case 'ArrowUp':
                event.preventDefault();
                this.promptAutocomplete.activeIndex =
                    (this.promptAutocomplete.activeIndex - 1 + items.length) % items.length;
                break;
            case 'Enter':
            case 'Tab': {
                event.preventDefault();
                const item = items[this.promptAutocomplete.activeIndex];
                if (item) this.insertStateKey(event.target, item);
                break;
            }
            case 'Escape':
                event.preventDefault();
                this.closePromptAutocomplete();
                break;
        }
    },

    /**
     * Rebuild the filtered list combining live OutputKey suggestions
     * (from the backend, already sorted by priority DESC) with the static
     * ADK syntax hints. Prefix match on the query — case-insensitive —
     * keeps the popover responsive as the user types.
     *
     * Priority tiers come from the backend (see
     * modules/agentdefinition/service.go StateKeySuggestion) and mirror
     * ADK's sample corpus — self > direct sub-agents > workflow peers
     * > library. Static syntax hints land below the live entries as a
     * low-priority "syntax reference" group.
     */
    refreshPromptAutocompleteItems() {
        const q = (this.promptAutocomplete.query || '').toLowerCase();
        const live = (this.availableStateKeys || []).map(k => ({
            key: k.key,
            scope: k.scope || 'session',
            source: k.source || '',
            priority: k.priority || 0,
            kind: 'state',
        }));
        const statics = (this.promptStaticHints || []).map(h => ({
            key: h.key,
            scope: h.scope,
            source: h.detail || '',
            priority: -1, // below all live suggestions
            kind: 'syntax',
        }));
        const all = [...live, ...statics];
        this.promptAutocomplete.items = q
            ? all.filter(it => it.key.toLowerCase().startsWith(q))
            : all;
    },

    /**
     * Return a short group label for a suggestion's priority, used to
     * render a sticky divider above rows that change tier. Mirrors the
     * backend tiers in service.go GetAvailableStateKeys.
     */
    promptSuggestionGroup(item) {
        if (!item) return '';
        if (item.kind === 'syntax') return 'Syntax reference';
        const p = item.priority || 0;
        if (p >= 100) return 'This agent';
        if (p >= 80)  return 'Direct sub-agents';
        if (p >= 70)  return 'Workflow children';
        if (p >= 60)  return 'Workflow peers';
        return 'Other agents in your library';
    },

    /**
     * Whether the given item should show a group header above it — true
     * only when the previous item belonged to a different group.
     */
    promptShowGroupHeader(idx) {
        if (idx === 0) return true;
        const items = this.promptAutocomplete.items;
        return this.promptSuggestionGroup(items[idx]) !== this.promptSuggestionGroup(items[idx - 1]);
    },

    /**
     * Replace the {<typed> fragment with {<key>} and place the caret
     * after the closing brace. For syntax hints that already contain
     * their own punctuation (e.g. `key?`, `artifact.name`) we insert
     * verbatim so the user can just replace the placeholder name.
     */
    insertStateKey(textarea, item) {
        const trigger = this.promptAutocomplete.triggerIndex;
        if (trigger < 0) {
            this.closePromptAutocomplete();
            return;
        }
        const value = textarea.value;
        const caret = textarea.selectionStart;
        const before = value.slice(0, trigger);
        const after = value.slice(caret);
        // The '{' is already in the text at trigger; we append the key +
        // '}'. The key may include its own trailing '?' for optional.
        const insertion = '{' + item.key + '}';
        const next = before + insertion + after;
        textarea.value = next;
        this.form.systemPrompt = next;
        const newCaret = before.length + insertion.length;
        textarea.setSelectionRange(newCaret, newCaret);
        textarea.focus();
        this.closePromptAutocomplete();
    },

    closePromptAutocomplete() {
        this.promptAutocomplete.open = false;
        this.promptAutocomplete.triggerIndex = -1;
        this.promptAutocomplete.query = '';
        this.promptAutocomplete.items = [];
        this.promptAutocomplete.activeIndex = 0;
    },


    /**
     * Return the list of template references in the prompt that don't
     * match any known OutputKey. We only support {key} and {key?} — any
     * other form (scoped prefixes, artifact refs) is flagged so the user
     * either declares the key or rewrites the reference.
     *
     * Mirrors ADK's placeholder regex (`{+[^{}]*}+`) in
     * .READ/adk-go/internal/llminternal/instruction_processor.go, but
     * applies our tighter identifier rule before flagging unknowns.
     */
    get unknownPromptKeys() {
        const text = this.form.systemPrompt || '';
        if (!text.includes('{')) return [];
        const matches = text.match(/\{+[^{}]*\}+/g) || [];
        const known = new Set((this.availableStateKeys || []).map(k => k.key));
        const unknown = [];
        const seen = new Set();
        for (const raw of matches) {
            let v = raw.replace(/^\{+|\}+$/g, '').trim();
            if (v.endsWith('?')) v = v.slice(0, -1);
            if (!v) continue;
            // Non-identifier strings are treated as literal text by ADK
            // (see isValidStateName) — no point flagging them as typos.
            if (!/^[a-zA-Z_][a-zA-Z0-9_]*$/.test(v)) continue;
            if (known.has(v)) continue;
            if (seen.has(v)) continue;
            seen.add(v);
            unknown.push(v);
        }
        return unknown;
    },

    // ══════════════════════════════════════════

    /** Open the templates/catalog view */
    openTemplatesView() {
        this.view = 'templates';
        this.catalogSearchQuery = '';
        this.catalogSelectedCategory = '';
        if (this.catalogEntries.length === 0) {
            this.fetchCatalog();
        }
    },

    /** Fetch the full catalog */
    async fetchCatalog() {
        this.catalogLoading = true;
        try {
            const response = await fetch('/v1/agent-catalog');
            if (response.ok) {
                const data = await response.json();
                this.catalogEntries = data.entries || [];
                this.catalogCategories = data.categories || [];
                this.catalogTotal = data.total || 0;
            }
        } catch (err) {
            console.error('Failed to fetch catalog:', err);
        } finally {
            this.catalogLoading = false;
        }
    },

    /** Search the catalog */
    async searchCatalog() {
        if (!this.catalogSearchQuery.trim()) {
            this.applyCatalogFilter();
            return;
        }
        try {
            const response = await fetch(`/v1/agent-catalog/search?q=${encodeURIComponent(this.catalogSearchQuery)}`);
            if (response.ok) {
                const data = await response.json();
                this.catalogEntries = data.entries || [];
                this.catalogTotal = data.total || 0;
            }
        } catch (err) {
            console.error('Failed to search catalog:', err);
            this.applyCatalogFilter();
        }
    },

    /** Filter catalog by category */
    applyCatalogFilter() {
        // For simplicity, re-fetch full catalog and client-side filter
        if (this.catalogEntries.length === 0) {
            this.fetchCatalog();
        }
    },

    /** Get filtered catalog entries */
    get filteredCatalogEntries() {
        let entries = this.catalogEntries;
        if (this.catalogSelectedCategory) {
            entries = entries.filter(e => e.category === this.catalogSelectedCategory);
        }
        return entries;
    },

    /** Open the fork name dialog for a catalog entry */
    promptForForkName(entry) {
        if (this.catalogAddingIds.includes(entry.id)) return;
        this.forkNameEntry = entry;
        this.forkNameInput = entry.name || '';
        this.showForkNameDialog = true;
    },

    /** Cancel the fork name dialog */
    cancelForkNameDialog() {
        this.showForkNameDialog = false;
        this.forkNameEntry = null;
        this.forkNameInput = '';
    },

    /** Confirm fork with the user-provided name */
    async confirmForkName() {
        const name = this.forkNameInput.trim();
        if (!name) {
            this.showToast('Agent name is required', 'error');
            return;
        }
        if (!this.forkNameEntry) return;

        const entry = this.forkNameEntry;
        this.forkNameAdding = true;
        this.showForkNameDialog = false;

        await this.addFromStore(entry, name);

        this.forkNameAdding = false;
        this.forkNameEntry = null;
        this.forkNameInput = '';
    },

    /** Fork a template from the catalog */
    async addFromStore(entry, customName) {
        if (this.catalogAddingIds.includes(entry.id)) return;
        this.catalogAddingIds.push(entry.id);
        try {
            const body = customName ? { name: customName } : {};
            const response = await fetch(`/v1/agent-catalog/${entry.id}/add`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: customName ? JSON.stringify(body) : undefined,
            });
            if (response.ok) {
                const data = await response.json();

                // Fetch the full agent definition directly using the returned ID
                const agentId = data.id;
                if (agentId) {
                    const fullAgent = await this.fetchAgentById(agentId);
                    console.log('[addFromStore] fetched agent:', fullAgent);
                    if (fullAgent) {
                        // Refresh agents list in background
                        this.fetchAgents();
                        window.dispatchEvent(new CustomEvent('agents-updated'));

                        if (fullAgent.isComposed || fullAgent.is_composed) {
                            this.showEditPipelineForm(fullAgent);
                        } else {
                            this.showEditNodeForm(fullAgent);
                        }
                        this.showToast('Template forked — editing now', 'success', 3000);
                        return;
                    }
                }
                // Fallback: go to list view
                this.view = 'list';
            } else {
                const errData = await response.text();
                console.error('Failed to fork template:', response.status, errData);
                this.showToast('Failed to fork template', 'error');
            }
        } catch (err) {
            console.error('Failed to add template:', err);
            this.showToast('Failed to add template', 'error');
        } finally {
            this.catalogAddingIds = this.catalogAddingIds.filter(id => id !== entry.id);
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
                let results = data.agents || [];
                // Apply type filter
                if (this.agentFilter === 'agent') {
                    results = results.filter(a => !a.isComposed);
                } else if (this.agentFilter === 'workflow') {
                    results = results.filter(a => a.isComposed);
                }
                this.filteredAgents = results;
            }
        } catch (err) {
            console.error('Failed to search agents:', err);
            this.applyFilter();
        }
    },

    applyFilter() {
        let results = [...this.allAgents];
        
        // Apply type filter
        if (this.agentFilter === 'agent') {
            results = results.filter(a => !a.isComposed);
        } else if (this.agentFilter === 'workflow') {
            results = results.filter(a => a.isComposed);
        }
        
        // Apply search filter
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
            subAgents: [],
            agentTools: [],
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
            memoryPolicy: null,
            artifactPolicy: null,
            codeExecutionPolicy: null,
            streamingPolicy: null,
        };
        this.showTools = false;
        this.showKnowledgeBases = false;
        this.showFilesystems = false;
        this.showSubAgents = false;
        this.showAgentTools = false;
        this.showCalledAsTool = false;
        this.showOutputSchema = false;
        this.showModelConfig = false;
        this.showRequirementsPanel = false;
        this.showGlobalInstruction = false;
        this.showMemory = false;
        this.showArtifacts = false;
        this.showCodeExecution = false;
        this.showStreaming = false;
        this.agentPlugins = [];
        this.agentWorkspaces = [];
        this.agentFilesystems = [];
        this.providerModelOptions = [];
        this.providerModelsError = '';
        this.hydrateSchemaRows('inputSchema');
        this.hydrateSchemaRows('outputSchema');
        this.closePromptAutocomplete();
        this.availableStateKeys = [];
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
            subAgents: agent.subAgents ? agent.subAgents.map(sa => ({...sa})) : [],
            agentTools: agent.agentTools ? agent.agentTools.map(at => ({...at})) : [],
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
            memoryPolicy: agent.memoryPolicy
                ? { enabled: !!agent.memoryPolicy.enabled, preload: !!agent.memoryPolicy.preload }
                : null,
            artifactPolicy: agent.artifactPolicy
                ? {
                    enabled: !!agent.artifactPolicy.enabled,
                    autoSaveUserBlobs: !!agent.artifactPolicy.autoSaveUserBlobs,
                }
                : null,
            codeExecutionPolicy: agent.codeExecutionPolicy
                ? { enabled: !!agent.codeExecutionPolicy.enabled }
                : null,
            streamingPolicy: agent.streamingPolicy
                ? { enabled: !!agent.streamingPolicy.enabled }
                : null,
        };
        this.showTools = false;
        this.showKnowledgeBases = false;
        this.showFilesystems = false;
        this.showSubAgents = false;
        this.showAgentTools = false;
        this.showCalledAsTool = false;
        this.showOutputSchema = false;
        this.showModelConfig = false;
        this.showRequirementsPanel = false;
        this.showGlobalInstruction = false;
        this.showMemory = false;
        this.showArtifacts = false;
        this.showCodeExecution = false;
        this.showStreaming = false;
        this.providerModelOptions = [];
        this.providerModelsError = '';
        this.hydrateSchemaRows('inputSchema');
        this.hydrateSchemaRows('outputSchema');
        this.closePromptAutocomplete();
        this.view = 'agentForm';

        // Load tool/KB/FS data
        this.fetchAgentPlugins(agent.id);
        this.fetchAgentWorkspaces(agent.id);
        this.fetchAgentFilesystems(agent.id);
        this.fetchAvailableStateKeys(agent.id);

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
                subAgents: this.form.subAgents.filter(sa => sa.agent_id && sa.agent_id.trim()),
                agentTools: (this.form.agentTools || [])
                    .filter(at => at.agent_id && at.agent_id.trim())
                    .map(at => ({
                        agent_id: at.agent_id,
                        skip_summarization: !!at.skip_summarization,
                    })),
                disallowTransferToParent: this.form.disallowTransferToParent || false,
                disallowTransferToPeers: this.form.disallowTransferToPeers || false,
                outputKey: this.form.outputKey.trim(),
                includeContents: this.form.includeContents || '',
                globalInstruction: this.form.globalInstruction.trim(),
                inputSchema: this.serializeSchema('inputSchema'),
                outputSchema: this.serializeSchema('outputSchema'),
                modelConfig: Object.keys(modelConfig).length > 0 ? modelConfig : null,
                modelOverride,
                memoryPolicy: this.form.memoryPolicy
                    ? {
                        enabled: !!this.form.memoryPolicy.enabled,
                        preload: !!this.form.memoryPolicy.preload,
                    }
                    : null,
                artifactPolicy: this.form.artifactPolicy
                    ? {
                        enabled: !!this.form.artifactPolicy.enabled,
                        autoSaveUserBlobs: !!this.form.artifactPolicy.autoSaveUserBlobs,
                    }
                    : null,
                codeExecutionPolicy: this.form.codeExecutionPolicy
                    ? { enabled: !!this.form.codeExecutionPolicy.enabled }
                    : null,
                streamingPolicy: this.form.streamingPolicy
                    ? { enabled: !!this.form.streamingPolicy.enabled }
                    : null,
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
                this.showToast(this.editingAgent ? 'Agent updated' : 'Agent created', 'success', 2500);
                window.dispatchEvent(new CustomEvent('agents-updated'));
            } else {
                const err = await response.json().catch(() => ({}));
                console.error('Failed to save agent:', err);
                this.showToast(err.error || 'Failed to save agent', 'error');
            }
        } catch (err) {
            console.error('Failed to save agent:', err);
            this.showToast('Failed to save agent', 'error');
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
                this.showToast('Agent deleted', 'success', 2500);
                window.dispatchEvent(new CustomEvent('agents-updated'));
            } else {
                const err = await response.json().catch(() => ({}));
                this.showToast(err.error || 'Failed to delete agent', 'error');
            }
        } catch (err) {
            console.error('Failed to delete agent:', err);
            this.showToast('Failed to delete agent', 'error');
        } finally {
            this.saving = false;
        }
    },

    confirmDeleteAll() {
        this.view = 'deleteAll';
    },

    async deleteAllAgents() {
        this.saving = true;
        try {
            const response = await fetch('/v1/agent-definitions', {
                method: 'DELETE',
            });
            if (response.ok) {
                const result = await response.json();
                await this.fetchAgents();
                this.view = 'list';
                if (result.failed === 0) {
                    this.showToast(`Deleted ${result.deleted} agents`, 'success', 3000);
                } else {
                    this.showToast(`Deleted ${result.deleted}, ${result.failed} failed`, 'error', 3000);
                }
                window.dispatchEvent(new CustomEvent('agents-updated'));
            } else {
                this.showToast('Failed to delete agents', 'error');
            }
        } catch (err) {
            console.error('Failed to delete all agents:', err);
            this.showToast('Failed to delete agents', 'error');
        } finally {
            this.saving = false;
        }
    },

    // ══════════════════════════════════════════
    // Tools, Knowledge Bases, Filesystems
    // ══════════════════════════════════════════

    get toolsSummary() {
        if (this.loadingTools) return '';
        const blocked = this.agentPlugins.filter(p => !this.isPluginEnabled(p)).length;
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

    // Sub-Agent Management
    get availableAgentsForSubAgent() {
        const currentId = this.editingAgent?.id;
        const selectedIds = new Set((this.form.subAgents || []).map(sa => sa.agent_id));
        return this.allAgents.filter(a => a.id !== currentId && !selectedIds.has(a.id));
    },
    openSubAgentDialog() {
        this.subAgentDialogOpen = true;
    },
    toggleSubAgentFromDialog(agentId) {
        if (!this.form.subAgents) this.form.subAgents = [];
        const idx = this.form.subAgents.findIndex(sa => sa.agent_id === agentId);
        if (idx > -1) {
            this.form.subAgents.splice(idx, 1);
        } else {
            this.form.subAgents.push({ agent_id: agentId, output_key: '' });
        }
    },
    isAgentSelected(agentId) {
        return (this.form.subAgents || []).some(sa => sa.agent_id === agentId);
    },
    removeSubAgent(index) {
        if (this.form.subAgents) this.form.subAgents.splice(index, 1);
    },
    getAgentName(agentId) {
        const agent = this.allAgents.find(a => a.id === agentId);
        return agent ? agent.name : 'Unknown Agent';
    },

    // Schema Row Builder — shared by Input Schema (M4) and Output Schema (M5).
    //
    // Renders form[<field>] as a list of {name, type, required, description}
    // rows the user edits without writing JSON. Serialized back to a JSON
    // Schema object at save time. Falls back to a raw JSON view when the
    // persisted schema is too complex to round-trip (nested objects, arrays of
    // objects, $ref, etc.).
    //
    // State is keyed by field so the two schemas are fully independent:
    //   this.inputSchemaMode / inputSchemaRows / inputSchemaJSONError
    //   this.outputSchemaMode / outputSchemaRows / outputSchemaJSONError
    //
    // Pass the field key ("inputSchema" | "outputSchema") to every helper; it
    // tells the helper which form field to read and which state slot to write.
    showCalledAsTool: false,
    showOutputSchema: false,
    inputSchemaMode: 'visual',
    inputSchemaRows: [],
    inputSchemaJSONError: '',
    outputSchemaMode: 'visual',
    outputSchemaRows: [],
    outputSchemaJSONError: '',

    /** Supported simple types in the visual builder — shared across fields. */
    get schemaTypeOptions() {
        return ['string', 'number', 'integer', 'boolean'];
    },

    /**
     * Parse form[field] (raw JSON) into editable rows. If the schema is
     * missing, empty, malformed, or uses features beyond flat scalar properties
     * (nested objects, arrays, enums with objects, $ref), we switch to JSON
     * mode so the user can edit the raw schema instead of silently losing data.
     */
    hydrateSchemaRows(field) {
        this[field + 'JSONError'] = '';
        const raw = (this.form[field] || '').trim();
        if (!raw) {
            this[field + 'Rows'] = [];
            this[field + 'Mode'] = 'visual';
            return;
        }
        let parsed;
        try {
            parsed = JSON.parse(raw);
        } catch (e) {
            this[field + 'Mode'] = 'json';
            this[field + 'JSONError'] = e.message;
            return;
        }
        const simple = this.schemaToRows(parsed);
        if (simple === null) {
            this[field + 'Mode'] = 'json';
            return;
        }
        this[field + 'Rows'] = simple;
        this[field + 'Mode'] = 'visual';
    },

    /**
     * Convert a JSON Schema object to flat rows.
     * Returns null if the schema can't be represented as flat scalar rows.
     */
    schemaToRows(schema) {
        if (!schema || typeof schema !== 'object') return null;
        if (schema.type !== 'object' || !schema.properties || typeof schema.properties !== 'object') return null;
        const required = Array.isArray(schema.required) ? schema.required : [];
        const rows = [];
        for (const [name, propRaw] of Object.entries(schema.properties)) {
            if (!propRaw || typeof propRaw !== 'object') return null;
            const t = propRaw.type;
            if (!this.schemaTypeOptions.includes(t)) return null;
            rows.push({
                name,
                type: t,
                required: required.includes(name),
                description: typeof propRaw.description === 'string' ? propRaw.description : '',
            });
        }
        return rows;
    },

    addSchemaRow(field) {
        this[field + 'Rows'].push({ name: '', type: 'string', required: false, description: '' });
    },

    removeSchemaRow(field, idx) {
        this[field + 'Rows'].splice(idx, 1);
    },

    switchSchemaToJSON(field) {
        // Serialize current rows into the JSON textarea, then let the user edit freely.
        this.form[field] = this.serializeSchema(field);
        this[field + 'Mode'] = 'json';
    },

    switchSchemaToVisual(field) {
        // Try to re-parse whatever is in the textarea. If it's too complex, stay in JSON mode.
        this.hydrateSchemaRows(field);
    },

    /**
     * Build a JSON Schema string from either the visual rows or the raw JSON,
     * depending on the current mode. Empty result → "" (persisted as NULL).
     */
    serializeSchema(field) {
        if (this[field + 'Mode'] === 'json') {
            return (this.form[field] || '').trim();
        }
        const rows = (this[field + 'Rows'] || []).filter(r => r.name && r.name.trim());
        if (rows.length === 0) return '';
        const properties = {};
        const required = [];
        for (const r of rows) {
            const prop = { type: r.type };
            if (r.description && r.description.trim()) prop.description = r.description.trim();
            properties[r.name.trim()] = prop;
            if (r.required) required.push(r.name.trim());
        }
        const schema = { type: 'object', properties };
        if (required.length > 0) schema.required = required;
        return JSON.stringify(schema);
    },

    // Agent-as-Tool Management
    // Orthogonal to SubAgents: agent-tools are callable children whose
    // output is returned to the parent LLM, not delegated-to peers.
    showAgentTools: false,
    agentToolsDialogOpen: false,

    get availableAgentsForAgentTool() {
        const currentId = this.editingAgent?.id;
        const selectedIds = new Set((this.form.agentTools || []).map(at => at.agent_id));
        return this.allAgents.filter(a => a.id !== currentId && !selectedIds.has(a.id));
    },
    openAgentToolsDialog() {
        this.agentToolsDialogOpen = true;
    },
    isAgentToolSelected(agentId) {
        return (this.form.agentTools || []).some(at => at.agent_id === agentId);
    },
    toggleAgentToolFromDialog(agentId) {
        if (!this.form.agentTools) this.form.agentTools = [];
        const idx = this.form.agentTools.findIndex(at => at.agent_id === agentId);
        if (idx > -1) {
            this.form.agentTools.splice(idx, 1);
        } else {
            this.form.agentTools.push({ agent_id: agentId, skip_summarization: false });
        }
    },
    removeAgentTool(index) {
        if (this.form.agentTools) this.form.agentTools.splice(index, 1);
    },
    toggleAgentToolSkipSummarization(index) {
        if (!this.form.agentTools || !this.form.agentTools[index]) return;
        this.form.agentTools[index].skip_summarization = !this.form.agentTools[index].skip_summarization;
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
        this.showGlobalInstruction = false;
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
                this.showToast(err.error || 'Failed to save requirement config', 'error');
            }
        } catch (err) {
            console.error('Failed to save requirement config:', err);
            this.showToast('Failed to save requirement config', 'error');
        } finally {
            this.savingReqConfig = false;
        }
    },

    // ══════════════════════════════════════════
    // Pipeline Compose Mode — CRUD
    // ══════════════════════════════════════════

    showImportYamlView() {
        this.importYamlText = '';
        this.importYamlURL = '';
        this.view = 'importYaml';
    },

    onImportYamlFile(event) {
        const input = event.target;
        const file = input.files && input.files[0];
        if (!file) return;
        const reader = new FileReader();
        reader.onload = () => {
            this.importYamlText = String(reader.result || '');
        };
        reader.readAsText(file);
        input.value = '';
    },

    async submitImportYaml() {
        if (this.importYamlLoading) return;
        const urlTrim = this.importYamlURL.trim();
        const yamlTrim = this.importYamlText.trim();
        if (urlTrim && yamlTrim) {
            this.showToast('Provide either URL or YAML, not both', 'error');
            return;
        }
        if (!urlTrim && !yamlTrim) {
            this.showToast('Paste YAML, choose a file, or enter a URL', 'error');
            return;
        }
        this.importYamlLoading = true;
        try {
            const body = urlTrim ? { url: urlTrim } : { yaml: this.importYamlText };
            const response = await fetch('/v1/agent-definitions/import-yaml', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body),
                credentials: 'same-origin',
            });
            const data = await response.json().catch(() => ({}));
            if (!response.ok) {
                this.showToast(data.error || 'Import failed', 'error');
                return;
            }
            await this.fetchAgents();
            const agent = await this.fetchAgentById(data.id);
            if (!agent) {
                this.showToast('Imported but could not load agent', 'error');
                this.view = 'list';
                return;
            }
            this.showToast('Imported successfully', 'success', 2500);
            if (data.isComposed) {
                this.showEditPipelineForm(agent);
            } else {
                this.showEditNodeForm(agent);
            }
        } catch (err) {
            console.error('YAML import failed:', err);
            this.showToast('Import failed', 'error');
        } finally {
            this.importYamlLoading = false;
        }
    },

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
        // Fetch consumers (workflows that use this workflow as a sub-agent)
        this.fetchConsumers(pipeline.id);
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
                this.showToast(this.editingPipeline ? 'Workflow updated' : 'Workflow created', 'success', 2500);
                window.dispatchEvent(new CustomEvent('agents-updated'));
            } else {
                const err = await response.json().catch(() => ({}));
                this.showToast(err.error || 'Failed to save pipeline', 'error');
            }
        } catch (err) {
            console.error('Failed to save pipeline:', err);
            this.showToast('Failed to save pipeline', 'error');
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
                this.showToast(err.error || 'Failed to delete pipeline', 'error');
            }
        } catch (err) {
            console.error('Failed to delete pipeline:', err);
            this.showToast('Failed to delete pipeline', 'error');
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
            errors.push('Workflow requires a name.');
        }

        // Root must be composite
        const rootType = this.pipelineGraph.type;
        if (rootType === 'agent' || rootType === 'remote') {
            errors.push('The root must be Sequential, Parallel, or Loop. Use "Single Agent" for a standalone agent.');
        }

        const walk = (node, path, depth) => {
            if (!node) return;
            const isComposite = node.type === 'sequential' || node.type === 'parallel' || node.type === 'loop';

            if (node.type === 'agent') {
                if (!node.agent_id) {
                    errors.push(`${path}: Please select an agent for this node.`);
                }
            } else if (node.type === 'remote') {
                if (!node.a2a_connector_id) {
                    errors.push(`${path}: Please select a remote connector for this node.`);
                }
            } else if (isComposite) {
                if (!node.children || node.children.length === 0) {
                    errors.push(`${path}: This ${node.type} node needs at least one agent inside it.`);
                }
                if (node.type === 'loop' && (!node.max_iterations || node.max_iterations < 1)) {
                    errors.push(`${path}: Loop must repeat at least 1 time.`);
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


    get currentAgentId() {
        if (this.editingPipeline) return this.editingPipeline.id;
        if (this.editingAgent) return this.editingAgent.id;
        return null;
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
