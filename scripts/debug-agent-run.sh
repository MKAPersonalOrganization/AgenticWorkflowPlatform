#!/usr/bin/env bash
# debug-agent-run.sh — post a catalog YAML agent + a user prompt to the
# internal debug endpoint and pretty-print the JSON response.
#
# Usage:
#   scripts/debug-agent-run.sh <path/to/catalog.yaml> <prompt>
#
# Examples:
#   scripts/debug-agent-run.sh modules/templates/catalog/sequential-workflow.yaml "pick a city"
#   MODEL=gpt-4o scripts/debug-agent-run.sh modules/templates/catalog/output-schema.yaml "hi"
#
# Env (all optional):
#   PROVIDER    LLM provider        default: openai
#   MODEL       model name          default: gpt-4o-mini
#   API_KEY     API key             default: $OPENAI_API_KEY
#   DEBUG_URL   endpoint URL        default: http://127.0.0.1:4015/debug/agent:run

set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <catalog.yaml> <prompt>" >&2
  exit 2
fi

yaml_path="$1"
prompt="$2"

if [[ ! -f "$yaml_path" ]]; then
  echo "catalog file not found: $yaml_path" >&2
  exit 2
fi

provider="${PROVIDER:-openai}"
model="${MODEL:-gpt-4o-mini}"
api_key="${API_KEY:-${OPENAI_API_KEY:-}}"
debug_url="${DEBUG_URL:-http://127.0.0.1:4015/debug/agent:run}"

if [[ -z "$api_key" ]]; then
  echo "API_KEY (or OPENAI_API_KEY) must be set" >&2
  exit 2
fi

# jq -n --arg ... --rawfile ... keeps every string safely escaped — no manual
# quoting, no surprises with backticks/newlines/quotes in the YAML or prompt.
body="$(
  jq -n \
    --arg provider "$provider" \
    --arg model "$model" \
    --arg api_key "$api_key" \
    --arg prompt "$prompt" \
    --rawfile yaml "$yaml_path" \
    '{provider: $provider, model: $model, api_key: $api_key, prompt: $prompt, agent_yaml: $yaml}'
)"

curl -sS --fail-with-body -X POST "$debug_url" \
  -H 'content-type: application/json' \
  -d "$body" \
| jq .
