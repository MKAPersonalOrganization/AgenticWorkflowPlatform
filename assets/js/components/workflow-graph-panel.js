export function workflowGraphPanel() {
    return {
        graph: null,
        agentsList: [],
        isLoading: false,

        async fetchInitialAgents() {
            try {
                const res = await fetch('/v1/agents');
                if (res.ok) {
                    const data = await res.json();
                    this.agentsList = data.agents || [];
                }
            } catch (err) {
                console.error("Failed to fetch agents for workflow graph panel:", err);
            }
        },

        async handleWorkflowChange(agentIdentifier) {
            if (!agentIdentifier) {
                this.graph = null;
                return;
            }
            if (!this.agentsList || this.agentsList.length === 0) {
                await this.fetchInitialAgents();
            }
            await this.loadGraph(agentIdentifier);
        },

        async loadGraph(agentIdentifier) {
            this.isLoading = true;
            this.graph = null; // show loader state clearly
            try {
                let targetId = agentIdentifier;
                const agent = this.agentsList.find(a => a.name === agentIdentifier || a.id === agentIdentifier);
                if (agent) {
                    targetId = agent.id;
                    if (agent.graph) {
                        this.graph = agent.graph;
                        return;
                    }
                }

                if (!targetId) return;

                const res = await fetch('/v1/agents/' + encodeURIComponent(targetId));
                if (res.ok) {
                    const agentDetails = await res.json();
                    this.graph = agentDetails.graph || null;
                }
            } catch (err) {
                console.error("Failed to load workflow graph for agent:", agentIdentifier, err);
                this.graph = null;
            } finally {
                this.isLoading = false;
            }
        }
    };
}
