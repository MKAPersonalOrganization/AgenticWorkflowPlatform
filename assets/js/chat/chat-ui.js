/**
 * Chat UI Actions
 *
 * Handles UI interactions including:
 * - Toggle actions (sidebar, debug panel, etc.)
 * - Toast notifications
 * - Clipboard operations
 * - Event log display
 * - Error display
 * - Scroll management
 */

/**
 * Create UI action methods mixin
 * @returns {Object} Methods for UI actions
 */
export function createUIMethods() {
    return {
        // ============================================
        // Toggle Actions
        // ============================================

        toggle() {
            this.sidebarOpen = !this.sidebarOpen;
        },

        toggleDebugPanel() {
            this.showDebugPanel = !this.showDebugPanel;
        },

        toggleKeyboardShortcuts() {
            this.showKeyboardShortcuts = !this.showKeyboardShortcuts;
        },

        toggleSettings() {
            this.showSettings = !this.showSettings;
            if (this.showSettings) {
                this.fetchProfile();
            }
        },

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
                this.showToast('Failed to load profile', 'error');
            } finally {
                this.profileLoading = false;
            }
        },

        async updateProfile() {
            try {
                const response = await fetch('/api/profile', {
                    method: 'PUT',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(this.profile)
                });
                if (response.ok) {
                    this.showToast('Profile updated successfully', 'success');
                } else {
                    this.showToast('Failed to update profile', 'error');
                }
            } catch (error) {
                console.error('Failed to update profile:', error);
                this.showToast('Failed to update profile', 'error');
            }
        },

        toggleEventLog() {
            this.showEventLog = !this.showEventLog;
            if (this.showEventLog) {
                this.$nextTick(() => {
                    const logContainer = this.$refs?.eventLogContainer;
                    if (logContainer?.isConnected) {
                        logContainer.scrollTop = logContainer.scrollHeight;
                    }
                });
            }
        },

        // ============================================
        // Toast Notification System
        // ============================================

        /**
         * Show a toast notification
         * @param {string} message - The message to display
         * @param {string} type - Type: 'success', 'error', 'info', 'warning'
         * @param {number} duration - Duration in ms (default: 4000, 0 = persistent)
         * @returns {number} Toast ID for manual dismissal
         */
        showToast(message, type = 'info', duration = 4000) {
            const id = ++this.__toastIdCounter;
            const toast = {
                id,
                message,
                type,
                visible: true,
                leaving: false
            };

            this.toasts.push(toast);

            if (duration > 0) {
                setTimeout(() => {
                    this.dismissToast(id);
                }, duration);
            }

            this.__announce(message, type === 'error' ? 'assertive' : 'polite');

            return id;
        },

        /**
         * Dismiss a toast notification
         * @param {number} id - Toast ID to dismiss
         */
        dismissToast(id) {
            const toast = this.toasts.find(t => t.id === id);
            if (toast && !toast.leaving) {
                toast.leaving = true;
                setTimeout(() => {
                    this.toasts = this.toasts.filter(t => t.id !== id);
                }, 200);
            }
        },

        /**
         * Get toast icon based on type
         * @param {string} type - Toast type
         * @returns {string} SVG icon HTML
         */
        getToastIcon(type) {
            switch (type) {
                case 'success':
                    return '<svg class="w-5 h-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"></path></svg>';
                case 'error':
                    return '<svg class="w-5 h-5 text-destructive" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>';
                case 'warning':
                    return '<svg class="w-5 h-5 text-accent-foreground" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"></path></svg>';
                default:
                    return '<svg class="w-5 h-5 text-primary" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>';
            }
        },

        /**
         * Copy text to clipboard with toast feedback
         * @param {string} text - Text to copy
         * @param {string} successMessage - Message to show on success
         * @returns {Promise<boolean>} Success status
         */
        async copyToClipboard(text, successMessage = 'Copied to clipboard') {
            try {
                await navigator.clipboard.writeText(text);
                this.showToast(successMessage, 'success', 2000);
                return true;
            } catch (error) {
                console.error('Failed to copy:', error);
                this.showToast('Failed to copy to clipboard', 'error');
                return false;
            }
        },

        /**
         * Copy message content to clipboard
         * @param {string} messageId - ID of the message to copy
         */
        async copyMessage(messageId) {
            const state = this.__getThreadState();
            const message = state.messages.find(m => m.id === messageId);
            if (message) {
                const content = message.content || message.args || '';
                await this.copyToClipboard(content, 'Message copied');
            }
        },

        /**
         * Copy code block content
         * @param {Event} event - Click event from code block
         */
        async copyCodeBlock(event) {
            const button = event.currentTarget;
            const codeBlock = button.closest('pre')?.querySelector('code');
            if (codeBlock) {
                await this.copyToClipboard(codeBlock.textContent, 'Code copied');
            }
        },

        // ============================================
        // Event Log
        // ============================================

        /**
         * Add event to log
         * @param {Object} event - Event to add
         * @param {string} threadId - Thread ID
         */
        __addEvent(event, threadId) {
            const state = this.__getThreadState(threadId);

            state.events.push({
                id: `event_${Date.now()}_${Math.random()}`,
                type: event.type,
                time: new Date().toLocaleTimeString('en-US', {
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit',
                    fractionalSecondDigits: 3
                }),
                data: event
            });
        },

        /**
         * Get event details for display
         * @param {Object} event - Event object
         * @returns {string} Event details
         */
        getEventDetails(event) {
            switch (event.type) {
                case 'RUN_STARTED': return `Run ${event.runId?.slice(-8)}`;
                case 'TEXT_MESSAGE_START': return `Message ${event.messageId?.slice(-8)}`;
                case 'TEXT_MESSAGE_CONTENT': return `${event.delta?.length || 0} chars`;
                case 'TEXT_MESSAGE_END': return `Complete`;
                case 'RUN_FINISHED': return `Complete`;
                case 'RUN_ERROR': return `Error: ${event.code}`;
                default: return '';
            }
        },

        /**
         * Get event type color class
         * @param {string} type - Event type
         * @returns {string} CSS class
         */
        getEventTypeColor(type) {
            if (type.startsWith('RUN_')) return 'text-primary';
            if (type.startsWith('TEXT_MESSAGE_')) return 'text-primary';
            if (type.startsWith('TOOL_CALL_')) return 'text-accent-foreground';
            if (type.startsWith('STATE_')) return 'text-accent-foreground';
            if (type.startsWith('STEP_')) return 'text-accent-foreground';
            return 'text-muted-foreground';
        },

        /**
         * Clear event log
         */
        clear() {
            const state = this.__getThreadState();
            const oldEvents = state.events;
            state.events = [];
            oldEvents.length = 0;
        },

        /**
         * Copy event log to clipboard
         */
        async copy() {
            const state = this.__getThreadState();

            return await this.__tryCatchAsync('copy event log', { eventCount: state.events.length }, async () => {
                const logText = state.events.map(event => {
                    let details = `[${event.time}] ${event.type} (ID: ${event.id})`;

                    if (Object.keys(event.data).length > 0) {
                        details += '\n';
                        for (const [key, value] of Object.entries(event.data)) {
                            if (typeof value === 'object') {
                                details += `  ${key}: ${JSON.stringify(value, null, 2).split('\n').join('\n  ')}\n`;
                            } else {
                                details += `  ${key}: ${value}\n`;
                            }
                        }
                    }

                    return details;
                }).join('\n---\n');

                await navigator.clipboard.writeText(logText);
                console.log('Event log copied to clipboard!');

                this.eventLogCopied = true;
                setTimeout(() => {
                    this.eventLogCopied = false;
                }, 2000);
            });
        },

        // ============================================
        // Error Display
        // ============================================

        /**
         * Show error message
         * @param {string} message - Error message
         * @param {string|null} threadId - Thread ID
         */
        __showError(message, threadId = null) {
            const state = this.__getThreadState(threadId);
            state.errorMessage = message;

            if (!threadId || threadId === this.__currentThreadId) {
                this.$nextTick(() => this.__scrollToBottom());
            }

            setTimeout(() => {
                state.errorMessage = null;
            }, 5000);
        },

        // ============================================
        // Scroll Management
        // ============================================

        /**
         * Scroll to bottom of message list
         */
        __scrollToBottom() {
            const container = this.$refs?.messageList;
            if (!container || !container.isConnected) {
                return;
            }

            if (typeof container.scrollTo !== 'function') {
                console.warn('Container does not support scrollTo');
                return;
            }

            this.__tryCatch('scroll to bottom', {}, () => {
                container.scrollTo({
                    top: container.scrollHeight,
                    behavior: 'smooth'
                });
            });
        },

        /**
         * Notify on new message
         * @param {Object} message - Message object
         */
        __notifyNewMessage(message) {
            this.__announce('New message received from assistant');
            console.log('New message received:', message.id);
        }
    };
}
