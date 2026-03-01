/**
 * TTS Settings Manager
 * Alpine.js component for managing text-to-speech provider, model, and voice configuration
 * in the Speech tab of the Settings dialog.
 */
function ttsSettingsManager() {
    return {
        // All providers with their TTS capabilities
        allProviders: [],
        // User's configured AI settings (providers with API keys)
        userSettings: [],
        // Filtered list of providers that support TTS and have an API key configured
        ttsProviders: [],
        // Available models for the selected TTS provider
        ttsModels: [],
        // Available voices for the selected TTS model
        ttsVoices: [],

        // Form state
        ttsForm: {
            provider: '',
            model: '',
            voice: '',
        },

        // UI state
        ttsSaving: false,
        ttsSaved: false,
        ttsStatusMessage: '',
        ttsStatusType: 'success',

        // Current saved config (for display)
        currentTTSConfig: null,

        async loadTTSSettings() {
            try {
                // Load providers with TTS info
                const pRes = await fetch('/api/ai-settings/providers');
                if (pRes.ok) {
                    this.allProviders = await pRes.json();
                }

                // Load user's configured settings
                const sRes = await fetch('/api/ai-settings');
                if (sRes.ok) {
                    this.userSettings = await sRes.json();
                }

                // Build TTS provider list: only providers that support TTS AND have an API key configured
                this.ttsProviders = this.allProviders
                    .filter(p => p.supports_tts && p.tts_models && p.tts_models.length > 0)
                    .filter(p => this.userSettings.some(s => s.provider === p.id && s.api_key_set))
                    .map(p => ({
                        id: p.id,
                        name: p.name,
                        tts_models: p.tts_models,
                    }));

                // Load current TTS config from saved settings
                this._loadCurrentConfig();

            } catch (err) {
                console.error('Failed to load TTS settings:', err);
            }
        },

        _loadCurrentConfig() {
            // Find user setting that has TTS model configured
            for (const s of this.userSettings) {
                if (s.tts_model && s.tts_voice) {
                    const provider = this.allProviders.find(p => p.id === s.provider);
                    this.currentTTSConfig = {
                        provider: s.provider,
                        provider_name: provider ? provider.name : s.provider,
                        model: s.tts_model,
                        voice: s.tts_voice,
                    };
                    // Pre-fill the form
                    this.ttsForm.provider = s.provider;
                    this.onTTSProviderChange();
                    this.ttsForm.model = s.tts_model;
                    this.onTTSModelChange();
                    this.ttsForm.voice = s.tts_voice;
                    return;
                }
            }
            this.currentTTSConfig = null;
        },

        onTTSProviderChange() {
            const provider = this.allProviders.find(p => p.id === this.ttsForm.provider);
            this.ttsModels = provider ? (provider.tts_models || []) : [];
            this.ttsForm.model = this.ttsModels.length === 1 ? this.ttsModels[0] : '';
            this.ttsVoices = [];
            this.ttsForm.voice = '';
            this.ttsSaved = false;
            if (this.ttsForm.model) {
                this.onTTSModelChange();
            }
        },

        async onTTSModelChange() {
            this.ttsForm.voice = '';
            this.ttsSaved = false;
            if (!this.ttsForm.model) {
                this.ttsVoices = [];
                return;
            }
            try {
                const res = await fetch(`/api/ai-settings/tts-voices?model=${encodeURIComponent(this.ttsForm.model)}`);
                if (res.ok) {
                    const data = await res.json();
                    this.ttsVoices = data.voices || [];
                    // Default to 'coral' if available
                    if (this.ttsVoices.includes('coral')) {
                        this.ttsForm.voice = 'coral';
                    } else if (this.ttsVoices.length > 0) {
                        this.ttsForm.voice = this.ttsVoices[0];
                    }
                }
            } catch (err) {
                console.error('Failed to load TTS voices:', err);
                // Fallback voices
                this.ttsVoices = ['alloy', 'ash', 'coral', 'echo', 'fable', 'nova', 'onyx', 'sage', 'shimmer'];
            }
        },

        async saveTTSSettings() {
            if (!this.ttsForm.provider || !this.ttsForm.model || !this.ttsForm.voice) return;
            this.ttsSaving = true;
            this.ttsSaved = false;
            this.ttsStatusMessage = '';

            try {
                // Save TTS settings to the provider's AI settings entry
                const res = await fetch('/api/ai-settings', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        provider: this.ttsForm.provider,
                        tts_model: this.ttsForm.model,
                        tts_voice: this.ttsForm.voice,
                    }),
                });

                if (res.ok) {
                    this.ttsSaved = true;
                    this.ttsStatusMessage = 'TTS settings saved successfully.';
                    this.ttsStatusType = 'success';
                    // Reload settings to reflect the update
                    await this.loadTTSSettings();
                    setTimeout(() => {
                        this.ttsSaved = false;
                        this.ttsStatusMessage = '';
                    }, 3000);
                } else {
                    const errText = await res.text();
                    this.ttsStatusMessage = errText || 'Failed to save TTS settings.';
                    this.ttsStatusType = 'error';
                    setTimeout(() => { this.ttsStatusMessage = ''; }, 5000);
                }
            } catch (err) {
                this.ttsStatusMessage = 'Network error. Please try again.';
                this.ttsStatusType = 'error';
                console.error('Failed to save TTS settings:', err);
                setTimeout(() => { this.ttsStatusMessage = ''; }, 5000);
            } finally {
                this.ttsSaving = false;
            }
        },
    };
}
