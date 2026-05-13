package a2a

import (
	"aicompass/foundation/agent"
	"aicompass/foundation/logger"
	"aicompass/foundation/toolregistry"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"iter"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	a2atypes "github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2aclient"
	"github.com/a2aproject/a2a-go/v2/a2aclient/agentcard"
)

// --------------------------------------------------------------------------
// Configuration constants
// --------------------------------------------------------------------------

const (
	maxConnectorsPerUser = 20

	// Agent card discovery settings
	discoverTimeout    = 15 * time.Second
	discoverMaxRetries = 2
	discoverRetryDelay = 2 * time.Second

	// Task execution settings
	taskTimeout = 120 * time.Second

	// Cache settings
	connectorCacheTTL = 30 * time.Second
	cardCacheTTL      = 5 * time.Minute
)

// Service orchestrates A2A connector lifecycle: create, discover agent card,
// manage skills, and execute tasks on remote agents.
// Designed for horizontal scaling — no persistent connections held in memory.
type Service struct {
	log                  *logger.Logger
	repo                 A2AConnectorRepository
	toolRegistry         *toolregistry.Registry
	allowPrivateNetworks bool
	connectorCache       *ttlCache[*A2AConnector]
	cardCache            *ttlCache[*a2atypes.AgentCard]
	httpClient           *http.Client
	stopCleanup          chan struct{}
}

// NewService creates a new A2A discovery service.
func NewService(log *logger.Logger, repo A2AConnectorRepository, toolRegistry *toolregistry.Registry) *Service {
	allowPrivate := os.Getenv("A2A_ALLOW_PRIVATE_NETWORKS") == "true"
	s := &Service{
		log:                  log,
		repo:                 repo,
		toolRegistry:         toolRegistry,
		allowPrivateNetworks: allowPrivate,
		connectorCache:       newTTLCache[*A2AConnector](connectorCacheTTL),
		cardCache:            newTTLCache[*a2atypes.AgentCard](cardCacheTTL),
		httpClient: &http.Client{
			Timeout: discoverTimeout,
			Transport: &http.Transport{
				MaxIdleConns:        50,
				MaxIdleConnsPerHost: 5,
				IdleConnTimeout:     90 * time.Second,
			},
		},
		stopCleanup: make(chan struct{}),
	}

	// Background cache cleanup every 60s.
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.connectorCache.Cleanup()
				s.cardCache.Cleanup()
			case <-s.stopCleanup:
				return
			}
		}
	}()

	return s
}

// Shutdown stops background goroutines.
func (s *Service) Shutdown() {
	close(s.stopCleanup)
}

// --------------------------------------------------------------------------
// CRUD operations
// --------------------------------------------------------------------------

// CreateConnector registers a new remote A2A agent endpoint.
func (s *Service) CreateConnector(ctx context.Context, userID, agentURL, title, authType, authToken, authHeader string) (*A2AConnector, error) {
	normalizedURL, err := s.resolveAgentURL(agentURL)
	if err != nil {
		return nil, err
	}

	if err := s.checkConnectorQuota(ctx, userID, normalizedURL); err != nil {
		return nil, err
	}

	if title == "" {
		title = titleFromURL(normalizedURL)
	}
	if authType == "" {
		authType = "none"
	}

	connector := &A2AConnector{
		UserID:     userID,
		Title:      title,
		AgentURL:   normalizedURL,
		Status:     ConnectorStatusCreated,
		AuthType:   authType,
		AuthToken:  authToken,
		AuthHeader: authHeader,
	}

	if err := s.repo.Create(ctx, connector); err != nil {
		return nil, fmt.Errorf("creating connector: %w", err)
	}

	return connector, nil
}

// resolveAgentURL parses and validates the agent URL, applies SSRF protection
// (or — in private-networks mode — rewrites host-local addresses for Docker),
// and returns the URL string the connector should be persisted with.
func (s *Service) resolveAgentURL(agentURL string) (string, error) {
	parsed, err := url.Parse(agentURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("invalid agent URL: must be http or https")
	}
	if !s.allowPrivateNetworks {
		if err := validateExternalURL(parsed); err != nil {
			return "", err
		}
		return agentURL, nil
	}
	rewritten, _ := rewriteForDocker(agentURL, parsed)
	return rewritten, nil
}

