package main

import (
	"context"
	"crypto/tls"
	"errors"
	"expvar"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"school-with-ai/adapters/adkgo"
	"school-with-ai/adapters/gemini"
	"school-with-ai/components"
	"school-with-ai/components/docs"
	"school-with-ai/components/landing"
	"school-with-ai/components/pricing"
	"school-with-ai/foundation/agent"
	"school-with-ai/foundation/auth"
	"school-with-ai/foundation/debug"
	"school-with-ai/foundation/event"
	es "school-with-ai/foundation/eventsourcing"
	"school-with-ai/foundation/i18n"
	"school-with-ai/foundation/logger"
	"school-with-ai/foundation/middleware"
	"school-with-ai/infra/githubauth"
	"school-with-ai/infra/googleauth"
	"school-with-ai/infra/projection/subscriber"
	"school-with-ai/infra/sqldb"
	"school-with-ai/infra/sqldb/migrate"
	chatmemory "school-with-ai/modules/chat/memory"
	"school-with-ai/modules/chat/thread"
	"school-with-ai/modules/knowledge/edgequake"
	"school-with-ai/platform/artifact"
	"school-with-ai/plugins/askuser"
	"school-with-ai/plugins/calculator"
	"school-with-ai/plugins/codebase"
	"school-with-ai/plugins/cognitive"
	"school-with-ai/plugins/document"
	"school-with-ai/plugins/knowledgebase"
	"school-with-ai/plugins/library"
	"school-with-ai/plugins/planmode"
	"school-with-ai/plugins/todos"
	"school-with-ai/plugins/tracker"
	"school-with-ai/plugins/tts"
	"school-with-ai/plugins/websearch"
	"school-with-ai/shared"

	"school-with-ai/modules/chat/taskmanagement"
	"school-with-ai/modules/knowledge/rag/users"
	permworkspace "school-with-ai/modules/permissions/workspace"
	"school-with-ai/platform/toolregistry"

	pgstore "school-with-ai/infra/eventstore/postgres"

	projectionStats "school-with-ai/modules/analytics/statistics"

	"school-with-ai/plugins/fsplugin"
	memoryplugin "school-with-ai/plugins/memory"

	healthhandlers "school-with-ai/foundation/health"
	a2amod "school-with-ai/modules/a2a"
	agentstoremod "school-with-ai/modules/agentstore"
	aisettingsmod "school-with-ai/modules/aisettings"
	analyticsmod "school-with-ai/modules/analytics"
	billingmod "school-with-ai/modules/billing"
	chatmod "school-with-ai/modules/chat"
	taskmgmthandlers "school-with-ai/modules/chat/taskmgmt"
	filesystemmod "school-with-ai/modules/filesystem"
	knowledgemod "school-with-ai/modules/knowledge"
	mcpmod "school-with-ai/modules/mcp"
	permissionsmod "school-with-ai/modules/permissions"
	pluginsettingsmod "school-with-ai/modules/pluginsettings"
	resourcesmod "school-with-ai/modules/resources"
	"school-with-ai/modules/toolreqs"
	userprofilemod "school-with-ai/modules/userprofile"

	"github.com/ardanlabs/conf/v3"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stripe/stripe-go/v84"
)

var build = "develop"

