package agentorchestrator

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"iter"
	"strings"
	"sync"
	"time"

	"aicompass/foundation/agent"
	es "aicompass/foundation/eventsourcing"
	"aicompass/foundation/logger"
	"aicompass/modules/chat/chatcontracts"
	"aicompass/modules/chat/thread"
	"aicompass/modules/chat/threadmgmt"
	"aicompass/platform/artifact"
)

// EventEmitter is the interface for broadcasting events.
type EventEmitter interface {
	Dispatch(ctx context.Context, event thread.Event) error
}

// Runner is the interface for executing agents specifically within this service's thread.
type Runner interface {
	Run(ctx context.Context, req *agent.RunRequest) iter.Seq2[thread.Event, error]
}

// Service handles orchestration of agent events, providing persistence and broadcasting.
type Service struct {
	log           *logger.Logger
	writeModel    thread.EventStore
	threadRepo    *thread.ThreadRepository
	emitter       EventEmitter
	artifactStore artifact.Service
	profileRepo   chatcontracts.UserProfileRepository
	snapshotStore es.SnapshotStore

	runnerMu sync.RWMutex
	runner   Runner // Domain-aware runner interface

	// Internal components
	threadMgmt *threadmgmt.Manager
}

// Option is a functional option for configuring Service.
type Option func(*Service)

// New creates a new framework-agnostic agent orchestration service.
func New(
	log *logger.Logger,
	eventLog thread.EventStore,
	opts ...Option,
) *Service {
	s := &Service{
		log:        log,
		writeModel: eventLog,
	}

	for _, opt := range opts {
		opt(s)
	}

	// Build thread repo after options so snapshot store is available.
	sharedRepo := thread.NewThreadRepository(threadmgmt.NewEventStoreAdapter(eventLog), s.snapshotStore)
	s.threadRepo = sharedRepo
	s.threadMgmt = threadmgmt.New(log, sharedRepo)

	return s
}

// WithEmitter configures the agent with an event emitter for broadcasting.
func WithEmitter(emitter EventEmitter) Option {
	return func(s *Service) {
		s.emitter = emitter
	}
}

// WithArtifactService configures the agent with an artifact service.
func WithArtifactService(store artifact.Service) Option {
	return func(s *Service) {
		s.artifactStore = store
	}
}

// WithUserProfileRepository configures the agent with a user profile repository.
func WithUserProfileRepository(repo chatcontracts.UserProfileRepository) Option {
	return func(s *Service) {
		s.profileRepo = repo
	}
}

// WithSnapshotStore configures thread repositories to use snapshot-accelerated loading.
func WithSnapshotStore(store es.SnapshotStore) Option {
	return func(s *Service) {
		s.snapshotStore = store
	}
}

// WithRunner configures the agent with a runner.
func WithRunner(runner Runner) Option {
	return func(s *Service) {
		s.runner = runner
	}
}

// SetRunner sets the runner for the agent orchestrator.
func (s *Service) SetRunner(runner Runner) {
	s.runnerMu.Lock()
	defer s.runnerMu.Unlock()
	s.runner = runner
}

// ProcessEvents orchestrates a stream of domain events, providing persistence and broadcasting.
func (s *Service) ProcessEvents(ctx context.Context, events iter.Seq2[thread.Event, error]) iter.Seq2[thread.Event, error] {
	return func(yield func(thread.Event, error) bool) {
		eventCount := 0
		durableCount := 0
		transientCount := 0

		for event, err := range events {
			eventCount++

			if err != nil {
				s.log.Error(ctx, "Orchestrator: Event stream error", "eventNumber", eventCount, "error", err)
				yield(nil, err)
				return
			}

			// Handle persistence
			durable := s.isDurable(event)
			s.log.Debug(ctx, "Orchestrator: Processing event", "eventNumber", eventCount, "eventType", event.EventType(), "isDurable", durable)

			if durable {
				durableCount++
				s.log.Debug(ctx, "Orchestrator: Persisting durable event", "eventType", event.EventType())
				if pErr := s.persistDurableEvent(ctx, event); pErr != nil {
					s.log.Error(ctx, "Orchestrator: Failed to persist durable event", "eventType", event.EventType(), "error", pErr)
					yield(event, fmt.Errorf("persisting event: %w", pErr))
					return
				}
				s.log.Debug(ctx, "Orchestrator: Successfully persisted durable event", "eventType", event.EventType())
			} else {
				transientCount++
				s.log.Debug(ctx, "Orchestrator: Processing transient event", "eventType", event.EventType())
			}

			// Handle broadcasting
			if s.emitter != nil {
				// Broadcast ALL events (Durable and Transient) to prevent race conditions
				// where Runner finishes and closes SSE before outbox polling finishes.
				// The JS frontend event state machines are idempotent and safely swallow
				// the guaranteed duplicate events generated later by ProjectionUpdater.
				if bErr := s.emitter.Dispatch(ctx, event); bErr != nil {
					s.log.Error(ctx, "Orchestrator: Failed to dispatch event", "eventType", event.EventType(), "error", bErr)
				} else {
					s.log.Debug(ctx, "Orchestrator: Successfully dispatched event", "eventType", event.EventType())
				}
			}

			if !yield(event, nil) {
				s.log.Info(ctx, "Orchestrator: Event stream terminated early", "totalEvents", eventCount, "durable", durableCount, "transient", transientCount)
				return
			}
		}

		s.log.Info(ctx, "Orchestrator: Event stream completed", "totalEvents", eventCount, "durable", durableCount, "transient", transientCount)
	}
}