// checkConnectorQuota enforces per-user limits before creating a new connector:
// max count per user and URL uniqueness within that user.
func (s *Service) checkConnectorQuota(ctx context.Context, userID, agentURL string) error {
	count, err := s.repo.CountByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("checking connector count: %w", err)
	}
	if count >= maxConnectorsPerUser {
		return ErrConnectorLimitReached
	}
	existing, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("checking duplicates: %w", err)
	}
	for _, c := range existing {
		if c.AgentURL == agentURL {
			return ErrDuplicateConnector
		}
	}
	return nil
}

// GetConnector retrieves a connector, scoped to user.
func (s *Service) GetConnector(ctx context.Context, id, userID string) (*A2AConnector, error) {
	cacheKey := id + ":" + userID
	if cached, ok := s.connectorCache.Get(cacheKey); ok {
		return cached, nil
	}

	c, err := s.repo.GetByIDAndUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrConnectorNotFound
	}

	s.connectorCache.Set(cacheKey, c)
	return c, nil
}

// GetConnectorInfo retrieves connector metadata for use as a workflow sub-agent.
// Implements adkgo.A2ASubAgentProvider.
func (s *Service) GetConnectorInfo(ctx context.Context, connectorID, userID string) (ConnectorInfo, error) {
	c, err := s.GetConnector(ctx, connectorID, userID)
	if err != nil {
		return ConnectorInfo{}, err
	}
	name := c.AgentName
	if name == "" {
		name = c.Title
	}
	return ConnectorInfo{
		ID:          c.ID,
		Name:        name,
		Description: c.AgentDescription,
		AgentURL:    c.AgentURL,
	}, nil
}

// ListConnectors returns all connectors for a user.
func (s *Service) ListConnectors(ctx context.Context, userID string) ([]A2AConnector, error) {
	return s.repo.ListByUser(ctx, userID)
}

// DeleteConnector removes a connector and its skills.
func (s *Service) DeleteConnector(ctx context.Context, id, userID string) error {
	// Unregister from tool registry before deletion
	s.unregisterConnectorFromToolRegistry(id)

	if err := s.repo.Delete(ctx, id, userID); err != nil {
		return err
	}

	s.connectorCache.Delete(id + ":" + userID)
	return nil
}

// --------------------------------------------------------------------------
// Connection lifecycle
// --------------------------------------------------------------------------

// ConnectConnector discovers the remote agent's card and populates skills.
func (s *Service) ConnectConnector(ctx context.Context, id, userID string) (*A2AConnector, error) {
	connector, err := s.repo.GetByIDAndUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if connector == nil {
		return nil, ErrConnectorNotFound
	}

	// Transition to discovering
	if err := connector.Transition(ConnectorStatusDiscovering, "", false); err != nil {
		return nil, ErrInvalidConnectorState
	}
	connector.ErrorMsg = ""
	_ = s.repo.Update(ctx, connector)

	// Discover agent card
	card, err := s.discoverAgentCard(ctx, connector)
	if err != nil {
		s.log.Error(ctx, "a2a: agent card discovery failed", "error", err, "url", connector.AgentURL)
		connector.ErrorMsg = err.Error()
		_ = connector.Transition(ConnectorStatusError, "", false)
		_ = s.repo.Update(ctx, connector)
		return connector, fmt.Errorf("agent card discovery failed: %w", err)
	}

	// Populate connector from agent card
	s.populateFromCard(connector, card)

	// Transition to connected
	if err := connector.Transition(ConnectorStatusConnected, "", false); err != nil {
		return connector, err
	}
	if err := s.repo.Update(ctx, connector); err != nil {
		return connector, fmt.Errorf("updating connector after discovery: %w", err)
	}

	// Extract and persist skills
	skills := extractSkills(connector.ID, card)
	if err := s.repo.UpsertSkills(ctx, connector.ID, skills); err != nil {
		s.log.Error(ctx, "a2a: failed to persist skills", "error", err)
	}

	// Register in tool registry for permission management
	s.registerConnectorInToolRegistry(connector, skills)

	s.connectorCache.Delete(id + ":" + userID)
	return connector, nil
}

