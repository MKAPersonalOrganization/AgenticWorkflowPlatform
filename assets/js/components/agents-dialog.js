/**
 * Agents Dialog Component
 * Handles agent selection and details logic
 */
export const agentsDialog = () => ({
    agents: [],
    loading: false,
    selectedAgent: '',
    toolsets: [],
    workspaces: [],
    loadingDetails: false,

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
        if (this.selectedAgent) {
            await this.fetchAgentDetails();
        }
    },

    async fetchAgents() {
        this.loading = true;
        try {
            const response = await fetch('/v1/agents');
            if (response.ok) {
                const data = await response.json();
                if (data.agents && Array.isArray(data.agents)) {
                    this.agents = data.agents;
                }
            }
        } catch (err) {
            console.error('Failed to fetch agents:', err);
        } finally {
            this.loading = false;
        }
    },

    async fetchAgentDetails() {
        if (!this.selectedAgent) {
            this.toolsets = [];
            this.workspaces = [];
            return;
        }
        this.loadingDetails = true;
        try {
            const [toolsRes, workspacesRes] = await Promise.all([
                fetch(`/v1/agents/${this.selectedAgent}/toolsets`),
                fetch(`/v1/agents/${this.selectedAgent}/workspaces`)
            ]);
            if (toolsRes.ok) {
                const data = await toolsRes.json();
                this.toolsets = (data.toolsets || []).filter(ts => ts.enabled);
            } else {
                this.toolsets = [];
            }
            if (workspacesRes.ok) {
                const data = await workspacesRes.json();
                this.workspaces = (data.workspaces || []).filter(w => w.enabled);
            } else {
                this.workspaces = [];
            }
        } catch (err) {
            console.error('Failed to fetch agent details:', err);
            this.toolsets = [];
            this.workspaces = [];
        } finally {
            this.loadingDetails = false;
        }
    },

    selectAgent(id) {
        this.selectedAgent = id;
        window.dispatchEvent(new CustomEvent('agent-selected', {
            detail: {
                id: id
            }
        }));
        this.fetchAgentDetails();
    },

    get selectedAgentName() {
        const a = this.agents.find(a => a.id === this.selectedAgent);
        return a ? a.name : '';
    },

    get enabledToolCount() {
        let count = 0;
        this.toolsets.forEach(ts => {
            count += (ts.tools || []).filter(t => t.enabled).length;
        });
        return count;
    },

    toolSetCategoryColor(category) {
        const colors = {
            'Calculator': 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400',
            'Visualization': 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-400',
            'Documents': 'bg-green-100 text-green-800 dark:bg-green-900/30 dark:text-green-400',
            'Memory': 'bg-amber-100 text-amber-800 dark:bg-amber-900/30 dark:text-amber-400',
        };
        return colors[category] || 'bg-gray-100 text-gray-800 dark:bg-gray-900/30 dark:text-gray-400';
    },

    toolSetBorderColor(category) {
        const colors = {
            'Calculator': 'border-l-blue-400 dark:border-l-blue-500',
            'Visualization': 'border-l-purple-400 dark:border-l-purple-500',
            'Documents': 'border-l-green-400 dark:border-l-green-500',
            'Memory': 'border-l-amber-400 dark:border-l-amber-500',
        };
        return colors[category] || 'border-l-gray-400 dark:border-l-gray-500';
    }
});
