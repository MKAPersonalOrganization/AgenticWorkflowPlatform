# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Created `core.RunContext` in the Agent SDK to encapsulate execution state (thread, IDs, options).
- Introduced Go 1.23+ iterator support (`iter.Seq2`) for reactive agent event streaming.

### Changed
- **Optimal Agent Engine Refactor**: 
    - Replaced `IterationContext` with a more comprehensive `RunContext`.
    - Refactored `AgentOrchestrator` interface to return an event stream (`iter.Seq2[core.Event, error]`).
    - Decoupled `Engine` from `EventNotifier` side-effects, making it a pure event generator.
    - Updated `Service` layer to consume engine iterators and handle notifications reactively.
    - Improved recursive tool-calling logic to yield events from sub-iterations seamlessly.
- Consolidated `RunWithThread` and `RunIteration` into a unified `Run` method.
- Refactored tool `Invoke` methods to return `core.ToolResult` for structured results and control flow (e.g., `StopIteration`).

### Removed
- `core.IterationContext` (replaced by `RunContext`).
- Direct `EventNotifier` dependency from the execution `Engine`.
- Unused `IsCommand` utility function.