// DisconnectConnector marks a connector as disconnected.
func (s *Service) DisconnectConnector(ctx context.Context, id, userID, reason string) (*A2AConnector, error) {
	connector, err := s.repo.GetByIDAndUser(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if connector == nil {
		return nil, ErrConnectorNotFound
	}

	if reason == "" {
		reason = "user requested disconnect"
	}
	if err := connector.Transition(ConnectorStatusDisconnected, reason, false); err != nil {
		return nil, ErrInvalidConnectorState
	}
	if err := s.repo.Update(ctx, connector); err != nil {
		return nil, fmt.Errorf("updating connector: %w", err)
	}

	s.unregisterConnectorFromToolRegistry(id)
	s.connectorCache.Delete(id + ":" + userID)
	return connector, nil
}

// RefreshSkills re-discovers the agent card and updates skills.
func (s *Service) RefreshSkills(ctx context.Context, id, userID string) (*A2AConnector, error) {
	// Force reconnect to refresh card + skills
	return s.ConnectConnector(ctx, id, userID)
}

// --------------------------------------------------------------------------
// Agent card discovery
// --------------------------------------------------------------------------

func (s *Service) discoverAgentCard(ctx context.Context, connector *A2AConnector) (*a2atypes.AgentCard, error) {
	// Check cache first
	if card, ok := s.cardCache.Get(connector.AgentURL); ok {
		return card, nil
	}

	// Build resolver options for auth
	opts := s.buildResolveOptions(connector)

	var lastErr error
	for attempt := 0; attempt <= discoverMaxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(discoverRetryDelay)
		}

		resolver := &agentcard.Resolver{Client: s.httpClient}
		card, err := resolver.Resolve(ctx, connector.AgentURL, opts...)
		if err != nil {
			lastErr = err
			s.log.Error(ctx, "a2a: card discovery attempt failed",
				"attempt", attempt+1, "error", err, "url", connector.AgentURL)
			continue
		}

		// Validate card
		if card.Name == "" {
			return nil, ErrAgentCardInvalid
		}

		// Cache the card
		s.cardCache.Set(connector.AgentURL, card)
		return card, nil
	}

	return nil, fmt.Errorf("agent card discovery failed after %d attempts: %w", discoverMaxRetries+1, lastErr)
}

func (s *Service) buildResolveOptions(connector *A2AConnector) []agentcard.ResolveOption {
	var opts []agentcard.ResolveOption

	switch connector.AuthType {
	case "bearer":
		if connector.AuthToken != "" {
			opts = append(opts, agentcard.WithRequestHeader("Authorization", "Bearer "+connector.AuthToken))
		}
	case "api_key":
		headerName := "X-API-Key"
		if connector.AuthHeader != "" {
			headerName = connector.AuthHeader
		}
		if connector.AuthToken != "" {
			opts = append(opts, agentcard.WithRequestHeader(headerName, connector.AuthToken))
		}
	case "custom_header":
		if connector.AuthHeader != "" && connector.AuthToken != "" {
			opts = append(opts, agentcard.WithRequestHeader(connector.AuthHeader, connector.AuthToken))
		}
	}

	return opts
}

func (s *Service) populateFromCard(connector *A2AConnector, card *a2atypes.AgentCard) {
	connector.AgentName = card.Name
	connector.AgentDescription = card.Description
	connector.AgentVersion = card.Version

	if card.Provider != nil {
		providerJSON, _ := json.Marshal(card.Provider)
		connector.AgentProvider = string(providerJSON)
	}

	cardJSON, _ := json.Marshal(card)
	connector.AgentCardJSON = string(cardJSON)

	capsJSON, _ := json.Marshal(card.Capabilities)
	connector.Capabilities = string(capsJSON)

	connector.IconURL = card.IconURL
}

// --------------------------------------------------------------------------
// Skill extraction
// --------------------------------------------------------------------------

