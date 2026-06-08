# Local Dev

Install dependencies and run:

```bash
pnpm install
pnpm test
pnpm cli ask "What is AegisLink?"
```

Run the backend with the local echo model:

```bash
MODEL_PROVIDER=echo pnpm run dev:server
```

Run the backend with a real OpenAI-compatible model provider:

```bash
MODEL_API_KEY=... MODEL_NAME=... pnpm run dev:server
```

Call the chat API:

```bash
curl -sS http://127.0.0.1:4321/api/chat \
  -H "content-type: application/json" \
  -d '{"question":"What does my agent remember?","agentId":"local-agent"}'
```