// isDurable returns true if the event should be persisted to the event store.
// Per EVENT_CONTRACTS.md: only business facts are persisted, not framework lifecycle events.
func (s *Service) isDurable(event thread.Event) bool {
	switch event.(type) {
	// Application Service Events (Persisted)
	case *thread.ThreadCreated,
		*thread.ThreadDeleted,
		*thread.ThreadTitleUpdated,
		*thread.UserMessageAdded:
		return true

	// Agent Framework Events - Persisted
	case *thread.AssistantMessageAdded,
		*thread.ToolResultAdded,
		*thread.TokenUsageReported,
		*thread.ToolNeedsInput,
		*thread.ToolNeedsAuth,
		*thread.InputProvided,
		*thread.AuthGranted,
		*thread.AuthDenied,
		*thread.ToolCancelled,
		*thread.StateChangeReported:
		return true

	// Agent Framework Events - Transient (NOT persisted)
	// Run lifecycle events are for frontend UI only, not thread state
	case *thread.RunStarted,
		*thread.RunFinished,
		*thread.RunError,
		*thread.AssistantMessageStarted,
		*thread.AssistantContentStreamed,
		*thread.AssistantMessageCompleted,
		*thread.ToolStart,
		*thread.ToolContentStreamed,
		*thread.AgentTransferStarted:
		return false

	default:
		return false
	}
}

// persistDurableEvent persists a domain event based on its type.
func (s *Service) persistDurableEvent(ctx context.Context, event thread.Event) error {
	switch e := event.(type) {
	case *thread.ThreadCreated:
		_, _, err := s.threadMgmt.CreateThread(ctx, e.ThreadID, e.UserID, e.Title)
		return err
	case *thread.ThreadDeleted:
		_, err := s.threadMgmt.DeleteThread(ctx, e.ThreadID)
		return err
	case *thread.UserMessageAdded:
		_, _, _, err := s.threadMgmt.AddUserMessage(ctx, e.ThreadID, e.MessageID, e.Content, e.UserID, e.Attachments...)
		return err
	case *thread.AssistantMessageAdded:
		_, _, err := s.threadMgmt.AddAssistantMessage(ctx, e.ThreadID, e.MessageID, e.Content, e.ToolCalls, e.AgentName)
		return err
	case *thread.ToolResultAdded:
		_, _, err := s.threadMgmt.AddToolResultMessage(ctx, e.ThreadID, e.MessageID, e.ToolCallID, e.ToolName, e.Result, e.AgentName)
		return err
	case *thread.ThreadTitleUpdated:
		_, err := s.threadMgmt.UpdateThreadTitle(ctx, e.ThreadID, e.Title)
		return err
	case *thread.TokenUsageReported:
		_, err := s.threadMgmt.AddTokenUsage(ctx, e)
		return err
	case *thread.ToolNeedsInput,
		*thread.ToolNeedsAuth,
		*thread.InputProvided,
		*thread.AuthGranted,
		*thread.AuthDenied,
		*thread.ToolCancelled:
		_, err := s.threadMgmt.RecordToolEvent(ctx, event.AggregateID(), event)
		return err
	case *thread.StateChangeReported:
		// State is persisted via the event store; the ThreadAggregate.Apply()
		// method updates the State map. No additional action needed here.
		return nil
	default:
		return nil
	}
}