func extractSkills(connectorID string, card *a2atypes.AgentCard) []A2ADiscoveredSkill {
	skills := make([]A2ADiscoveredSkill, 0, len(card.Skills))
	for _, skill := range card.Skills {
		tagsJSON, _ := json.Marshal(skill.Tags)
		examplesJSON, _ := json.Marshal(skill.Examples)
		inputModesJSON, _ := json.Marshal(skill.InputModes)
		outputModesJSON, _ := json.Marshal(skill.OutputModes)

		skills = append(skills, A2ADiscoveredSkill{
			ConnectorID: connectorID,
			SkillID:     skill.ID,
			Name:        skill.Name,
			Description: skill.Description,
			Tags:        string(tagsJSON),
			Examples:    string(examplesJSON),
			InputModes:  string(inputModesJSON),
			OutputModes: string(outputModesJSON),
		})
	}
	return skills
}

// --------------------------------------------------------------------------
// Task execution (invoke remote A2A agent)
// --------------------------------------------------------------------------

// SendMessage sends a message to a remote A2A agent and returns the result.
// This is the core method used by A2ARemoteTool when agents invoke remote skills.
func (s *Service) SendMessage(ctx context.Context, connectorID, userID, message string) (string, error) {
	connector, err := s.GetConnector(ctx, connectorID, userID)
	if err != nil {
		return "", err
	}
	if connector.Status != ConnectorStatusConnected {
		return "", ErrConnectorNotConnected
	}

	// Resolve agent card (cached)
	card, err := s.discoverAgentCard(ctx, connector)
	if err != nil {
		return "", fmt.Errorf("resolving agent card: %w", err)
	}

	// Build A2A client with auth
	clientOpts := s.buildClientOptions(connector)
	client, err := a2aclient.NewFromCard(ctx, card, clientOpts...)
	if err != nil {
		return "", fmt.Errorf("creating A2A client: %w", err)
	}
	defer func() { _ = client.Destroy() }()

	// Build request
	userMsg := a2atypes.NewMessage(a2atypes.MessageRoleUser, a2atypes.NewTextPart(message))
	req := &a2atypes.SendMessageRequest{
		Message: userMsg,
	}

	// Execute with timeout
	taskCtx, cancel := context.WithTimeout(ctx, taskTimeout)
	defer cancel()

	result, err := client.SendMessage(taskCtx, req)
	if err != nil {
		return "", fmt.Errorf("sending message to remote agent: %w", err)
	}

	// Extract response text
	return extractResponseText(result), nil
}

// SendStreamingMessage streams chunks from a remote A2A agent.
// Text deltas (Delta != "") carry incremental LLM output; artifact chunks
// (IsArtifact=true) carry complete artifacts produced by the remote agent.
// Implements adkgo.A2ASubAgentProvider.
func (s *Service) SendStreamingMessage(ctx context.Context, connectorID, userID, message string) iter.Seq2[StreamChunk, error] {
	return func(yield func(StreamChunk, error) bool) {
		connector, err := s.GetConnector(ctx, connectorID, userID)
		if err != nil {
			yield(StreamChunk{}, err)
			return
		}
		if connector.Status != ConnectorStatusConnected {
			yield(StreamChunk{}, ErrConnectorNotConnected)
			return
		}

		card, err := s.discoverAgentCard(ctx, connector)
		if err != nil {
			yield(StreamChunk{}, fmt.Errorf("resolving agent card: %w", err))
			return
		}

		clientOpts := s.buildClientOptions(connector)
		client, err := a2aclient.NewFromCard(ctx, card, clientOpts...)
		if err != nil {
			yield(StreamChunk{}, fmt.Errorf("creating A2A client: %w", err))
			return
		}
		defer func() { _ = client.Destroy() }()

		userMsg := a2atypes.NewMessage(a2atypes.MessageRoleUser, a2atypes.NewTextPart(message))
		req := &a2atypes.SendMessageRequest{
			Message: userMsg,
		}

		taskCtx, cancel := context.WithTimeout(ctx, taskTimeout)
		defer cancel()

		for event, streamErr := range client.SendStreamingMessage(taskCtx, req) {
			if streamErr != nil {
				yield(StreamChunk{}, fmt.Errorf("streaming from remote agent: %w", streamErr))
				return
			}

			chunks, err := extractEventChunks(event)
			if err != nil {
				yield(StreamChunk{}, err)
				return
			}
			for _, chunk := range chunks {
				if !yield(chunk, nil) {
					return
				}
			}
		}
	}
}

