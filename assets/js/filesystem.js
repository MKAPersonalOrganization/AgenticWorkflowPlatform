/**
 * filesystemManager — Alpine.js data component that drives the file manager
 * dialog.  Supports list / create / browse / permissions views with a
 * Windows Explorer / GitHub-like UI.
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
        selectedEntry: null,
        dragOver: false,

        // ── View mode & sorting ──────────────────────────────────────────────
        viewMode: 'list',   // 'list' | 'grid'
        sortBy: 'name',     // 'name' | 'size' | 'date'
        sortDir: 'asc',     // 'asc' | 'desc'

        // ── Navigation history (back/forward like a browser) ─────────────────
        history: [],
        historyIndex: -1,
        _navigating: false, // guard to prevent history push during back/fwd

        // ── Context menu ─────────────────────────────────────────────────────
        contextMenu: { show: false, x: 0, y: 0, entry: null },

        // ── Permissions state ────────────────────────────────────────────────
        permissions: [],
        permAgentId: '',

        // ── Computed ─────────────────────────────────────────────────────────
        get pathParts() {
            if (this.currentPath === '/') return [];
            return this.currentPath.replace(/^\//, '').replace(/\/$/, '').split('/');
        },

        /** Sorted entries: directories first, then files, then by sortBy */
        get sortedEntries() {
            const dirs = this.entries.filter(e => e.is_dir);
            const files = this.entries.filter(e => !e.is_dir);

            const cmp = (a, b) => {
                let va, vb;
                if (this.sortBy === 'name') {
                    va = a.name.toLowerCase();
                    vb = b.name.toLowerCase();
                    return this.sortDir === 'asc' ? va.localeCompare(vb) : vb.localeCompare(va);
                }
                if (this.sortBy === 'size') {
                    va = a.size || 0;
                    vb = b.size || 0;
                    return this.sortDir === 'asc' ? va - vb : vb - va;
                }
                if (this.sortBy === 'date') {
                    va = new Date(a.mod_time || 0).getTime();
                    vb = new Date(b.mod_time || 0).getTime();
                    return this.sortDir === 'asc' ? va - vb : vb - va;
                }
                return 0;
            };

            dirs.sort(cmp);
            files.sort(cmp);
            return [...dirs, ...files];
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
                    this.checkGitHubStatus();
                    this.fetchGitHubRepos();
                }
            };
            window.addEventListener('message', this._ghListener);

            // Close context menu on Escape
            this._escListener = (e) => {
                if (e.key === 'Escape') this.contextMenu.show = false;
            };
            window.addEventListener('keydown', this._escListener);
        },

        destroy() {
            if (this._ghListener) window.removeEventListener('message', this._ghListener);
            if (this._escListener) window.removeEventListener('keydown', this._escListener);
        },

        goBack() {
            if (this.view === 'browse' || this.view === 'permissions') {
                this.view = 'list';
                this.currentFS = null;
                this.selectedEntry = null;
                this.history = [];
                this.historyIndex = -1;
            } else {
                this.view = 'list';
            }
        },

        // ── Sorting ──────────────────────────────────────────────────────────

        setSortBy(field) {
            if (this.sortBy === field) {
                this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
            } else {
                this.sortBy = field;
                this.sortDir = 'asc';
            }
        },

        // ── Context Menu ─────────────────────────────────────────────────────

        openContextMenu(event, entry) {
            this.contextMenu = {
                show: true,
                x: event.clientX,
                y: event.clientY,
                entry: entry,
            };
            if (entry) this.selectedEntry = entry;
        },

        copyPath(entry) {
            if (!entry) return;
            const p = this.currentPath === '/' ? `/${entry.name}` : `${this.currentPath}/${entry.name}`;
            navigator.clipboard.writeText(p).catch(() => { });
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
                    if (data.connected) this.fetchGitHubRepos();
                }
            } catch (e) {
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
                if (resp.ok) this.githubRepos = await resp.json();
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
                if (resp.ok) this.filesystems = await resp.json();
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
                if (resp.ok || resp.status === 204) await this.fetchFilesystems();
                else {
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
            this.selectedEntry = null;
            this.history = ['/'];
            this.historyIndex = 0;
            this.fetchEntries();
        },

        async fetchEntries() {
            if (!this.currentFS) return;
            this.browseLoading = true;
            this.selectedEntry = null;
            try {
                const resp = await fetch(`/api/filesystems/${this.currentFS.id}/ls?path=${encodeURIComponent(this.currentPath)}`);
                if (resp.ok) this.entries = await resp.json();
            } catch (e) {
                console.error('Failed to fetch entries', e);
            } finally {
                this.browseLoading = false;
            }
        },

        /** Navigate to a path and push it to history */
        navigateTo(path) {
            if (path === this.currentPath) {
                this.fetchEntries();
                return;
            }
            this.currentPath = path;
            // Push to history (trim forward history)
            if (!this._navigating) {
                this.history = this.history.slice(0, this.historyIndex + 1);
                this.history.push(path);
                this.historyIndex = this.history.length - 1;
            }
            this.fetchEntries();
        },

        navigateToIndex(idx) {
            const parts = this.pathParts.slice(0, idx + 1);
            this.navigateTo('/' + parts.join('/'));
        },

        navigateHistoryBack() {
            if (this.historyIndex <= 0) return;
            this._navigating = true;
            this.historyIndex--;
            this.currentPath = this.history[this.historyIndex];
            this.fetchEntries();
            this._navigating = false;
        },

        navigateHistoryForward() {
            if (this.historyIndex >= this.history.length - 1) return;
            this._navigating = true;
            this.historyIndex++;
            this.currentPath = this.history[this.historyIndex];
            this.fetchEntries();
            this._navigating = false;
        },

        clickEntry(entry) {
            if (entry.is_dir) {
                const newPath = this.currentPath === '/' ? `/${entry.name}` : `${this.currentPath}/${entry.name}`;
                this.navigateTo(newPath);
            } else {
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
            await this._uploadFiles(files);
            event.target.value = '';
            await this.fetchEntries();
        },

        async handleDrop(event) {
            this.dragOver = false;
            const files = event.dataTransfer?.files;
            if (!files || files.length === 0) return;
            await this._uploadFiles(files);
            await this.fetchEntries();
        },

        async _uploadFiles(files) {
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
        },

        async deleteEntry(entry) {
            const entryPath = this.currentPath === '/' ? `/${entry.name}` : `${this.currentPath}/${entry.name}`;
            const kind = entry.is_dir ? 'directory' : 'file';
            if (!confirm(`Delete ${kind} "${entry.name}"?`)) return;
            try {
                const resp = await fetch(`/api/filesystems/${this.currentFS.id}/rm?path=${encodeURIComponent(entryPath)}`, { method: 'DELETE' });
                if (resp.ok || resp.status === 204) {
                    if (this.selectedEntry?.name === entry.name) this.selectedEntry = null;
                    await this.fetchEntries();
                } else {
                    const err = await resp.json();
                    alert(err.error || `Failed to delete ${kind}`);
                }
            } catch (e) {
                console.error('Failed to delete entry', e);
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
                if (resp.ok) this.permissions = await resp.json();
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
                if (resp.ok) await this.fetchPermissions();
            } catch (e) {
                console.error('Failed to revoke permission', e);
            }
        },

        // ── Helpers ──────────────────────────────────────────────────────────

        formatSize(bytes) {
            if (!bytes || bytes === 0) return '';
            const units = ['B', 'KB', 'MB', 'GB', 'TB'];
            let i = 0;
            let size = bytes;
            while (size >= 1024 && i < units.length - 1) {
                size /= 1024;
                i++;
            }
            return `${size.toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
        },

        formatDate(dateStr) {
            if (!dateStr) return '—';
            const d = new Date(dateStr);
            if (isNaN(d.getTime())) return '—';
            const now = new Date();
            const diff = now - d;

            // If less than 24 hours ago, show relative time
            if (diff < 86400000 && diff > 0) {
                if (diff < 60000) return 'Just now';
                if (diff < 3600000) return `${Math.floor(diff / 60000)} min ago`;
                return `${Math.floor(diff / 3600000)}h ago`;
            }

            // If this year, omit the year
            const opts = { month: 'short', day: 'numeric' };
            if (d.getFullYear() !== now.getFullYear()) opts.year = 'numeric';
            return d.toLocaleDateString('en-US', opts) + ', ' +
                d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false });
        },

        // ── File type icons ──────────────────────────────────────────────────
        // Returns an SVG string for the appropriate Lucide icon based on file extension.

        _iconCache: {},

        /** Get a colored SVG icon for the given filename */
        getFileIcon(name, large) {
            const ext = (name.split('.').pop() || '').toLowerCase();
            const size = large ? 'size-10' : 'size-5';
            const info = this._fileTypeInfo(ext);
            const cacheKey = `${info.icon}-${size}-${info.color}`;

            if (this._iconCache[cacheKey]) return this._iconCache[cacheKey];

            // Look up the icon element already rendered in the DOM or create inline SVG.
            // We use Lucide icon names and produce inline SVG with proper classes.
            const svg = this._lucideSvg(info.icon, size, info.color);
            this._iconCache[cacheKey] = svg;
            return svg;
        },

        /** Get folder icon SVG */
        getFolderIconSvg(large) {
            const size = large ? 'size-10' : 'size-5';
            const key = `folder-${size}`;
            if (this._iconCache[key]) return this._iconCache[key];
            const svg = this._lucideSvg('folder', size, 'text-yellow-500', 'fill-yellow-500/20');
            this._iconCache[key] = svg;
            return svg;
        },

        /** Map file extensions to icon names and colors */
        _fileTypeInfo(ext) {
            const map = {
                // Documents
                pdf: { icon: 'file-text', color: 'text-red-500' },
                doc: { icon: 'file-text', color: 'text-blue-600' },
                docx: { icon: 'file-text', color: 'text-blue-600' },
                txt: { icon: 'file-text', color: 'text-muted-foreground' },
                md: { icon: 'file-text', color: 'text-muted-foreground' },
                rtf: { icon: 'file-text', color: 'text-blue-500' },

                // Spreadsheets
                xls: { icon: 'file-spreadsheet', color: 'text-green-600' },
                xlsx: { icon: 'file-spreadsheet', color: 'text-green-600' },
                csv: { icon: 'file-spreadsheet', color: 'text-green-500' },

                // Code
                js: { icon: 'file-code', color: 'text-yellow-500' },
                ts: { icon: 'file-code', color: 'text-blue-500' },
                jsx: { icon: 'file-code', color: 'text-cyan-500' },
                tsx: { icon: 'file-code', color: 'text-blue-400' },
                py: { icon: 'file-code', color: 'text-green-500' },
                go: { icon: 'file-code', color: 'text-cyan-600' },
                rs: { icon: 'file-code', color: 'text-orange-500' },
                rb: { icon: 'file-code', color: 'text-red-500' },
                java: { icon: 'file-code', color: 'text-red-400' },
                c: { icon: 'file-code', color: 'text-blue-400' },
                cpp: { icon: 'file-code', color: 'text-blue-500' },
                h: { icon: 'file-code', color: 'text-purple-400' },
                cs: { icon: 'file-code', color: 'text-green-600' },
                php: { icon: 'file-code', color: 'text-indigo-400' },
                swift: { icon: 'file-code', color: 'text-orange-500' },
                kt: { icon: 'file-code', color: 'text-purple-500' },
                r: { icon: 'file-code', color: 'text-blue-500' },
                sql: { icon: 'file-code', color: 'text-orange-400' },
                sh: { icon: 'file-terminal', color: 'text-green-400' },
                bash: { icon: 'file-terminal', color: 'text-green-400' },
                zsh: { icon: 'file-terminal', color: 'text-green-400' },

                // Web
                html: { icon: 'file-code', color: 'text-orange-500' },
                htm: { icon: 'file-code', color: 'text-orange-500' },
                css: { icon: 'file-code', color: 'text-blue-500' },
                scss: { icon: 'file-code', color: 'text-pink-400' },
                less: { icon: 'file-code', color: 'text-blue-400' },
                vue: { icon: 'file-code', color: 'text-green-500' },
                svelte: { icon: 'file-code', color: 'text-orange-500' },

                // Data / Config
                json: { icon: 'file-braces', color: 'text-yellow-500' },
                yaml: { icon: 'file-code', color: 'text-red-400' },
                yml: { icon: 'file-code', color: 'text-red-400' },
                xml: { icon: 'file-code', color: 'text-orange-400' },
                toml: { icon: 'file-code', color: 'text-gray-500' },
                env: { icon: 'file-lock', color: 'text-yellow-600' },
                ini: { icon: 'file-code', color: 'text-gray-500' },

                // Images
                png: { icon: 'file-image', color: 'text-purple-500' },
                jpg: { icon: 'file-image', color: 'text-purple-500' },
                jpeg: { icon: 'file-image', color: 'text-purple-500' },
                gif: { icon: 'file-image', color: 'text-purple-400' },
                svg: { icon: 'file-image', color: 'text-orange-400' },
                webp: { icon: 'file-image', color: 'text-purple-400' },
                ico: { icon: 'file-image', color: 'text-blue-400' },
                bmp: { icon: 'file-image', color: 'text-purple-300' },

                // Audio
                mp3: { icon: 'file-music', color: 'text-pink-500' },
                wav: { icon: 'file-music', color: 'text-pink-400' },
                ogg: { icon: 'file-music', color: 'text-pink-400' },
                flac: { icon: 'file-music', color: 'text-pink-500' },
                aac: { icon: 'file-music', color: 'text-pink-400' },

                // Video
                mp4: { icon: 'file-video-camera', color: 'text-red-500' },
                webm: { icon: 'file-video-camera', color: 'text-red-400' },
                avi: { icon: 'file-video-camera', color: 'text-red-400' },
                mov: { icon: 'file-video-camera', color: 'text-red-400' },
                mkv: { icon: 'file-video-camera', color: 'text-red-500' },

                // Archives
                zip: { icon: 'file-archive', color: 'text-amber-600' },
                tar: { icon: 'file-archive', color: 'text-amber-600' },
                gz: { icon: 'file-archive', color: 'text-amber-600' },
                rar: { icon: 'file-archive', color: 'text-amber-600' },
                '7z': { icon: 'file-archive', color: 'text-amber-600' },

                // Lock / Key
                pem: { icon: 'file-key', color: 'text-yellow-600' },
                key: { icon: 'file-key', color: 'text-yellow-600' },
                crt: { icon: 'file-lock', color: 'text-green-500' },
                cert: { icon: 'file-lock', color: 'text-green-500' },
            };

            return map[ext] || { icon: 'file', color: 'text-muted-foreground' };
        },

        /**
         * Build an inline SVG string for a Lucide icon.
         * This uses the Lucide SVG path data that ships with the icon component.
         * We inline a simplified SVG for common icons to avoid DOM queries.
         */
        _lucideSvg(name, sizeClass, colorClass, fillClass) {
            // Lucide icon paths (simplified subset for file manager)
            const paths = {
                'file': '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/>',
                'file-text': '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M10 13H8"/><path d="M16 17H8"/><path d="M16 13h-2"/>',
                'file-code': '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="m10 13-2 2 2 2"/><path d="m14 17 2-2-2-2"/>',
                'file-image': '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><circle cx="10" cy="12" r="2"/><path d="m20 17-1.296-1.296a2.41 2.41 0 0 0-3.408 0L9 22"/>',
                'file-music': '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><circle cx="10" cy="18" r="2"/><path d="M12 18V9.5l4-1.5"/>',
                'file-video-camera': '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="m10 15.5 4-2.5-4-2.5z"/>',
                'file-archive': '<path d="M16 22h2a2 2 0 0 0 2-2V7l-5-5H6a2 2 0 0 0-2 2v18"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><circle cx="10" cy="20" r="2"/><path d="M10 7V6"/><path d="M10 12v-1"/><path d="M10 18v-2"/>',
                'file-spreadsheet': '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M8 13h2"/><path d="M14 13h2"/><path d="M8 17h2"/><path d="M14 17h2"/>',
                'file-braces': '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="M8 10a1 1 0 0 1 1-1h1a1 1 0 0 1 1 1v1a1 1 0 0 1-1 1 1 1 0 0 1 1 1v1a1 1 0 0 1-1 1H9a1 1 0 0 1-1-1"/><path d="M16 10a1 1 0 0 0-1-1h-1a1 1 0 0 0-1 1v1a1 1 0 0 0 1 1 1 1 0 0 0-1 1v1a1 1 0 0 0 1 1h1a1 1 0 0 0 1-1"/>',
                'file-terminal': '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><path d="m8 16 2-2-2-2"/><path d="M13 16h3"/>',
                'file-key': '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><circle cx="10" cy="16" r="2"/><path d="m16 10-4.5 4.5"/><path d="m15 11 1 1"/>',
                'file-lock': '<path d="M15 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V7Z"/><path d="M14 2v4a2 2 0 0 0 2 2h4"/><rect x="8" y="12" width="8" height="6" rx="1"/><path d="M10 12v-2a2 2 0 1 1 4 0v2"/>',
                'folder': '<path d="M20 20a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.9a2 2 0 0 1-1.69-.9L9.6 3.9A2 2 0 0 0 7.93 3H4a2 2 0 0 0-2 2v13a2 2 0 0 0 2 2Z"/>',
            };

            const fill = fillClass || 'fill-none';
            const d = paths[name] || paths['file'];
            return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" class="${sizeClass} ${colorClass} ${fill}">${d}</svg>`;
        },
    }));
});
