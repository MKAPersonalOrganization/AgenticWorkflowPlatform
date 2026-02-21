/**
 * AI Settings Manager
 * Alpine.js component for managing per-user AI provider configuration
 * and knowledge graph (EdgeQuake) model settings.
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

        // Knowledge Graph embedding state (global, not per-workspace)
        kgProviderStatus: null,
        kgLoading: false,
        kgEmbProvider: '',
        kgEmbModel: '',
        kgApiKeySource: 'reuse',
        kgApiKey: '',
        kgEmbProviders: [],       // [{id, name}]
        kgCurrentEmbModels: [],   // [model_name, ...]
        kgAllEmbModels: [],       // [{provider, name, dimension, ...}]
        kgSaving: false,
        kgSaved: false,
        kgError: '',

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
            // Load knowledge graph settings in parallel
            this.loadKgSettings();
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

        // === Knowledge Graph Embedding Settings ===

        get kgMatchingProviderSetting() {
            if (!this.kgEmbProvider) return null;
            const match = this.settings.find(s => s.provider === this.kgEmbProvider);
            if (match && match.api_key) {
                return { ...match, api_key_masked: match.api_key.slice(0, 3) + '***' + match.api_key.slice(-4) };
            }
            // Check if provider is configured (API key hidden) - use masked indicator
            if (match) {
                return { ...match, api_key_masked: '••••••••' };
            }
            return null;
        },

        async loadKgSettings() {
            this.kgLoading = true;
            try {
                const [embRes, statusRes] = await Promise.all([
                    fetch('/api/knowledge/models/embedding'),
                    fetch('/api/knowledge/settings/status'),
                ]);
                if (embRes.ok) {
                    const data = await embRes.json();
                    this.kgAllEmbModels = data.models || [];
                    // Derive unique providers that have embedding models
                    const seen = new Set();
                    this.kgEmbProviders = [];
                    for (const m of this.kgAllEmbModels) {
                        if (!seen.has(m.provider)) {
                            seen.add(m.provider);
                            this.kgEmbProviders.push({
                                id: m.provider,
                                name: m.provider_display_name || this.providerName(m.provider),
                            });
                        }
                    }
                }
                if (statusRes.ok) {
                    this.kgProviderStatus = await statusRes.json();
                    // Pre-select from current status if available
                    if (this.kgProviderStatus?.embedding?.provider && !this.kgEmbProvider) {
                        this.kgEmbProvider = this.kgProviderStatus.embedding.provider;
                        this.onKgEmbProviderChange();
                        if (this.kgProviderStatus.embedding.model) {
                            this.kgEmbModel = this.kgProviderStatus.embedding.model;
                        }
                    }
                }
            } catch (err) {
                console.error('Failed to load KG settings:', err);
            } finally {
                this.kgLoading = false;
            }
        },

        onKgEmbProviderChange() {
            // Filter models for selected provider
            this.kgCurrentEmbModels = this.kgAllEmbModels
                .filter(m => m.provider === this.kgEmbProvider)
                .map(m => m.name);
            this.kgEmbModel = this.kgCurrentEmbModels.length === 1 ? this.kgCurrentEmbModels[0] : '';
            // Default to reuse if a matching provider is already configured
            this.kgApiKeySource = this.kgMatchingProviderSetting ? 'reuse' : 'custom';
            this.kgApiKey = '';
            this.kgSaved = false;
            this.kgError = '';
        },

        async saveKgEmbedding() {
            if (!this.kgEmbProvider || !this.kgEmbModel) return;
            this.kgSaving = true;
            this.kgSaved = false;
            this.kgError = '';
            try {
                // Determine API key
                let apiKey = '';
                if (this.kgApiKeySource === 'reuse' && this.kgMatchingProviderSetting) {
                    // Reuse - send empty api_key, backend keeps existing
                    apiKey = '';
                } else {
                    apiKey = this.kgApiKey;
                    if (!apiKey) {
                        this.kgError = 'API key is required';
                        this.kgSaving = false;
                        return;
                    }
                }

                // Find embedding dimension from model info
                const modelInfo = this.kgAllEmbModels.find(
                    m => m.provider === this.kgEmbProvider && m.name === this.kgEmbModel
                );
                const dimension = modelInfo?.dimension || modelInfo?.capabilities?.embedding_dimension || 0;

                // Save via AI settings endpoint (updates the provider's embed model config)
                const payload = {
                    provider: this.kgEmbProvider,
                    default_embed_model: this.kgEmbModel,
                    embedding_dimensions: dimension,
                };
                if (apiKey) {
                    payload.api_key = apiKey;
                }

                const res = await fetch('/api/ai-settings', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload),
                });
                if (res.ok) {
                    this.kgSaved = true;
                    // Reload settings so kgMatchingProviderSetting updates
                    await this.loadSettings();
                    setTimeout(() => { this.kgSaved = false; }, 3000);
                } else {
                    const errText = await res.text();
                    this.kgError = errText || 'Failed to save embedding settings';
                    setTimeout(() => { this.kgError = ''; }, 5000);
                }
            } catch (err) {
                this.kgError = 'Network error';
                console.error('Failed to save KG embedding:', err);
                setTimeout(() => { this.kgError = ''; }, 5000);
            } finally {
                this.kgSaving = false;
            }
        },
    };
}