// extractEventChunks mirrors AgentStack's handleStatusUpdate + handleArtifactUpdate
// from client.ts, translating raw A2A events into typed StreamChunks.
//
// Rules (same as AgentStack):
//   - failed/rejected state   → return error (AgentStack throws)
//   - working state + message → text delta chunks + file chunks from message parts
//   - artifact-update, text only   → artifact card chunk + file chunks
//   - artifact-update, files only  → file chunks (no card wrapper)
//   - artifact-update, text+files  → artifact card chunk + file chunks
func extractEventChunks(event a2atypes.Event) ([]StreamChunk, error) {
	switch v := event.(type) {
	case nil:
		return nil, nil
	case *a2atypes.Message:
		return chunksFromMessage(v), nil
	case *a2atypes.TaskStatusUpdateEvent:
		return chunksFromStatusUpdate(v)
	case *a2atypes.TaskArtifactUpdateEvent:
		return chunksFromArtifactUpdate(v), nil
	case *a2atypes.Task:
		return chunksFromTask(v), nil
	}
	return nil, nil
}

// chunksFromMessage handles the non-streaming final-response shape (agents
// using sendMessage): extract text deltas + file chunks from all parts.
func chunksFromMessage(m *a2atypes.Message) []StreamChunk {
	return splitPartsToChunks(m.Parts, "")
}

// chunksFromStatusUpdate handles task-status transitions:
// failed/rejected propagate as errors; only working-state updates with a
// carrying message emit streaming chunks.
func chunksFromStatusUpdate(v *a2atypes.TaskStatusUpdateEvent) ([]StreamChunk, error) {
	state := v.Status.State
	if state == a2atypes.TaskStateFailed || state == a2atypes.TaskStateRejected {
		return nil, fmt.Errorf("%s", statusFailureMessage(v))
	}
	if state != a2atypes.TaskStateWorking || v.Status.Message == nil {
		return nil, nil
	}
	return splitPartsToChunks(v.Status.Message.Parts, ""), nil
}

// statusFailureMessage extracts a human-readable failure message from a
// failed/rejected status event, falling back to a generic string.
func statusFailureMessage(v *a2atypes.TaskStatusUpdateEvent) string {
	const fallback = "remote agent task failed"
	if v.Status.Message == nil {
		return fallback
	}
	if t := extractTextFromParts(v.Status.Message.Parts); t != "" {
		return t
	}
	return fallback
}

// chunksFromArtifactUpdate mirrors handleArtifactUpdate: skip partial
// append chunks (wait for LastChunk) and split the final artifact into
// text-card + file chunks.
func chunksFromArtifactUpdate(v *a2atypes.TaskArtifactUpdateEvent) []StreamChunk {
	if v.Artifact == nil || (v.Append && !v.LastChunk) {
		return nil
	}
	return buildArtifactChunks(v.Artifact)
}

// chunksFromTask handles the blocking-response shape: text delta from
// the status message plus any attached artifacts.
func chunksFromTask(v *a2atypes.Task) []StreamChunk {
	var chunks []StreamChunk
	if v.Status.Message != nil {
		chunks = append(chunks, splitPartsToChunks(v.Status.Message.Parts, "")...)
	}
	for _, a := range v.Artifacts {
		chunks = append(chunks, buildArtifactChunks(a)...)
	}
	return chunks
}

// buildArtifactChunks mirrors handleArtifactUpdate from AgentStack client.ts:
// - Split artifact parts into text chunks and file chunks.
// - If only files (no text) → return file chunks directly (no card).
// - If has text → return [artifactCard, ...fileChunks].
func buildArtifactChunks(artifact *a2atypes.Artifact) []StreamChunk {
	textChunks, fileChunks := splitPartsTyped(artifact.Parts)

	if len(textChunks) == 0 {
		// Files only — return as plain file attachment chunks (no artifact card).
		return fileChunks
	}

	// Concatenate all text parts into one artifact card.
	var sb strings.Builder
	for _, c := range textChunks {
		sb.WriteString(c.Content)
	}
	artifactChunk := StreamChunk{
		IsArtifact: true,
		ArtifactID: string(artifact.ID),
		Title:      artifact.Name,
		Content:    sb.String(),
	}
	return append([]StreamChunk{artifactChunk}, fileChunks...)
}

