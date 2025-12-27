// Alpine.js chat component
// Register component before Alpine initializes
document.addEventListener('alpine:init', () => {
    Alpine.data('chat', () => ({
        messages: [],
        currentMessage: '',
        isStreaming: false,
        showTypingIndicator: false,
        currentStreamingId: null,
        currentStreamingContent: '',
        threadId: null,
        errorMessage: null,
        events: [],
        sessions: [],
        showDebugPanel: false,

        init() {
            // Load sessions from backend
            this.loadSessions();
        },

        async loadSessions() {
            try {
                const response = await fetch('/api/threads');
                if (response.ok) {
                    const threads = await response.json();
                    // Only show threads that have messages
                    this.sessions = threads
                        .filter(thread => thread.messages && thread.messages.length > 0)
                        .map(thread => ({
                            id: thread.id,
                            title: thread.title,
                            created: thread.createdAt,
                            lastMessage: thread.updatedAt
                        }));

                    // Get or create current thread ID
                    this.threadId = localStorage.getItem('currentThreadId');
                    if (!this.threadId || !this.sessions.find(s => s.id === this.threadId)) {
                        if (this.sessions.length > 0) {
                            // Use the most recent session
                            this.switchSession(this.sessions[0].id);
                        } else {
                            // Create a new session
                            this.createNewSession();
                        }
                    } else {
                        // Load existing messages for this thread
                        this.loadMessages();
                    }
                }
            } catch (error) {
                console.error('Failed to load sessions:', error);
                // Fallback: create new session
                this.createNewSession();
            }
        },

        createNewSession() {
            const newSessionId = `thread_${Date.now()}_${Math.random().toString(36).substring(2, 11)}`;

            // Just switch to the new session without adding it to the list yet
            // It will be added when the first message is sent
            this.threadId = newSessionId;
            localStorage.setItem('currentThreadId', newSessionId);
            this.messages = [];
            this.events = [];
            this.errorMessage = null;
        },

        switchSession(sessionId) {
            if (this.isStreaming) return;

            this.threadId = sessionId;
            localStorage.setItem('currentThreadId', sessionId);
            this.messages = [];
            this.events = [];
            this.errorMessage = null;
            this.loadMessages();
        },

        async deleteSession(sessionId) {
            if (this.isStreaming) return;

            // Confirm deletion
            if (!confirm('Are you sure you want to delete this session?')) {
                return;
            }

            try {
                const response = await fetch(`/api/threads/${sessionId}`, {
                    method: 'DELETE'
                });

                if (response.ok) {
                    // Remove session from list
                    this.sessions = this.sessions.filter(s => s.id !== sessionId);

                    // If we deleted the current session, switch to another or create new
                    if (this.threadId === sessionId) {
                        if (this.sessions.length > 0) {
                            this.switchSession(this.sessions[0].id);
                        } else {
                            this.createNewSession();
                        }
                    }
                } else {
                    this.showError('Failed to delete session');
                }
            } catch (error) {
                console.error('Failed to delete session:', error);
                this.showError('Failed to delete session');
            }
        },

        async updateSessionTitle() {
            let session = this.sessions.find(s => s.id === this.threadId);

            // If session doesn't exist in the list yet, create it
            if (!session && this.messages.length > 0) {
                const now = new Date().toISOString();
                session = {
                    id: this.threadId,
                    title: 'New Chat',
                    created: now,
                    lastMessage: now
                };
                this.sessions.unshift(session);
            }

            if (session && this.messages.length > 0) {
                const firstUserMessage = this.messages.find(m => m.role === 'user');
                if (firstUserMessage && session.title === 'New Chat') {
                    const newTitle = firstUserMessage.content.substring(0, 30) + (firstUserMessage.content.length > 30 ? '...' : '');
                    session.title = newTitle;
                    session.lastMessage = new Date().toISOString();

                    // Update title on backend
                    try {
                        await fetch(`/api/threads/${this.threadId}/title`, {
                            method: 'PATCH',
                            headers: {
                                'Content-Type': 'application/json'
                            },
                            body: JSON.stringify({ title: newTitle })
                        });
                    } catch (error) {
                        console.error('Failed to update session title:', error);
                    }
                }
            }
        },

        formatSessionTime(timestamp) {
            const date = new Date(timestamp);
            const now = new Date();
            const diffMs = now - date;
            const diffMins = Math.floor(diffMs / 60000);

            if (diffMins < 1) return 'Just now';
            if (diffMins < 60) return `${diffMins}m ago`;

            const diffHours = Math.floor(diffMins / 60);
            if (diffHours < 24) return `${diffHours}h ago`;

            const diffDays = Math.floor(diffHours / 24);
            if (diffDays < 7) return `${diffDays}d ago`;

            return date.toLocaleDateString();
        },

        async loadMessages() {
            try {
                const response = await fetch(`/api/threads/${this.threadId}/messages`);
                if (response.ok) {
                    const messages = await response.json();
                    this.messages = messages.map(msg => ({
                        id: msg.id,
                        role: msg.role,
                        content: msg.content,
                        timestamp: msg.timestamp,
                        streaming: false
                    }));
                    // Scroll to bottom after loading
                    this.$nextTick(() => this.scrollToBottom());
                }
            } catch (error) {
                console.error('Failed to load messages:', error);
            }
        },

        sendMessage() {
            if (!this.currentMessage.trim() || this.isStreaming) return;

            const userMessage = this.currentMessage.trim();

            // Add user message to UI
            this.messages.push({
                id: `msg_${Date.now()}`,
                role: 'user',
                content: userMessage,
                timestamp: new Date().toISOString()
            });

            // Update session title with first message
            this.updateSessionTitle();

            // Clear input
            this.currentMessage = '';

            // Scroll to bottom
            this.$nextTick(() => this.scrollToBottom());

            // Start SSE connection
            this.streamResponse(userMessage);
        },

        streamResponse(message) {
            this.isStreaming = true;
            this.showTypingIndicator = true;
            this.errorMessage = null;

            const url = `/send?threadId=${encodeURIComponent(this.threadId)}&message=${encodeURIComponent(message)}`;
            const eventSource = new EventSource(url);

            eventSource.onmessage = (event) => {
                const data = JSON.parse(event.data);
                this.handleEvent(data, eventSource);
            };

            eventSource.onerror = (error) => {
                console.error('SSE error:', error);
                eventSource.close();
                this.isStreaming = false;
                this.showTypingIndicator = false;
                this.showError('Connection error. Please try again.');
            };
        },

        handleEvent(event, eventSource) {
            // Add event to log (keep only last 10)
            this.addEvent(event);

            switch(event.type) {
                case 'RUN_STARTED':
                    // Run started - we can show a loading indicator if needed
                    console.log('Run started:', event.runId);
                    break;

                case 'TEXT_MESSAGE_START':
                    this.showTypingIndicator = false;
                    this.currentStreamingId = event.messageId;
                    this.currentStreamingContent = '';

                    // Add empty assistant message
                    this.messages.push({
                        id: event.messageId,
                        role: 'assistant',
                        content: '',
                        timestamp: new Date(event.timestamp).toISOString(),
                        streaming: true
                    });
                    break;

                case 'TEXT_MESSAGE_CONTENT':
                    this.currentStreamingContent += event.delta;

                    // Update the streaming message
                    const msgIndex = this.messages.findIndex(m => m.id === event.messageId);
                    if (msgIndex !== -1) {
                        this.messages[msgIndex].content = this.currentStreamingContent;
                    }

                    // Auto-scroll
                    this.$nextTick(() => this.scrollToBottom());
                    break;

                case 'TEXT_MESSAGE_END':
                    // Mark as not streaming
                    const endMsgIndex = this.messages.findIndex(m => m.id === event.messageId);
                    if (endMsgIndex !== -1) {
                        this.messages[endMsgIndex].streaming = false;
                    }

                    this.currentStreamingId = null;
                    this.currentStreamingContent = '';
                    break;

                case 'RUN_FINISHED':
                    // Run completed successfully
                    console.log('Run finished:', event.runId);
                    this.isStreaming = false;
                    eventSource.close();
                    break;

                case 'RUN_ERROR':
                    // Handle error
                    console.error('Run error:', event.message);
                    this.showTypingIndicator = false;
                    this.isStreaming = false;
                    this.showError(event.message || 'An error occurred');
                    eventSource.close();
                    break;
            }
        },

        addEvent(event) {
            // For TEXT_MESSAGE_CONTENT, update the last event if it's also content
            if (event.type === 'TEXT_MESSAGE_CONTENT') {
                const lastEvent = this.events[this.events.length - 1];
                if (lastEvent && lastEvent.type === 'TEXT_MESSAGE_CONTENT') {
                    // Update the existing content event with cumulative count
                    const currentChars = parseInt(lastEvent.details.split(' ')[0]) || 0;
                    const newChars = event.delta?.length || 0;
                    lastEvent.details = `${currentChars + newChars} chars`;
                    lastEvent.time = new Date().toLocaleTimeString('en-US', {
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit'
                    });
                    return;
                }
            }

            this.events.push({
                id: `event_${Date.now()}_${Math.random()}`,
                type: event.type,
                time: new Date().toLocaleTimeString('en-US', {
                    hour: '2-digit',
                    minute: '2-digit',
                    second: '2-digit'
                }),
                details: this.getEventDetails(event)
            });

            // Keep only last 20 events
            if (this.events.length > 20) {
                this.events.shift();
            }
        },

        getEventDetails(event) {
            switch(event.type) {
                case 'RUN_STARTED':
                    return `Run ${event.runId?.slice(-8)}`;
                case 'TEXT_MESSAGE_START':
                    return `Message ${event.messageId?.slice(-8)}`;
                case 'TEXT_MESSAGE_CONTENT':
                    return `${event.delta?.length || 0} chars`;
                case 'TEXT_MESSAGE_END':
                    return `Complete`;
                case 'RUN_FINISHED':
                    return `Complete`;
                case 'RUN_ERROR':
                    return `Error: ${event.code}`;
                default:
                    return '';
            }
        },

        showError(message) {
            this.errorMessage = message;
            // Auto-hide error after 5 seconds
            setTimeout(() => {
                this.errorMessage = null;
            }, 5000);
        },

        scrollToBottom() {
            const container = this.$refs.messageList;
            if (container) {
                container.scrollTop = container.scrollHeight;
            }
        },

        formatTime(timestamp) {
            return new Date(timestamp).toLocaleTimeString('en-US', {
                hour: 'numeric',
                minute: '2-digit'
            });
        }
    }));
});
