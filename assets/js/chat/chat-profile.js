/**
 * Chat Profile & Banner
 *
 * User profile CRUD and global banner management.
 *
 * Alpine.js patterns applied:
 * - this.profile is a reactive object — Alpine re-renders bound inputs on change
 * - Optimistic UI: profile fields are set immediately, errors show via toast
 * - Banner state drives conditional rendering in templates
 */

/**
 * Create profile and banner methods mixin
 * @returns {Object} Methods for profile and banner management
 */
export function createProfileMethods() {
    return {
        /**
         * Toggle settings panel and load profile
         */
        toggleSettings() {
            this.showSettings = !this.showSettings;
            if (this.showSettings) {
                this.fetchProfile();
            }
        },

        /**
         * Fetch user profile from server
         */
        async fetchProfile() {
            this.profileLoading = true;
            try {
                const response = await fetch('/api/profile');
                if (response.ok) {
                    const data = await response.json();
                    this.profile = {
                        ...data,
                        struggles: Array.isArray(data.struggles) ? data.struggles.join(', ') : (data.struggles || ''),
                        strengths: Array.isArray(data.strengths) ? data.strengths.join(', ') : (data.strengths || '')
                    };
                }
            } catch (error) {
                console.error('Failed to fetch profile:', error);
                this.showToast(window.i18n.t('Failed to load profile'), 'error');
            } finally {
                this.profileLoading = false;
            }
        },

        /**
         * Update user profile on server
         */
        async updateProfile() {
            try {
                const response = await fetch('/api/profile', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(this.profile)
                });
                if (response.ok) {
                    this.showToast(window.i18n.t('Profile updated successfully'), 'success');
                } else {
                    this.showToast(window.i18n.t('Failed to update profile'), 'error');
                }
            } catch (error) {
                console.error('Failed to update profile:', error);
                this.showToast(window.i18n.t('Failed to update profile'), 'error');
            }
        },

        /**
         * Fetch global banner from server
         */
        async fetchBanner() {
            try {
                const response = await fetch('/api/banner');
                console.log('[Banner] Fetching banner...', response.status);
                if (response.ok) {
                    const data = await response.json();
                    console.log('[Banner] Data:', data);
                    console.log('[Banner] Dismissed IDs:', this.bannerDismissedIds);
                    this.banner = data;
                }
            } catch (error) {
                console.error('Failed to fetch banner:', error);
            }
        },

        /**
         * Dismiss the current banner
         */
        dismissBanner() {
            if (this.banner && !this.banner.persistable) {
                this.bannerDismissedIds.push(this.banner.id);
                localStorage.setItem('bannerDismissedIds', JSON.stringify(this.bannerDismissedIds));
                this.banner = null;
            }
        },
    };
}
