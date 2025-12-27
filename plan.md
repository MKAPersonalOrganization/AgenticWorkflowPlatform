# AG-UI Protocol Implementation Plan

## Overview
Implement AG-UI protocol in Go + Templ chat application with SSE streaming, mock backend, in-memory sessions, and full UI for message history and typing indicators.

## User Requirements
- **SSE Streaming**: Real-time Server-Sent Events for message streaming
- **Mock Backend**: Echo/intelligent responses (no LLM API initially)
- **In-Memory Sessions**: Thread-based conversation storage
- **MVP Features**: Text chat, message history display, typing indicators, error handling

## Current Architecture
- Simple Go HTTP server with `http.NewServeMux`
- Templ templates (.templ files) with component Props pattern
- Tailwind CSS + templui components (button, input)
- No state management, sessions, or database
- Single route: GET / renders chat form (POST /send unimplemented)

---

## Phase 1: SSE Infrastructure & Basic Event Streaming

### Goal
Set up SSE endpoint with basic event streaming (no full AG-UI protocol yet).

### Files to Create
- `handlers/chat.go` - SSE handler with basic streaming
- `services/mock/responses.go` - Simple mock response generator

### Files to Modify
- `main.go` - Add POST /send route and handler registration

### Implementation
```go
// handlers/chat.go
package handlers

func HandleSendMessage() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Setup SSE headers
        w.Header().Set("Content-Type", "text/event-stream")
        w.Header().Set("Cache-Control", "no-cache")
        w.Header().Set("Connection", "keep-alive")

        flusher := w.(http.Flusher)

        message := r.URL.Query().Get("message")

        // Simple JSON events for now
        fmt.Fprintf(w, "data: {\"content\":\"Echo: %s\",\"done\":false}\n\n", message)
        flusher.Flush()

        time.Sleep(500 * time.Millisecond)

        fmt.Fprintf(w, "data: {\"content\":\"\",\"done\":true}\n\n")
        flusher.Flush()
    }
}
```

### Testing
1. Send message via URL: `/send?message=hello`
2. Observe SSE connection in Network tab
3. Verify events arrive correctly
4. Check connection closes with `done: true`

---

## Phase 2: AG-UI Event Types

### Goal
Implement AG-UI compliant event streaming with proper event types.

### Files to Create
- `internal/agui/events.go` - AG-UI event type definitions
- `internal/agui/writer.go` - SSE event writer helper

### Implementation
```go
// internal/agui/events.go
package agui

type TextMessageStartEvent struct {
    Type      string `json:"type"`
    MessageID string `json:"messageId"`
    Role      string `json:"role,omitempty"`
    Timestamp int64  `json:"timestamp,omitempty"`
}

type TextMessageContentEvent struct {
    Type      string `json:"type"`
    MessageID string `json:"messageId"`
    Delta     string `json:"delta"`
    Timestamp int64  `json:"timestamp,omitempty"`
}

type TextMessageEndEvent struct {
    Type      string `json:"type"`
    MessageID string `json:"messageId"`
    Timestamp int64  `json:"timestamp,omitempty"`
}

func GenerateMessageID() string {
    return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}
```

```go
// internal/agui/writer.go
package agui

func WriteSSEEvent(w http.ResponseWriter, event interface{}) error {
    data, err := json.Marshal(event)
    if err != nil {
        return err
    }
    fmt.Fprintf(w, "data: %s\n\n", data)
    return nil
}
```

### Update Handler
```go
// handlers/chat.go - Updated to use AG-UI events
messageID := agui.GenerateMessageID()

// START
agui.WriteSSEEvent(w, &agui.TextMessageStartEvent{
    Type:      "TEXT_MESSAGE_START",
    MessageID: messageID,
    Role:      "assistant",
    Timestamp: time.Now().UnixMilli(),
})
flusher.Flush()

// CONTENT (can be multiple)
agui.WriteSSEEvent(w, &agui.TextMessageContentEvent{
    Type:      "TEXT_MESSAGE_CONTENT",
    MessageID: messageID,
    Delta:     "Hello there!",
    Timestamp: time.Now().UnixMilli(),
})
flusher.Flush()

// END
agui.WriteSSEEvent(w, &agui.TextMessageEndEvent{
    Type:      "TEXT_MESSAGE_END",
    MessageID: messageID,
    Timestamp: time.Now().UnixMilli(),
})
flusher.Flush()
```

