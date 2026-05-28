import { LocalAgentClient, type LlmClient } from "@aegislink/agent-client";
import { CompositeMemoryStore, JsonConversationStore, SoulMdStore } from "@aegislink/storage-local";

export interface AskCommandInput {
  question: string;
  agentId?: string;
  dataDir?: string;
}

export async function runAskCommand(input: AskCommandInput): Promise<void> {
  const agentId = input.agentId ?? "local-agent";
  const dataDir = input.dataDir ?? `data/agents/${agentId}`;
  const memory = new CompositeMemoryStore([
    new JsonConversationStore(`${dataDir}/conversations.jsonl`),
    new SoulMdStore(`${dataDir}/soul.md`, agentId),
  ]);

  const client = new LocalAgentClient({
    agentId,
    memory,
    llm: new EchoLlm(),
  });

  const result = await client.ask({
    question: input.question,
    useMemory: true,
    useRag: true,
  });

  process.stdout.write(`${result.answer}\n`);
  if (result.sources.length > 0) {
    process.stdout.write("\nSources:\n");
    for (const source of result.sources) {
      process.stdout.write(`- ${source.sourceType} ${source.score?.toFixed(3) ?? "n/a"}: ${source.content}\n`);
    }
  }
}

class EchoLlm implements LlmClient {
  async complete(input: { prompt: string }): Promise<string> {
    const question = input.prompt.split("User question:").at(-1)?.trim() ?? "";
    return `Local Phase 1 response: I received "${question}". Memory and RAG context were attached to the prompt.`;
  }
}
