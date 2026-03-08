/**
 * Plugin Settings Manager
 * Alpine.js component for managing per-user, per-plugin configuration.
 *
 * Each plugin can declare a settings schema (text fields, password fields,
 * select dropdowns). This component fetches all schemas, loads the user's
 * current values, and renders a dynamic form for each plugin.
 *
 * API endpoints:
 *   GET  /api/plugin-settings/schemas     → list of { plugin_id, icon, schema }
 *   GET  /api/plugin-settings             → list of { plugin_id, settings }
 *   PUT  /api/plugin-settings/{pluginID}  → save settings for one plugin
 *   DELETE /api/plugin-settings/{pluginID} → delete all settings for one plugin
 */
function pluginSettingsManager() {
    return {
        // ─── State ───────────────────────────────────────────────────
        schemas: [],          // Array of { plugin_id, icon, schema: [...fields] }
        allSettings: {},      // plugin_id → { key → value }
        forms: {},            // plugin_id → { key → value } (live form state)
        loading: true,
        saving: {},           // plugin_id → boolean
        saved: {},            // plugin_id → boolean (flash after save)
        errors: {},           // plugin_id → string
        expandedPlugin: '',   // Currently expanded plugin accordion

        // ─── Lifecycle ───────────────────────────────────────────────

        async init() {
            await this.load();
        },

        async load() {
            this.loading = true;
            try {
                const [schemasRes, settingsRes] = await Promise.all([
                    fetch('/api/plugin-settings/schemas'),
                    fetch('/api/plugin-settings'),
                ]);

                if (schemasRes.ok) {
                    this.schemas = await schemasRes.json();
                }

                if (settingsRes.ok) {
                    const settingsList = await settingsRes.json();
                    this.allSettings = {};
                    for (const item of (settingsList || [])) {
                        this.allSettings[item.plugin_id] = item.settings || {};
                    }
                }

                this._initForms();

                // Auto-expand first plugin if there's only one
                if (this.schemas.length === 1 && !this.expandedPlugin) {
                    this.expandedPlugin = this.schemas[0].plugin_id;
                }
            } catch (err) {
                console.error('Failed to load plugin settings:', err);
            } finally {
                this.loading = false;
            }
        },

        // Refresh settings from server without toggling loading state
        // (keeps the accordion open and form intact).
        async _refreshSettings() {
            try {
                const res = await fetch('/api/plugin-settings');
                if (res.ok) {
                    const settingsList = await res.json();
                    this.allSettings = {};
                    for (const item of (settingsList || [])) {
                        this.allSettings[item.plugin_id] = item.settings || {};
                    }
                }
                this._initForms();
            } catch (err) {
                console.error('Failed to refresh plugin settings:', err);
            }
        },

        // Build form state from schemas + saved values. Also pre-initializes
        // the per-plugin saving/saved/errors maps so Alpine tracks them.
        _initForms() {
            const newForms = {};
            const newSaving = {};
            const newSaved = {};
            const newErrors = {};

            for (const plugin of this.schemas) {
                const saved = this.allSettings[plugin.plugin_id] || {};
                const form = {};
                for (const field of plugin.schema) {
                    if (field.type === 'password') {
                        // For password fields, show empty placeholder —
                        // the backend stores the real value; sending empty
                        // preserves the existing value.
                        form[field.key] = '';
                    } else {
                        form[field.key] = saved[field.key] || field.default || '';
                    }
                }
                newForms[plugin.plugin_id] = form;
                newSaving[plugin.plugin_id] = this.saving[plugin.plugin_id] || false;
                newSaved[plugin.plugin_id] = this.saved[plugin.plugin_id] || false;
                newErrors[plugin.plugin_id] = this.errors[plugin.plugin_id] || '';
            }

            this.forms = newForms;
            this.saving = newSaving;
            this.saved = newSaved;
            this.errors = newErrors;
        },

        // ─── Field accessors (safe for nested reactive access) ───────

        getField(pluginId, key) {
            const form = this.forms[pluginId];
            return form ? (form[key] || '') : '';
        },

        setField(pluginId, key, value) {
            if (!this.forms[pluginId]) this.forms[pluginId] = {};
            this.forms[pluginId][key] = value;
        },

        // ─── Helpers ─────────────────────────────────────────────────

        pluginName(id) {
            const names = {
                'websearch': 'Web Search',
                'tts': 'Text to Speech',
                'knowledgebase': 'Knowledge Base',
                'calculator': 'Calculator',
                'memory': 'Memory',
                'document': 'Document',
                'todos': 'Todos',
                'library': 'Library',
                'codebase': 'Codebase',
                'cognitive': 'Cognitive',
                'askuser': 'Ask User',
                'planmode': 'Plan Mode',
            };
            return names[id] || id.charAt(0).toUpperCase() + id.slice(1);
        },

        togglePlugin(pluginId) {
            this.expandedPlugin = this.expandedPlugin === pluginId ? '' : pluginId;
        },

        isConfigured(pluginId) {
            const settings = this.allSettings[pluginId];
            if (!settings) return false;
            // Check if any non-empty, non-masked value is present
            return Object.values(settings).some(v =>
                v && v !== '' && !v.startsWith('****')
            );
        },

        // ─── Save / Delete ───────────────────────────────────────────

        async savePlugin(pluginId) {
            const form = this.forms[pluginId];
            if (!form) return;

            this.saving[pluginId] = true;
            this.saved[pluginId] = false;
            this.errors[pluginId] = '';

            try {
                // Build settings payload.
                // - Skip empty password fields (server preserves the existing value).
                // - Include all other non-empty values.
                const settings = {};
                const plugin = this.schemas.find(s => s.plugin_id === pluginId);
                if (plugin) {
                    for (const field of plugin.schema) {
                        const val = form[field.key];
                        if (field.type === 'password') {
                            // Only send password if user typed a new value
                            if (val && val !== '') {
                                settings[field.key] = val;
                            }
                            // Otherwise omit → server preserves existing
                        } else if (val && val !== '') {
                            settings[field.key] = val;
                        }
                    }
                }

                const res = await fetch('/api/plugin-settings/' + pluginId, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ settings: settings }),
                });

                if (res.ok) {
                    this.saved[pluginId] = true;
                    // Refresh settings in the background without flashing loading state
                    await this._refreshSettings();
                    setTimeout(() => { this.saved[pluginId] = false; }, 3000);
                } else {
                    const errText = await res.text();
                    this.errors[pluginId] = errText || 'Failed to save settings.';
                    setTimeout(() => { this.errors[pluginId] = ''; }, 5000);
                }
            } catch (err) {
                console.error('Plugin settings save error:', err);
                this.errors[pluginId] = 'Network error. Please try again.';
                setTimeout(() => { this.errors[pluginId] = ''; }, 5000);
            } finally {
                this.saving[pluginId] = false;
            }
        },

        async deletePlugin(pluginId) {
            if (!confirm('Remove all settings for this plugin? The plugin will use default values.')) return;

            this.saving[pluginId] = true;
            this.errors[pluginId] = '';

            try {
                const res = await fetch('/api/plugin-settings/' + pluginId, {
                    method: 'DELETE',
                });

                if (res.ok) {
                    await this._refreshSettings();
                } else {
                    this.errors[pluginId] = 'Failed to remove settings.';
                    setTimeout(() => { this.errors[pluginId] = ''; }, 5000);
                }
            } catch (err) {
                this.errors[pluginId] = 'Network error.';
                setTimeout(() => { this.errors[pluginId] = ''; }, 5000);
            } finally {
                this.saving[pluginId] = false;
            }
        },
    };
}