### Testing
1. Verify TEXT_MESSAGE_START arrives first
2. Verify TEXT_MESSAGE_CONTENT has delta field
3. Verify TEXT_MESSAGE_END arrives last
4. Verify all events share same messageId
5. Validate JSON structure matches AG-UI spec

---

## Phase 3: Session Management (In-Memory Threads)

### Goal
Track conversation history in memory with thread-based sessions.

### Files to Create
- `internal/session/types.go` - Message and Thread types
- `internal/session/store.go` - In-memory thread storage

### Implementation
```go
// internal/session/types.go
package session

type Message struct {
    ID        string    `json:"id"`
    Role      string    `json:"role"` // "user" | "assistant"
    Content   string    `json:"content"`
    Timestamp time.Time `json:"timestamp"`
}

type Thread struct {
    ID        string    `json:"id"`
    Messages  []Message `json:"messages"`
    CreatedAt time.Time `json:"createdAt"`
    UpdatedAt time.Time `json:"updatedAt"`
}
```

```go
// internal/session/store.go
package session

type Store struct {
    mu      sync.RWMutex
    threads map[string]*Thread
}

func NewStore() *Store {
    return &Store{
        threads: make(map[string]*Thread),
    }
}

func (s *Store) GetOrCreateThread(threadID string) *Thread {
    s.mu.Lock()
    defer s.mu.Unlock()

    if thread, exists := s.threads[threadID]; exists {
        return thread
    }

    thread := &Thread{
        ID:        threadID,
        Messages:  []Message{},
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }
    s.threads[threadID] = thread
    return thread
}

func (s *Store) AddMessage(threadID string, msg Message) {
    s.mu.Lock()
    defer s.mu.Unlock()

    if thread, exists := s.threads[threadID]; exists {
        thread.Messages = append(thread.Messages, msg)
        thread.UpdatedAt = time.Now()
    }
}

func (s *Store) GetMessages(threadID string) []Message {
    s.mu.RLock()
    defer s.mu.RUnlock()

    if thread, exists := s.threads[threadID]; exists {
        return thread.Messages
    }
    return []Message{}
}
```

### Update main.go
```go
// Initialize store globally
store := session.NewStore()

// Pass to handler
mux.HandleFunc("/send", HandleSendMessage(store))
mux.HandleFunc("/api/threads/{threadId}/messages", HandleGetMessages(store))
```

### New Handler for Message Retrieval
```go
// handlers/chat.go
func HandleGetMessages(store *session.Store) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        threadID := r.PathValue("threadId")
        messages := store.GetMessages(threadID)

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(messages)
    }
}
```

### Testing
1. Send message with threadId parameter
2. Send another message with same threadId
3. Call GET /api/threads/{threadId}/messages
4. Verify both messages are stored
5. Use different threadId, verify separate conversation

---

## Phase 4: Run Lifecycle Events

### Goal
Add RUN_STARTED, RUN_FINISHED, and RUN_ERROR events.

### Files to Modify
- `internal/agui/events.go` - Add run event types
- `handlers/chat.go` - Wrap message events with run lifecycle

### Implementation
```go
// internal/agui/events.go - Add these types
type RunStartedEvent struct {
    Type      string `json:"type"`
    ThreadID  string `json:"threadId"`
    RunID     string `json:"runId"`
    Timestamp int64  `json:"timestamp,omitempty"`
}

type RunFinishedEvent struct {
    Type      string      `json:"type"`
    ThreadID  string      `json:"threadId"`
    RunID     string      `json:"runId"`
    Result    interface{} `json:"result,omitempty"`
    Timestamp int64       `json:"timestamp,omitempty"`
}

type RunErrorEvent struct {
    Type      string `json:"type"`
    RunID     string `json:"runId,omitempty"`
    Code      string `json:"code,omitempty"`
    Message   string `json:"message"`
    Timestamp int64  `json:"timestamp,omitempty"`
}

func GenerateRunID() string {
    return fmt.Sprintf("run_%d", time.Now().UnixNano())
}
```

