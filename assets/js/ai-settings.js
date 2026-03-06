/**
 * AI Settings Manager
 * Alpine.js component for managing per-user AI provider configuration
 * and per-capability settings (LLM, TTS, Embedding, etc.).
 *
 * Architecture:
 *   1. Providers — credential storage (API keys) per provider (OpenAI, Anthropic, …)
 *   2. Capabilities — per-capability config selecting which provider/model to use
 *      (e.g. use OpenAI for LLM, Anthropic for TTS, etc.)
 */
function aiSettingsManager() {
    return {
        // ─── Provider State ──────────────────────────────────────────
        providers: [],
        settings: [],
        editing: false,
        saving: false,
        statusMessage: '',
        statusType: 'success',
        form: {
            provider: '',
            api_key: '',
        },

        // ─── Capability State ────────────────────────────────────────
        capabilities: {},
        activeCapTab: 'tts',
        capSaving: false,
        capSaved: false,
        capStatusMessage: '',
        capStatusType: 'success',

        // Chat (LLM) capability form
        chatForm: { provider: '', model: '', max_tokens: 0, temperature: 0.7 },
        chatProviders: [],
        chatCurrentModels: [],
        chatSaving: false,
        chatSaved: false,
        chatStatusMessage: '',
        chatStatusType: 'success',

        // TTS capability form
        ttsForm: { provider: '', model: '', voice: '' },
        ttsModels: [],
        ttsVoices: [],
        ttsProviders: [],

        // Knowledge Graph Embedding state
        kgProviderStatus: null,
        kgLoading: false,
        kgEmbProvider: '',
        kgEmbModel: '',
        kgApiKeySource: 'reuse',
        kgApiKey: '',
        kgEmbProviders: [],
        kgCurrentEmbModels: [],
        kgAllEmbModels: [],
        kgSaving: false,
        kgSaved: false,
        kgError: '',

        // ─── Lifecycle ───────────────────────────────────────────────

        async loadSettings() {
            try {
                const [pRes, sRes, capRes] = await Promise.all([
                    fetch('/api/ai-settings/providers'),
                    fetch('/api/ai-settings'),
                    fetch('/api/ai-settings/capabilities'),
                ]);
                if (pRes.ok) this.providers = await pRes.json();
                if (sRes.ok) this.settings = await sRes.json();
                if (capRes.ok) {
                    const caps = await capRes.json();
                    this.capabilities = {};
                    for (const c of (caps || [])) {
                        this.capabilities[c.capability] = c;
                    }
                }
            } catch (err) {
                console.error('Failed to load AI settings:', err);
            }
            this._loadChatProviders();
            this._loadCurrentChatConfig();
            this._loadTTSProviders();
            this._loadCurrentTTSConfig();
            this.loadKgSettings();
        },

        // ═══════════════════════════════════════════════════════════════
        //  Provider CRUD
        // ═══════════════════════════════════════════════════════════════

        onProviderChange() {
            // No-op: provider selection no longer drives model lists.
            // Model selection is done per-capability.
        },

        providerName(id) {
            const names = { openai: 'OpenAI', anthropic: 'Anthropic', gemini: 'Google Gemini' };
            return names[id] || id;
        },

        editProvider(s) {
            this.editing = true;
            this.form = {
                provider: s.provider,
                api_key: '',
            };
        },

        cancelEdit() {
            this.editing = false;
            this.resetForm();
        },

        resetForm() {
            this.form = {
                provider: '',
                api_key: '',
            };
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
                if (res.ok) await this.loadSettings();
            } catch (err) {
                console.error('Failed to set default:', err);
            }
        },

        get hasConfiguredProvider() {
            return this.settings.some(s => s.api_key_set);
        },

        // ═══════════════════════════════════════════════════════════════
        //  Chat (LLM) Capability Configuration
        // ═══════════════════════════════════════════════════════════════

        _loadChatProviders() {
            this.chatProviders = this.providers
                .filter(p => p.models && p.models.length > 0)
                .filter(p => this.settings.some(s => s.provider === p.id && s.api_key_set))
                .map(p => ({
                    id: p.id,
                    name: p.name,
                    models: p.models,
                }));
        },

        _loadCurrentChatConfig() {
            const cap = this.capabilities['llm'];
            if (cap) {
                this.chatForm.provider = cap.provider;
                this.onChatProviderChange();
                this.chatForm.model = cap.model;
                if (cap.config) {
                    if (cap.config.max_tokens) this.chatForm.max_tokens = cap.config.max_tokens;
                    if (cap.config.temperature) this.chatForm.temperature = cap.config.temperature;
                }
            }
        },

        get currentChatConfig() {
            const cap = this.capabilities['llm'];
            if (cap) {
                const provider = this.providers.find(p => p.id === cap.provider);
                return {
                    provider: cap.provider,
                    provider_name: provider ? provider.name : cap.provider,
                    model: cap.model,
                    max_tokens: cap.config?.max_tokens || 0,
                    temperature: cap.config?.temperature || 0.7,
                };
            }
            return null;
        },

        onChatProviderChange() {
            const provider = this.chatProviders.find(p => p.id === this.chatForm.provider);
            this.chatCurrentModels = provider ? provider.models : [];
            this.chatForm.model = '';
            this.chatSaved = false;
        },

        async saveChatCapability() {
            if (!this.chatForm.provider || !this.chatForm.model) return;
            this.chatSaving = true;
            this.chatSaved = false;
            this.chatStatusMessage = '';
            try {
                const config = {};
                if (this.chatForm.max_tokens > 0) config.max_tokens = Number(this.chatForm.max_tokens);
                if (this.chatForm.temperature > 0) config.temperature = Number(this.chatForm.temperature);
                const res = await fetch('/api/ai-settings/capabilities/llm', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        provider: this.chatForm.provider,
                        model: this.chatForm.model,
                        config,
                    }),
                });
                if (res.ok) {
                    this.chatSaved = true;
                    this.chatStatusMessage = 'Chat model saved successfully.';
                    this.chatStatusType = 'success';
                    await this.loadSettings();
                    setTimeout(() => { this.chatSaved = false; this.chatStatusMessage = ''; }, 3000);
                } else {
                    const errText = await res.text();
                    this.chatStatusMessage = errText || 'Failed to save chat model.';
                    this.chatStatusType = 'error';
                    setTimeout(() => { this.chatStatusMessage = ''; }, 5000);
                }
            } catch (err) {
                this.chatStatusMessage = 'Network error. Please try again.';
                this.chatStatusType = 'error';
                setTimeout(() => { this.chatStatusMessage = ''; }, 5000);
            } finally {
                this.chatSaving = false;
            }
        },

        // ═══════════════════════════════════════════════════════════════
        //  TTS Capability Configuration
        // ═══════════════════════════════════════════════════════════════

        _loadTTSProviders() {
            this.ttsProviders = this.providers
                .filter(p => p.supports_tts && p.tts_models && p.tts_models.length > 0)
                .filter(p => this.settings.some(s => s.provider === p.id && s.api_key_set))
                .map(p => ({
                    id: p.id,
                    name: p.name,
                    tts_models: p.tts_models,
                }));
        },

        _loadCurrentTTSConfig() {
            const cap = this.capabilities['tts'];
            if (cap) {
                this.ttsForm.provider = cap.provider;
                this.onTTSProviderChange();
                this.ttsForm.model = cap.model;
                this.onTTSModelChange();
                if (cap.config && cap.config.voice) {
                    this.ttsForm.voice = cap.config.voice;
                }
            }
        },

        get currentTTSConfig() {
            const cap = this.capabilities['tts'];
            if (cap) {
                const provider = this.providers.find(p => p.id === cap.provider);
                return {
                    provider: cap.provider,
                    provider_name: provider ? provider.name : cap.provider,
                    model: cap.model,
                    voice: cap.config?.voice || 'coral',
                };
            }
            return null;
        },

        onTTSProviderChange() {
            const provider = this.providers.find(p => p.id === this.ttsForm.provider);
            this.ttsModels = provider ? (provider.tts_models || []) : [];
            this.ttsForm.model = this.ttsModels.length === 1 ? this.ttsModels[0] : '';
            this.ttsVoices = [];
            this.ttsForm.voice = '';
            this.capSaved = false;
            if (this.ttsForm.model) {
                this.onTTSModelChange();
            }
        },

        async onTTSModelChange() {
            this.ttsForm.voice = '';
            this.capSaved = false;
            if (!this.ttsForm.model) {
                this.ttsVoices = [];
                return;
            }
            try {
                const res = await fetch('/api/ai-settings/tts-voices?model=' + encodeURIComponent(this.ttsForm.model));
                if (res.ok) {
                    const data = await res.json();
                    this.ttsVoices = data.voices || [];
                    if (this.ttsVoices.includes('coral')) {
                        this.ttsForm.voice = 'coral';
                    } else if (this.ttsVoices.length > 0) {
                        this.ttsForm.voice = this.ttsVoices[0];
                    }
                }
            } catch (err) {
                console.error('Failed to load TTS voices:', err);
                this.ttsVoices = ['alloy', 'ash', 'coral', 'echo', 'fable', 'nova', 'onyx', 'sage', 'shimmer'];
            }
        },

        async saveTTSCapability() {
            if (!this.ttsForm.provider || !this.ttsForm.model || !this.ttsForm.voice) return;
            this.capSaving = true;
            this.capSaved = false;
            this.capStatusMessage = '';
            try {
                const res = await fetch('/api/ai-settings/capabilities/tts', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        provider: this.ttsForm.provider,
                        model: this.ttsForm.model,
                        config: { voice: this.ttsForm.voice },
                    }),
                });
                if (res.ok) {
                    this.capSaved = true;
                    this.capStatusMessage = 'Voice settings saved successfully.';
                    this.capStatusType = 'success';
                    await this.loadSettings();
                    setTimeout(() => { this.capSaved = false; this.capStatusMessage = ''; }, 3000);
                } else {
                    const errText = await res.text();
                    this.capStatusMessage = errText || 'Failed to save voice settings.';
                    this.capStatusType = 'error';
                    setTimeout(() => { this.capStatusMessage = ''; }, 5000);
                }
            } catch (err) {
                this.capStatusMessage = 'Network error. Please try again.';
                this.capStatusType = 'error';
                setTimeout(() => { this.capStatusMessage = ''; }, 5000);
            } finally {
                this.capSaving = false;
            }
        },

        // ═══════════════════════════════════════════════════════════════
        //  Knowledge Graph Embedding Settings
        // ═══════════════════════════════════════════════════════════════

        get kgMatchingProviderSetting() {
            if (!this.kgEmbProvider) return null;
            const match = this.settings.find(s => s.provider === this.kgEmbProvider);
            if (match && match.api_key) {
                return { ...match, api_key_masked: match.api_key.slice(0, 3) + '***' + match.api_key.slice(-4) };
            }
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
            this.kgCurrentEmbModels = this.kgAllEmbModels
                .filter(m => m.provider === this.kgEmbProvider)
                .map(m => m.name);
            this.kgEmbModel = this.kgCurrentEmbModels.length === 1 ? this.kgCurrentEmbModels[0] : '';
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
                const modelInfo = this.kgAllEmbModels.find(
                    m => m.provider === this.kgEmbProvider && m.name === this.kgEmbModel
                );
                const dimension = modelInfo?.dimension || modelInfo?.capabilities?.embedding_dimension || 0;
                const payload = {
                    provider: this.kgEmbProvider,
                    model: this.kgEmbModel,
                    config: { dimensions: dimension },
                };
                const res = await fetch('/api/ai-settings/capabilities/embedding', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(payload),
                });
                if (res.ok) {
                    this.kgSaved = true;
                    await this.loadSettings();
                    setTimeout(() => { this.kgSaved = false; }, 3000);
                } else {
                    const errText = await res.text();
                    this.kgError = errText || 'Failed to save embedding settings';
                    setTimeout(() => { this.kgError = ''; }, 5000);
                }
            } catch (err) {
                this.kgError = 'Network error';
                setTimeout(() => { this.kgError = ''; }, 5000);
            } finally {
                this.kgSaving = false;
            }
        },
    };
}
