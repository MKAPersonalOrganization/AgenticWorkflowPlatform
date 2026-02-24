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

            for (let i = 0; i < files.length; i++) {
                const file = files[i];
                const reader = new FileReader();

                reader.onload = (e) => {
                    this.files.push({
                        name: file.name,
                        type: file.type,
                        size: file.size,
                        data: e.target.result // Base64 data
                    });
                };

                reader.readAsDataURL(file);
            }

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