### Event Sequence in Handler
```go
runID := agui.GenerateRunID()

// 1. RUN_STARTED
agui.WriteSSEEvent(w, &agui.RunStartedEvent{
    Type:      "RUN_STARTED",
    ThreadID:  threadID,
    RunID:     runID,
    Timestamp: time.Now().UnixMilli(),
})
flusher.Flush()

// 2-4. TEXT_MESSAGE_START/CONTENT/END
// ... message events ...

// 5. RUN_FINISHED
agui.WriteSSEEvent(w, &agui.RunFinishedEvent{
    Type:      "RUN_FINISHED",
    ThreadID:  threadID,
    RunID:     runID,
    Timestamp: time.Now().UnixMilli(),
})
flusher.Flush()
```

### Error Handling
```go
defer func() {
    if err := recover(); err != nil {
        agui.WriteSSEEvent(w, &agui.RunErrorEvent{
            Type:    "RUN_ERROR",
            RunID:   runID,
            Message: fmt.Sprintf("%v", err),
            Code:    "INTERNAL_ERROR",
        })
        flusher.Flush()
    }
}()
```

### Testing
1. Verify RUN_STARTED is first event
2. Verify RUN_FINISHED is last event
3. Trigger error, verify RUN_ERROR event
4. Check runId consistency across events

---

## Phase 5: UI Components & Message Display

### Goal
Render conversation history and streaming messages in real-time.

### Files to Create
- `components/chat/message.templ` - Single message component
- `components/chat/message-list.templ` - Message container
- `assets/js/chat.js` - Chat JavaScript client

### Implementation

**Message Component:**
```go
// components/chat/message.templ
package chat

import (
    "school-with-ai/utils"
    "time"
)

type MessageProps struct {
    ID        string
    Role      string
    Content   string
    Timestamp time.Time
    Streaming bool
}

templ Message(props MessageProps) {
    <div
        class={utils.TwMerge(
            "flex gap-3 mb-4 p-4 rounded-lg",
            utils.If(props.Role == "user", "bg-primary/10 ml-auto max-w-[80%]"),
            utils.If(props.Role == "assistant", "bg-secondary/10 mr-auto max-w-[80%]"),
        )}
        data-message-id={props.ID}
    >
        <div class="flex flex-col gap-1 flex-1">
            <div class="text-sm font-medium text-muted-foreground">
                if props.Role == "user" {
                    You
                } else {
                    Assistant
                }
                <span class="ml-2 text-xs">
                    {props.Timestamp.Format("3:04 PM")}
                </span>
            </div>
            <div class="text-foreground whitespace-pre-wrap" data-message-content>
                {props.Content}
            </div>
            if props.Streaming {
                <div class="text-xs text-muted-foreground animate-pulse">●</div>
            }
        </div>
    </div>
}
```

