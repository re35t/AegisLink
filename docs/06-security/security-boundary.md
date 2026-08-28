# Current security boundary

## Authentication and ownership

- Email/password registration uses Argon2id. The browser receives an opaque `HttpOnly`, `SameSite=Strict` session cookie; PostgreSQL stores only its hash.
- The authenticated Human Principal comes from the server-side session. Clients cannot submit or override `principal_id`.
- Resource access is checked against both Principal ownership and authoritative Agent scope. Non-owner lookups use not-found semantics where exposing existence would leak data.
- CORS and origin checks allow the configured Web origin. Production deployments must enable secure cookies and terminate HTTPS correctly.

## Agent execution

- Historical messages supplied in AG-UI input are not authoritative; the Conversation and Agent are reloaded from PostgreSQL.
- Composer selections contain only opaque Mention IDs and actions. The server re-resolves enabled Skill/MCP bindings, Tool risk, connection state, and ownership before a Run starts.
- Only explicitly enabled `read-only` MCP Tools enter Runtime. `external-write` and `destructive` Tools remain blocked because persisted per-call Approval is not implemented.
- Provider keys, MCP credentials, owner identifiers, and private chain-of-thought must not enter API responses, Profile projections, events, or logs. A System Prompt may be returned only through its authenticated, owner-scoped Agent Instructions endpoint; it must not enter Agent lists, Profile projections, AgentFacts, events, or logs.

## Profile disclosure

Disclosure Policy is enforced when AgentFacts is built. The default is private. Restricted disclosure requires exact token audience matching, and indexing is valid only for public AgentFacts subjects. Impressions and user/project/task Facts cannot be externally disclosed. Scope-reducing policy changes, Fact revocation, publication disablement, and key rotation revoke the active publication transactionally.

Messages, Tool results, and Memory supplied to the curator are untrusted evidence. The curator has no Tools, emits schema-validated drafts, and cannot write Confirmed Facts. Harness labels Impressions as fallible low-priority context and never injects their raw evidence or disclosure metadata.

Agent signing uses Ed25519. Private keys are encrypted with AES-256-GCM under the base64 32-byte `AGENT_KEY_ENCRYPTION_KEY`, with Agent/key IDs as associated data. If the master key is absent, Profile/Impression behavior remains available but all publication/key operations are blocked. Query-token secrets are returned once; only SHA-256 hashes are stored.

## Storage and operations

- Goose migrations are immutable and authoritative; GORM `AutoMigrate` is prohibited.
- Repository integration tests are destructive and only accept a database name ending in `_test`. The dedicated Compose test service is separate from development data.
- Skill imports reject traversal, absolute paths, symlinks, duplicate normalized paths, oversized archives/files, and undeclared roots. Stored scripts have no execution authority.

## Standalone Index

The Index uses separate PostgreSQL + pgvector, a static Registration/Publication Bearer Token of at least 32 characters, and a separate Query Token. The HTTP boundary compares Tokens in constant time and never logs headers, request bodies, or complete vectors. Registration persists only a SHA-256 Token-scoped idempotency digest. Publication accepts only disclosure-approved vectors and digests produced by an Agent Server, never Fact text. There is no Facts URL, LSH input, outbound fetch path, or public resolution.

The static Token authenticates access to the controlled Registry/Publication API; it does not prove Agent identity or authorize an Agent Server to update a specific record. AgentAddr is therefore explicitly unsigned. Scoped credentials, signing, update/revocation, disclosure-approved publisher automation, and discovery leases remain required before treating registration as a stronger trust statement. Candidate results must always be verified against current signed AgentFacts before use.

## Known gaps

Email verification, password reset, MFA, login rate limiting, delegated access, MCP OAuth/secret storage, write/destructive Tool Approval, third-party Agent credentials/attestations, Index scoped identity/signing/rate limiting, A2A request authentication, audit retention policy, and sandboxed Skill execution are not implemented. Deployment documentation must not claim these protections.
