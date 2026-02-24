/**
 * Conversation Search Dialog Component
 * Handles searching through conversation history
 */
export const convSearchDialog = () => ({
    results: [],
    loading: false,
    query: '',
    debounceTimer: null,

    async init() {
        this.results = [];
        this.query = '';
        this.$watch('query', (val) => {
            clearTimeout(this.debounceTimer);
            if (!val.trim()) {
                this.results = [];
                return;
            }
            this.debounceTimer = setTimeout(() => this.search(), 300);
        });
    },

    async search() {
        const q = this.query.trim();
        if (!q) {
            this.results = [];
            return;
        }
        this.loading = true;
        try {
            const response = await fetch(`/api/agui/threads?q=${encodeURIComponent(q)}&limit=20`);
            if (response.ok) {
                const data = await response.json();
                this.results = data.threads || [];
            }
        } catch (err) {
            console.error('Failed to search conversations:', err);
        } finally {
            this.loading = false;
        }
    },

    selectThread(threadId) {
        const chatEl = document.querySelector('[x-data="chat"]');
        if (chatEl) {
            try {
                const chatData = Alpine.$data(chatEl);
                if (chatData && typeof chatData.switchSession === 'function') {
                    chatData.switchSession(threadId);
                }
            } catch (e) {
                console.error('Failed to switch session:', e);
            }
        }
        // Close the dialog
        const closeBtn = document.querySelector('[data-tui-dialog-close="conv-search-dialog"]');
        if (closeBtn) closeBtn.click();
    },

    formatDate(dateStr) {
        const date = new Date(dateStr);
        const now = new Date();
        const diff = now - date;
        const days = Math.floor(diff / (1000 * 60 * 60 * 24));
        if (days === 0) return 'Today';
        if (days === 1) return 'Yesterday';
        if (days < 7) return days + 'd ago';
        return date.toLocaleDateString();
    }
});
