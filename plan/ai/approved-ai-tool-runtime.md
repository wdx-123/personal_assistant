# AI Tool Runtime Minimum Loop

## Summary

Implement the first-stage AI tool loop without adding HTTP routes, controllers, repositories, database schema, interrupt, checkpoint, A2UI, MCP, or write-operation tools. The AI layer keeps the existing MVC shell and adds domain protocols, service orchestration, Eino tool calling, and trace projection.

## Key Changes

- Add stable AI tool protocol types in `internal/domain/ai`, extend `StreamInput`, and add tool call events.
- Add service-side tool principal, registry, policy filtering, prompt building, helpers, and the full first-stage read-only tool catalog.
- Keep user authorization fact-based instead of mapping users into fixed role buckets. Tool visibility and execution authorization are controlled by per-tool policy.
- Inject Authorization, OJ, OJTask, and Observability service contracts into AIService through `AIDeps`.
- Use Eino tool calling when tools are available, and preserve the existing pure text stream path when no tool is available.
- Persist tool events through the existing sink/projector path into `trace_items_json`.

## Permission Policy

- Personal tools are self-only.
- OJ task and organization tools use `consts.CapabilityCodeOJTaskManage` and do parameter-level authorization at execution time.
- Observability tools are super-admin only.
- No API resource permission mapping and no virtual tool resource table will be added in this phase.

## Tests

- Registry and policy filtering tests for self-only, org capability, and super-admin tools.
- Execution authorization tests for denied org capability and denied super-admin access.
- Projector tests for tool started/finished trace items.
- Eino adapter tests with fake tools/model behavior where feasible.
- Run targeted AI tests and `go test ./...`.
