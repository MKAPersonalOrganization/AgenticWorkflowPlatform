/**
 * Evaluation Dialog
 * Alpine.js component for running evaluations and viewing results.
 */
export const evaluationDialog = () => ({
    // State
    view: 'runs',            // 'runs' | 'results' | 'rubrics' | 'launch'
    loading: false,
    agentsLoading: false,
    evalSetsLoading: false,
    showLaunchForm: false,
    showRubricForm: false,
    launching: false,
    creatingRubric: false,
    evalSetId: '',           // Current EvalSet context (set from parent)
    runs: [],
    evalSets: [],
    results: [],
    rubrics: [],
    agents: [],
    profiles: [],

    // Forms
    launchForm: {
        agent_id: '',
        profile_id: '',
        num_runs: 1,
    },
    rubricForm: {
        name: '',
        description: '',
        levels: [
            { score: 1, description: '' },
            { score: 3, description: '' },
            { score: 5, description: '' },
        ],
    },

    async open() {
        this.view = 'runs';
        this.showLaunchForm = false;
        this.showRubricForm = false;
        await this.loadRuns();
    },

    async loadRuns() {
        this.loading = true;
        try {
            const resp = await fetch('/api/evaluation/runs', { credentials: 'same-origin' });
            if (resp.ok) {
                this.runs = await resp.json();
            }
        } catch (err) {
            console.error('Failed to load evaluation runs:', err);
        } finally {
            this.loading = false;
        }
    },

    async loadAgents() {
        this.agentsLoading = true;
        try {
            const resp = await fetch('/v1/agent-definitions', { credentials: 'same-origin' });
            if (resp.ok) {
                const data = await resp.json();
                // Only show user's own agents, not catalog templates
                this.agents = (data.agents || []).filter(a => !a.sourceCatalogId);
            }
        } catch (err) {
            console.error('Failed to load agents:', err);
        } finally {
            this.agentsLoading = false;
        }
    },

    async loadEvalSets() {
        this.evalSetsLoading = true;
        try {
            const resp = await fetch('/api/eval-sets', { credentials: 'same-origin' });
            if (resp.ok) {
                this.evalSets = await resp.json();
            }
        } catch (err) {
            console.error('Failed to load eval sets:', err);
        } finally {
            this.evalSetsLoading = false;
        }
    },

    async loadProfiles() {
        try {
            const resp = await fetch('/api/evaluation/profiles', { credentials: 'same-origin' });
            if (resp.ok) {
                this.profiles = await resp.json();
            }
        } catch (err) {
            console.error('Failed to load profiles:', err);
        }
    },

    async launchEvaluation() {
        if (!this.launchForm.eval_set_id) {
            this.showToast('Please select an EvalSet', 'error');
            return;
        }
        if (!this.launchForm.agent_id) {
            this.showToast('Please select an agent', 'error');
            return;
        }

        this.launching = true;
        try {
            const body = {
                eval_set_id: this.launchForm.eval_set_id,
                agent_id: this.launchForm.agent_id,
                num_runs: this.launchForm.num_runs || 1,
            };
            if (this.launchForm.profile_id) {
                body.profile_id = this.launchForm.profile_id;
            }

            const resp = await fetch('/api/evaluation/runs', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body),
                credentials: 'same-origin',
            });

            if (resp.ok || resp.status === 201) {
                const agentName = this.agents.find(a => a.id === this.launchForm.agent_id)?.name || 'Agent';
                this.showLaunchForm = false;
                this.launchForm = { eval_set_id: '', agent_id: '', profile_id: '', num_runs: 1 };
                await this.loadRuns();
                this.showToast('Evaluation launched for ' + agentName, 'success');
            } else {
                const data = await resp.json().catch(() => ({}));
                this.showToast(data.message || 'Failed to launch evaluation', 'error');
            }
        } catch (err) {
            this.showToast('Network error', 'error');
            console.error('Failed to launch evaluation:', err);
        } finally {
            this.launching = false;
        }
    },

    async viewResults(run) {
        try {
            const resp = await fetch(`/api/evaluation/runs/${run.id}/results`, { credentials: 'same-origin' });
            if (resp.ok) {
                this.results = await resp.json();
                this.view = 'results';
            }
        } catch (err) {
            this.showToast('Failed to load results', 'error');
            console.error('Failed to load results:', err);
        }
    },

    async deleteRun(run) {
        if (!confirm(`Delete evaluation run? This cannot be undone.`)) return;
        try {
            const resp = await fetch(`/api/evaluation/runs/${run.id}`, {
                method: 'DELETE',
                credentials: 'same-origin',
            });
            if (resp.ok || resp.status === 204) {
                await this.loadRuns();
                this.showToast('Run deleted', 'success');
            }
        } catch (err) {
            this.showToast('Failed to delete run', 'error');
        }
    },

    async loadRubrics() {
        try {
            const resp = await fetch('/api/evaluation/rubrics', { credentials: 'same-origin' });
            if (resp.ok) {
                this.rubrics = await resp.json();
            }
        } catch (err) {
            console.error('Failed to load rubrics:', err);
        }
    },

    async createRubric() {
        if (!this.rubricForm.name.trim()) return;
        if (this.rubricForm.levels.length < 3 || this.rubricForm.levels.length > 5) return;

        this.creatingRubric = true;
        try {
            const body = {
                name: this.rubricForm.name,
                description: this.rubricForm.description || null,
                levels: this.rubricForm.levels,
            };

            const resp = await fetch('/api/evaluation/rubrics', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(body),
                credentials: 'same-origin',
            });

            if (resp.ok || resp.status === 201) {
                this.showRubricForm = false;
                this.rubricForm = {
                    name: '',
                    description: '',
                    levels: [
                        { score: 1, description: '' },
                        { score: 3, description: '' },
                        { score: 5, description: '' },
                    ],
                };
                await this.loadRubrics();
                this.showToast('Rubric created', 'success');
            } else {
                const data = await resp.json().catch(() => ({}));
                this.showToast(data.message || 'Failed to create rubric', 'error');
            }
        } catch (err) {
            this.showToast('Network error', 'error');
        } finally {
            this.creatingRubric = false;
        }
    },

    async deleteRubric(r) {
        if (!confirm(`Delete rubric "${r.name}"? This cannot be undone.`)) return;
        try {
            const resp = await fetch(`/api/evaluation/rubrics/${r.id}`, {
                method: 'DELETE',
                credentials: 'same-origin',
            });
            if (resp.ok || resp.status === 204) {
                await this.loadRubrics();
                this.showToast('Rubric deleted', 'success');
            }
        } catch (err) {
            this.showToast('Failed to delete rubric', 'error');
        }
    },

    formatDate(dateStr) {
        if (!dateStr) return '';
        const d = new Date(dateStr);
        return d.toLocaleDateString() + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    },

    showToast(message, type = 'info', duration = 3000) {
        if (window.dispatchEvent) {
            window.dispatchEvent(new CustomEvent('toast-show', {
                detail: { message, type, duration },
            }));
        }
    },
});
