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
        sidebarOpen: true,
        hideLifecycleEvents: false,
        currentInteractionId: null,
        toolCalls: {}, // Track active tool calls by ID

        init() {
            // Load sessions from backend
            this.loadSessions();
        },

        toggleSidebar() {
            this.sidebarOpen = !this.sidebarOpen;
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
                // Title will be auto-generated by backend AI and updated via CUSTOM event
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
            const interactionId = `interaction_${Date.now()}`;

            // Add user message to UI with interaction ID
            this.messages.push({
                id: `msg_${Date.now()}`,
                role: 'user',
                content: userMessage,
                timestamp: new Date().toISOString(),
                interactionId: interactionId,
                lifecycleEvents: []
            });

            // Store current interaction ID for this request
            this.currentInteractionId = interactionId;

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

            // Add lifecycle event as a message if not hidden
            // Track all AG UI Protocol event types except TEXT_MESSAGE_CONTENT (too frequent)
            const lifecycleEventTypes = [
                // Lifecycle Events
                'RUN_STARTED', 'RUN_FINISHED', 'RUN_ERROR', 'STEP_STARTED', 'STEP_FINISHED',
                // Text Message Events
                'TEXT_MESSAGE_START', 'TEXT_MESSAGE_END',
                // Tool Call Events
                'TOOL_CALL_START', 'TOOL_CALL_ARGS', 'TOOL_CALL_END', 'TOOL_CALL_RESULT',
                // State Management Events
                'STATE_SNAPSHOT', 'STATE_DELTA', 'MESSAGES_SNAPSHOT',
                // Activity Events
                'ACTIVITY_SNAPSHOT', 'ACTIVITY_DELTA',
                // Special Events
                'RAW', 'CUSTOM',
                // Draft Events (Reasoning)
                'REASONING_START', 'REASONING_MESSAGE_START', 'REASONING_MESSAGE_END', 'REASONING_END',
                // Draft Events (Meta)
                'META_EVENT'
            ];
            if (!this.hideLifecycleEvents && lifecycleEventTypes.includes(event.type)) {
                this.addLifecycleMessage(event);
            }

            switch (event.type) {
                case 'RUN_STARTED':
                    // Run started - we can show a loading indicator if needed
                    break;

                case 'STEP_STARTED':
                    // Step started within agent run
                    break;

                case 'STEP_FINISHED':
                    // Step finished within agent run
                    break;

                case 'STATE_SNAPSHOT':
                    // Complete state snapshot received
                    break;

                case 'STATE_DELTA':
                    // Incremental state update received
                    break;

                case 'MESSAGES_SNAPSHOT':
                    // Messages snapshot received
                    break;

                case 'ACTIVITY_SNAPSHOT':
                    // Activity snapshot received
                    break;

                case 'ACTIVITY_DELTA':
                    // Activity delta received
                    break;

                case 'REASONING_START':
                    // Reasoning started
                    break;

                case 'REASONING_MESSAGE_START':
                    // Reasoning message started
                    break;

                case 'REASONING_MESSAGE_END':
                    // Reasoning message ended
                    break;

                case 'REASONING_END':
                    // Reasoning ended
                    break;

                case 'META_EVENT':
                    // Meta event received (e.g., user feedback)
                    break;

                case 'RAW':
                    // Raw event from external system
                    break;

                case 'CUSTOM':
                    // Custom application-specific event
                    if (event.name === 'THREAD_TITLE_UPDATED') {
                        // Update session title in real-time
                        const session = this.sessions.find(s => s.id === event.value.threadId);
                        if (session) {
                            session.title = event.value.title;
                        }
                    }
                    break;

                case 'TOOL_CALL_START':
                    // Tool call started - create a new tool call message
                    this.toolCalls[event.toolCallId] = {
                        id: event.toolCallId,
                        name: event.toolCallName,
                        args: '',
                        status: 'streaming',
                        timestamp: new Date(event.timestamp).toISOString()
                    };

                    this.messages.push({
                        id: event.toolCallId,
                        type: 'tool_call',
                        name: event.toolCallName,
                        args: '',
                        status: 'streaming',
                        timestamp: new Date(event.timestamp).toISOString()
                    });

                    this.$nextTick(() => this.scrollToBottom());
                    break;

                case 'TOOL_CALL_ARGS':
                    // Tool call arguments received - accumulate them
                    if (this.toolCalls[event.toolCallId]) {
                        this.toolCalls[event.toolCallId].args += event.delta;

                        // Update the message
                        const toolMsgIndex = this.messages.findIndex(m => m.id === event.toolCallId);
                        if (toolMsgIndex !== -1) {
                            this.messages[toolMsgIndex].args = this.toolCalls[event.toolCallId].args;
                        }
                    }
                    break;

                case 'TOOL_CALL_END':
                    // Tool call ended - mark as complete
                    if (this.toolCalls[event.toolCallId]) {
                        this.toolCalls[event.toolCallId].status = 'completed';

                        const toolMsgIndex = this.messages.findIndex(m => m.id === event.toolCallId);
                        if (toolMsgIndex !== -1) {
                            this.messages[toolMsgIndex].status = 'completed';

                            // Try to parse args as JSON for display
                            try {
                                this.messages[toolMsgIndex].parsedArgs = JSON.parse(this.messages[toolMsgIndex].args);
                            } catch (e) {
                                this.messages[toolMsgIndex].parsedArgs = { raw: this.messages[toolMsgIndex].args };
                            }
                        }
                    }
                    break;

                case 'TOOL_CALL_RESULT':
                    // Tool call result received - add result message
                    this.messages.push({
                        id: event.messageId,
                        type: 'tool_result',
                        toolCallId: event.toolCallId,
                        content: event.content,
                        timestamp: new Date(event.timestamp).toISOString()
                    });

                    this.$nextTick(() => this.scrollToBottom());
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

        addLifecycleMessage(event) {
            // Find the last user message with the current interaction ID
            const userMessageIndex = this.messages.findIndex(m =>
                m.role === 'user' && m.interactionId === this.currentInteractionId
            );

            if (userMessageIndex !== -1) {
                const lifecycleEvent = {
                    id: `lifecycle_${Date.now()}_${Math.random()}`,
                    type: event.type,
                    content: this.getLifecycleContent(event),
                    timestamp: new Date().toISOString()
                };

                // Add to the user message's lifecycle events array
                if (!this.messages[userMessageIndex].lifecycleEvents) {
                    this.messages[userMessageIndex].lifecycleEvents = [];
                }
                this.messages[userMessageIndex].lifecycleEvents.push(lifecycleEvent);
            }
        },

        getLifecycleContent(event) {
            switch (event.type) {
                // Lifecycle Events
                case 'RUN_STARTED':
                    return 'Started';
                case 'RUN_FINISHED':
                    return event.outcome === 'interrupt' ? 'Paused' : 'Completed';
                case 'RUN_ERROR':
                    return `Error: ${event.message || 'Unknown'}`;
                case 'STEP_STARTED':
                    return `Step: ${event.stepName || 'Unknown'}`;
                case 'STEP_FINISHED':
                    return `${event.stepName || 'Step'} Done`;
                // Text Message Events
                case 'TEXT_MESSAGE_START':
                    return 'Responding';
                case 'TEXT_MESSAGE_END':
                    return 'Response Done';
                // Tool Call Events
                case 'TOOL_CALL_START':
                    return `Tool: ${event.toolCallName || 'Unknown'}`;
                case 'TOOL_CALL_ARGS':
                    return 'Args';
                case 'TOOL_CALL_END':
                    return 'Tool Done';
                case 'TOOL_CALL_RESULT':
                    return 'Result';
                // State Management Events
                case 'STATE_SNAPSHOT':
                    return 'State Snapshot';
                case 'STATE_DELTA':
                    return 'State Update';
                case 'MESSAGES_SNAPSHOT':
                    return 'Messages Sync';
                // Activity Events
                case 'ACTIVITY_SNAPSHOT':
                    return `Activity: ${event.activityType || 'Unknown'}`;
                case 'ACTIVITY_DELTA':
                    return 'Activity Update';
                // Reasoning Events (Draft)
                case 'REASONING_START':
                    return 'Thinking';
                case 'REASONING_MESSAGE_START':
                    return 'Reasoning';
                case 'REASONING_MESSAGE_END':
                    return 'Thought Complete';
                case 'REASONING_END':
                    return 'Thinking Done';
                // Meta Events (Draft)
                case 'META_EVENT':
                    return event.metaType || 'Meta';
                // Special Events
                case 'RAW':
                    return `Raw: ${event.source || 'External'}`;
                case 'CUSTOM':
                    return event.name || 'Custom';
                default:
                    return event.type;
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
            switch (event.type) {
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
            // Scroll to show the error
            this.$nextTick(() => this.scrollToBottom());
            // Auto-hide error after 5 seconds
            setTimeout(() => {
                this.errorMessage = null;
            }, 5000);
        },

        scrollToBottom() {
            const container = this.$refs.messageList;
            if (container) {
                // Smooth scroll to bottom
                container.scrollTo({
                    top: container.scrollHeight,
                    behavior: 'smooth'
                });
            }
        },

        formatTime(timestamp) {
            return new Date(timestamp).toLocaleTimeString('en-US', {
                hour: 'numeric',
                minute: '2-digit'
            });
        },

        isValidJSON(str) {
            try {
                JSON.parse(str);
                return true;
            } catch (e) {
                return false;
            }
        },

        // Debounce helper for auto-scroll optimization
        debounce(func, wait) {
            let timeout;
            return function executedFunction(...args) {
                const later = () => {
                    clearTimeout(timeout);
                    func(...args);
                };
                clearTimeout(timeout);
                timeout = setTimeout(later, wait);
            };
        }
    }));
});