// GetAvailableAgents returns a list of available agents
func (s *Service) GetAvailableAgents() []string {
	// Note: In framework-agnostic version, this should probably come from a central registry.
	// For now we return empty or implement a registry injection.
	return nil
}

// GetThreadManagement returns the thread management component.
func (s *Service) GetThreadManagement() *threadmgmt.Manager {
	return s.threadMgmt
}

// FileAttachment represents a file attached to a message (image, audio, PDF, etc.).
type FileAttachment struct {
	URL      string // Pre-uploaded artifact URL (e.g. /v1/threads/.../artifacts/file.wav)
	MimeType string // e.g. "image/png", "audio/wav", "application/pdf"
	FileName string // Original filename (e.g. "screenshot.png")
}

// RunExecutionRequest contains parameters for executing a run.
type RunExecutionRequest struct {
	ThreadID    string           // ID of the conversation thread
	UserID      string           // User making the request
	AgentName   string           // Agent to execute
	Message     string           // User message content
	MessageID   string           // Message ID (used as RunID)
	ToolCallID  string           // (Optional) Correlates this message as an answer to a tool elicitation
	ParentRunID string           // (Optional) Lineage pointer for branching/time travel
	Attachments []FileAttachment // (Optional) Attached files (images, audio, PDFs, etc.)
}

// ExecuteRun orchestrates a framework-agnostic run execution with lifecycle events.
// This method emits RunStarted, calls the runner, and emits RunFinished or RunError.
// It provides a consistent run lifecycle regardless of which framework adapter is used.
func (s *Service) ExecuteRun(
	ctx context.Context,
	req RunExecutionRequest,
) iter.Seq2[thread.Event, error] {
	return func(yield func(thread.Event, error) bool) {
		// 1. Emit RunStarted
		runStartedEvent := &thread.RunStarted{
			ThreadID:    req.ThreadID,
			RunID:       req.MessageID,
			ParentRunID: req.ParentRunID,
			Input:       req.Message,
			Timestamp:   time.Now(),
		}

		runStartedIter := func(y func(thread.Event, error) bool) {
			y(runStartedEvent, nil)
		}
		processedStart := s.ProcessEvents(ctx, runStartedIter)
		for evt, err := range processedStart {
			if err != nil {
				yield(nil, fmt.Errorf("failed to emit RunStarted: %w", err))
				return
			}
			if !yield(evt, nil) {
				return
			}
		}

		// 2. If this message is an answer to a tool elicitation, emit InputProvided
		if req.ToolCallID != "" {
			inputProvidedEvent := &thread.InputProvided{
				ThreadID:   req.ThreadID,
				MessageID:  req.MessageID,
				ToolCallID: req.ToolCallID,
				Input:      req.Message,
				AgentName:  req.AgentName,
				Timestamp:  time.Now(),
			}

			inputIter := func(y func(thread.Event, error) bool) {
				y(inputProvidedEvent, nil)
			}
			processedInput := s.ProcessEvents(ctx, inputIter)
			for evt, err := range processedInput {
				if err != nil {
					yield(nil, fmt.Errorf("failed to emit InputProvided: %w", err))
					return
				}
				if !yield(evt, nil) {
					return
				}
			}
		}

		// 3. Call the framework runner to get events
		s.runnerMu.RLock()
		runner := s.runner
		s.runnerMu.RUnlock()
		if runner == nil {
			yield(nil, fmt.Errorf("runner not configured"))
			return
		}

		// Save attachments to artifacts and build refs for event persistence
		attachmentRefs := s.saveAttachments(ctx, req)

		runnerReq := &agent.RunRequest{
			SessionID: req.ThreadID,
			UserID:    req.UserID,
			AgentName: req.AgentName,
			Message: &agent.Message{
				ID:      req.MessageID,
				Content: s.buildMessageContent(ctx, req),
			},
			Context: map[string]any{
				"attachmentRefs": attachmentRefs,
			},
		}
		rawDomainEvents := runner.Run(ctx, runnerReq)

		// 4. Process events for persistence and broadcasting
		processedEvents := s.ProcessEvents(ctx, rawDomainEvents)

		// 5. Yield orchestrated events to caller and check for errors
		var executionError error
		awaitingInteraction := false
		for de, err := range processedEvents {
			// Check if context was cancelled (client disconnected)
			if ctx.Err() != nil {
				s.log.Info(ctx, "Orchestrator: Context cancelled, stopping execution")
				return
			}
			if err != nil {
				executionError = err
				break
			}
			// Detect if the run paused for user interaction (input or auth).
			// When this happens we must NOT emit RunFinished — the run is
			// suspended, not done.
			switch de.(type) {
			case *thread.ToolNeedsInput, *thread.ToolNeedsAuth:
				awaitingInteraction = true
			}
			if !yield(de, nil) {
				return
			}
		}

		// 6. Emit RunError if execution failed, otherwise emit RunFinished
		if executionError != nil {
			runErrorEvent := &thread.RunError{
				ThreadID:  req.ThreadID,
				RunID:     req.MessageID,
				Message:   executionError.Error(),
				Code:      "RUN_ERROR",
				Timestamp: time.Now(),
			}

			runErrorIter := func(y func(thread.Event, error) bool) {
				y(runErrorEvent, nil)
			}
			processedError := s.ProcessEvents(ctx, runErrorIter)
			for evt, err := range processedError {
				if err != nil {
					yield(nil, fmt.Errorf("failed to emit RunError: %w", err))
					return
				}
				if !yield(evt, nil) {
					return
				}
			}

			// Yield the original execution error
			yield(nil, executionError)
			return
		}

		// 7. If the run is paused awaiting user interaction, do NOT emit
		// RunFinished. The SSE stream will end without a terminal lifecycle
		// event, signalling to the frontend that the run is suspended.
		if awaitingInteraction {
			return
		}

		// 8. Emit RunFinished on success
		runFinishedEvent := &thread.RunFinished{
			ThreadID:  req.ThreadID,
			RunID:     req.MessageID,
			Result:    nil, // TODO: Capture result from framework events if available
			Timestamp: time.Now(),
		}

		runFinishedIter := func(y func(thread.Event, error) bool) {
			y(runFinishedEvent, nil)
		}
		processedFinish := s.ProcessEvents(ctx, runFinishedIter)
		for evt, err := range processedFinish {
			if err != nil {
				yield(nil, fmt.Errorf("failed to emit RunFinished: %w", err))
				return
			}
			if !yield(evt, nil) {
				return
			}
		}
	}
}

