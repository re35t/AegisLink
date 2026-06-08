import { mkdir, readFile, mkdtemp, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { Server } from "node:http";
import { afterEach, describe, expect, it } from "vitest";
import { ModelLlmClient, ModelProviderError, type LlmClient } from "@aegislink/agent-client";
import { AegisLinkServer } from "../apps/server/src/aegislink-server.js";
import { ChatService } from "../apps/server/src/chat-service.js";
import { createAegisLinkHttpServer } from "../apps/server/src/http.js";

describe("ModelLlmClient", () => {
  it("calls an OpenAI-compatible chat completion endpoint", async () => {
    let requestedUrl = "";
    let requestedBody: unknown;
    const fetchFn: typeof fetch = async (url, init) => {
      requestedUrl = String(url);
      requestedBody = JSON.parse(String(init?.body));
      return jsonResponse(200, {
        choices: [{ message: { content: "model answer" } }],
      });
    };

    const llm = new ModelLlmClient({
      apiKey: "test-key",
      baseUrl: "https://model.example/v1/",
      model: "test-model",
      fetch: fetchFn,
    });

    await expect(llm.complete({ prompt: "hello" })).resolves.toBe("model answer");
    expect(requestedUrl).toBe("https://model.example/v1/chat/completions");
    expect(requestedBody).toMatchObject({
      model: "test-model",
      messages: [{ role: "user", content: "hello" }],
    });
  });

  it("surfaces provider errors without leaking request details", async () => {
    const llm = new ModelLlmClient({
      apiKey: "test-key",
      model: "test-model",
      fetch: async () => jsonResponse(429, { error: { message: "rate limited" } }),
    });

    await expect(llm.complete({ prompt: "hello" })).rejects.toMatchObject({
      message: "rate limited",
      statusCode: 429,
    });
  });
});

describe("ChatService", () => {
  it("answers through LocalAgentClient and persists conversation history", async () => {
    const baseDataDir = await mkdtemp(join(tmpdir(), "aegislink-chat-"));
    const agentDir = join(baseDataDir, "local-agent");
    await mkdir(agentDir, { recursive: true });
    await writeFile(join(agentDir, "soul.md"), "# local-agent soul\n\n- The user likes private agents.\n");

    const chat = new ChatService({ llm: new StaticLlm("static answer"), baseDataDir });
    const result = await chat.ask({ question: "What agents does the user like?", conversationId: "chat-test" });
    const history = await readFile(join(agentDir, "conversations.jsonl"), "utf8");

    expect(result.answer).toBe("static answer");
    expect(result.sources.some((source) => source.sourceType === "soul_md")).toBe(true);
    expect(history).toContain("What agents does the user like?");
    expect(history).toContain("static answer");
  });

  it("can answer without returning memory sources", async () => {
    const baseDataDir = await mkdtemp(join(tmpdir(), "aegislink-chat-no-memory-"));
    const agentDir = join(baseDataDir, "local-agent");
    await mkdir(agentDir, { recursive: true });
    await writeFile(join(agentDir, "soul.md"), "# local-agent soul\n\n- Hidden local memory.\n");

    const chat = new ChatService({ llm: new StaticLlm("no-memory answer"), baseDataDir });
    const result = await chat.ask({ question: "What is hidden?", useMemory: false });

    expect(result.answer).toBe("no-memory answer");
    expect(result.sources).toHaveLength(0);
  });
});

describe("HTTP chat API", () => {
  let server: Server | undefined;

  afterEach(async () => {
    if (server) {
      await new Promise<void>((resolve, reject) => {
        server?.close((error) => (error ? reject(error) : resolve()));
      });
      server = undefined;
    }
  });

  it("returns chat answers and sources", async () => {
    const baseDataDir = await mkdtemp(join(tmpdir(), "aegislink-http-chat-"));
    await mkdir(join(baseDataDir, "local-agent"), { recursive: true });
    await writeFile(join(baseDataDir, "local-agent", "soul.md"), "# local-agent soul\n\n- The user researches private agent communication.\n");
    server = createAegisLinkHttpServer({
      service: new AegisLinkServer(),
      chat: new ChatService({ llm: new StaticLlm("static answer"), baseDataDir }),
    });
    const url = await listen(server);

    const response = await fetch(`${url}/api/chat`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ question: "What communication topic?", conversationId: "test" }),
    });
    const body = await response.json();

    expect(response.status).toBe(200);
    expect(body.answer).toBe("static answer");
    expect(body.sources.some((source: { sourceType: string }) => source.sourceType === "soul_md")).toBe(true);
  });

  it("validates chat request bodies", async () => {
    server = createAegisLinkHttpServer({
      service: new AegisLinkServer(),
      chat: new ChatService({ llm: new StaticLlm("unused") }),
    });
    const url = await listen(server);

    const response = await fetch(`${url}/api/chat`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ question: "" }),
    });
    const body = await response.json();

    expect(response.status).toBe(400);
    expect(body.error).toBe("question must be a non-empty string");
  });
});

class StaticLlm implements LlmClient {
  constructor(private readonly answer: string) {}

  async complete(): Promise<string> {
    return this.answer;
  }
}

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "content-type": "application/json" },
  });
}

async function listen(server: Server): Promise<string> {
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (!address || typeof address === "string") {
    throw new Error("expected an ephemeral TCP address");
  }
  return `http://127.0.0.1:${address.port}`;
}
