/**
 * Agents Manager Component
 * Handles agent CRUD operations, search, and selection
 */
export const agentsManager = () => ({
    // State
    view: 'list', // 'list' | 'form' | 'delete'
    agents: [],
    filteredAgents: [],
    loading: false,
    saving: false,
    searchQuery: '',

    // Form state
    editingAgent: null, // null = create mode, object = edit mode
    form: {
        name: '',
        description: '',
        systemPrompt: '',
        modelId: '',
        temperature: 0.7,
        maxIterations: 10,
    },
    showAdvanced: false,

    // Delete state
    deletingAgent: null,

    async open() {
        this.view = 'list';
        this.searchQuery = '';
        await this.fetchAgents();
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
        if (!this.searchQuery.trim()) {
            this.filteredAgents = [...this.agents];
        } else {
            const q = this.searchQuery.toLowerCase();
            this.filteredAgents = this.agents.filter(a =>
                a.name.toLowerCase().includes(q) ||
                (a.description && a.description.toLowerCase().includes(q))
            );
        }
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
            modelId: '',
            temperature: 0.7,
            maxIterations: 10,
        };
        this.showAdvanced = false;
        this.view = 'form';
    },

    showEditForm(agent) {
        this.editingAgent = agent;
        this.form = {
            name: agent.name || '',
            description: agent.description || '',
            systemPrompt: agent.systemPrompt || '',
            modelId: agent.modelId || '',
            temperature: agent.temperature ?? 0.7,
            maxIterations: agent.maxIterations ?? 10,
        };
        this.showAdvanced = !!(agent.modelId || (agent.temperature != null && agent.temperature !== 0.7) || (agent.maxIterations != null && agent.maxIterations !== 10));
        this.view = 'form';
    },

    async saveAgent() {
        if (!this.form.name.trim()) return;
        this.saving = true;
        try {
            const payload = {
                name: this.form.name.trim(),
                description: this.form.description.trim(),
                systemPrompt: this.form.systemPrompt.trim(),
                modelId: this.form.modelId.trim(),
                temperature: parseFloat(this.form.temperature),
                maxIterations: parseInt(this.form.maxIterations, 10) || 10,
            };

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
                this.deletingAgent = null;
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
        window.dispatchEvent(new CustomEvent('agent-selected', {
            detail: { id: agentId }
        }));
        // Close the dialog using TUI dialog API
        if (window.tui?.dialog?.close) {
            window.tui.dialog.close('agents-manager-dialog');
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
