/**
 * Chat Event Log
 *
 * AG-UI Protocol event logging, display, and clipboard export.
 *
 * Alpine.js patterns applied:
 * - Events stored in per-thread reactive arrays (state.events)
 * - $refs for DOM access (eventLogContainer)
 * - $nextTick for scroll-after-render
 */

/**
 * Create event log methods mixin
 * @returns {Object} Methods for event logging
 */
export function createEventLogMethods() {
    return {
        // Drag state for the floating button
        eventLogX: parseInt(localStorage.getItem('eventLogX')) || (window.innerWidth - 64),
        eventLogY: parseInt(localStorage.getItem('eventLogY')) || (window.innerHeight - 64),
        eventLogDragging: false,
        eventLogDragMoved: false,
        _elDragOX: 0,
        _elDragOY: 0,

        /**
         * Begin dragging the event log button via pointer capture.
         */
        startEventLogDrag(e) {
            this.eventLogDragging = true;
            this.eventLogDragMoved = false;
            this._elDragOX = e.clientX - this.eventLogX;
            this._elDragOY = e.clientY - this.eventLogY;
            e.currentTarget.setPointerCapture(e.pointerId);
        },

        /**
         * Update position while dragging, clamped to viewport.
         */
        onEventLogMove(e) {
            if (!this.eventLogDragging) return;
            const nx = e.clientX - this._elDragOX;
            const ny = e.clientY - this._elDragOY;
            if (Math.abs(nx - this.eventLogX) > 3 || Math.abs(ny - this.eventLogY) > 3) {
                this.eventLogDragMoved = true;
            }
            this.eventLogX = Math.max(0, Math.min(window.innerWidth - 48, nx));
            this.eventLogY = Math.max(0, Math.min(window.innerHeight - 48, ny));
        },

        /**
         * End drag and persist position.
         */
        stopEventLogDrag() {
            if (!this.eventLogDragging) return;
            this.eventLogDragging = false;
            localStorage.setItem('eventLogX', this.eventLogX);
            localStorage.setItem('eventLogY', this.eventLogY);
        },

        /**
         * Add event to log
         * @param {Object} event - Event to add
         * @param {string} threadId - Thread ID
         */
        __addEvent(event, threadId) {
            if (!this.eventLogEnabled) return;

            // Skip high-frequency events that flood the log
            if (event.type === 'TEXT_MESSAGE_CONTENT') return;

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
                case 'TEXT_MESSAGE_END': return window.i18n.t('Complete');
                case 'RUN_FINISHED': return window.i18n.t('Complete');
                case 'RUN_ERROR': return `${window.i18n.t('Error')}: ${event.code}`;
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
         * Enable or disable event collection. Persisted to localStorage.
         * When disabled, __addEvent becomes a no-op so no reactive updates occur.
         */
        toggleEventLogEnabled() {
            this.eventLogEnabled = !this.eventLogEnabled;
            localStorage.setItem('eventLogEnabled', this.eventLogEnabled);
        },

        /**
         * Toggle event log visibility
         */
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
                console.log(window.i18n.t('Event log copied to clipboard!'));

                this.eventLogCopied = true;
                setTimeout(() => {
                    this.eventLogCopied = false;
                }, 2000);
            });
        },
    };
}