**Chat JavaScript:**
```javascript
// assets/js/chat.js
class ChatManager {
    constructor() {
        this.threadId = localStorage.getItem('threadId') || this.generateThreadId();
        this.currentMessageId = null;
        this.currentMessageContent = '';
        this.init();
    }

    generateThreadId() {
        const id = `thread_${Date.now()}_${Math.random().toString(36).substr(2, 9)}`;
        localStorage.setItem('threadId', id);
        return id;
    }

    init() {
        this.loadMessages();

        const form = document.getElementById('chat-form');
        form.addEventListener('submit', (e) => {
            e.preventDefault();
            const input = document.getElementById('message-input');
            this.sendMessage(input.value);
            input.value = '';
        });
    }

    async loadMessages() {
        try {
            const response = await fetch(`/api/threads/${this.threadId}/messages`);
            const messages = await response.json();
            messages.forEach(msg => this.appendMessage(msg));
        } catch (error) {
            console.error('Failed to load messages:', error);
        }
    }

    sendMessage(text) {
        // Add user message to UI
        this.appendMessage({
            id: `msg_${Date.now()}`,
            role: 'user',
            content: text,
            timestamp: new Date().toISOString()
        });

        // Connect to SSE endpoint
        const url = `/send?threadId=${this.threadId}&message=${encodeURIComponent(text)}`;
        const eventSource = new EventSource(url);

        eventSource.onmessage = (event) => {
            const agEvent = JSON.parse(event.data);
            this.handleEvent(agEvent, eventSource);
        };

        eventSource.onerror = (error) => {
            console.error('SSE error:', error);
            eventSource.close();
        };
    }

    handleEvent(agEvent, eventSource) {
        switch(agEvent.type) {
            case 'TEXT_MESSAGE_START':
                this.currentMessageId = agEvent.messageId;
                this.currentMessageContent = '';
                this.appendMessage({
                    id: agEvent.messageId,
                    role: 'assistant',
                    content: '',
                    timestamp: new Date(agEvent.timestamp).toISOString(),
                    streaming: true
                });
                break;

            case 'TEXT_MESSAGE_CONTENT':
                this.currentMessageContent += agEvent.delta;
                this.updateMessage(agEvent.messageId, this.currentMessageContent);
                break;

            case 'TEXT_MESSAGE_END':
                this.removeStreamingIndicator(agEvent.messageId);
                break;

            case 'RUN_FINISHED':
                eventSource.close();
                break;

            case 'RUN_ERROR':
                console.error('Run error:', agEvent.message);
                eventSource.close();
                break;
        }
    }

    appendMessage(msg) {
        const messageList = document.getElementById('message-list');
        const messageEl = this.createMessageElement(msg);
        messageList.appendChild(messageEl);
        messageList.scrollTop = messageList.scrollHeight;
    }

    createMessageElement(msg) {
        const div = document.createElement('div');
        div.className = `flex gap-3 mb-4 p-4 rounded-lg ${
            msg.role === 'user' ? 'bg-primary/10 ml-auto max-w-[80%]' : 'bg-secondary/10 mr-auto max-w-[80%]'
        }`;
        div.dataset.messageId = msg.id;

        const timestamp = new Date(msg.timestamp).toLocaleTimeString('en-US', {
            hour: 'numeric',
            minute: '2-digit'
        });

        div.innerHTML = `
            <div class="flex flex-col gap-1 flex-1">
                <div class="text-sm font-medium text-muted-foreground">
                    ${msg.role === 'user' ? 'You' : 'Assistant'}
                    <span class="ml-2 text-xs">${timestamp}</span>
                </div>
                <div class="text-foreground whitespace-pre-wrap" data-message-content>
                    ${msg.content}
                </div>
                ${msg.streaming ? '<div class="text-xs text-muted-foreground animate-pulse">●</div>' : ''}
            </div>
        `;
        return div;
    }

    updateMessage(messageId, content) {
        const messageEl = document.querySelector(`[data-message-id="${messageId}"]`);
        if (messageEl) {
            const contentEl = messageEl.querySelector('[data-message-content]');
            contentEl.textContent = content;
        }
    }

    removeStreamingIndicator(messageId) {
        const messageEl = document.querySelector(`[data-message-id="${messageId}"]`);
        const indicator = messageEl?.querySelector('.animate-pulse');
        indicator?.remove();
    }
}

document.addEventListener('DOMContentLoaded', () => {
    new ChatManager();
});
```

**Update hello.templ:**
```go
templ ChatInputScreen() {
    <!DOCTYPE html>
    <html lang="en">
        <head>
            <meta charset="UTF-8"/>
            <meta name="viewport" content="width=device-width, initial-scale=1.0"/>
            <title>Chat</title>
            <link rel="stylesheet" href="/assets/css/output.css"/>
            @input.Script()
            <script defer src="/assets/js/chat.js"></script>
        </head>
        <body class="flex justify-center items-center min-h-screen p-5">
            <div class="bg-card rounded-xl shadow-md w-full max-w-2xl p-6 flex flex-col">
                <h1 class="text-2xl font-semibold mb-6 text-foreground">Chat</h1>

                <div id="message-list" class="flex flex-col gap-2 overflow-y-auto max-h-[60vh] mb-4">
                </div>

                <form id="chat-form" class="flex flex-col gap-3">
                    @input.Input(input.Props{
                        Name:        "message",
                        Placeholder: "Type your message...",
                        Attributes: templ.Attributes{
                            "required":  true,
                            "id":        "message-input",
                        },
                    })
                    @button.Button(button.Props{
                        Type:      button.TypeSubmit,
                        FullWidth: true,
                    }) {
                        Send
                    }
                </form>
            </div>
        </body>
    </html>
}
```

