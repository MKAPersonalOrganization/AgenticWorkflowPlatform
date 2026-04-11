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
import { trackWorkflowEvent, trackAgentMessage, appendAgentMessageContent, finishAgentMessage } from '../utils/workflow-tracker.js';

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
    showSubAgents: false,
    showModelSettings: false,
    showModelOverride: false,

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
    pipelineConsumers: [],

    // A2A connectors (for remote nodes)
    connectors: [],

    // Canvas interaction
    selectedNodePath: null,
    dragState: null,

    // Sidebar visibility (for property panel)
    propertyPanelVisible: true,

    // ══════════════════════════════════════════
    // Test Panel State
    // ══════════════════════════════════════════

    testWorkflowState: {
        activeAgentNames: {},
        completedNodes: {},
        failedNodes: {},
        agentMessages: {}, // { [agentName]: [{ id, role, content, streaming, timestamp }] }
    },

    /** Which agent's contribution panel is expanded (null = none) */
    testPanelExpandedAgent: null,

    /** Whether the amber play button is active and the panel is visible */
    testModeActive: false,

    /** Whether the panel is collapsed to its 40px header bar */
    testPanelCollapsed: false,

    /**
     * Messages rendered in the test panel chat.
     * Each entry: { id, role: 'user'|'assistant'|'error', content, streaming, timestamp }
     */
    testPanelMessages: [],

    /** True while a fetch+SSE stream is in-flight */
    testPanelStreaming: false,

    /** True while waiting for the first token (shows typing indicator) */
    testPanelTyping: false,

    /** Bound to the chat input textarea */
    testPanelInput: '',

    /**
     * Ephemeral thread ID for this test session.
     * Generated on first send via crypto.randomUUID(); cleared when test mode is toggled off.
     */
    testPanelThreadId: null,

    /** Whether the event log side pane is visible */
    testPanelShowEventLog: false,

    /**
     * Raw SSE events logged for the event log side pane.
     * Each entry: { type, name? }
     */
    testPanelEvents: [],

    /** AbortController for the in-flight fetch — allows cancelling mid-stream */
    testPanelAbortController: null,

    // ══════════════════════════════════════════
    // Lifecycle
    // ══════════════════════════════════════════

    init() {
        // Listen for toggle agent contribution events from workflow tree
        window.addEventListener('test-panel-toggle-agent', (e) => {
            this.toggleAgentContribution(e.detail.agentName);
        });
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
    // Test Panel — Toggle & Utility Methods
    // ══════════════════════════════════════════

    /**
     * Toggle test mode on/off.
     * Turning off aborts any in-flight stream and clears all test state.
     */
    toggleTestMode() {
        if (this.testModeActive) {
            // Abort any active stream before clearing
            if (this.testPanelAbortController) {
                this.testPanelAbortController.abort();
                this.testPanelAbortController = null;
            }
            this.testModeActive = false;
            this.testPanelCollapsed = false;
            this.testWorkflowState = {
                activeAgentNames: {},
                completedNodes: {},
                failedNodes: {},
                agentMessages: {},
                userMessages: [],
                activeWorkflowContext: null
            };
            this.testPanelExpandedAgent = null;
            this.testPanelStreaming = false;
            this.testPanelTyping = false;
            this.testPanelInput = '';
            this.testPanelThreadId = null;
            this.testPanelShowEventLog = false;
            this.testPanelEvents = [];
        } else {
            this.testModeActive = true;
            this.testPanelCollapsed = false;
        }
    },

    /** Collapse the panel to its 40px header bar, or expand it back to 280px. */
    toggleTestPanel() {
        this.testPanelCollapsed = !this.testPanelCollapsed;
    },

    /** Show or hide the raw SSE event log side pane. */
    toggleTestEventLog() {
        this.testPanelShowEventLog = !this.testPanelShowEventLog;
    },

    /** Toggle the contribution panel for a specific agent */
    toggleAgentContribution(agentName) {
        if (this.testPanelExpandedAgent === agentName) {
            this.testPanelExpandedAgent = null;
        } else {
            this.testPanelExpandedAgent = agentName;
        }
    },

    /**
     * Clear all messages, events and the ephemeral thread ID.
     * Starts a fresh test session on the next send.
     */
    clearTestSession() {
        if (this.testPanelAbortController) {
            this.testPanelAbortController.abort();
            this.testPanelAbortController = null;
        }
        this.testPanelMessages = [];
        this.testPanelEvents = [];
        this.testPanelThreadId = null;
        this.testPanelStreaming = false;
        this.testPanelTyping = false;
        this.testPanelInput = '';
    },

    /**
     * Handle keydown in the test panel input.
     * Sends on Enter; Shift+Enter inserts a newline (default).
     * Stops streaming on Escape.
     * @param {KeyboardEvent} event
     */
    handleTestKeydown(event) {
        if (event.key === 'Enter' && !event.shiftKey) {
            event.preventDefault();
            if (this.testPanelStreaming) {
                this.abortTestMessage();
            } else {
                this.sendTestMessage();
            }
        } else if (event.key === 'Escape' && this.testPanelStreaming) {
            event.preventDefault();
            this.abortTestMessage();
        }
    },

    /**
     * Abort the currently streaming conversation.
     */
    abortTestMessage() {
        if (this.testPanelAbortController) {
            this.testPanelAbortController.abort();
            this.testPanelAbortController = null;
        }
        this.testPanelStreaming = false;
        this.testPanelTyping = false;
    },

    // ══════════════════════════════════════════
    // Test Panel — Send Message
    // ══════════════════════════════════════════

    /**
     * Send a message from the test panel to the current agent.
     * Pushes a user bubble immediately, then starts the SSE stream.
     */
    sendTestMessage() {
        const message = this.testPanelInput.trim();
        if (!message || this.testPanelStreaming) return;

        // Resolve the target agent ID — workflow takes priority over single agent
        const agentName = this.editingPipeline?.id || this.editingAgent?.id || '';
        if (!agentName) {
            console.warn('[TestPanel] No agent selected — cannot send message');
            return;
        }

        // Generate a thread ID for this test session if we don't have one yet
        if (!this.testPanelThreadId) {
            this.testPanelThreadId = crypto.randomUUID();
        }

        // Reset the tracker state for the new turn and update canvas
        this.testWorkflowState = {
            activeAgentNames: {},
            completedNodes: {},
            failedNodes: {},
            agentMessages: {},
            userMessages: [],
            activeWorkflowContext: null
        };
        this.testPanelExpandedAgent = null;
        window.dispatchEvent(new CustomEvent('test-panel-workflow-tracking', { detail: { active: {}, completed: {}, failed: {} }}));
        window.dispatchEvent(new CustomEvent('test-panel-contributions', { detail: { agentMessages: {}, userMessages: [] }}));

        // Store user message in workflow state for contributions display
        this.testWorkflowState.userMessages.push({
            id: crypto.randomUUID(),
            role: 'user',
            content: message,
            timestamp: new Date().toISOString(),
        });
        window.dispatchEvent(new CustomEvent('test-panel-contributions', { detail: { 
            agentMessages: this.testWorkflowState.agentMessages,
            userMessages: this.testWorkflowState.userMessages 
        }}));

        // Clear the input
        this.testPanelInput = '';

        // Start streaming
        this.__testStreamResponse(message, agentName);
    },

    // ══════════════════════════════════════════
    // Test Panel — Streaming (Steps 4 & 5)
    // ══════════════════════════════════════════

    /**
     * POST to /v1/message:send and hand the response body to the SSE processor.
     * @param {string} message   — the text the user typed
     * @param {string} agentName — the agent/workflow ID to target
     */
    async __testStreamResponse(message, agentName) {
        this.testPanelStreaming = true;
        this.testPanelTyping   = true;

        const controller = new AbortController();
        this.testPanelAbortController = controller;

        try {
            const body = {
                contextId: this.testPanelThreadId,
                message,
                agentName,
            };

            const response = await fetch('/v1/message:send', {
                method:  'POST',
                headers: { 'Content-Type': 'application/json' },
                body:    JSON.stringify(body),
                signal:  controller.signal,
            });

            if (!response.ok) {
                const errText = await response.text();
                throw new Error(`Server error ${response.status}: ${errText}`);
            }

            const contentType = response.headers.get('content-type') || '';
            if (!contentType.includes('text/event-stream')) {
                throw new Error('Unexpected response type: ' + contentType);
            }

            this.__testProcessSSEStream(response.body);

        } catch (error) {
            if (error.name === 'AbortError') return; // intentional cancel — no error bubble
            console.error('[TestPanel] Stream error:', error);
            this.testPanelMessages.push({
                id:        crypto.randomUUID(),
                role:      'error',
                content:   error.message || 'An error occurred',
                streaming: false,
                timestamp: new Date().toISOString(),
            });
            this.testPanelStreaming = false;
            this.testPanelTyping   = false;
        }
    },

    /**
     * Read a ReadableStream as SSE, buffering partial lines.
     * Dispatches each complete event to __testHandleEvent().
     * @param {ReadableStream} stream
     */
    __testProcessSSEStream(stream) {
        const reader  = stream.getReader();
        const decoder = new TextDecoder('utf-8');
        let   buffer  = '';

        const pump = () => {
            reader.read().then(({ done, value }) => {
                if (done) {
                    this.testPanelStreaming = false;
                    this.testPanelTyping   = false;
                    return;
                }

                buffer += decoder.decode(value, { stream: true });
                const lines = buffer.split('\n');
                buffer = lines.pop() || ''; // keep incomplete last line

                for (const line of lines) {
                    if (!line.startsWith('data:')) continue;
                    const data = line.slice(5).trim();
                    if (!data) continue;
                    try {
                        const event = JSON.parse(data);
                        this.__testHandleEvent(event);
                    } catch (e) {
                        console.warn('[TestPanel] SSE parse error:', e);
                    }
                }

                pump(); // read next chunk
            }).catch(err => {
                if (err.name === 'AbortError') return;
                console.error('[TestPanel] Stream read error:', err);
                this.testPanelMessages.push({
                    id:        crypto.randomUUID(),
                    role:      'error',
                    content:   'Stream interrupted: ' + err.message,
                    streaming: false,
                    timestamp: new Date().toISOString(),
                });
                this.testPanelStreaming = false;
                this.testPanelTyping   = false;
            });
        };

        pump();
    },

    /**
     * Handle a single parsed SSE event object.
     * Updates messages (streaming bubbles) and the event log.
     * @param {Object} event
     */
    __testHandleEvent(event) {
        if (!event?.type) return;

        // Always log to the event log side pane
        this.testPanelEvents.push({ type: event.type, name: event.name || null });

        // Handle CUSTOM events that contain workflow tracking info
        if (event.type === 'CUSTOM') {
            const eventName = event.name;
            const value = event.value || {};
            
            if (eventName === 'AGENT_TRANSFER_STARTED') {
                trackWorkflowEvent({ type: 'AGENT_TRANSFER_STARTED', ...value }, this.testWorkflowState);
                window.dispatchEvent(new CustomEvent('test-panel-workflow-tracking', { detail: {
                    active: this.testWorkflowState.activeAgentNames || {},
                    completed: this.testWorkflowState.completedNodes || {},
                    failed: this.testWorkflowState.failedNodes || {},
                }}));
                return;
            }
            
            if (eventName === 'WORKFLOW_NODE_COMPLETED') {
                trackWorkflowEvent({ type: 'WORKFLOW_NODE_COMPLETED', ...value }, this.testWorkflowState);
                window.dispatchEvent(new CustomEvent('test-panel-workflow-tracking', { detail: {
                    active: this.testWorkflowState.activeAgentNames || {},
                    completed: this.testWorkflowState.completedNodes || {},
                    failed: this.testWorkflowState.failedNodes || {},
                }}));
                return;
            }
        }

        switch (event.type) {
            case 'TEXT_MESSAGE_START': {
                // Start a new streaming message for the currently active agent
                const agentName = this.testWorkflowState.activeAgentName || 'unknown';
                this.testPanelTyping = false;
                trackAgentMessage(this.testWorkflowState, agentName, {
                    id:        event.messageId || crypto.randomUUID(),
                    role:      'assistant',
                    content:   '',
                    streaming: true,
                    timestamp: new Date().toISOString(),
                });
                // Auto-expand the agent's contribution panel when they start responding
                this.testPanelExpandedAgent = agentName;
                window.dispatchEvent(new CustomEvent('test-panel-contributions', { detail: { 
                    agentMessages: this.testWorkflowState.agentMessages,
                    userMessages: this.testWorkflowState.userMessages
                }}));
                break;
            }

            case 'TEXT_MESSAGE_CONTENT': {
                // Append token delta to the last message of the active agent
                const agentName = this.testWorkflowState.activeAgentName || 'unknown';
                appendAgentMessageContent(this.testWorkflowState, agentName, event.delta || '');
                window.dispatchEvent(new CustomEvent('test-panel-contributions', { detail: { 
                    agentMessages: this.testWorkflowState.agentMessages,
                    userMessages: this.testWorkflowState.userMessages
                }}));
                break;
            }

            case 'TEXT_MESSAGE_END': {
                // Mark the last message of the active agent as complete
                const agentName = this.testWorkflowState.activeAgentName || 'unknown';
                finishAgentMessage(this.testWorkflowState, agentName);
                window.dispatchEvent(new CustomEvent('test-panel-contributions', { detail: { 
                    agentMessages: this.testWorkflowState.agentMessages,
                    userMessages: this.testWorkflowState.userMessages
                }}));
                break;
            }

            case 'RUN_FINISHED': {
                this.testPanelStreaming = false;
                this.testPanelTyping   = false;
                break;
            }

            case 'RUN_ERROR': {
                const errMsg = event.message || event.error || 'Agent run failed';
                const agentName = this.testWorkflowState.activeAgentName || 'unknown';
                trackAgentMessage(this.testWorkflowState, agentName, {
                    id:        crypto.randomUUID(),
                    role:      'error',
                    content:   errMsg,
                    streaming: false,
                    timestamp: new Date().toISOString(),
                });
                window.dispatchEvent(new CustomEvent('test-panel-contributions', { detail: { 
                    agentMessages: this.testWorkflowState.agentMessages,
                    userMessages: this.testWorkflowState.userMessages
                }}));
                this.testPanelStreaming = false;
                this.testPanelTyping   = false;
                break;
            }

            // All other events (STEP_STARTED, TOOL_*, WORKFLOW_NODE_*, etc.)
            // are already captured in testPanelEvents above — no bubble needed.
            default:
                break;
        }
    },

    // ══════════════════════════════════════════
    // Data Fetching
    // ══════════════════════════════════════════

    async fetchAgentById(agentId) {
        try {
            const response = await fetch(`/v1/agent-definitions/${encodeURIComponent(agentId)}`);
            if (response.ok) {
                return await response.json();
            }
        } catch (err) {
            console.error('Failed to fetch agent:', err);
        }
        return null;
    },

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

    // ══════════════════════════════════════════
    // Templates / Catalog
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
        this.showSubAgents = false;
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
            subAgents: agent.subAgents ? agent.subAgents.map(sa => ({...sa})) : [],
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
        this.showSubAgents = false;
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
                subAgents: this.form.subAgents.filter(sa => sa.agent_id && sa.agent_id.trim()),
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