// splitPartsToChunks converts a slice of Parts into StreamChunks.
// Text parts → Delta chunks; URL/Raw parts → file chunks; Data parts → skipped.
func splitPartsToChunks(parts a2atypes.ContentParts, _ string) []StreamChunk {
	textChunks, fileChunks := splitPartsTyped(parts)
	// For plain message parts, text becomes Delta chunks (streaming text).
	var result []StreamChunk
	for _, c := range textChunks {
		result = append(result, StreamChunk{Delta: c.Content})
	}
	return append(result, fileChunks...)
}

// splitPartsTyped separates parts into text-content chunks and file chunks.
func splitPartsTyped(parts a2atypes.ContentParts) (textChunks []StreamChunk, fileChunks []StreamChunk) {
	for _, part := range parts {
		if part == nil {
			continue
		}
		text, file := classifyPart(part)
		if text != nil {
			textChunks = append(textChunks, *text)
		}
		if file != nil {
			fileChunks = append(fileChunks, *file)
		}
	}
	return
}

// classifyPart routes a single Part to either the text-chunk or file-chunk
// shape based on the underlying Content type. Data parts (structured
// JSON) and empty values return (nil, nil) — no UI representation yet.
func classifyPart(part *a2atypes.Part) (text *StreamChunk, file *StreamChunk) {
	switch c := part.Content.(type) {
	case a2atypes.Text:
		if string(c) == "" {
			return nil, nil
		}
		return &StreamChunk{Content: string(c)}, nil
	case a2atypes.URL:
		if string(c) == "" {
			return nil, nil
		}
		return nil, &StreamChunk{
			IsFile:   true,
			FileURL:  string(c),
			FileName: part.Filename,
			MimeType: part.MediaType,
		}
	case a2atypes.Raw:
		if len(c) == 0 {
			return nil, nil
		}
		return nil, rawPartToFileChunk(part, []byte(c))
	}
	return nil, nil
}

// rawPartToFileChunk renders an inline Raw part as a data-URL file chunk,
// defaulting the mime type when the part didn't declare one.
func rawPartToFileChunk(part *a2atypes.Part, raw []byte) *StreamChunk {
	mimeType := part.MediaType
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return &StreamChunk{
		IsFile:   true,
		FileURL:  "data:" + mimeType + ";base64," + encodeBase64(raw),
		FileName: part.Filename,
		MimeType: mimeType,
	}
}

// encodeBase64 encodes bytes to standard base64 string.
func encodeBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// extractTextFromParts extracts and joins text content from A2A parts.
func extractTextFromParts(parts []*a2atypes.Part) string {
	var texts []string
	for _, part := range parts {
		if text, ok := part.Content.(a2atypes.Text); ok {
			texts = append(texts, string(text))
		}
	}
	return strings.Join(texts, "")
}

func (s *Service) buildClientOptions(connector *A2AConnector) []a2aclient.FactoryOption {
	var opts []a2aclient.FactoryOption

	// Add auth interceptor if needed
	if connector.AuthType != "none" && connector.AuthType != "" && connector.AuthToken != "" {
		opts = append(opts, a2aclient.WithCallInterceptors(&authInterceptor{
			authType:   connector.AuthType,
			authToken:  connector.AuthToken,
			authHeader: connector.AuthHeader,
		}))
	}

	return opts
}

// authInterceptor implements a2aclient.CallInterceptor for auth token injection.
type authInterceptor struct {
	authType   string
	authToken  string
	authHeader string
}