### Testing
1. Send message, verify it appears in UI
2. Verify assistant response streams in real-time
3. Verify streaming indicator
4. Refresh page, verify messages reload
5. Test auto-scroll to bottom

---

## Phase 6: Typing Indicators & Error Handling

### Goal
Add visual feedback for loading states and error messages.

### Files to Create
- `components/chat/typing-indicator.templ` - Typing animation

### Implementation

**Typing Indicator:**
```go
// components/chat/typing-indicator.templ
package chat

templ TypingIndicator() {
    <div id="typing-indicator" class="hidden flex gap-3 mb-4 p-4 rounded-lg bg-secondary/10 mr-auto max-w-[80%]">
        <div class="flex flex-col gap-1 flex-1">
            <div class="text-sm font-medium text-muted-foreground">Assistant</div>
            <div class="flex gap-1">
                <span class="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" style="animation-delay: 0ms"></span>
                <span class="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" style="animation-delay: 150ms"></span>
                <span class="w-2 h-2 bg-muted-foreground rounded-full animate-bounce" style="animation-delay: 300ms"></span>
            </div>
        </div>
    </div>
}
```

**JavaScript Updates:**
```javascript
// In ChatManager class
sendMessage(text) {
    this.appendMessage({ /* user message */ });

    this.showTypingIndicator();
    this.setInputDisabled(true);

    const eventSource = new EventSource(url);
    // ... existing code
}

handleEvent(agEvent, eventSource) {
    switch(agEvent.type) {
        case 'TEXT_MESSAGE_START':
            this.hideTypingIndicator();
            // ... existing code
            break;

        case 'RUN_ERROR':
            this.hideTypingIndicator();
            this.showError(agEvent.message);
            this.setInputDisabled(false);
            eventSource.close();
            break;
    }
}

showTypingIndicator() {
    document.getElementById('typing-indicator')?.classList.remove('hidden');
    this.scrollToBottom();
}

hideTypingIndicator() {
    document.getElementById('typing-indicator')?.classList.add('hidden');
}

showError(message) {
    const messageList = document.getElementById('message-list');
    const errorEl = document.createElement('div');
    errorEl.className = 'flex gap-3 mb-4 p-4 rounded-lg bg-destructive/10 border border-destructive/20';
    errorEl.innerHTML = `
        <div class="flex flex-col gap-1 flex-1">
            <div class="text-sm font-medium text-destructive">Error</div>
            <div class="text-sm text-destructive/90">${message}</div>
        </div>
    `;
    messageList.appendChild(errorEl);
    this.scrollToBottom();
}

setInputDisabled(disabled) {
    document.getElementById('message-input').disabled = disabled;
    document.querySelector('button[type="submit"]').disabled = disabled;
}
```

**Update hello.templ:**
```go
// Add after message-list div
@chat.TypingIndicator()
```

### Testing
1. Send message, verify typing indicator appears
2. Verify typing indicator hides when content starts
3. Verify input disabled during processing
4. Trigger error, verify error UI
5. Test network error handling

---

## Phase 7: Mock Intelligence & Realistic Streaming

### Goal
Create varied, realistic mock responses with natural streaming behavior.

### Files to Create
- `services/mock/responses.go` - Response templates and topic detection
- `services/mock/streamer.go` - Streaming simulation with delays

### Implementation

