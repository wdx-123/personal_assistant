# Qdrant Config Integration

# Goal

Add Qdrant vector database configuration support and configure the local `.env`.

# Scope

- Expose Qdrant config through `global.Config.Qdrant.Endpoint` and `global.Config.Qdrant.APIKey`.
- Bind `QDRANT_ENDPOINT` and `QDRANT_API_KEY` from environment variables.
- Add template placeholders without real credentials.
- Store the real local Qdrant API key only in ignored `.env`.

# Changes

- Add a Qdrant config struct under `internal/model/config`.
- Register Qdrant in the root `Config` object and `NewConfig()`.
- Add default values and env bindings in `internal/core/config.go`.
- Add Qdrant placeholders to `.env.example` and `configs/configs.yaml`.
- Add local `.env` values using the existing MySQL host and port `6333`.

# Verification

- Run `go test ./internal/model/config ./internal/core ./internal/infrastructure/ai/...`.
- Verify `.env` contains `QDRANT_ENDPOINT` and `QDRANT_API_KEY` without printing secrets.
- Check `git status` to ensure `.env` remains ignored.

# Assumptions

- Qdrant HTTP endpoint uses port `6333`.
- The provided Qdrant password is used as the Qdrant API key.
- No Qdrant client, collection initialization, embedding, indexer, or retriever logic is added in this task.
