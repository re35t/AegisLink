import { join } from "node:path";
import { LocalAgentClient, type AskResult, type LlmClient } from "@aegislink/agent-client";
import { CompositeMemoryStore, JsonConversationStore, SoulMdStore } from "@aegislink/storage-local";
import { ValidationError } from "./aegislink-server.js";

export interface ChatServiceOptions {
  llm: LlmClient;
  baseDataDir?: string;
  defaultAgentId?: string;
}

export interface ChatRequestInput {
  question: string;
  agentId?: string;
  conversationId?: string;
  useMemory?: boolean;
}

export class ChatService {
  private readonly clients = new Map<string, LocalAgentClient>();
  private readonly baseDataDir: string;
  private readonly defaultAgentId: string;

  constructor(private readonly options: ChatServiceOptions) {
    this.baseDataDir = options.baseDataDir ?? "data/agents";
    this.defaultAgentId = options.defaultAgentId ?? "local-agent";
  }

  async ask(input: ChatRequestInput): Promise<AskResult> {
    const agentId = input.agentId ?? this.defaultAgentId;
    assertSafeAgentId(agentId);

    return this.clientFor(agentId).ask({
      question: input.question,
      conversationId: input.conversationId,
      useMemory: input.useMemory,
      useRag: true,
    });
  }

  private clientFor(agentId: string): LocalAgentClient {
    const cached = this.clients.get(agentId);
    if (cached) {
      return cached;
    }

    const dataDir = join(this.baseDataDir, agentId);
    const memory = new CompositeMemoryStore([
      new JsonConversationStore(join(dataDir, "conversations.jsonl")),
      new SoulMdStore(join(dataDir, "soul.md"), agentId),
    ]);
    const client = new LocalAgentClient({
      agentId,
      memory,
      llm: this.options.llm,
    });
    this.clients.set(agentId, client);
    return client;
  }
}

function assertSafeAgentId(agentId: string): void {
  if (!/^[A-Za-z0-9_.-]+$/.test(agentId)) {
    throw new ValidationError("agentId may only contain letters, numbers, dot, underscore, and dash");
  }
}
