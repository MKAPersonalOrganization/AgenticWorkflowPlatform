/**
 * Chat Component
 *
 * This file re-exports the modular chat implementation.
 * See ./chat/ directory for the full implementation:
 *
 * - constants.js: AG-UI Protocol event types and configuration
 * - markdown-renderer.js: Markdown rendering with syntax highlighting
 * - chat-utils.js: Utility functions (throttle, debounce, formatting)
 * - chat-state.js: State initialization and thread management
 * - chat-sessions.js: Session CRUD operations
 * - chat-messaging.js: Message handling and conversion
 * - chat-streaming.js: SSE streaming and AG-UI event processing
 * - chat-ui.js: UI actions, toasts, clipboard, resize panel
 * - chat-keyboard.js: Keyboard shortcuts
 * - chat-theme.js: Theme management (light/dark/system)
 * - chat-accessibility.js: Screen reader support
 * - chat-health.js: Connection health monitoring
 * - alpine-extensions.js: Custom Alpine.js magics and stores
 * - index.js: Main entry point that composes everything
 */

// Re-export everything from the modular implementation
import './chat/index.js';
