# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Created `core.RunContext` in the Agent SDK to encapsulate execution state (thread, IDs, options).
- Introduced Go 1.23+ iterator support (`iter.Seq2`) for reactive agent event streaming.
- Introduced `core.UnmarshalArgs` helper for type-safe tool argument parsing.

### Changed
- **Tool Interface Refactor (ADK-Go Alignment)**:
    - Updated `Tool.Invoke` signature from `Invoke(ctx ToolContext, arguments string) (ToolResult, error)` to `Invoke(ctx ToolContext, args any) (any, error)`.
    - Tools now receive unmarshaled `map[string]any` or structured data, reducing JSON boilerplate.
    - Updated `ToolResult` to support `any` in its `Result` field for flexible response types.
    - Updated `ToolExecutor` to handle automatic unmarshaling of input and marshaling of outputs.
- **Optimal Agent Engine Refactor**: 
    - Replaced `IterationContext` with a more comprehensive `RunContext`.
    - Refactored `AgentOrchestrator` interface to return an event stream (`iter.Seq2[core.Event, error]`).
    - Decoupled `Engine` from `EventNotifier` side-effects, making it a pure event generator.
    - Updated `Service` layer to consume engine iterators and handle notifications reactively.
    - Improved recursive tool-calling logic to yield events from sub-iterations seamlessly.
- Consolidated `RunWithThread` and `RunIteration` into a unified `Run` method.
- Refactored `calculator`, `whiteboard`, `handoff`, `todo`, `plot`, `animate`, `statistics`, and `mermaid` tools to use typed arguments.

### Fixed
- **Infinite Handoff Loop**: Restored `Command` metadata propagation in `ToolExecutor`, ensuring `StopIteration` is correctly detected after agent transfers.
- **Test Suite Modernization**: Updated comprehensive test suite to handle typed arguments and fix type assertion mismatches (e.g., `int` vs `float64`).

### Removed
- `core.IterationContext` (replaced by `RunContext`).
- Direct `EventNotifier` dependency from the execution `Engine`.
- Unused `IsCommand` utility function.

