/**
 * Metric Configuration Dialog
 * Alpine.js component for selecting and configuring evaluation metrics.
 */
export const metricConfigDialog = () => ({
    // State
    profiles: [],
    metrics: [],
    configs: {},       // metric_id -> {enabled: bool, threshold: number}
    selectedProfileId: '',
    profileName: '',
    saving: false,
    errors: {},         // metric_id -> error message, or 'general'

    typeLabels: {
        'trajectory':    'Trajectory',
        'response':      'Response',
        'safety':        'Safety',
        'hallucination': 'Hallucination',
        'rubric':        'Rubric',
        'simulation':    'Simulation',
        'multi_turn':    'Multi-Turn',
    },

    get enabledCount() {
        return Object.values(this.configs).filter(c => c.enabled).length;
    },

    async open() {
        await Promise.all([this.loadCatalog(), this.loadProfiles()]);
        this.resetToDefaults();
        this.selectedProfileId = '';
        this.profileName = '';
        this.errors = {};
    },

    async loadCatalog() {
        try {
            const resp = await fetch('/api/evaluation/metrics', { credentials: 'same-origin' });
            if (resp.ok) {
                this.metrics = await resp.json();
            }
        } catch (err) {
            console.error('Failed to load metric catalog:', err);
        }
    },

    async loadProfiles() {
        try {
            const resp = await fetch('/api/evaluation/profiles', { credentials: 'same-origin' });
            if (resp.ok) {
                this.profiles = await resp.json();
            }
        } catch (err) {
            console.error('Failed to load metric profiles:', err);
        }
    },

    resetToDefaults() {
        this.configs = {};
        for (const m of this.metrics) {
            this.configs[m.id] = {
                enabled: true,
                threshold: m.default_threshold,
            };
        }
    },

    onProfileChange() {
        if (!this.selectedProfileId) {
            this.resetToDefaults();
            this.profileName = '';
            this.errors = {};
            return;
        }
        const profile = this.profiles.find(p => p.id === this.selectedProfileId);
        if (!profile) return;

        this.profileName = profile.name;
        // Reset all configs
        this.configs = {};
        for (const m of this.metrics) {
            const mc = profile.metrics?.find(x => x.metric_id === m.id);
            this.configs[m.id] = {
                enabled: mc ? mc.enabled : true,
                threshold: mc ? mc.threshold : m.default_threshold,
            };
        }
        this.errors = {};
    },

    toggleMetric(metricId) {
        if (this.configs[metricId]) {
            this.configs[metricId].enabled = !this.configs[metricId].enabled;
        }
        // Clear any validation error for this metric
        if (this.errors[metricId]) {
            delete this.errors[metricId];
        }
        if (this.errors.general) {
            delete this.errors.general;
        }
    },

    resetToDefault(m) {
        if (this.configs[m.id]) {
            this.configs[m.id].threshold = m.default_threshold;
        }
        if (this.errors[m.id]) {
            delete this.errors[m.id];
        }
    },

    validate() {
        this.errors = {};

        // Check profile name for new profiles
        if (!this.selectedProfileId && !this.profileName.trim()) {
            this.errors.general = 'Profile name is required for new profiles.';
            return false;
        }

        // Check at least one metric enabled
        if (this.enabledCount === 0) {
            this.errors.general = 'At least one metric must be enabled.';
            return false;
        }

        // Check thresholds in range
        for (const m of this.metrics) {
            const cfg = this.configs[m.id];
            if (!cfg || !cfg.enabled) continue;
            if (m.range_min !== m.range_max) {
                if (cfg.threshold < m.range_min || cfg.threshold > m.range_max) {
                    this.errors[m.id] = `Threshold must be between ${m.range_min} and ${m.range_max}`;
                }
            }
        }

        return Object.keys(this.errors).length === 0;
    },

    async saveProfile() {
        if (!this.validate()) return;

        this.saving = true;
        try {
            const metricsPayload = this.metrics
                .filter(m => this.configs[m.id]?.enabled)
                .map(m => ({
                    metric_id: m.id,
                    enabled: true,
                    threshold: this.configs[m.id].threshold,
                }));

            // Also include disabled metrics with enabled: false
            const allMetrics = this.metrics.map(m => ({
                metric_id: m.id,
                enabled: this.configs[m.id]?.enabled ?? false,
                threshold: this.configs[m.id]?.threshold ?? m.default_threshold,
            }));

            const body = {
                name: this.profileName.trim() || `Profile_${Date.now()}`,
                description: `Auto-generated metric profile`,
                metrics: allMetrics,
            };

            let resp;
            if (this.selectedProfileId) {
                resp = await fetch(`/api/evaluation/profiles/${this.selectedProfileId}`, {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body),
                    credentials: 'same-origin',
                });
            } else {
                resp = await fetch('/api/evaluation/profiles', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body),
                    credentials: 'same-origin',
                });
            }

            if (resp.ok || resp.status === 201 || resp.status === 204) {
                await this.loadProfiles();
                if (!this.selectedProfileId) {
                    // Select the newly created profile
                    const newProfile = this.profiles.find(p => p.name === body.name);
                    if (newProfile) this.selectedProfileId = newProfile.id;
                }
                this.showToast('Profile saved', 'success');
            } else {
                const data = await resp.json().catch(() => ({}));
                this.errors.general = data.message || 'Failed to save profile';
            }
        } catch (err) {
            this.errors.general = 'Network error';
            console.error('Failed to save profile:', err);
        } finally {
            this.saving = false;
        }
    },

    showToast(message, type = 'info', duration = 3000) {
        // Use the global toast system if available
        if (window.dispatchEvent) {
            window.dispatchEvent(new CustomEvent('toast-show', {
                detail: { message, type, duration },
            }));
        }
    },
});
