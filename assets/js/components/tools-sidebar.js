/**
 * Tools Sidebar Component
 * Handles plugin browsing and inline profile/management display.
 * Everything stays within the SPA — no full-page navigations.
 *
 * Sidebar: Search & browse only (like VS Code / IntelliJ extensions sidebar).
 * Detail panel: Opens in the editor area with profile info + enable/disable + per-tool toggles.
 */
export const toolsSidebar = () => ({
    open: localStorage.getItem('toolsSidebarOpen') === 'true',
    plugins: [],
    loading: false,
    search: '',
    _loaded: false,

    /** The plugin whose detail panel is open (null = closed) */
    profilePlugin: null,
    /** Active tab in the detail panel: 'details' | 'tools' */
    profileTab: 'details',

    init() {
        // Restore tools sidebar — if it was open, load plugins eagerly
        if (this.open && !this._loaded) {
            this.fetchPlugins().then(() => { this._loaded = true; });
            // Also collapse thread-history-sidebar since tools sidebar is open
            const wrapper = document.querySelector('[data-tui-sidebar-wrapper][data-tui-sidebar-id="thread-history-sidebar"]');
            if (wrapper && wrapper.getAttribute('data-tui-sidebar-state') === 'expanded') {
                wrapper.setAttribute('data-tui-sidebar-state', 'collapsed');
            }
        }

        // Refresh plugins when MCP connection state changes
        window.addEventListener('mcp-state-changed', () => {
            this.fetchPlugins();
        });

        // When thread-history-sidebar opens, close tools sidebar
        const threadSidebar = document.querySelector('[data-tui-sidebar-wrapper][data-tui-sidebar-id="thread-history-sidebar"]');
        if (threadSidebar) {
            new MutationObserver((mutations) => {
                for (const m of mutations) {
                    if (m.attributeName === 'data-tui-sidebar-state' &&
                        m.target.getAttribute('data-tui-sidebar-state') === 'expanded' &&
                        this.open) {
                        this.open = false;
                        localStorage.setItem('toolsSidebarOpen', 'false');
                    }
                }
            }).observe(threadSidebar, {
                attributes: true,
                attributeFilter: ['data-tui-sidebar-state']
            });
        }
    },

    async toggle() {
        this.open = !this.open;
        localStorage.setItem('toolsSidebarOpen', this.open ? 'true' : 'false');
        if (this.open) {
            // Collapse thread-history-sidebar
            const wrapper = document.querySelector('[data-tui-sidebar-wrapper][data-tui-sidebar-id="thread-history-sidebar"]');
            if (wrapper && wrapper.getAttribute('data-tui-sidebar-state') === 'expanded') {
                wrapper.setAttribute('data-tui-sidebar-state', 'collapsed');
            }
            if (!this._loaded) {
                await this.fetchPlugins();
                this._loaded = true;
            }
        } else {
            // When closing the sidebar, also close the detail dialog
            this.closeProfile();
        }
    },

    async fetchPlugins() {
        this.loading = true;
        try {
            const response = await fetch('/v1/plugins');
            if (response.ok) {
                const data = await response.json();
                if (data.plugins && Array.isArray(data.plugins)) {
                    this.plugins = data.plugins;
                }
            } else {
                this.plugins = [];
            }
        } catch (err) {
            console.error('Failed to fetch plugins:', err);
            this.plugins = [];
        } finally {
            this.loading = false;
        }
    },

    get filteredPlugins() {
        if (!this.search) return this.plugins;
        const q = this.search.toLowerCase();
        return this.plugins.filter(plugin =>
            plugin.name.toLowerCase().includes(q) ||
            plugin.description.toLowerCase().includes(q) ||
            plugin.category.toLowerCase().includes(q) ||
            plugin.tools.some(tool =>
                tool.name.toLowerCase().includes(q) ||
                tool.description.toLowerCase().includes(q)
            )
        );
    },

    get pluginSummary() {
        let totalTools = 0;
        this.plugins.forEach(p => {
            totalTools += (p.tools || []).length;
        });
        return {
            plugins: this.plugins.length,
            tools: totalTools,
        };
    },

    // ── Navigation ──────────────────────────────────────────────────────

    /** Show the plugin detail dialog */
    openProfile(plugin) {
        this.profilePlugin = plugin;
        this.profileTab = 'details';
        // Open the dialog via the templui dialog API
        this.$nextTick(() => {
            if (window.tui?.dialog) {
                window.tui.dialog.open('plugin-profile-dialog');
            }
        });
    },

    /** Close the detail dialog */
    closeProfile() {
        if (window.tui?.dialog) {
            window.tui.dialog.close('plugin-profile-dialog');
        }
        this.profilePlugin = null;
    },

});