func main() {
	var log *logger.Logger

	events := logger.Events{
		Error: func(ctx context.Context, r logger.Record) {
			log.Info(ctx, "******* SEND ALERT *******")
		},
	}

	log = logger.NewTextWithEvents(os.Stdout, logger.LevelInfo, "APP", nil, events)

	// -------------------------------------------------------------------------

	ctx := context.Background()

	if err := run(ctx, log); err != nil {
		log.Error(ctx, "startup", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, log *logger.Logger) error {
	// -------------------------------------------------------------------------
	// GOMAXPROCS

	log.Info(ctx, "startup", "GOMAXPROCS", runtime.GOMAXPROCS(0))

	// -------------------------------------------------------------------------
	// Configuration

	cfg := struct {
		conf.Version
		Web struct {
			ReadTimeout       time.Duration `conf:"default:60s"`
			WriteTimeout      time.Duration `conf:"default:60s"`
			IdleTimeout       time.Duration `conf:"default:60s"`
			ReadHeaderTimeout time.Duration `conf:"default:10s"`
			ShutdownTimeout   time.Duration `conf:"default:20s"`
			APIHost           string        `conf:"default:localhost:8090"`
			DebugHost         string        `conf:"default:0.0.0.0:4010"`
			TLSCert           string        `conf:"default:./tls/cert.pem"`
			TLSKey            string        `conf:"default:./tls/key.pem"`
		}
		Auth struct {
			AppURL             string `conf:"default:https://localhost:8090"`
			GoogleClientID     string
			GoogleClientSecret string
			GitHubClientID     string
			GitHubClientSecret string
			SessionSecret      string `conf:"default:change-me-in-production-32-chars!"`
		}
		DashboardAdmin struct {
			User string `conf:"default:admin"`
			Pass string `conf:"default:admin,mask"`
		}
		DB struct {
			User         string `conf:"default:appuser"`
			Password     string `conf:"default:dev-password,mask"`
			Host         string `conf:"default:localhost:5435"`
			Name         string `conf:"default:projections"`
			MaxIdleConns int    `conf:"default:100"`
			MaxOpenConns int    `conf:"default:100"`
			DisableTLS   bool   `conf:"default:true"`
		}
		Redis struct {
			Addr     string `conf:"default:localhost:6379"`
			Password string `conf:"mask"`
		}
		NATS struct {
			URL      string `conf:"default:nats://localhost:4222"`
			Password string `conf:"mask"`
			Replicas int    `conf:"default:1"`
		}
		Orchestration struct {
			Concurrency         int           `conf:"default:5"`
			ShutdownTimeout     time.Duration `conf:"default:10s"`
			HealthCheckInterval time.Duration `conf:"default:15s"`
		}

		Artifacts struct {
			S3Endpoint  string `conf:"default:localhost:9100"`
			S3AccessKey string `conf:"default:minio"`
			S3SecretKey string `conf:"default:miniosecret"`
			S3Bucket    string `conf:"default:artifacts"`
			S3UseSSL    bool   `conf:"default:false"`
			S3Region    string `conf:"default:us-east-1"`
		}
		Edgequake struct {
			BaseURL  string `conf:"default:http://localhost:8081"`
			TenantID string
		}
		Stripe struct {
			SecretKey      string `conf:"mask"`
			PublishableKey string
			WebhookSecret  string `conf:"mask"`
		}
	}{
		Version: conf.Version{
			Build: build,
			Desc:  "AI Chat Platform",
		},
	}

	const prefix = "APP"
	help, err := conf.Parse(prefix, &cfg)
	if err != nil {
		if errors.Is(err, conf.ErrHelpWanted) {
			fmt.Println(help)
			return nil
		}
		return fmt.Errorf("parsing config: %w", err)
	}

	log.Info(ctx, "startup", "config_tenant_id", cfg.Edgequake.TenantID)

	// -------------------------------------------------------------------------
	// Production Configuration Guard
	//
	// Reject known development defaults when the binary is built for
	// production (build != "develop").

	if err := validateProductionConfig(log, cfg.Build, cfg.DashboardAdmin.User, cfg.DashboardAdmin.Pass, cfg.DB.Password, cfg.Artifacts.S3AccessKey, cfg.Artifacts.S3SecretKey, cfg.Stripe.SecretKey, cfg.Stripe.WebhookSecret); err != nil {
		return err
	}

	// -------------------------------------------------------------------------
	// Initialize I18n

	if err := i18n.LoadCatalogs("assets/locales"); err != nil {
		log.Error(ctx, "failed to load i18n catalogs", "error", err)
		// Don't fail matching, just continue with defaults
	}

	// -------------------------------------------------------------------------
	// App Starting

	log.Info(ctx, "starting service", "version", cfg.Build)
	defer log.Info(ctx, "shutdown complete")

	out, err := conf.String(&cfg)
	if err != nil {
		return fmt.Errorf("generating config for output: %w", err)
	}
	log.Info(ctx, "startup", "config", out)

	expvar.NewString("build").Set(cfg.Build)

	// -------------------------------------------------------------------------
	// Initialize Debug Mux

	debugMux := debug.Mux()

	// -------------------------------------------------------------------------
	// Database Support (Postgres — single database for events + projections)

	log.Info(ctx, "startup", "status", "initializing database support", "host", cfg.DB.Host)

	db, err := sqldb.Open(sqldb.Config{
		User:         cfg.DB.User,
		Password:     cfg.DB.Password,
		Host:         cfg.DB.Host,
		Name:         cfg.DB.Name,
		MaxIdleConns: cfg.DB.MaxIdleConns,
		MaxOpenConns: cfg.DB.MaxOpenConns,
		DisableTLS:   cfg.DB.DisableTLS,
	})
	if err != nil {
		return fmt.Errorf("connecting to db: %w", err)
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			log.Error(ctx, "shutdown", "status", "failed to close database", "ERROR", closeErr)
		}
	}()

	// Initialize Event Deserializer Registry
	// Each module registers its events so the event store and projection
	// subscriber can deserialize them from JSON without a central switch.
	eventRegistry := es.NewDeserializerRegistry()
	thread.RegisterThreadEvents(eventRegistry)
	billingmod.RegisterEvents(eventRegistry)

	// Initialize PostgreSQL Event Store
	log.Info(ctx, "startup", "status", "initializing PostgreSQL event store")
	baseEventStore := pgstore.NewPostgresEventStore(db, eventRegistry)

	// Initialize Snapshot Store (accelerates thread loading for long conversations)
	snapshotStore := pgstore.NewSnapshotStore(db)

	// Initialize User Profile module (identity + preferences + banner)
	userProfileModule := userprofilemod.New(log, db)
	userProfileRepo := userProfileModule.UserProfileRepo()

	// Initialize Agent Store module (agent definitions + catalog + composition + sharing)
	agentStoreModule := agentstoremod.New(log, db)

	// Run migrations (skipped when APP_SKIP_MIGRATIONS=true, e.g. in a
	// cluster where an init container handles migrations before pods start).
	if os.Getenv("APP_SKIP_MIGRATIONS") != "true" {
		log.Info(ctx, "startup", "status", "running database migrations")
		if err := migrate.Migrate(ctx, db); err != nil {
			return fmt.Errorf("migrating database: %w", err)
		}
	} else {
		log.Info(ctx, "startup", "status", "skipping migrations (APP_SKIP_MIGRATIONS=true)")
	}

	// Seed agent store catalog (only inserts if table is empty)
	log.Info(ctx, "startup", "status", "seeding agent store catalog")
	if err := agentStoreModule.SeedCatalog(ctx); err != nil {
		return fmt.Errorf("seeding agent catalog: %w", err)
	}

	// -------------------------------------------------------------------------
	// Initialize Projections
	// persistenceRouter is used by the ProjectionUpdater exclusively to update Read Models (Projections).
	persistenceRouter := event.NewRouter()

	// transientRouter is used by the orchestrator for non-persistent events (streaming deltas, etc.)
	transientRouter := event.NewRouter()

	// Initialize NATS JetStream broadcaster for cross-pod event fan-out and replay
	natsCfg := subscriber.DefaultNATSBroadcasterConfig()
	natsCfg.StreamReplicas = cfg.NATS.Replicas
	broadcaster, err := subscriber.NewNATSBroadcaster(log, cfg.NATS.URL, cfg.NATS.Password, eventRegistry, natsCfg)
	if err != nil {
		return fmt.Errorf("NATS broadcaster: %w", err)
	}
	defer func() {
		if closeErr := broadcaster.Close(); closeErr != nil {
			log.Error(ctx, "shutdown", "status", "failed to close NATS broadcaster", "ERROR", closeErr)
		}
	}()

	// Initialize Redis client for distributed rate limiting (shared across pods)
	rateLimitRedis := goredis.NewClient(&goredis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password})
	defer func() {
		if closeErr := rateLimitRedis.Close(); closeErr != nil {
			log.Error(ctx, "shutdown", "status", "failed to close rate-limit Redis client", "ERROR", closeErr)
		}
	}()
	log.Info(ctx, "startup", "status", "rate-limit Redis client initialized", "addr", cfg.Redis.Addr)

	// Register Redis broadcaster ONLY to transientRouter.
	// The orchestrator directly dispatches all events (Durable and Transient) to transientRouter.
	// persistenceRouter is strictly for updating Read Models (Projections) and should not re-broadcast.
	transientRouter.RegisterAny(broadcaster.Broadcast)

	// Use the Postgres event store directly as the write model
	eventLog := baseEventStore

	// -------------------------------------------------------------------------
	// Start Projection Updater (polls Postgres event store for new events)

	log.Info(ctx, "startup", "status", "starting projection updater")

	// Create projection updater with the persistence router
	projectionUpdater := subscriber.NewProjectionUpdater(
		log,
		baseEventStore,
		db,
		persistenceRouter,
	)

	// Initialize Statistics Projection (Postgres)
	statsProjection := projectionStats.NewPostgresProjection(db)
	statsProjection.RegisterHandlers(persistenceRouter)

	// Initialize Billing Module (event-sourced payment system)
	billingModule := billingmod.New(log, db, baseEventStore, cfg.Stripe.WebhookSecret, cfg.Stripe.PublishableKey)
	billingModule.RegisterProjection(persistenceRouter)

	// Projection updater start is deferred until all module projections are registered.
	projCtx, projCancel := context.WithCancel(ctx)
	defer projCancel()

	// Initialize per-user AI settings (modules/aisettings)
	aiSettingsModule := aisettingsmod.New(log, db)
	log.Info(ctx, "startup", "status", "ai settings module initialized (per-user)")

	// Initialize per-user plugin settings (modules/pluginsettings)
	pluginSettingsModule := pluginsettingsmod.New(log, db)
	log.Info(ctx, "startup", "status", "plugin settings module initialized (per-user per-plugin)")

	// Initialize artifact service
	log.Info(ctx, "startup", "status", "initializing s3 artifact service", "endpoint", cfg.Artifacts.S3Endpoint, "bucket", cfg.Artifacts.S3Bucket)
	artifactService, err := artifact.NewS3Service(ctx, artifact.S3Config{
		Endpoint:        cfg.Artifacts.S3Endpoint,
		AccessKeyID:     cfg.Artifacts.S3AccessKey,
		SecretAccessKey: cfg.Artifacts.S3SecretKey,
		UseSSL:          cfg.Artifacts.S3UseSSL,
		BucketName:      cfg.Artifacts.S3Bucket,
		Region:          cfg.Artifacts.S3Region,
	})
	if err != nil {
		return fmt.Errorf("initializing s3 artifact service: %w", err)
	}

	// Initialize resources module
	resourcesModule := resourcesmod.New(db, artifactService)

	// Initialize filesystem module (VFS backed by S3/MinIO)
	var ghClient *githubauth.Client
	var ghStore *githubauth.Store
	if cfg.Auth.GitHubClientID != "" && cfg.Auth.GitHubClientSecret != "" {
		ghClient = githubauth.NewClient(githubauth.Config{
			ClientID:     cfg.Auth.GitHubClientID,
			ClientSecret: cfg.Auth.GitHubClientSecret,
			RedirectURL:  cfg.Auth.AppURL + "/api/github/callback",
		})
		ghStore = githubauth.NewStore(db)
		log.Info(ctx, "startup", "status", "GitHub OAuth configured for filesystem module")
	}

	filesystemModule, err := filesystemmod.New(db, filesystemmod.S3Config{
		Endpoint:        cfg.Artifacts.S3Endpoint,
		AccessKeyID:     cfg.Artifacts.S3AccessKey,
		SecretAccessKey: cfg.Artifacts.S3SecretKey,
		UseSSL:          cfg.Artifacts.S3UseSSL,
		BucketName:      cfg.Artifacts.S3Bucket,
		Region:          cfg.Artifacts.S3Region,
	}, ghClient, ghStore, cfg.Auth.AppURL)
	if err != nil {
		return fmt.Errorf("initializing filesystem module: %w", err)
	}
	log.Info(ctx, "startup", "status", "filesystem module initialized")

	// Create and initialize tool registry
	toolRegistry := toolregistry.NewRegistry(log, toolregistry.NewPostgresToolRegistryRepository(db))
	toolregistry.RegisterPlugin(&calculator.Provider{})
	toolregistry.RegisterPlugin(&codebase.Provider{})
	toolregistry.RegisterPlugin(&cognitive.Provider{})
	toolregistry.RegisterPlugin(&document.Provider{})
	toolregistry.RegisterPlugin(&askuser.Provider{})
	toolregistry.RegisterPlugin(&knowledgebase.Provider{})
	toolregistry.RegisterPlugin(&library.Provider{})
	toolregistry.RegisterPlugin(&memoryplugin.Provider{})
	toolregistry.RegisterPlugin(&planmode.Provider{})
	toolregistry.RegisterPlugin(&todos.Provider{})
	toolregistry.RegisterPlugin(&tracker.Provider{})
	toolregistry.RegisterPlugin(&tts.Provider{})
	toolregistry.RegisterPlugin(websearch.NewProvider(func(apiKey, model string) websearch.GroundedSearchClient {
		return gemini.NewSearchClient(apiKey, model)
	}))
	toolregistry.RegisterPlugin(&fsplugin.Provider{FS: &agentFSAdapter{source: filesystemModule.AgentFS()}})
	if err := toolRegistry.Initialize(ctx); err != nil {
		return fmt.Errorf("failed to initialize tool registry: %w", err)
	}
	log.Info(ctx, "startup", "status", "tool registry initialized")

	// Create MCP module
	mcpModule := mcpmod.New(log, db, cfg.Auth.AppURL+"/v1/mcp/oauth/callback", toolRegistry)
	log.Info(ctx, "startup", "status", "mcp module initialized")

	// Create A2A module
	a2aModule := a2amod.New(log, db, toolRegistry)
	defer a2aModule.Shutdown()
	log.Info(ctx, "startup", "status", "a2a module initialized")

	// Initialize Redis client for distributed permission caching (shared across pods)
	permCacheRedis := goredis.NewClient(&goredis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password})
	defer func() {
		if closeErr := permCacheRedis.Close(); closeErr != nil {
			log.Error(ctx, "shutdown", "status", "failed to close perm-cache Redis client", "ERROR", closeErr)
		}
	}()
	log.Info(ctx, "startup", "status", "perm-cache Redis client initialized", "addr", cfg.Redis.Addr)

	// Create and initialize permissions service
	permissionsRepo := toolreqs.NewPostgresRepository(db)
	log.Info(ctx, "startup", "status", "permissions repository initialized")

	// Create workspace permissions service
	workspacePermissionsService := permworkspace.NewService(log, permworkspace.NewPostgresWorkspacePermissionsRepository(db), permCacheRedis)
	log.Info(ctx, "startup", "status", "workspace permissions service initialized")

	// ── Tool Requirements Module ──
	// Single authority for all tool access decisions: permissions, confirmation, etc.
	toolReqsModule := toolreqs.New(log, permissionsRepo, permCacheRedis)
	toolReqsService := toolReqsModule.Service()
	log.Info(ctx, "startup", "status", "tool requirements module initialized")

	// Initialize Knowledge module (owns Edgequake / RAG services)
	embeddingResolver := knowledgemod.EmbeddingConfigResolverFunc(func(ctx context.Context, userID string) (*knowledgemod.EmbeddingConfig, error) {
		ec, err := aiSettingsModule.GetEmbeddingConfigForUser(ctx, userID)
		if err != nil || ec == nil {
			return nil, err
		}
		return &knowledgemod.EmbeddingConfig{
			Provider:  ec.Provider,
			Model:     ec.Model,
			Dimension: ec.Dimension,
			APIKey:    ec.APIKey,
		}, nil
	})
	knowledgeModule := knowledgemod.New(log, cfg.Edgequake.BaseURL, embeddingResolver, cfg.Edgequake.TenantID)
	edgequakeService := knowledgeModule.Service()
	log.Info(ctx, "startup", "status", "knowledge module initialized", "baseURL", cfg.Edgequake.BaseURL)

	// Initialize Agents
	slogLogger := slog.New(log)

	// All agents (including the default assistant) live in the database.
	// In the unified model, both simple and composed agents are AgentDefinitions.
	agentProvider := newUserAwareAgentProvider(
		agentStoreModule.AgentDefinitionRepo(),
		[]agent.Middleware{agent.ToolLogging(slogLogger)},
	)

	// -------------------------------------------------------------------------
	// Initialize Chat Module (core chat bounded context — ADR 014)

	authSvc := &contextAuthService{}
	chatModule := chatmod.New(chatmod.Dependencies{
		Log:                      log,
		DB:                       db,
		EventStore:               eventLog,
		SnapshotStore:            snapshotStore,
		ArtifactService:          artifactService,
		UserProfileRepo:          &chatProfileAdapter{repo: userProfileRepo},
		Emitter:                  transientRouter,
		AgentProvider:            agentProvider,
		AuthService:              authSvc,
		ToolRegistry:             toolRegistry,
		RequirementConfigService: toolReqsService,
		DocumentProvider:         &DocumentProviderAdapter{svc: edgequakeService},
		TenantID:                 cfg.Edgequake.TenantID,
		AllowedOrigin:            cfg.Auth.AppURL,
		SSE:                      taskmgmthandlers.DefaultSSEConfig(),
		Broadcaster:              broadcaster,
		AsynqConfig: taskmanagement.AsynqConfig{
			RedisAddr:           cfg.Redis.Addr,
			RedisPassword:       cfg.Redis.Password,
			Concurrency:         cfg.Orchestration.Concurrency,
			ShutdownTimeout:     cfg.Orchestration.ShutdownTimeout,
			HealthCheckInterval: cfg.Orchestration.HealthCheckInterval,
		},
	})

	// Register chat projections (AGUI + LLM context) before starting the updater.
	chatModule.RegisterProjection(persistenceRouter)

	// Register per-projection checkpoint names. Each projection gets its own
	// checkpoint row so a single projection can be rebuilt independently.
	projectionUpdater.RegisterProjectionName("stats")
	projectionUpdater.RegisterProjectionName("billing")
	projectionUpdater.RegisterProjectionName("agui")
	projectionUpdater.RegisterProjectionName("llm-context")

	// Start projection updater in background (all projections now registered).
	go func() {
		if err := projectionUpdater.Start(projCtx); err != nil && projCtx.Err() == nil {
			log.Error(ctx, "projection updater stopped", "ERROR", err)
		}
	}()
	log.Info(ctx, "startup", "status", "projection updater started")

	// Create agent runner and complete chat module initialisation (two-phase construction).
	// The runner needs the orchestrator (from the chat module), and the orchestrator
	// needs the runner — this circular dependency is resolved via SetRunner.

	// Build the tool set factory builder — closes over the tool registry so the adapter
	// never imports service packages (dependency inversion via closure).
	newToolSetFactory := adkgo.NewToolSetFactoryFunc(func(cfg adkgo.ToolFactoryConfig) adkgo.ToolSetFactory {
		f := toolregistry.NewExecutableToolFactory(artifactService)
		f.SetToolRegistry(toolRegistry)
		f.SetMemoryFactory(func(store artifact.Service) toolregistry.MemoryService {
			return chatmemory.NewArtifactMemoryService(store)
		})
		// Read tool-specific config from the generic Extra map.
		if url := cfg.Extra["edgequake_base_url"]; url != "" {
			f.SetEdgequakeConfig(url, cfg.ModelAPIKey, cfg.ModelID)
		}
		if apiKey := cfg.Extra["tts_api_key"]; apiKey != "" {
			f.SetTTSConfig(apiKey, cfg.Extra["tts_model"], cfg.Extra["tts_voice"])
		}
		if cfg.PluginSettings != nil {
			f.SetPluginSettings(cfg.PluginSettings)
		}
		if ns := cfg.Extra["allowed_kb_namespaces"]; ns != "" {
			f.SetAllowedKBNamespaces(strings.Split(ns, ","))
		}
		return f
	})

	// Resolve per-user, per-agent tool config at request time (TTS, service URLs, KB namespaces, etc.)
	// so the adapter layer stays tool-agnostic.
	extraConfigFn := adkgo.ExtraConfigFunc(func(ctx context.Context, userID, agentName string) map[string]string {
		extra := map[string]string{
			"edgequake_base_url": cfg.Edgequake.BaseURL,
		}
		if apiKey, model, voice, err := aiSettingsModule.GetTTSConfigForUserFlat(ctx, userID); err == nil && apiKey != "" {
			extra["tts_api_key"] = apiKey
			extra["tts_model"] = model
			extra["tts_voice"] = voice
		}
		// Resolve per-agent knowledge-base workspace permissions.
		if agentName != "" && edgequakeService != nil {
			if wsIDs, err := workspacePermissionsService.GetEnabledWorkspaces(ctx, userID, agentName); err == nil && len(wsIDs) > 0 {
				extra["allowed_kb_namespaces"] = strings.Join(wsIDs, ",")
			}
		}
		return extra
	})

	kbPromptInjector := &knowledgeBasePromptInjector{workspaceSvc: workspacePermissionsService}

	adkRunner, err := adkgo.NewRunnerAdapter(adkgo.RunnerAdapterConfig{
		Log:                    log,
		ThreadMgr:              chatModule.Orchestrator().GetThreadManagement(),
		AgentProvider:          agentProvider,
		ConfigResolver:         aiSettingsModule,
		PluginSettingsResolver: pluginSettingsModule,
		ProfileProvider:        &adkgoProfileAdapter{repo: userProfileRepo},
		ToolReqs:               toolReqsService,
		BlockEnforcer:          toolReqsService,
		ConfirmEnforcer:        toolReqsService,
		MCPResolver:            mcpModule.Service(),
		A2AResolver:            a2aModule.Service(),
		NewToolSetFactory:      newToolSetFactory,
		PromptInjectors:        []adkgo.PromptInjector{kbPromptInjector},
		ExtraConfigFn:          extraConfigFn,
	})
	if err != nil {
		return fmt.Errorf("creating adk-go runner: %w", err)
	}
	chatModule.SetRunner(adkRunner)
	defer chatModule.Shutdown()

	log.Info(ctx, "startup", "status", "chat module initialized",
		"redis", cfg.Redis.Addr,
		"concurrency", cfg.Orchestration.Concurrency)

	// -------------------------------------------------------------------------
	// Start Debug Service

	debugServer := &http.Server{
		Addr:              cfg.Web.DebugHost,
		Handler:           debugMux,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info(ctx, "startup", "status", "debug server started", "host", cfg.Web.DebugHost)

		if err := debugServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error(ctx, "shutdown", "status", "debug server closed", "host", cfg.Web.DebugHost, "msg", err)
		}
	}()

	// Create health handler with dependency checkers
	healthHandler := healthhandlers.NewHealthHandler(log,
		healthhandlers.WithChecker("db", healthhandlers.CheckerFunc(func(ctx context.Context) error {
			return db.PingContext(ctx)
		})),
		healthhandlers.WithChecker("s3", healthhandlers.CheckerFunc(func(ctx context.Context) error {
			return artifactService.HealthCheck(ctx)
		})),
	)

	// -------------------------------------------------------------------------
	// Initialize Router & Routes

	root, appRouter := buildRootRouter(log, rateLimitRedis)

	// Initialize Google OAuth client and auth handlers
	googleClient := googleauth.NewClient(googleauth.Config{
		ClientID:     cfg.Auth.GoogleClientID,
		ClientSecret: cfg.Auth.GoogleClientSecret,
		RedirectURL:  cfg.Auth.AppURL + "/auth/callback",
		SessionKey:   cfg.Auth.SessionSecret,
	}, db)
	sessionStore := googleauth.NewSessionStore(cfg.Auth.SessionSecret)
	authHandlers := googleauth.NewAuthHandlers(log, googleClient, sessionStore, cfg.Auth.AppURL)
	authMiddleware := auth.NewMiddleware(googleClient, sessionStore, cfg.Auth.AppURL, ctx)

	registerRoutes(appRouter, log, authHandlers, authMiddleware, healthHandler, routeModules{
		chat:           chatModule,
		userProfile:    userProfileModule,
		aiSettings:     aiSettingsModule,
		pluginSettings: pluginSettingsModule,
		agentStore:     agentStoreModule,
		knowledge:      knowledgeModule,
		mcp:            mcpModule,
		a2a:            a2aModule,
		resources:      resourcesModule,
		filesystem:     filesystemModule,
		billing:        billingModule,
		permissions:    permissionsmod.New(log, workspacePermissionsService, &WorkspaceProviderAdapter{svc: edgequakeService}, agentProvider, cfg.Edgequake.TenantID),
		analytics:      analyticsmod.New(log, statsProjection, cfg.DashboardAdmin.User, cfg.DashboardAdmin.Pass),
	}, edgequakeService)

	// Set Stripe API key — must be done before any Stripe SDK calls
	stripe.Key = cfg.Stripe.SecretKey

	// Configure TLS and create server with proper timeouts
	server := &http.Server{
		Addr:              cfg.Web.APIHost,
		Handler:           root,
		ReadTimeout:       cfg.Web.ReadTimeout,
		ReadHeaderTimeout: cfg.Web.ReadHeaderTimeout,
		WriteTimeout:      cfg.Web.WriteTimeout,
		IdleTimeout:       cfg.Web.IdleTimeout,
		TLSConfig: &tls.Config{
			CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		},
	}

	// -------------------------------------------------------------------------
	// Start API Service

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	serverErrors := make(chan error, 1)

	go func() {
		log.Info(ctx, "startup", "status", "api router started (HTTPS)", "host", server.Addr)

		serverErrors <- server.ListenAndServeTLS(cfg.Web.TLSCert, cfg.Web.TLSKey)
	}()

	log.Info(ctx, "startup", "msg", "Press CTRL+C or send SIGTERM to shutdown")

	// -------------------------------------------------------------------------
	// Shutdown

	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		log.Info(ctx, "shutdown", "status", "shutdown started", "signal", sig)
		defer log.Info(ctx, "shutdown", "status", "shutdown complete", "signal", sig)

		// Cancel the projection updater goroutine
		projCancel()

		ctx, cancel := context.WithTimeout(ctx, cfg.Web.ShutdownTimeout)
		defer cancel()

		// Shut down the debug server gracefully
		if err := debugServer.Shutdown(ctx); err != nil {
			log.Error(ctx, "shutdown", "status", "debug server shutdown error", "msg", err)
		}

		if err := server.Shutdown(ctx); err != nil {
			if closeErr := server.Close(); closeErr != nil {
				return fmt.Errorf("could not stop server gracefully (shutdown: %w, close: %v)", err, closeErr)
			}
			return fmt.Errorf("could not stop server gracefully: %w", err)
		}
	}

	return nil
}

