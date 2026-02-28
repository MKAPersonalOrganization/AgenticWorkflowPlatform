/**
 * Chat Input Composition
 *
 * Slash commands, file attachments, and input field management.
 *
 * Alpine.js patterns applied:
 * - x-model binding on currentMessage drives the textarea
 * - $refs.messageInput for focus management after prompt insertion
 * - filteredPrompts is a reactive array — Alpine auto-tracks mutations
 * - showSlashMenu toggles the command palette overlay
 */

/**
 * Create input composition methods mixin
 * @returns {Object} Methods for input handling
 */
export function createInputMethods() {
    return {
        // ============================================
        // File Management
        // ============================================

        /** Max file size: 20 MB */
        MAX_FILE_SIZE: 20 * 1024 * 1024,

        /** Max number of files per message */
        MAX_FILE_COUNT: 5,

        /** Accepted MIME types for file attachments */
        ACCEPTED_FILE_TYPES: [
            'image/jpeg', 'image/png', 'image/gif', 'image/webp',
            'audio/wav', 'audio/x-wav', 'audio/mpeg', 'audio/mp3', 'audio/ogg', 'audio/webm',
            'application/pdf',
            'text/plain', 'text/csv', 'text/markdown',
        ],

        /**
         * Validate a file by type and size.
         * Shows a user-visible toast on failure.
         * @param {File} file
         * @returns {boolean}
         */
        _isValidFile(file) {
            if (!this.ACCEPTED_FILE_TYPES.includes(file.type)) {
                this.showToast(window.i18n?.t?.('Unsupported file type: {type}', { type: file.type }) || `Unsupported file type: ${file.type}`, 'error');
                return false;
            }
            if (file.size > this.MAX_FILE_SIZE) {
                const sizeMB = (file.size / 1024 / 1024).toFixed(1);
                this.showToast(window.i18n?.t?.('File too large ({size} MB). Max is 20 MB.', { size: sizeMB }) || `File too large (${sizeMB} MB). Max is 20 MB.`, 'error');
                return false;
            }
            return true;
        },

        /**
         * Read files and add them to the files[] array.
         * Enforces MAX_FILE_COUNT per message.
         * @param {File[]} fileList
         */
        _addFiles(fileList) {
            for (const file of fileList) {
                if (this.files.length >= this.MAX_FILE_COUNT) {
                    this.showToast(window.i18n?.t?.('Maximum {count} files per message', { count: this.MAX_FILE_COUNT }) || `Maximum ${this.MAX_FILE_COUNT} files per message`, 'error');
                    break;
                }
                if (!this._isValidFile(file)) continue;

                const reader = new FileReader();
                reader.onload = (e) => {
                    this.files.push({
                        name: file.name || `clipboard_${Date.now()}.${file.type.split('/')[1]}`,
                        type: file.type,
                        size: file.size,
                        data: e.target.result // Base64 data URL
                    });
                };
                reader.readAsDataURL(file);
            }
        },

        /**
         * Handle paste events on the textarea (images from clipboard)
         * @param {ClipboardEvent} event
         */
        handlePaste(event) {
            const clipboardData = event.clipboardData;
            if (!clipboardData || clipboardData.files.length === 0) return;

            // Filter to supported file types
            const validFiles = Array.from(clipboardData.files).filter(f => this.ACCEPTED_FILE_TYPES.includes(f.type));
            if (validFiles.length === 0) return;

            // Prevent the default paste (which would insert weird text)
            event.preventDefault();
            this._addFiles(validFiles);
        },

        /**
         * Handle drop events on the chat input area
         * @param {DragEvent} event
         */
        handleDrop(event) {
            event.preventDefault();
            const dt = event.dataTransfer;
            if (!dt || dt.files.length === 0) return;

            const validFiles = Array.from(dt.files).filter(f => this.ACCEPTED_FILE_TYPES.includes(f.type));
            if (validFiles.length === 0) return;

            this._addFiles(validFiles);
        },

        /**
         * Handle dragover to allow drop
         * @param {DragEvent} event
         */
        handleDragOver(event) {
            event.preventDefault();
        },

        /**
         * Remove a file from the attachment list
         * @param {number} index - Index of file to remove
         */
        removeFile(index) {
            if (index >= 0 && index < this.files.length) {
                this.files.splice(index, 1);
            }
        },

        /**
         * Handle file selection from input
         * @param {Event} event - Change event
         */
        handleFileSelect(event) {
            const files = event.target.files;
            if (!files || files.length === 0) return;

            const validFiles = Array.from(files).filter(f => this._isValidFile(f));
            this._addFiles(validFiles);

            // Reset input so same file can be selected again
            event.target.value = '';
        },

        // ============================================
        // Slash Command Methods
        // ============================================

        /**
         * Handle input changes to detect slash commands
         */
        handleInput(event) {
            const value = event.target.value;
            const slashIndex = value.lastIndexOf('/');

            // If slash is found and is either at start or preceded by newline/space
            if (slashIndex !== -1 && (slashIndex === 0 || /[\s\n]/.test(value[slashIndex - 1]))) {
                const query = value.substring(slashIndex + 1);
                // Only show if query doesn't contain spaces (command mode)
                if (!/\s/.test(query)) {
                    this.slashQuery = query;
                    this.filteredPrompts = this.prompts.filter(p =>
                        p.command.toLowerCase().includes('/' + query.toLowerCase()) ||
                        p.description.toLowerCase().includes(query.toLowerCase())
                    );

                    if (this.filteredPrompts.length > 0) {
                        this.showSlashMenu = true;
                        this.slashMenuIndex = 0; // Reset selection
                        return;
                    }
                }
            }

            this.showSlashMenu = false;
        },

        /**
         * Handle keyboard navigation for slash menu
         */
        handleSlashKeydown(event) {
            if (!this.showSlashMenu) return false; // Not handled

            switch (event.key) {
                case 'ArrowUp':
                    event.preventDefault();
                    this.slashMenuIndex = Math.max(0, this.slashMenuIndex - 1);
                    return true;
                case 'ArrowDown':
                    event.preventDefault();
                    this.slashMenuIndex = Math.min(this.filteredPrompts.length - 1, this.slashMenuIndex + 1);
                    return true;
                case 'Enter':
                case 'Tab':
                    event.preventDefault();
                    if (this.filteredPrompts[this.slashMenuIndex]) {
                        this.selectPrompt(this.filteredPrompts[this.slashMenuIndex]);
                    }
                    return true;
                case 'Escape':
                    this.showSlashMenu = false;
                    return true;
            }
            return false;
        },

        /**
         * Insert selected prompt into message
         */
        selectPrompt(prompt) {
            const value = this.currentMessage;
            const slashIndex = value.lastIndexOf('/');

            if (slashIndex !== -1) {
                // Replace everything from the last slash with the prompt content
                this.currentMessage = value.substring(0, slashIndex) + prompt.content;
                this.showSlashMenu = false;
            } else {
                // Direct use (e.g. from Side Panel)
                this.currentMessage = prompt.content;
            }

            // Focus back on textarea (handled by x-model, but ensure focus)
            this.$nextTick(() => {
                // Check if refs exist before accessing
                if (this.$refs.messageInput) {
                    this.$refs.messageInput.focus();
                }
            });
        },
    };
}