// saveAttachments saves attached files to the artifact store and returns
// thread.Attachment slices for event persistence. If the artifact store is nil or
// no attachments are present, it returns nil.
//
// Pre-uploaded attachments (those with a URL starting with "/v1/threads/") are
// already in S3 and are passed through without re-uploading.
func (s *Service) saveAttachments(ctx context.Context, req RunExecutionRequest) []thread.Attachment {
	if s.artifactStore == nil || len(req.Attachments) == 0 {
		return nil
	}

	var refs []thread.Attachment
	for _, att := range req.Attachments {
		if att.URL == "" {
			continue
		}
		refs = append(refs, thread.Attachment{
			URL:      att.URL,
			MimeType: att.MimeType,
			FileName: att.FileName,
		})
	}
	return refs
}

// mimeToExt returns a file extension for common MIME types.
func mimeToExt(mimeType string) string {
	switch mimeType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "audio/wav", "audio/x-wav":
		return "wav"
	case "audio/mpeg", "audio/mp3":
		return "mp3"
	case "audio/ogg":
		return "ogg"
	case "audio/webm":
		return "webm"
	case "application/pdf":
		return "pdf"
	case "text/plain":
		return "txt"
	case "text/csv":
		return "csv"
	case "text/markdown":
		return "md"
	default:
		if idx := strings.LastIndex(mimeType, "/"); idx >= 0 {
			ext := mimeType[idx+1:]
			if plus := strings.Index(ext, "+"); plus >= 0 {
				ext = ext[:plus]
			}
			return ext
		}
		return "bin"
	}
}

