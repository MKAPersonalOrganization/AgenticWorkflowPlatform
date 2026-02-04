# ADR 005: Framework Adapter LLM Client Ownership

## Status
Accepted

## Context
Initially, the application was responsible for creating LLM clients (OpenAI, Gemini) and passing them to the agent framework adapters. This created several problems:

1. **Tight Coupling**: `main.go` needed to know about specific LLM client implementations from `foundation/agentsdk/llm`
2. **Framework Mismatch**: Different frameworks require different LLM client types:
   - Custom adapter uses `agentsdk/llm.ChatClient`
   - ADK-Go uses `google.golang.org/adk` models
   - Blades uses `github.com/go-kratos/blades.ModelProvider`
3. **Leaky Abstraction**: The abstraction layer exposed framework-specific implementation details (LLM clients) to the application
4. **Inflexibility**: Adding support for new frameworks required modifying the main application code

## Decision
We will make LLM client creation an implementation detail of each framework adapter:

1. **ModelConfig Abstraction**: The application provides framework-agnostic configuration containing provider name, model identifier, API key, and optional additional settings.

2. **Adapter Responsibility**: Each framework adapter receives configuration in its constructor and internally creates the appropriate LLM client type for its framework (e.g., `agentsdk/llm.ChatClient`, `adk.Model`, `blades.ModelProvider`).

3. **Application Simplification**: The application has no knowledge of LLM client implementations and no dependencies on framework-specific LLM packages.

## Consequences

### Positive
- **Better Separation of Concerns**: Application code is completely decoupled from LLM client implementations
- **Framework Independence**: Each adapter can use the LLM client type that best fits its framework
- **Easier Framework Addition**: New frameworks can be added without modifying application code
- **Simplified Dependencies**: Main application no longer imports framework-specific LLM packages
- **Consistent Interface**: All adapters accept the same `ModelConfig` abstraction

### Negative
- **Testing Complexity**: Testing requires mocking at the adapter level rather than injecting test clients

### Neutral
- **Configuration Centralization**: All LLM configuration flows through a single abstraction

## Related Decisions
- **ADR 001**: Establishes the framework adapter pattern that this decision extends
