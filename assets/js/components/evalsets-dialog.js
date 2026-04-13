/**
 * EvalSets Dialog
 * Alpine.js component for managing evaluation test suites.
 */
export const evalSetsDialog = () => ({
    // View state
    view: 'list', // 'list' | 'create' | 'edit' | 'cases' | 'case-form'
    loading: false,
    casesLoading: false,
    saving: false,
    caseSaving: false,

    // Data
    evalSets: [],
    cases: [],
    selectedSet: null,
    search: '',

    // Toasts
    toasts: [],
    __toastIdCounter: 0,

    // Forms
    showCreateForm: false,
    showCaseForm: false,
    formError: '',
    caseFormError: '',
    form: { id: '', name: '', description: '' },
    caseForm: { id: '', name: '', input: '', expected_trajectory: '', expected_response: '' },

    get filteredSets() {
        if (!this.search) return this.evalSets;
        const q = this.search.toLowerCase();
        return this.evalSets.filter(es =>
            es.name.toLowerCase().includes(q) ||
            (es.description && es.description.toLowerCase().includes(q))
        );
    },

    async open() {
        await this.loadSets();
        this.view = 'list';
        this.showCreateForm = false;
        this.search = '';
    },

    async loadSets() {
        this.loading = true;
        try {
            const resp = await fetch('/api/eval-sets', { credentials: 'same-origin' });
            if (resp.ok) {
                this.evalSets = await resp.json();
            }
        } catch (err) {
            console.error('Failed to load eval sets:', err);
        } finally {
            this.loading = false;
        }
    },

    // ─── EvalSet CRUD ─────────────────────────────────────────────────────

    showCreateFormView() {
        this.form = { id: '', name: '', description: '' };
        this.formError = '';
        this.view = 'create';
    },

    editEvalSet(es) {
        this.form = {
            id: es.id,
            name: es.name,
            description: es.description || '',
        };
        this.formError = '';
        this.view = 'edit';
    },

    async saveEvalSet() {
        this.formError = '';
        if (!this.form.name.trim()) {
            this.formError = 'Name is required';
            return;
        }

        this.saving = true;
        try {
            let resp;
            if (this.view === 'edit') {
                resp = await fetch(`/api/eval-sets/${this.form.id}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        name: this.form.name,
                        description: this.form.description || null,
                    }),
                    credentials: 'same-origin',
                });
            } else {
                resp = await fetch('/api/eval-sets', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        name: this.form.name,
                        description: this.form.description || null,
                    }),
                    credentials: 'same-origin',
                });
            }

            if (resp.ok || resp.status === 201 || resp.status === 204) {
                this.view = 'list';
                this.form = { id: '', name: '', description: '' };
                await this.loadSets();
                this.showToast(this.view === 'edit' ? 'Eval set updated' : 'Eval set created', 'success');
            } else {
                const data = await resp.json().catch(() => ({}));
                this.formError = data.message || 'Failed to save eval set';
            }
        } catch (err) {
            this.formError = 'Network error';
            console.error('Failed to save eval set:', err);
        } finally {
            this.saving = false;
        }
    },

    async deleteEvalSet(es) {
        const msg = es.case_count > 0
            ? `Delete "${es.name}"? This will also delete all ${es.case_count} test cases. This cannot be undone.`
            : `Delete "${es.name}"? This cannot be undone.`;
        if (!confirm(msg)) return;

        try {
            const resp = await fetch(`/api/eval-sets/${es.id}`, {
                method: 'DELETE',
                credentials: 'same-origin',
            });
            if (resp.ok || resp.status === 204) {
                await this.loadSets();
                this.showToast('Eval set deleted', 'success');
            } else {
                this.showToast('Failed to delete eval set', 'error');
            }
        } catch (err) {
            console.error('Failed to delete eval set:', err);
            this.showToast('Network error', 'error');
        }
    },

    // ─── Cases ─────────────────────────────────────────────────────────────

    async openCases(es) {
        this.selectedSet = es;
        this.view = 'cases';
        await this.loadCases();
    },

    async loadCases() {
        if (!this.selectedSet) return;
        this.casesLoading = true;
        try {
            const resp = await fetch(`/api/eval-sets/${this.selectedSet.id}/cases`, {
                credentials: 'same-origin',
            });
            if (resp.ok) {
                this.cases = await resp.json();
            }
        } catch (err) {
            console.error('Failed to load cases:', err);
        } finally {
            this.casesLoading = false;
        }
    },

    editCase(c) {
        this.caseForm = {
            id: c.id,
            name: c.name || '',
            input: c.input,
            expected_trajectory: c.expected_trajectory ? JSON.stringify(c.expected_trajectory, null, 2) : '',
            expected_response: c.expected_response || '',
        };
        this.caseFormError = '';
        this.view = 'case-form';
    },

    async saveCase() {
        this.caseFormError = '';
        if (!this.caseForm.input.trim()) {
            this.caseFormError = 'Input prompt is required';
            return;
        }

        // Validate trajectory JSON if provided
        let trajRaw = null;
        if (this.caseForm.expected_trajectory.trim()) {
            try {
                trajRaw = JSON.parse(this.caseForm.expected_trajectory);
            } catch {
                this.caseFormError = 'Expected trajectory must be valid JSON';
                return;
            }
        }

        this.caseSaving = true;
        try {
            let resp;
            const body = {
                name: this.caseForm.name || null,
                input: this.caseForm.input,
                expected_trajectory: trajRaw,
                expected_response: this.caseForm.expected_response || null,
            };

            if (this.caseForm.id) {
                resp = await fetch(`/api/eval-cases/${this.caseForm.id}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body),
                    credentials: 'same-origin',
                });
            } else {
                resp = await fetch(`/api/eval-sets/${this.selectedSet.id}/cases`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body),
                    credentials: 'same-origin',
                });
            }

            if (resp.ok || resp.status === 201 || resp.status === 204) {
                this.view = 'cases';
                this.caseForm = { id: '', name: '', input: '', expected_trajectory: '', expected_response: '' };
                await this.loadCases();
                await this.loadSets(); // refresh case counts
                this.showToast(this.caseForm.id ? 'Test case updated' : 'Test case added', 'success');
            } else {
                const data = await resp.json().catch(() => ({}));
                this.caseFormError = data.message || 'Failed to save test case';
            }
        } catch (err) {
            this.caseFormError = 'Network error';
            console.error('Failed to save case:', err);
        } finally {
            this.caseSaving = false;
        }
    },

    async deleteCase(c) {
        if (!confirm(`Delete test case "${c.name || c.input.substring(0, 30)}..."?`)) return;

        try {
            const resp = await fetch(`/api/eval-cases/${c.id}`, {
                method: 'DELETE',
                credentials: 'same-origin',
            });
            if (resp.ok || resp.status === 204) {
                await this.loadCases();
                await this.loadSets();
                this.showToast('Test case deleted', 'success');
            } else {
                this.showToast('Failed to delete test case', 'error');
            }
        } catch (err) {
            console.error('Failed to delete case:', err);
            this.showToast('Network error', 'error');
        }
    },

    // ─── Import / Export ───────────────────────────────────────────────────

    triggerImport() {
        this.$refs.importFile.click();
    },

    async handleImport(event) {
        const file = event.target.files[0];
        if (!file) return;
        event.target.value = ''; // reset

        const formData = new FormData();
        formData.append('file', file);

        try {
            const resp = await fetch('/api/eval-sets/import', {
                method: 'POST',
                body: formData,
                credentials: 'same-origin',
            });
            if (resp.ok || resp.status === 201) {
                await this.loadSets();
                this.showToast('Eval set imported', 'success');
            } else {
                const data = await resp.json().catch(() => ({}));
                this.showToast(data.message || 'Failed to import', 'error');
            }
        } catch (err) {
            console.error('Failed to import:', err);
            this.showToast('Network error', 'error');
        }
    },

    async exportEvalSet(es) {
        window.open(`/api/eval-sets/${es.id}/export`, '_blank');
    },

    // ─── Toast helper ──────────────────────────────────────────────────────

    showToast(message, type = 'info', duration = 3500) {
        const id = ++this.__toastIdCounter;
        this.toasts.push({ id, message, type, visible: true, leaving: false });
        if (duration > 0) {
            setTimeout(() => this.dismissToast(id), duration);
        }
    },

    dismissToast(id) {
        const toast = this.toasts.find(t => t.id === id);
        if (toast && !toast.leaving) {
            toast.leaving = true;
            setTimeout(() => { this.toasts = this.toasts.filter(t => t.id !== id); }, 200);
        }
    },
});
