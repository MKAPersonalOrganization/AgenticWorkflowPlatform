# Decouple Event Storage from SDK Goal

## Objective
Remove all event storage and persistence logic from the custom SDK (`foundation/agentsdk`). The SDK should purely yield events and maintain its own internal execution state, while the responsibility for durable storage is moved to the application's service layer.

## Core Principles

- **Yield-Only SDK**: The SDK execution loop (`engine.Run`) must not have side effects related to external storage. It emits events via `iter.Seq2`.
- **Synchronous Persistence**: The service layer (or adapter) is responsible for intercepting yielded events and persisting those that are "durable" (messages, tool results, usage) before continuing the execution stream.
- **Framework Agnosticism**: Persistence logic should operate on generic `agent.Event` types, ensuring that storage behavior is consistent across different agent frameworks (Custom, Blades, ADK-Go).
- **Safe State Management**: The SDK maintains its own in-memory history during a turn to ensure the LLM has context, but it does not "load" or "save" this history to a database directly.

## Success Criteria

1.  `foundation/agentsdk` has zero imports of `services/threadmgmt` or any other storage-related service.
2.  All existing tests pass with storage mocked or disabled at the SDK level.
3.  New agent frameworks can be integrated by only implementing the `agent.Runner` interface, with persistence handled automatically by the service layer.
