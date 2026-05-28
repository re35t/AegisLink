import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import { LocalAgentClient, type LlmClient } from "@aegislink/agent-client";
import { generateAgentIdentity, signPayload, verifyPayload } from "@aegislink/crypto-identity";
import { DefaultRagIndexer, HashEmbeddingProvider, RagRetriever } from "@aegislink/rag-core";
import { CompositeMemoryStore, JsonConversationStore, SoulMdStore } from "@aegislink/storage-local";
import { ChromaVectorStore } from "@aegislink/vector-chroma";

describe("Phase 1 local agent", () => {
  it("searches soul.md memory and writes conversation history", async () => {
    const dir = await mkdtemp(join(tmpdir(), "aegislink-"));
    await writeFile(join(dir, "soul.md"), "# local-agent soul\n\n- The user researches private agent communication.\n");

    const memory = new CompositeMemoryStore([
      new JsonConversationStore(join(dir, "conversations.jsonl")),
      new SoulMdStore(join(dir, "soul.md"), "local-agent"),
    ]);
    const client = new LocalAgentClient({
      agentId: "local-agent",
      memory,
      llm: new StaticLlm(),
    });

    const result = await client.ask({ question: "What communication topic does the user research?" });
    const history = await readFile(join(dir, "conversations.jsonl"), "utf8");

    expect(result.answer).toContain("static answer");
    expect(result.sources.some((source) => source.sourceType === "soul_md")).toBe(true);
    expect(history).toContain("What communication topic");
    expect(history).toContain("static answer");
  });

  it("indexes and retrieves document chunks through the Chroma-compatible vector adapter", async () => {
    const vectorStore = new ChromaVectorStore();
    const embeddings = new HashEmbeddingProvider();
    const indexer = new DefaultRagIndexer(vectorStore, embeddings);
    const retriever = new RagRetriever(vectorStore, embeddings);

    const count = await indexer.indexDocument({
      agentId: "local-agent",
      documentId: "doc-1",
      text: "AegisLink routes signed agent messages with least privilege capability claims.",
    });
    const result = await retriever.retrieve({
      agentId: "local-agent",
      query: "signed messages capability",
      limit: 1,
    });

    expect(count).toBeGreaterThan(0);
    expect(result.matches[0]?.text).toContain("signed agent messages");
  });

  it("generates Ed25519 identities and verifies signed payloads", () => {
    const identity = generateAgentIdentity("local-agent");
    const payload = { agentId: "local-agent", action: "agent.ask" };
    const signature = signPayload({ privateKeyPem: identity.privateKeyPem, payload });

    expect(verifyPayload({ publicKeyPem: identity.publicKeyPem, payload, signature })).toBe(true);
    expect(verifyPayload({ publicKeyPem: identity.publicKeyPem, payload: { ...payload, action: "memory.read" }, signature })).toBe(false);
  });
});

class StaticLlm implements LlmClient {
  async complete(): Promise<string> {
    return "static answer";
  }
}