func (a *authInterceptor) Before(ctx context.Context, req *a2aclient.Request) (context.Context, any, error) {
	if req.ServiceParams == nil {
		req.ServiceParams = make(a2aclient.ServiceParams)
	}
	switch a.authType {
	case "bearer":
		req.ServiceParams.Append("Authorization", "Bearer "+a.authToken)
	case "api_key":
		headerName := "X-API-Key"
		if a.authHeader != "" {
			headerName = a.authHeader
		}
		req.ServiceParams.Append(headerName, a.authToken)
	case "custom_header":
		if a.authHeader != "" {
			req.ServiceParams.Append(a.authHeader, a.authToken)
		}
	}
	return ctx, nil, nil
}

func (a *authInterceptor) After(_ context.Context, _ *a2aclient.Response) error {
	return nil
}

func extractResponseText(result a2atypes.SendMessageResult) string {
	var texts []string
	switch r := result.(type) {
	case nil:
		return ""
	case *a2atypes.Task:
		texts = textsFromTask(r)
	case *a2atypes.Message:
		texts = textsFromParts(r.Parts)
	}
	return strings.Join(texts, "\n")
}

// textsFromTask collects every text part reachable from a Task — the
// status message plus every artifact.
func textsFromTask(r *a2atypes.Task) []string {
	var texts []string
	if r.Status.Message != nil {
		texts = append(texts, textsFromParts(r.Status.Message.Parts)...)
	}
	for _, artifact := range r.Artifacts {
		texts = append(texts, textsFromParts(artifact.Parts)...)
	}
	return texts
}

// textsFromParts returns the non-empty string values of every Text part.
func textsFromParts(parts []*a2atypes.Part) []string {
	var texts []string
	for _, part := range parts {
		if text, ok := part.Content.(a2atypes.Text); ok {
			texts = append(texts, string(text))
		}
	}
	return texts
}

// --------------------------------------------------------------------------
// Tool resolution (for agent wiring)
// --------------------------------------------------------------------------

// GetConnectorSkills returns skills for a connector.
func (s *Service) GetConnectorSkills(ctx context.Context, connectorID, userID string) ([]A2ADiscoveredSkill, error) {
	c, err := s.repo.GetByIDAndUser(ctx, connectorID, userID)
	if err != nil {
		return nil, err
	}
	if c == nil {
		return nil, ErrConnectorNotFound
	}
	return s.repo.ListSkills(ctx, connectorID)
}

// CountSkillsByConnectors returns skill counts for multiple connectors.
func (s *Service) CountSkillsByConnectors(ctx context.Context, ids []string) (map[string]int, error) {
	return s.repo.CountSkillsByConnectors(ctx, ids)
}

// ResolveEnabledTools returns ONE executable tool per connected A2A agent.
//
// Unlike MCP (which exposes individual tools per server), an A2A agent is an
// opaque peer — the correct granularity is one tool per agent, matching the
// ADK-Go agenttool pattern. Enable/disable is at the agent level only;
// hasToolPerm is accepted for interface symmetry but not used.
func (s *Service) ResolveEnabledTools(
	ctx context.Context,
	userID string,
	hasToolSetPerm func(toolSetID string) bool,
	_ func(toolSetID, toolID string) bool, // unused — A2A agents are atomic
) []agent.ExecutableTool {
	connectors, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		s.log.Error(ctx, "a2a: failed to list connectors for tool resolution", "error", err)
		return nil
	}

	// Collect connected, permitted connector IDs.
	var connectedIDs []string
	connectorMap := make(map[string]A2AConnector)
	for _, c := range connectors {
		if c.Status != ConnectorStatusConnected {
			continue
		}
		toolSetID := "a2a-" + c.ID
		if hasToolSetPerm != nil && !hasToolSetPerm(toolSetID) {
			continue
		}
		connectedIDs = append(connectedIDs, c.ID)
		connectorMap[c.ID] = c
	}

	if len(connectedIDs) == 0 {
		return nil
	}

	// Batch fetch skills — used for the tool description so the LLM knows
	// the agent's capabilities, but they are NOT individual tools.
	allSkills, err := s.repo.ListSkillsByConnectors(ctx, connectedIDs)
	if err != nil {
		s.log.Error(ctx, "a2a: failed to batch list skills", "error", err)
		return nil
	}

	tools := make([]agent.ExecutableTool, 0, len(connectedIDs))
	for _, connID := range connectedIDs {
		c := connectorMap[connID]
		tools = append(tools, NewA2ARemoteTool(s, c, userID, allSkills[connID]))
	}

	return tools
}

