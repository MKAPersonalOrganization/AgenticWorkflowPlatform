/**
 * AI Settings Manager
 * Alpine.js component for managing per-user AI provider configuration.
 */
function aiSettingsManager() {
    return {
        providers: [],
        settings: [],
        editing: false,
        saving: false,
        statusMessage: '',
        statusType: 'success',
        form: {
            provider: '',
            api_key: '',
            default_llm_model: '',
            default_embed_model: '',
            max_tokens: 0,
            temperature: 0.7,
            embedding_dimensions: 0,
        },
        currentModels: [],
        currentEmbedModels: [],

        async loadSettings() {
            try {
                // Load providers
                const pRes = await fetch('/api/ai-settings/providers');
                if (pRes.ok) {
                    this.providers = await pRes.json();
                }
                // Load user settings
                const sRes = await fetch('/api/ai-settings');
                if (sRes.ok) {
                    this.settings = await sRes.json();
                }
            } catch (err) {
                console.error('Failed to load AI settings:', err);
            }
        },

        onProviderChange() {
            const p = this.providers.find(x => x.id === this.form.provider);
            this.currentModels = p ? p.models : [];
            this.currentEmbedModels = p ? (p.embedding_models || []) : [];
            // Reset model selections when provider changes
            if (!this.editing) {
                this.form.default_llm_model = this.currentModels.length ? this.currentModels[0] : '';
                this.form.default_embed_model = '';
            }
        },

        providerName(id) {
            const names = { openai: 'OpenAI', anthropic: 'Anthropic', gemini: 'Google Gemini' };
            return names[id] || id;
        },

        editProvider(s) {
            this.editing = true;
            this.form = {
                provider: s.provider,
                api_key: '',  // Don't pre-fill API key for security
                default_llm_model: s.default_llm_model,
                default_embed_model: s.default_embed_model,
                max_tokens: s.max_tokens || 0,
                temperature: s.temperature || 0.7,
                embedding_dimensions: s.embedding_dimensions || 0,
            };
            this.onProviderChange();
        },

        cancelEdit() {
            this.editing = false;
            this.resetForm();
        },

        resetForm() {
            this.form = {
                provider: '',
                api_key: '',
                default_llm_model: '',
                default_embed_model: '',
                max_tokens: 0,
                temperature: 0.7,
                embedding_dimensions: 0,
            };
            this.currentModels = [];
            this.currentEmbedModels = [];
        },

        async saveProvider() {
            if (!this.form.provider || !this.form.api_key) return;
            this.saving = true;
            this.statusMessage = '';
            try {
                const res = await fetch('/api/ai-settings', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(this.form),
                });
                if (res.ok) {
                    this.statusMessage = this.editing ? 'Provider updated successfully.' : 'Provider added successfully.';
                    this.statusType = 'success';
                    this.editing = false;
                    this.resetForm();
                    await this.loadSettings();
                } else {
                    const errText = await res.text();
                    this.statusMessage = errText || 'Failed to save provider.';
                    this.statusType = 'error';
                }
            } catch (err) {
                this.statusMessage = 'Network error. Please try again.';
                this.statusType = 'error';
            } finally {
                this.saving = false;
                // Auto-clear status after 4 seconds
                setTimeout(() => { this.statusMessage = ''; }, 4000);
            }
        },

        async deleteProvider(id) {
            if (!confirm('Are you sure you want to remove this provider?')) return;
            try {
                const res = await fetch(`/api/ai-settings/${id}`, { method: 'DELETE' });
                if (res.ok) {
                    this.statusMessage = 'Provider removed.';
                    this.statusType = 'success';
                    await this.loadSettings();
                } else {
                    this.statusMessage = 'Failed to remove provider.';
                    this.statusType = 'error';
                }
            } catch (err) {
                this.statusMessage = 'Network error.';
                this.statusType = 'error';
            }
            setTimeout(() => { this.statusMessage = ''; }, 4000);
        },

        async setDefault(id) {
            try {
                const res = await fetch(`/api/ai-settings/${id}/default`, { method: 'PUT' });
                if (res.ok) {
                    await this.loadSettings();
                }
            } catch (err) {
                console.error('Failed to set default:', err);
            }
        },
    };
}
