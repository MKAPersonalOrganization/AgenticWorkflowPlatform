/**
 * Settings Manager
 *
 * Alpine component backing the global Settings dialog (frontend/ui/nav/settings.templ).
 * Owns all state the dialog's inputs bind to (showSettings, profile, preferences,
 * sessions) so the dialog works on any BaseLayout page — not just /chat, whose
 * x-data="chat" scope historically supplied these.
 *
 * Preferences persist to localStorage; chat-state.js reads the same keys on
 * init so toggles made from any page survive navigation.
 */
export const settingsManager = () => ({
    showSettings: false,

    profile: {
        learning_style: '',
        struggles: '',
        strengths: '',
        goals: ''
    },
    profileLoading: false,

    preferences: {
        markdownEnabled: localStorage.getItem('markdownEnabled') !== 'false',
        autoScroll: localStorage.getItem('autoScroll') !== 'false',
        theme: localStorage.getItem('theme') || 'system',
        prefersReducedMotion: window.matchMedia('(prefers-reduced-motion: reduce)').matches,
        textToSpeechEnabled: localStorage.getItem('textToSpeechEnabled') === 'true'
    },

    // Local placeholder so the "Clear conversations" button's `sessions = []`
    // assignment doesn't ReferenceError on pages without a chat scope. The
    // button's canonical behavior lives in chat scope; on other pages it is a
    // harmless no-op.
    sessions: [],

    init() {
        this.$watch('preferences.markdownEnabled', (v) => localStorage.setItem('markdownEnabled', v ? 'true' : 'false'));
        this.$watch('preferences.autoScroll',      (v) => localStorage.setItem('autoScroll',      v ? 'true' : 'false'));
        this.$watch('preferences.textToSpeechEnabled', (v) => localStorage.setItem('textToSpeechEnabled', v ? 'true' : 'false'));
        this.$watch('preferences.theme', (v) => {
            localStorage.setItem('theme', v);
            // Keep document theme class in sync without forcing a reload.
            const root = document.documentElement;
            const systemDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
            const dark = v === 'dark' || (v === 'system' && systemDark);
            root.classList.toggle('dark', dark);
        });
    },

    async fetchProfile() {
        this.profileLoading = true;
        try {
            const r = await fetch('/api/profile');
            if (r.ok) {
                const d = await r.json();
                this.profile = {
                    ...d,
                    struggles: Array.isArray(d.struggles) ? d.struggles.join(', ') : (d.struggles || ''),
                    strengths: Array.isArray(d.strengths) ? d.strengths.join(', ') : (d.strengths || '')
                };
            }
        } catch (e) {
            console.error('Failed to load profile:', e);
        } finally {
            this.profileLoading = false;
        }
    },

    async updateProfile() {
        try {
            const r = await fetch('/api/profile', {
                method: 'PUT',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(this.profile)
            });
            const ok = r.ok;
            // showToast lives on the chat scope and isn't reachable from this
            // global component; fall back to a window event so any toast host
            // on the page can surface it, and a console log otherwise.
            window.dispatchEvent(new CustomEvent('show-toast', {
                detail: {
                    message: ok ? 'Profile updated successfully' : 'Failed to update profile',
                    type: ok ? 'success' : 'error'
                }
            }));
            if (!ok) console.warn('Profile update failed:', r.status);
        } catch (e) {
            console.error('Failed to update profile:', e);
        }
    }
});
