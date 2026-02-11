module school-with-ai

go 1.24.4

toolchain go1.24.11

tool github.com/a-h/templ/cmd/templ

replace github.com/lightrag/lightrag-go => ./foundation/lightrag-go

require (
	github.com/Oudwins/tailwind-merge-go v0.2.1
	github.com/a-h/templ v0.3.960
	github.com/ag-ui-protocol/ag-ui/sdks/community/go v0.0.0-20251230070606-5ae00423dc91
	github.com/ardanlabs/conf/v3 v3.10.0
	github.com/ardanlabs/darwin/v3 v3.3.1
	github.com/arl/statsviz v0.8.0
	github.com/avast/retry-go/v4 v4.7.0
	github.com/go-chi/chi/v5 v5.2.3
	github.com/google/uuid v1.6.0
	github.com/hibiken/asynq v0.25.1
	github.com/jackc/pgx/v5 v5.8.0
	github.com/jmoiron/sqlx v1.4.0
	github.com/kcmvp/archunit v0.2.0-alpha1
	github.com/kurrent-io/KurrentDB-Client-Go v1.1.1
	github.com/lib/pq v1.10.9
	github.com/lightrag/lightrag-go v0.0.0-00010101000000-000000000000
	github.com/minio/minio-go/v7 v7.0.98
	github.com/montanaflynn/stats v0.7.1
	github.com/ollama/ollama v0.13.5
	github.com/openai/openai-go/v3 v3.15.0
	github.com/ory/client-go v1.22.16
	github.com/pgvector/pgvector-go v0.3.0
	github.com/qmuntal/stateless v1.7.2
	github.com/redis/go-redis/v9 v9.7.0
	github.com/rs/zerolog v1.34.0
	github.com/sashabaranov/go-openai v1.41.2
	github.com/stretchr/testify v1.11.1
	github.com/typesense/typesense-go/v2 v2.0.0
	go.opentelemetry.io/otel v1.39.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.39.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.39.0
	go.opentelemetry.io/otel/sdk v1.39.0
	go.opentelemetry.io/otel/trace v1.39.0
	golang.org/x/sync v0.19.0
	golang.org/x/text v0.32.0
	google.golang.org/genai v1.43.0
)

require (
	cloud.google.com/go v0.116.0 // indirect
	cloud.google.com/go/auth v0.9.3 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/a-h/parse v0.0.0-20250122154542-74294addb73e // indirect
	github.com/andybalholm/brotli v1.1.0 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cli/browser v1.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.3 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.11 // indirect
	github.com/klauspost/crc32 v1.3.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/minio/crc64nvme v1.1.1 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/natefinch/atomic v1.0.1 // indirect
	github.com/oapi-codegen/runtime v1.1.1 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/robfig/cron/v3 v3.0.2-0.20210106135023-bc59245fe10e // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/samber/lo v1.52.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/sony/gobreaker v0.5.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/tinylib/msgp v1.6.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/otel/metric v1.39.0 // indirect
	go.opentelemetry.io/proto/otlp v1.9.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/mod v0.31.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	golang.org/x/tools v0.40.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260112192933-99fd39fd28a9 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.77.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gorm.io/driver/postgres v1.5.9 // indirect
	gorm.io/gorm v1.30.1 // indirect
)
