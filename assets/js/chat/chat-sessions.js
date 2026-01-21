/**
 * Chat Sessions Management
 *
 * Handles session/thread CRUD operations including:
 * - Loading sessions from server
 * - Creating new sessions
 * - Switching between sessions
 * - Deleting sessions
 */

import { generateThreadId } from './chat-utils.js';
import { createEmptyThreadState } from './chat-state.js';

/**
 * Create session management methods mixin
 * @returns {Object} Methods for session management
 */
export function createSessionMethods() {
    return {
        /**
         * Load sessions from the server (first page)
         */
        async __loadSessions() {
            return await this.__tryCatchAsync('load sessions', {}, async () => {
                const response = await fetch('/api/agui/threads?limit=10');
                if (response.ok) {
                    const data = await response.json();

                    // Map threads to session format
                    this.sessions = (data.threads || []).map(thread => ({
                        id: thread.id,
                        title: thread.title || 'New Chat',
                        lastMessage: thread.updatedAt
                    }));

                    // Store pagination state
                    this.sessionsNextCursor = data.nextCursor || null;
                    this.sessionsHasMore = data.hasMore || false;

                    // Get or create current thread ID
                    const savedThreadId = localStorage.getItem('currentThreadId');
                    if (!savedThreadId || !this.sessions.find(s => s.id === savedThreadId)) {
                        if (this.sessions.length > 0) {
                            this.switchSession(this.sessions[0].id);
                        } else {
                            this.createNewSession();
                        }
                    } else {
                        // Load existing messages for this thread
                        this.__currentThreadId = savedThreadId;
                        this.__currentState = this.__getThreadState(savedThreadId);
                        await this.__loadMessages(savedThreadId);
                    }

                } else {
                    throw new Error(`Failed to fetch threads: ${response.status}`);
                }
            }) || this.createNewSession();
        },

        /**
         * Load more sessions (infinite scroll)
         */
        async __loadMoreSessions() {
            if (!this.sessionsHasMore || this.sessionsLoading || !this.sessionsNextCursor) {
                return;
            }

            this.sessionsLoading = true;

            return await this.__tryCatchAsync('load more sessions', {}, async () => {
                const url = `/api/agui/threads?limit=10&cursor=${encodeURIComponent(this.sessionsNextCursor)}`;
                const response = await fetch(url);

                if (response.ok) {
                    const data = await response.json();

                    // Append new threads to existing sessions
                    const newSessions = (data.threads || []).map(thread => ({
                        id: thread.id,
                        title: thread.title || 'New Chat',
                        lastMessage: thread.updatedAt
                    }));

                    this.sessions = [...this.sessions, ...newSessions];

                    // Update pagination state
                    this.sessionsNextCursor = data.nextCursor || null;
                    this.sessionsHasMore = data.hasMore || false;
                } else {
                    throw new Error(`Failed to fetch more threads: ${response.status}`);
                }
            }).finally(() => {
                // Add a small delay to prevent rapid-fire loading and allow user to perceive the pagination
                setTimeout(() => {
                    this.sessionsLoading = false;
                }, 500);
            });
        },

        /**
         * Handle sidebar scroll for infinite loading
         * @param {Event} event - Scroll event
         */
        __onSidebarScroll(event) {
            const el = event.target;
            const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 100;

            if (nearBottom && this.sessionsHasMore && !this.sessionsLoading) {
                this.__loadMoreSessions();
            }
        },

        /**
         * Delete a session
         * @param {string} sessionId - Session ID to delete
         */
        async deleteSession(sessionId) {
            if (!confirm('Are you sure you want to delete this session?')) {
                return;
            }

            return await this.__tryCatchAsync('delete session', { sessionId }, async () => {
                const response = await fetch(`/api/agui/threads/${sessionId}`, {
                    method: 'DELETE'
                });

                if (response.ok) {
                    // Close EventSource if this thread is streaming
                    const state = this.__threadStates[sessionId];
                    if (state?.eventSource) {
                        state.eventSource.close();
                        state.eventSource = null;
                    }
                    if (state?.heartbeatInterval) {
                        clearInterval(state.heartbeatInterval);
                        state.heartbeatInterval = null;
                    }

                    // Clear large arrays for GC
                    if (state) {
                        state.messages?.splice(0);
                        state.events?.splice(0);
                    }

                    // Remove thread state
                    delete this.__threadStates[sessionId];

                    // Remove session from list
                    this.sessions = this.sessions.filter(s => s.id !== sessionId);

                    // If we deleted the current session, switch to another or create new
                    if (this.__currentThreadId === sessionId) {
                        if (this.sessions.length > 0) {
                            this.switchSession(this.sessions[0].id);
                        } else {
                            this.createNewSession();
                        }
                    }
                } else {
                    throw new Error(`Failed to delete session: ${response.status}`);
                }
            });
        },

        /**
         * Create a new session
         */
        createNewSession() {
            const newSessionId = generateThreadId();
            const newState = createEmptyThreadState();

            this.__threadStates[newSessionId] = newState;
            this.__currentThreadId = newSessionId;
            this.__currentState = newState;
            localStorage.setItem('currentThreadId', newSessionId);

            // Announce to screen readers
            this.__announce('New chat session created');
        },

        /**
         * Switch to a different session
         * @param {string} sessionId - Session ID to switch to
         */
        switchSession(sessionId) {
            this.__currentThreadId = sessionId;
            localStorage.setItem('currentThreadId', sessionId);

            // Note: __currentState is updated automatically via $watch
            const state = this.__getThreadState(sessionId);

            // Load messages if not already loaded
            if (state.messages.length === 0) {
                this.__loadMessages(sessionId);
            }

            // Scroll to bottom after switching
            this.$nextTick(() => this.__scrollToBottom());
        },

        /**
         * Update session title
         */
        async __updateSessionTitle() {
            let session = this.sessions.find(s => s.id === this.__currentThreadId);

            // If session doesn't exist in the list yet, create it
            const state = this.__getThreadState();
            if (!session && state.messages.length > 0) {
                const now = new Date().toISOString();
                session = {
                    id: this.__currentThreadId,
                    title: 'New Chat',
                    created: now,
                    lastMessage: now
                };
                this.sessions.unshift(session);
                // Title will be auto-generated by backend AI and updated via CUSTOM event
            }
        },

        /**
         * Navigate between sessions
         * @param {number} direction - 1 for next, -1 for previous
         */
        __navigateSessions(direction) {
            if (!this.sessions.length) return;

            const currentIndex = this.sessions.findIndex(s => s.id === this.__currentThreadId);
            const nextIndex = (currentIndex + direction + this.sessions.length) % this.sessions.length;

            this.switchSession(this.sessions[nextIndex].id);
        },

        /**
         * Handle thread switch event
         * @param {string} newThreadId - New thread ID
         * @param {string} oldThreadId - Old thread ID
         */
        /**
         * Group sessions by date for display
         * @returns {Array} Array of groups { title: string, sessions: Array }
         */
        getGroupedSessions() {
            const today = new Date();
            const yesterday = new Date(today);
            yesterday.setDate(yesterday.getDate() - 1);

            const groups = {
                'Today': [],
                'Yesterday': [],
                'Previous 7 Days': [],
                'Previous 30 Days': [],
                'Older': []
            };

            // Filter sessions first
            let filteredSessions = this.sessions;
            if (this.threadSearchQuery && this.threadSearchQuery.trim().length > 0) {
                const query = this.threadSearchQuery.toLowerCase();
                filteredSessions = this.sessions.filter(s =>
                    (s.title && s.title.toLowerCase().includes(query))
                );
            }

            filteredSessions.forEach(session => {
                const date = new Date(session.lastMessage || session.created || new Date());
                const diffTime = Math.abs(today - date);
                const diffDays = Math.ceil(diffTime / (1000 * 60 * 60 * 24));

                if (date.toDateString() === today.toDateString()) {
                    groups['Today'].push(session);
                } else if (date.toDateString() === yesterday.toDateString()) {
                    groups['Yesterday'].push(session);
                } else if (diffDays <= 7) {
                    groups['Previous 7 Days'].push(session);
                } else if (diffDays <= 30) {
                    groups['Previous 30 Days'].push(session);
                } else {
                    groups['Older'].push(session);
                }
            });

            // Return non-empty groups in order
            return Object.entries(groups)
                .filter(([_, items]) => items.length > 0)
                .map(([title, sessions]) => ({ title, sessions }));
        },

        __onThreadSwitch(newThreadId, oldThreadId) {
            // Pause/background old thread if streaming
            if (oldThreadId) {
                const oldState = this.__threadStates[oldThreadId];
                if (oldState?.eventSource) {
                    oldState.isBackground = true;
                }
            }

            // Announce thread switch to screen readers
            const session = this.sessions.find(s => s.id === newThreadId);
            if (session) {
                this.__announce(`Switched to ${session.title}`);
            }

            // Focus message input
            this.$nextTick(() => {
                const input = this.$refs?.messageInput;
                if (input?.isConnected && typeof input.focus === 'function') {
                    input.focus({ preventScroll: true });
                }
            });
        }
    };
}