// ─── Router Setup Helpers ───────────────────────────────────────────────────

// buildRootRouter creates the root chi router with global middleware and
// an app sub-router with per-IP rate limiting. Returns (root, appRouter).
func buildRootRouter(log *logger.Logger, rateLimitRedis *goredis.Client) (chi.Router, chi.Router) {
	root := chi.NewRouter()
	root.Use(middleware.Panics())
	root.Use(middleware.SecurityHeaders())
	root.Use(middleware.BodyLimit(0, "/v1/message:send"))
	root.Use(middleware.Logger(log))
	root.Use(middleware.Metrics())
	root.Use(middleware.I18n)

	// Static assets get a higher rate limit to support parallel loading.
	root.Group(func(r chi.Router) {
		r.Use(middleware.RateLimiter(rateLimitRedis, 100, 300))
		r.Mount("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
	})

	appRouter := chi.NewRouter()
	appRouter.Use(middleware.RateLimiter(rateLimitRedis, 20, 60))
	root.Mount("/", appRouter)

	return root, appRouter
}

// routeModules bundles all modules that register routes so they can be
// passed as a single argument to registerRoutes.
type routeModules struct {
	chat interface {
		RegisterRoutes(chi.Router, *auth.Middleware)
	}
	userProfile interface {
		RegisterRoutes(chi.Router, *auth.Middleware)
	}
	aiSettings interface {
		RegisterRoutes(chi.Router, *auth.Middleware)
	}
	pluginSettings interface {
		RegisterRoutes(chi.Router, *auth.Middleware)
	}
	agentStore interface {
		RegisterRoutes(chi.Router, *auth.Middleware)
	}
	knowledge interface {
		RegisterRoutes(chi.Router, *auth.Middleware)
	}
	mcp interface {
		RegisterRoutes(chi.Router, *auth.Middleware)
	}
	a2a interface {
		RegisterRoutes(chi.Router, *auth.Middleware)
	}
	resources interface {
		RegisterRoutes(chi.Router, *auth.Middleware)
	}
	filesystem interface {
		RegisterRoutes(chi.Router, *auth.Middleware)
	}
	billing interface {
		RegisterRoutes(chi.Router, *auth.Middleware)
	}
	permissions interface {
		RegisterRoutes(chi.Router, *auth.Middleware)
	}
	analytics interface {
		RegisterRoutes(interface {
			Get(string, http.HandlerFunc)
		})
	}
}

// registerRoutes wires all module and page routes onto the app router.
func registerRoutes(
	r chi.Router,
	log *logger.Logger,
	authHandlers *googleauth.AuthHandlers,
	authMW *auth.Middleware,
	healthHandler *healthhandlers.HealthHandler,
	mods routeModules,
	edgequakeService *edgequake.Service,
) {
	// Module routes
	mods.resources.RegisterRoutes(r, authMW)
	mods.filesystem.RegisterRoutes(r, authMW)
	mods.permissions.RegisterRoutes(r, authMW)
	mods.agentStore.RegisterRoutes(r, authMW)
	mods.userProfile.RegisterRoutes(r, authMW)
	mods.aiSettings.RegisterRoutes(r, authMW)
	mods.pluginSettings.RegisterRoutes(r, authMW)
	mods.billing.RegisterRoutes(r, authMW)
	mods.chat.RegisterRoutes(r, authMW)
	mods.mcp.RegisterRoutes(r, authMW)
	mods.a2a.RegisterRoutes(r, authMW)
	mods.knowledge.RegisterRoutes(r, authMW)
	mods.analytics.RegisterRoutes(r)

	// Health check endpoints (public)
	r.Get("/liveness", healthHandler.Liveness)
	r.Get("/readiness", healthHandler.Readiness)

	// Auth routes (public)
	r.Get("/auth/login", authHandlers.LoginPage)
	r.Get("/auth/callback", authHandlers.CallbackHandler)
	r.Get("/auth/logout", authHandlers.LogoutHandler)
	r.Get("/auth/error", authHandlers.ErrorPage)
	r.Get("/api/me", authMW.RequireAuthFunc(authHandlers.MeHandler))

	// Page routes
	r.Get("/", authMW.OptionalAuthFunc(HandleLandingPage(log)))
	r.Get("/docs", authMW.OptionalAuthFunc(HandleDocsPage(log)))
	r.Get("/pricing", authMW.RequireAuthFunc(HandlePricingPage(log)))
	r.Get("/chat", authMW.RequireAuthFunc(HandleViewChatInputScreen(log, edgequakeService)))
}

// ─── Page Handlers ──────────────────────────────────────────────────────────

func HandleViewChatInputScreen(log *logger.Logger, edgequakeService *edgequake.Service) http.HandlerFunc {
	// Track which users have already been synced to avoid redundant goroutines.
	var syncedUsers sync.Map

	return func(w http.ResponseWriter, r *http.Request) {
		// Get authenticated user from context
		user := shared.GetUserFromContext(r.Context())

		// Sync user to RAG at most once per process lifetime (idempotent create).
		if user != nil {
			if _, alreadySynced := syncedUsers.LoadOrStore(user.ID, struct{}{}); !alreadySynced {
				go func(u *shared.User) {
					// Generate a random password since RAG requires one, but we don't use it for auth
					randomPass := uuid.New().String()

					// Create user request
					req := &users.CreateUserRequest{
						Username: u.GetDisplayName(),
						Email:    u.Email,
						Password: randomPass,
						Role:     &u.Role,
					}

					// Use a bounded context so this goroutine cannot block forever
					syncCtx, syncCancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer syncCancel()

					_, err := edgequakeService.Users.Create(syncCtx, req)
					if err != nil {
						// Remove from map so next page load retries the sync
						syncedUsers.Delete(u.ID)
						log.Error(syncCtx, "failed to sync user to RAG", "error", err, "email", u.Email)
					} else {
						log.Info(syncCtx, "synced user to RAG", "email", u.Email, "role", u.Role)
					}
				}(user)
			}
		}

		component := components.ChatInputScreen(user)
		if err := component.Render(r.Context(), w); err != nil {
			log.Error(r.Context(), "failed to render chat input screen", "error", err)
		}
	}
}

func HandlePricingPage(log *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := shared.GetUserFromContext(r.Context())

		component := pricing.PricingPage(pricing.PricingPageProps{
			User: user,
		})
		if err := component.Render(r.Context(), w); err != nil {
			log.Error(r.Context(), "failed to render pricing page", "error", err)
		}
	}
}

