/**
 * filesystemManager — Alpine.js data component that drives the filesystem
 * dialog.  Supports list / create / browse / permissions views.
 */
document.addEventListener('alpine:init', () => {
    Alpine.data('filesystemManager', () => ({
        // ── State ────────────────────────────────────────────────────────────
        view: 'list', // 'list' | 'create' | 'browse' | 'permissions'
        loading: false,
        creating: false,
        browseLoading: false,
        permLoading: false,

        // ── Filesystem list ──────────────────────────────────────────────────
        filesystems: [],
        currentFS: null,

        // ── Create form ──────────────────────────────────────────────────────
        form: {
            name: '',
            description: '',
            type: 'local',
            github: { owner: '', repo: '', branch: 'main', selectedRepo: '' },
        },

        // ── GitHub OAuth state ───────────────────────────────────────────────
        githubStatus: { connected: false, username: '', avatar: '' },
        githubRepos: [],
        loadingRepos: false,

        // ── Browse state ─────────────────────────────────────────────────────
        currentPath: '/',
        entries: [],
        showMkdir: false,
        newDirName: '',

        // ── Permissions state ────────────────────────────────────────────────
        permissions: [],
        permAgentId: '',

        // ── Computed ─────────────────────────────────────────────────────────
        get pathParts() {
            if (this.currentPath === '/') return [];
            return this.currentPath.replace(/^\//, '').split('/');
        },

        // ── Lifecycle ────────────────────────────────────────────────────────
        init() {
            this.view = 'list';
            this.fetchFilesystems();
            this.checkGitHubStatus();

            // Listen for GitHub OAuth popup callback
            this._ghListener = (event) => {
                if (event.data && event.data.type === 'github-connected') {
                    this.githubStatus.connected = true;
                    this.githubStatus.username = event.data.username || '';
                    this.checkGitHubStatus(); // refresh full status
                    this.fetchGitHubRepos();
                }
            };
            window.addEventListener('message', this._ghListener);
        },

        destroy() {
            if (this._ghListener) {
                window.removeEventListener('message', this._ghListener);
            }
        },

        goBack() {
            if (this.view === 'browse' || this.view === 'permissions') {
                this.view = 'list';
                this.currentFS = null;
            } else {
                this.view = 'list';
            }
        },

        // ── GitHub OAuth ─────────────────────────────────────────────────────

        async checkGitHubStatus() {
            try {
                const resp = await fetch('/api/github/status');
                if (resp.ok) {
                    const data = await resp.json();
                    this.githubStatus = {
                        connected: data.connected,
                        username: data.username || '',
                        avatar: data.avatar || '',
                    };
                    if (data.connected) {
                        this.fetchGitHubRepos();
                    }
                }
            } catch (e) {
                // GitHub OAuth may not be configured — fail silently
                console.debug('GitHub status check failed', e);
            }
        },

        connectGitHub() {
            const w = 600, h = 700;
            const left = (screen.width - w) / 2;
            const top = (screen.height - h) / 2;
            window.open(
                '/api/github/connect',
                'github-oauth',
                `width=${w},height=${h},left=${left},top=${top},menubar=no,toolbar=no,status=no`
            );
        },

        async disconnectGitHub() {
            if (!confirm('Disconnect your GitHub account?')) return;
            try {
                await fetch('/api/github/disconnect', { method: 'DELETE' });
                this.githubStatus = { connected: false, username: '', avatar: '' };
                this.githubRepos = [];
                this.form.github = { owner: '', repo: '', branch: 'main', selectedRepo: '' };
            } catch (e) {
                console.error('Failed to disconnect GitHub', e);
            }
        },

        async fetchGitHubRepos() {
            this.loadingRepos = true;
            try {
                const resp = await fetch('/api/github/repos');
                if (resp.ok) {
                    this.githubRepos = await resp.json();
                }
            } catch (e) {
                console.error('Failed to fetch GitHub repos', e);
            } finally {
                this.loadingRepos = false;
            }
        },

        onRepoSelected() {
            const selected = this.githubRepos.find(r => r.full_name === this.form.github.selectedRepo);
            if (selected) {
                this.form.github.owner = selected.owner_login || selected.full_name.split('/')[0];
                this.form.github.repo = selected.name;
                this.form.github.branch = selected.default_branch || 'main';
            }
        },

        // ── Filesystem CRUD ──────────────────────────────────────────────────

        async fetchFilesystems() {
            this.loading = true;
            try {
                const resp = await fetch('/api/filesystems');
                if (resp.ok) {
                    this.filesystems = await resp.json();
                }
            } catch (e) {
                console.error('Failed to fetch filesystems', e);
            } finally {
                this.loading = false;
            }
        },

        async submitCreate() {
            this.creating = true;
            try {
                const body = {
                    name: this.form.name,
                    description: this.form.description,
                    type: this.form.type,
                };
                if (this.form.type === 'github') {
                    body.config = {
                        owner: this.form.github.owner,
                        repo: this.form.github.repo,
                        branch: this.form.github.branch || 'main',
                    };
                }

                const resp = await fetch('/api/filesystems', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(body),
                });

                if (resp.ok) {
                    this.form = { name: '', description: '', type: 'local', github: { owner: '', repo: '', branch: 'main', selectedRepo: '' } };
                    this.view = 'list';
                    await this.fetchFilesystems();
                } else {
                    const err = await resp.json();
                    alert(err.error || 'Failed to create filesystem');
                }
            } catch (e) {
                console.error('Failed to create filesystem', e);
            } finally {
                this.creating = false;
            }
        },

        async confirmDelete(fs) {
            if (!confirm(`Delete filesystem "${fs.name}"? This will remove all files and permissions.`)) return;

            try {
                const resp = await fetch(`/api/filesystems/${fs.id}`, { method: 'DELETE' });
                if (resp.ok || resp.status === 204) {
                    await this.fetchFilesystems();
                } else {
                    const err = await resp.json();
                    alert(err.error || 'Failed to delete filesystem');
                }
            } catch (e) {
                console.error('Failed to delete filesystem', e);
            }
        },

        // ── Browse ───────────────────────────────────────────────────────────

        browseFS(fs) {
            this.currentFS = fs;
            this.currentPath = '/';
            this.view = 'browse';
            this.fetchEntries();
        },

        async fetchEntries() {
            if (!this.currentFS) return;
            this.browseLoading = true;
            try {
                const resp = await fetch(`/api/filesystems/${this.currentFS.id}/ls?path=${encodeURIComponent(this.currentPath)}`);
                if (resp.ok) {
                    this.entries = await resp.json();
                }
            } catch (e) {
                console.error('Failed to fetch entries', e);
            } finally {
                this.browseLoading = false;
            }
        },

        navigateTo(path) {
            this.currentPath = path;
            this.fetchEntries();
        },

        navigateToIndex(idx) {
            const parts = this.pathParts.slice(0, idx + 1);
            this.currentPath = '/' + parts.join('/');
            this.fetchEntries();
        },

        clickEntry(entry) {
            if (entry.is_dir) {
                const newPath = this.currentPath === '/' ? `/${entry.name}` : `${this.currentPath}/${entry.name}`;
                this.navigateTo(newPath);
            } else {
                // Download/view file
                const filePath = this.currentPath === '/' ? `/${entry.name}` : `${this.currentPath}/${entry.name}`;
                window.open(`/api/filesystems/${this.currentFS.id}/read?path=${encodeURIComponent(filePath)}`, '_blank');
            }
        },

        async createDir() {
            if (!this.newDirName.trim()) return;
            const dirPath = this.currentPath === '/' ? `/${this.newDirName}` : `${this.currentPath}/${this.newDirName}`;

            try {
                const resp = await fetch(`/api/filesystems/${this.currentFS.id}/mkdir`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ path: dirPath }),
                });
                if (resp.ok || resp.status === 201) {
                    this.newDirName = '';
                    this.showMkdir = false;
                    await this.fetchEntries();
                } else {
                    const err = await resp.json();
                    alert(err.error || 'Failed to create directory');
                }
            } catch (e) {
                console.error('Failed to create directory', e);
            }
        },

        async handleFileUpload(event) {
            const files = event.target.files;
            if (!files || files.length === 0) return;

            for (const file of files) {
                const filePath = this.currentPath === '/' ? `/${file.name}` : `${this.currentPath}/${file.name}`;
                const formData = new FormData();
                formData.append('file', file);
                formData.append('path', filePath);

                try {
                    await fetch(`/api/filesystems/${this.currentFS.id}/upload`, {
                        method: 'POST',
                        body: formData,
                    });
                } catch (e) {
                    console.error(`Failed to upload ${file.name}`, e);
                }
            }

            // Reset input and refresh
            event.target.value = '';
            await this.fetchEntries();
        },

        async deleteEntry(entry) {
            const entryPath = this.currentPath === '/' ? `/${entry.name}` : `${this.currentPath}/${entry.name}`;
            const kind = entry.is_dir ? 'directory' : 'file';
            if (!confirm(`Delete ${kind} "${entry.name}"?`)) return;

            try {
                const resp = await fetch(`/api/filesystems/${this.currentFS.id}/rm?path=${encodeURIComponent(entryPath)}`, { method: 'DELETE' });
                if (resp.ok || resp.status === 204) {
                    await this.fetchEntries();
                } else {
                    const err = await resp.json();
                    alert(err.error || `Failed to delete ${kind}`);
                }
            } catch (e) {
                console.error(`Failed to delete entry`, e);
            }
        },

        // ── Permissions ──────────────────────────────────────────────────────

        openPermissions(fs) {
            this.currentFS = fs;
            this.view = 'permissions';
            this.fetchPermissions();
        },

        async fetchPermissions() {
            if (!this.currentFS) return;
            this.permLoading = true;
            try {
                const resp = await fetch(`/api/filesystems/${this.currentFS.id}/permissions`);
                if (resp.ok) {
                    this.permissions = await resp.json();
                }
            } catch (e) {
                console.error('Failed to fetch permissions', e);
            } finally {
                this.permLoading = false;
            }
        },

        async grantPerm() {
            if (!this.permAgentId.trim()) return;

            try {
                const resp = await fetch(`/api/filesystems/${this.currentFS.id}/permissions`, {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ agent_id: this.permAgentId.trim() }),
                });
                if (resp.ok) {
                    this.permAgentId = '';
                    await this.fetchPermissions();
                } else {
                    const err = await resp.json();
                    alert(err.error || 'Failed to grant permission');
                }
            } catch (e) {
                console.error('Failed to grant permission', e);
            }
        },

        async revokePerm(agentId) {
            if (!confirm(`Revoke access for agent "${agentId}"?`)) return;

            try {
                const resp = await fetch(`/api/filesystems/${this.currentFS.id}/permissions/${agentId}`, { method: 'DELETE' });
                if (resp.ok) {
                    await this.fetchPermissions();
                }
            } catch (e) {
                console.error('Failed to revoke permission', e);
            }
        },

        // ── Helpers ──────────────────────────────────────────────────────────

        formatSize(bytes) {
            if (!bytes || bytes === 0) return '';
            const units = ['B', 'KB', 'MB', 'GB'];
            let i = 0;
            let size = bytes;
            while (size >= 1024 && i < units.length - 1) {
                size /= 1024;
                i++;
            }
            return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
        },
    }));
});
