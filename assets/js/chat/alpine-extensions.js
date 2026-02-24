/**
 * Alpine.js Chat Plugin
 *
 * Registers all chat-specific Alpine extensions using Alpine's plugin pattern.
 * Alpine.plugin() is the idiomatic way to extend Alpine — it simply passes the
 * Alpine instance to a callback, giving it access to Alpine.store(), Alpine.magic(), etc.
 * See: packages/alpinejs/src/plugin.js
 *
 * Design principles (from Alpine.js source):
 * - Stores are for truly global cross-component state only
 * - Keep the API surface minimal — don't duplicate component methods
 * - Use Alpine.plugin() instead of scattered alpine:init listeners
 */

/**
 * Chat plugin — registers stores and debug utilities.
 *
 * Usage (inside alpine:init or before Alpine.start()):
 *   Alpine.plugin(chatPlugin)
 *
 * @param {Object} Alpine - The Alpine.js instance (injected by Alpine.plugin)
 */
export function chatPlugin(Alpine) {
    // ── Global Store ────────────────────────────────────────────────────────
    // Only stores truly global settings that outlive any single component.
    // Alpine.store() auto-wraps the object in reactive() — no manual setup needed.
    // Properties with init() are auto-called by Alpine after store registration.
    Alpine.store('chatSettings', {
        theme: localStorage.getItem('theme') ||
            (window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'),

        soundEnabled: true,
        notificationsEnabled: true,
        showTimestamps: true,
        compactMode: false,

        toggleSound() { this.soundEnabled = !this.soundEnabled; },
        toggleNotifications() { this.notificationsEnabled = !this.notificationsEnabled; },
        toggleCompactMode() { this.compactMode = !this.compactMode; },
    });

    // ── Debug Utilities (development only) ──────────────────────────────────
    // Uses Alpine.$data() — the canonical way to access component scope from outside.
    if (typeof window !== 'undefined') {
        window.__chatDebug = {
            getComponent() {
                const el = document.querySelector('[x-data*="chat"]');
                return el ? Alpine.$data(el) : null;
            },
            getMetrics() { return this.getComponent()?.getMetrics?.() ?? null; },
            getHealth() { return this.getComponent()?.getConnectionHealth?.() ?? null; },
            exportDiagnostics() { return this.getComponent()?.exportDiagnostics?.() ?? null; },
            resetMetrics() { return this.getComponent()?.resetMetrics?.(); },
            getSettings() { return Alpine.store('chatSettings'); },
            isHealthy() { return this.getComponent()?.isHealthy?.() ?? false; },
        };

        console.log('[Chat] Debug: window.__chatDebug available');
    }
}