**Response Generator:**
```go
// services/mock/responses.go
package mock

var responseTemplates = []string{
    "That's a great question! %s",
    "I understand. %s",
    "Here's what I think: %s",
    "Let me help you with that. %s",
}

var topicResponses = map[string][]string{
    "hello": {
        "Hello! How can I assist you today?",
        "Hi there! What would you like to know?",
    },
    "weather": {
        "I don't have real-time weather data, but I can help you find that information.",
    },
    "default": {
        "I'm a mock AI assistant demonstrating the AG-UI protocol with realistic streaming responses.",
    },
}

type ResponseGenerator struct {
    random *rand.Rand
}

func NewResponseGenerator() *ResponseGenerator {
    return &ResponseGenerator{
        random: rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

func (g *ResponseGenerator) GenerateResponse(userMessage string) string {
    lowerMsg := strings.ToLower(userMessage)

    for topic, resps := range topicResponses {
        if strings.Contains(lowerMsg, topic) {
            return resps[g.random.Intn(len(resps))]
        }
    }

    return topicResponses["default"][0]
}
```

**Streamer:**
```go
// services/mock/streamer.go
package mock

type Streamer struct {
    random *rand.Rand
}

func NewStreamer() *Streamer {
    return &Streamer{
        random: rand.New(rand.NewSource(time.Now().UnixNano())),
    }
}

func (s *Streamer) StreamResponse(response string, yieldFn func(chunk string)) {
    words := strings.Fields(response)

    for i, word := range words {
        chunk := word
        if i > 0 {
            chunk = " " + chunk
        }

        delay := 20 + s.random.Intn(60)
        time.Sleep(time.Duration(delay) * time.Millisecond)

        yieldFn(chunk)
    }
}
```

**Update Handler:**
```go
// handlers/chat.go
func HandleSendMessage(store *session.Store) http.HandlerFunc {
    mockGen := mock.NewResponseGenerator()
    streamer := mock.NewStreamer()

    return func(w http.ResponseWriter, r *http.Request) {
        // ... SSE setup ...

        message := r.URL.Query().Get("message")
        response := mockGen.GenerateResponse(message)

        // ... RUN_STARTED, TEXT_MESSAGE_START ...

        // Stream content
        streamer.StreamResponse(response, func(chunk string) {
            agui.WriteSSEEvent(w, &agui.TextMessageContentEvent{
                Type:      "TEXT_MESSAGE_CONTENT",
                MessageID: messageID,
                Delta:     chunk,
                Timestamp: time.Now().UnixMilli(),
            })
            flusher.Flush()
        })

        // ... TEXT_MESSAGE_END, RUN_FINISHED ...
    }
}
```

### Testing
1. Send various messages, verify different responses
2. Verify realistic streaming delays
3. Test topic detection
4. Check word-by-word streaming
5. Test rapid successive messages

---

## Critical Files Summary

### New Files to Create
1. `handlers/chat.go` - Main SSE handler and message routing
2. `internal/agui/events.go` - AG-UI event type definitions
3. `internal/agui/writer.go` - SSE event writer utilities
4. `internal/session/types.go` - Message and Thread types
5. `internal/session/store.go` - In-memory thread storage
6. `services/mock/responses.go` - Mock response generator
7. `services/mock/streamer.go` - Streaming simulation
8. `components/chat/message.templ` - Message component
9. `components/chat/typing-indicator.templ` - Typing animation
10. `assets/js/chat.js` - Frontend chat client

### Files to Modify
1. `main.go` - Add routes, initialize store, register handlers
2. `components/hello.templ` - Integrate message list and chat JS

### Dependencies to Add
- None required (using stdlib only)

---

## Estimated Implementation Time
- Phase 1: 1-2 hours
- Phase 2: 2-3 hours
- Phase 3: 2-3 hours
- Phase 4: 1-2 hours
- Phase 5: 3-4 hours
- Phase 6: 2-3 hours
- Phase 7: 2-3 hours

**Total: 13-20 hours**

---

## Post-MVP Enhancements
- Database persistence (SQLite/PostgreSQL)
- Real LLM integration (OpenAI, Anthropic, Ollama)
- Markdown rendering in messages
- Code syntax highlighting
- File uploads
- Multi-modal support (images)
- User authentication
