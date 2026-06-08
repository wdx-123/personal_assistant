# Qdrant Client And Collection Init

# Goal

Initialize the official Qdrant Go client and ensure the base vector collection exists at application startup.

# Scope

- Add Qdrant gRPC client configuration alongside the existing HTTP endpoint.
- Initialize a global Qdrant client in the runtime infrastructure layer.
- Create or validate the `ai_knowledge_chunks` collection with 1024-dimensional cosine vectors.
- Do not add RAG, embedding, indexer, retriever, or API behavior in this task.

# Changes

- Extend `internal/model/config.Qdrant` with enabled, gRPC, collection, timeout, and TLS settings.
- Bind the new `QDRANT_*` environment variables and defaults in `internal/core/config.go`.
- Add `internal/core/qdrant.go` for health check, client creation, collection existence check, creation, and existing schema validation.
- Add `global.QdrantClient` and initialize it from `internal/init/init.go` after config and logger setup.
- Add `github.com/qdrant/go-client` as the official gRPC client dependency.

# Verification

- Run `go test ./internal/model/config ./internal/core ./internal/infrastructure/ai/...`.
- Verify `.env.example` and `configs/configs.yaml` contain only Qdrant placeholders.
- Verify local `.env` contains the required Qdrant keys without printing secrets.

# Assumptions

- The Go client uses Qdrant gRPC on port `6334`; HTTP/REST remains on `6333`.
- The first collection is `ai_knowledge_chunks`, vector size `1024`, distance `cosine`.
- If Qdrant is enabled and initialization fails, startup must fail fast.
