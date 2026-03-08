/**
 * Skills Manager Component
 * Handles skill CRUD operations, search, selection, and per-skill agent permissions.
 * Includes Skill Store (shared catalog) tab for browsing and adding pre-built skills.
 */
export const skillsManager = () => ({
    // State
    view: 'list', // 'list' | 'form' | 'delete' | 'permissions'
    activeTab: 'my-skills', // 'my-skills' | 'store'
    skills: [],
    filteredSkills: [],
    loading: false,
    saving: false,
    searchQuery: '',

    // Form state
    editingSkill: null, // null = create mode, object = edit mode
    form: {
        name: '',
        description: '',
        category: '',
        instructions: '',
    },

    // Delete state
    deletingSkill: null,

    // Permissions state
    permSkill: null,        // the skill we're managing permissions for
    permAgents: [],         // list of agents with { ...agent, hasPermission: bool }
    loadingAgents: false,

    // ── Skill Store state ──
    storeEntries: [],
    storeCategories: [],
    storeSearchQuery: '',
    selectedCategory: '',
    storeLoading: false,
    storeAddingIds: [],     // catalog IDs currently being added
    addedCatalogIds: [],    // catalog IDs that the user has already added
    storeSuccessId: null,   // catalog ID that was just successfully added
    storeTotal: 0,

    async open() {
        this.view = 'list';
        this.searchQuery = '';
        this.storeSuccessId = null;
        await this.fetchSkills();
        // Pre-fetch store data in background
        if (this.storeEntries.length === 0) {
            await this.fetchCatalog();
        }
    },

    // ============================================
    // Data fetching
    // ============================================

    async fetchSkills() {
        this.loading = true;
        try {
            const response = await fetch('/v1/skills');
            if (response.ok) {
                const data = await response.json();
                this.skills = data.skills || [];
                this.applyFilter();
            }
        } catch (err) {
            console.error('Failed to fetch skills:', err);
        } finally {
            this.loading = false;
        }
    },

    async searchSkills() {
        if (!this.searchQuery.trim()) {
            this.applyFilter();
            return;
        }
        try {
            const response = await fetch(`/v1/skills/search?q=${encodeURIComponent(this.searchQuery)}`);
            if (response.ok) {
                const data = await response.json();
                this.filteredSkills = data.skills || [];
            }
        } catch (err) {
            console.error('Failed to search skills:', err);
            this.applyFilter();
        }
    },

    applyFilter() {
        if (!this.searchQuery.trim()) {
            this.filteredSkills = [...this.skills];
        } else {
            const q = this.searchQuery.toLowerCase();
            this.filteredSkills = this.skills.filter(s =>
                s.name.toLowerCase().includes(q) ||
                (s.description && s.description.toLowerCase().includes(q)) ||
                (s.category && s.category.toLowerCase().includes(q))
            );
        }
    },

    // ============================================
    // Form operations
    // ============================================

    showCreateForm() {
        this.editingSkill = null;
        this.form = {
            name: '',
            description: '',
            category: '',
            instructions: '',
        };
        this.view = 'form';
    },

    showEditForm(skill) {
        this.editingSkill = skill;
        this.form = {
            name: skill.name || '',
            description: skill.description || '',
            category: skill.category || '',
            instructions: skill.instructions || '',
        };
        this.view = 'form';
    },

    async saveSkill() {
        if (!this.form.name.trim() || !this.form.instructions.trim()) return;
        this.saving = true;
        try {
            const payload = {
                name: this.form.name.trim(),
                description: this.form.description.trim(),
                category: this.form.category.trim(),
                instructions: this.form.instructions.trim(),
            };

            let response;
            if (this.editingSkill) {
                response = await fetch(`/v1/skills/${this.editingSkill.id}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload),
                });
            } else {
                response = await fetch('/v1/skills', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload),
                });
            }

            if (response.ok) {
                await this.fetchSkills();
                this.view = 'list';
            } else {
                const err = await response.json().catch(() => ({}));
                console.error('Failed to save skill:', err);
                alert(err.error || 'Failed to save skill');
            }
        } catch (err) {
            console.error('Failed to save skill:', err);
            alert('Failed to save skill');
        } finally {
            this.saving = false;
        }
    },

    // ============================================
    // Delete operations
    // ============================================

    confirmDelete(skill) {
        this.deletingSkill = skill;
        this.view = 'delete';
    },

    async deleteSkill() {
        if (!this.deletingSkill) return;
        this.saving = true;
        try {
            const response = await fetch(`/v1/skills/${this.deletingSkill.id}`, {
                method: 'DELETE',
            });
            if (response.ok) {
                await this.fetchSkills();
                this.view = 'list';
            } else {
                const err = await response.json().catch(() => ({}));
                console.error('Failed to delete skill:', err);
                alert(err.error || 'Failed to delete skill');
            }
        } catch (err) {
            console.error('Failed to delete skill:', err);
            alert('Failed to delete skill');
        } finally {
            this.saving = false;
        }
    },

    // ============================================
    // Permissions (skill ↔ agent)
    // ============================================

    async showPermissions(skill) {
        this.permSkill = skill;
        this.permAgents = [];
        this.loadingAgents = true;
        this.view = 'permissions';

        try {
            // Fetch user's agents and skill permissions in parallel
            const [agentsRes, permsRes] = await Promise.all([
                fetch('/v1/agent-definitions'),
                fetch(`/v1/skills/${skill.id}/permissions`),
            ]);

            let agents = [];
            let grantedAgentIds = [];

            if (agentsRes.ok) {
                const agentsData = await agentsRes.json();
                agents = agentsData.agents || [];
            }
            if (permsRes.ok) {
                const permsData = await permsRes.json();
                const perms = permsData.permissions || [];
                grantedAgentIds = perms.filter(p => p.enabled).map(p => p.agent_id);
            }

            // Merge: augment each agent with hasPermission flag
            this.permAgents = agents.map(ag => ({
                ...ag,
                hasPermission: grantedAgentIds.includes(ag.id),
            }));
        } catch (err) {
            console.error('Failed to load permissions:', err);
        } finally {
            this.loadingAgents = false;
        }
    },

    async togglePermission(agent) {
        if (!this.permSkill) return;
        const newState = !agent.hasPermission;
        try {
            const response = await fetch(`/v1/skills/${this.permSkill.id}/permissions/${agent.id}`, {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled: newState }),
            });
            if (response.ok) {
                agent.hasPermission = newState;
            }
        } catch (err) {
            console.error('Failed to toggle permission:', err);
        }
    },

    // ============================================
    // Skill Store (shared catalog)
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
            let url = '/v1/skill-catalog?limit=100';
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
                this.addedCatalogIds = data.addedCatalogIds || [];
            }
        } catch (err) {
            console.error('Failed to fetch skill catalog:', err);
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
            const response = await fetch(`/v1/skill-catalog/search?q=${encodeURIComponent(this.storeSearchQuery)}`);
            if (response.ok) {
                const data = await response.json();
                this.storeEntries = data.entries || [];
                this.storeTotal = data.total || 0;
                if (data.addedCatalogIds) {
                    this.addedCatalogIds = data.addedCatalogIds;
                }
            }
        } catch (err) {
            console.error('Failed to search skill catalog:', err);
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
        if (this.addedCatalogIds.includes(entry.id)) return;
        if (this.storeAddingIds.includes(entry.id)) return;
        this.storeAddingIds.push(entry.id);
        try {
            const response = await fetch(`/v1/skill-catalog/${entry.id}/add`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
            });
            if (response.ok) {
                if (!this.addedCatalogIds.includes(entry.id)) {
                    this.addedCatalogIds.push(entry.id);
                }
                // Refresh skills list in the background
                this.fetchSkills();
                // Show brief inline success toast
                this.storeSuccessId = entry.id;
                setTimeout(() => {
                    if (this.storeSuccessId === entry.id) this.storeSuccessId = null;
                }, 2500);
            } else if (response.status === 409) {
                if (!this.addedCatalogIds.includes(entry.id)) {
                    this.addedCatalogIds.push(entry.id);
                }
            } else {
                const errData = await response.text();
                console.error('Failed to add skill from store:', response.status, errData);
                try {
                    const errJson = JSON.parse(errData);
                    alert(errJson.error || 'Failed to add skill');
                } catch {
                    alert('Failed to add skill');
                }
            }
        } catch (err) {
            console.error('Failed to add skill from store:', err);
            alert('Failed to add skill');
        } finally {
            this.storeAddingIds = this.storeAddingIds.filter(id => id !== entry.id);
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
