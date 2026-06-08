import { ModelLlmClient, type LlmClient } from "@aegislink/agent-client";
import { AegisLinkServer } from "./aegislink-server.js";
import { ChatService } from "./chat-service.js";
import { createAegisLinkHttpServer } from "./http.js";

const port = Number.parseInt(process.env.PORT ?? "4321", 10);
const host = process.env.HOST ?? "127.0.0.1";
const service = new AegisLinkServer();
const llm = createLlmClient();
const chat = new ChatService({
  llm,
  baseDataDir: process.env.AGENT_DATA_ROOT ?? "data/agents",
  defaultAgentId: process.env.DEFAULT_AGENT_ID ?? "local-agent",
});
const server = createAegisLinkHttpServer({ service, chat });

server.on("error", (error: NodeJS.ErrnoException) => {
  if (error.code === "EADDRINUSE") {
    console.error(`Port ${port} is already in use. Try PORT=4322 pnpm run dev:server.`);
  } else if (error.code === "EPERM") {
    console.error(`Cannot bind ${host}:${port}. Check local sandbox/firewall permissions or try another PORT/HOST.`);
  } else {
    console.error(error);
  }
  process.exitCode = 1;
});

server.listen(port, host, () => {
  console.log(`AegisLink server listening on http://${host}:${port}`);
});

function createLlmClient(): LlmClient {
  if (process.env.MODEL_PROVIDER === "echo") {
    return {
      async complete(input: { prompt: string }): Promise<string> {
        const question = input.prompt.split("User question:").at(-1)?.trim() ?? "";
        return `Local echo response: I received "${question}".`;
      },
    };
  }

  return new ModelLlmClient({
    apiKey: process.env.MODEL_API_KEY ?? "",
    baseUrl: process.env.MODEL_BASE_URL,
    model: process.env.MODEL_NAME ?? "",
    timeoutMs: Number.parseInt(process.env.MODEL_TIMEOUT_MS ?? "30000", 10),
  });
}