// --------------------------------------------------------------------------
// Tool registry integration
// --------------------------------------------------------------------------

func (s *Service) registerConnectorInToolRegistry(connector *A2AConnector, skills []A2ADiscoveredSkill) {
	if s.toolRegistry == nil {
		return
	}

	ctx := context.Background()
	toolSetID := "a2a-" + connector.ID

	// A2A agents are atomic — ONE tool set with ONE tool per agent.
	name := connector.Title
	if connector.AgentName != "" {
		name = connector.AgentName
	}

	toolID := toolSetID // single tool has same ID as the set

	ts := &toolregistry.ToolSet{
		ID:          toolSetID,
		Name:        name,
		Description: "A2A Agent: " + connector.AgentURL,
		Category:    "a2a",
		ToolIDs:     []string{toolID},
		Metadata: map[string]string{
			"type":      "a2a",
			"agent_url": connector.AgentURL,
			"user_id":   connector.UserID,
		},
	}

	regTools := []*toolregistry.Tool{{
		ID:          toolID,
		Name:        name,
		Description: connector.AgentDescription,
		Category:    "a2a",
		ToolSetID:   toolSetID,
	}}

	if err := s.toolRegistry.RegisterToolSet(ctx, ts); err != nil {
		s.log.Error(ctx, "a2a: failed to register tool set", "error", err, "connector_id", connector.ID)
		return
	}
	if err := s.toolRegistry.RegisterTools(ctx, regTools); err != nil {
		s.log.Error(ctx, "a2a: failed to register tools", "error", err, "connector_id", connector.ID)
	}
}

func (s *Service) unregisterConnectorFromToolRegistry(connectorID string) {
	if s.toolRegistry == nil {
		return
	}
	ctx := context.Background()
	if err := s.toolRegistry.UnregisterToolSet(ctx, "a2a-"+connectorID); err != nil {
		s.log.Error(ctx, "a2a: failed to unregister tool set", "error", err, "connector_id", connectorID)
	}
}

// --------------------------------------------------------------------------
// Helpers
// --------------------------------------------------------------------------

func titleFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "A2A Agent"
	}
	return parsed.Hostname()
}

// validateExternalURL blocks requests to internal/private networks (SSRF protection).
func validateExternalURL(parsed *url.URL) error {
	if isLocalhostHost(parsed.Hostname()) {
		return fmt.Errorf("cannot connect to localhost addresses")
	}
	ips, err := net.LookupIP(parsed.Hostname())
	if err != nil {
		return nil // allow — DNS may not resolve yet
	}
	for _, ip := range ips {
		if blockErr := blockedIPError(ip); blockErr != nil {
			return blockErr
		}
	}
	return nil
}

// isLocalhostHost returns true for the set of hostnames that bypass DNS
// and always point at the loopback interface.
func isLocalhostHost(host string) bool {
	switch host {
	case "localhost", "127.0.0.1", "::1", "0.0.0.0":
		return true
	}
	return false
}

// blockedIPError returns a non-nil error when the IP belongs to a
// loopback/private/link-local range or to the AWS/GCP/Azure cloud
// metadata endpoint; otherwise nil.
func blockedIPError(ip net.IP) error {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return fmt.Errorf("cannot connect to private/internal network addresses")
	}
	if ip.String() == "169.254.169.254" {
		return fmt.Errorf("cannot connect to cloud metadata endpoint")
	}
	return nil
}

// rewriteForDocker replaces host-local addresses (0.0.0.0, 127.0.0.1, localhost)
// with host.docker.internal so that containers can reach the host machine.
func rewriteForDocker(rawURL string, parsed *url.URL) (string, *url.URL) {
	host := parsed.Hostname()
	switch host {
	case "0.0.0.0", "127.0.0.1", "::1", "localhost":
		parsed.Host = strings.Replace(parsed.Host, host, "host.docker.internal", 1)
		return parsed.String(), parsed
	}
	return rawURL, parsed
}