// buildMessageContent constructs the Content slice for a user message,
// combining text and optional file attachments. It classifies each attachment
// by MIME type into the appropriate agent content type (image, audio, or file).
//
// For pre-uploaded attachments (URL set, Data empty), the file data is loaded
// from S3 and a data URL is reconstructed for the LLM.
func (s *Service) buildMessageContent(ctx context.Context, req RunExecutionRequest) []agent.Content {
	text := req.Message
	attachments := req.Attachments

	var content []agent.Content

	if text != "" {
		content = append(content, agent.Content{
			Type: agent.ContentTypeText,
			Text: text,
		})
	}

	for _, att := range attachments {
		// Load the artifact from S3 and convert to a data URL for the LLM.
		if att.URL == "" || s.artifactStore == nil {
			continue
		}

		// Extract the actual S3 filename from the artifact URL.
		// URL format: /v1/threads/{threadId}/artifacts/{uuid_originalName}
		// att.FileName has the original name (e.g. "Logo.jpg") but S3
		// stores the UUID-prefixed version (e.g. "65f313b5_Logo.jpg").
		s3FileName := att.FileName
		if parts := strings.Split(att.URL, "/"); len(parts) > 0 {
			s3FileName = parts[len(parts)-1]
		}
		dataURL, err := s.loadArtifactAsDataURL(ctx, req.UserID, req.ThreadID, s3FileName, att.MimeType)
		if err != nil {
			s.log.Warn(ctx, "Failed to load pre-uploaded attachment for LLM, skipping",
				"error", err, "fileName", s3FileName, "url", att.URL)
			continue
		}
		if dataURL == "" {
			continue
		}

		switch {
		case strings.HasPrefix(att.MimeType, "image/"):
			content = append(content, agent.Content{
				Type: agent.ContentTypeImageURL,
				ImageURL: &agent.ImageURL{
					URL:    dataURL,
					Detail: "auto",
				},
			})
		case strings.HasPrefix(att.MimeType, "audio/"):
			content = append(content, agent.Content{
				Type: agent.ContentTypeInputAudio,
				AudioInput: &agent.AudioInput{
					Data:   stripDataURLPrefix(dataURL),
					Format: audioFormat(att.MimeType),
				},
			})
		default:
			content = append(content, agent.Content{
				Type: agent.ContentTypeFile,
				FileInput: &agent.FileInput{
					Data:     stripDataURLPrefix(dataURL),
					FileName: att.FileName,
					MimeType: att.MimeType,
				},
			})
		}
	}

	// Ensure at least one content part
	if len(content) == 0 {
		content = append(content, agent.Content{
			Type: agent.ContentTypeText,
			Text: "",
		})
	}

	return content
}

// maxArtifactLoadSize caps how many bytes loadArtifactAsDataURL will read
// into memory. This prevents a single large file from causing OOM under
// concurrent load (100K users × 20 MB = 2 TB if unbounded).
const maxArtifactLoadSize = 20 << 20 // 20 MB

// loadArtifactAsDataURL loads a file from the artifact store and returns it as
// a base64 data URL suitable for sending to LLM APIs.
// Reads are capped at maxArtifactLoadSize to prevent OOM.
func (s *Service) loadArtifactAsDataURL(ctx context.Context, userID, threadID, fileName, mimeType string) (string, error) {
	reader, meta, err := s.artifactStore.Load(ctx, userID, threadID, fileName)
	if err != nil {
		return "", fmt.Errorf("load artifact: %w", err)
	}
	defer reader.Close()

	data, err := io.ReadAll(io.LimitReader(reader, maxArtifactLoadSize+1))
	if err != nil {
		return "", fmt.Errorf("read artifact data: %w", err)
	}
	if int64(len(data)) > maxArtifactLoadSize {
		return "", fmt.Errorf("artifact too large for LLM: %s (%d bytes exceeds %d byte limit)",
			fileName, meta.Size, maxArtifactLoadSize)
	}

	if mimeType == "" {
		mimeType = meta.ContentType
	}
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64.StdEncoding.EncodeToString(data)), nil
}

// stripDataURLPrefix removes the data:...;base64, prefix from a data URL,
// returning just the raw base64 content.
func stripDataURLPrefix(dataURL string) string {
	if idx := strings.Index(dataURL, ","); idx >= 0 {
		return dataURL[idx+1:]
	}
	return dataURL
}

// audioFormat returns the audio format string for the LLM from a MIME type.
func audioFormat(mimeType string) string {
	switch mimeType {
	case "audio/wav", "audio/x-wav":
		return "wav"
	case "audio/mpeg", "audio/mp3":
		return "mp3"
	default:
		return "wav"
	}
}