func HandleLandingPage(log *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := shared.GetUserFromContext(r.Context())

		component := landing.LandingPage(landing.LandingPageProps{
			User: user,
		})
		if err := component.Render(r.Context(), w); err != nil {
			log.Error(r.Context(), "failed to render landing page", "error", err)
		}
	}
}

func HandleDocsPage(log *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := shared.GetUserFromContext(r.Context())

		component := docs.DocsPage(docs.DocsPageProps{
			User: user,
		})
		if err := component.Render(r.Context(), w); err != nil {
			log.Error(r.Context(), "failed to render docs page", "error", err)
		}
	}
}

// validateProductionConfig rejects known insecure defaults when the
// binary is built for production (build != "develop"). This prevents
// accidental deployment with development credentials.
func validateProductionConfig(log *logger.Logger, buildTag, adminUser, adminPass, dbPass, s3Key, s3Secret, stripeSecret, stripeWebhook string) error {
	if buildTag == "develop" {
		return nil
	}

	insecure := map[string]string{
		"dashboard admin user":     adminUser,
		"dashboard admin password": adminPass,
		"database password":        dbPass,
		"S3 access key":            s3Key,
		"S3 secret key":            s3Secret,
	}

	defaults := map[string]bool{
		"admin":        true,
		"dev-password": true,
		"minio":        true,
		"miniosecret":  true,
	}

	for label, val := range insecure {
		if defaults[val] {
			return fmt.Errorf("production config guard: %s is set to the development default %q — set a secure value", label, val)
		}
	}

	// Stripe keys: secret key MUST be set in production.
	// Webhook secret is strongly recommended — without it, payment confirmation
	// relies solely on lazy-verification (client-side redirect).
	if stripeSecret == "" {
		return fmt.Errorf("production config guard: Stripe secret key is empty — payment features require this value")
	}
	if stripeWebhook == "" {
		log.Info(context.Background(), "production config guard: WARNING — Stripe webhook secret is empty. Payment confirmations will rely on lazy-verification only. Set APP_STRIPE_WEBHOOK_SECRET for reliable webhook processing.")
	}

	return nil
}

// contextAuthService implements shared.AuthService by reading the
// authenticated user directly from the request context. This replaces
// the former services/auth package (a trivial delegation wrapper).
type contextAuthService struct{}

func (*contextAuthService) GetUserFromContext(ctx context.Context) *shared.User {
	return shared.GetUserFromContext(ctx)
}

func (s *contextAuthService) IsAuthenticated(ctx context.Context) bool {
	return s.GetUserFromContext(ctx) != nil
}
