import type { MemoryQuery, MemorySearchResult } from "@aegislink/storage-core";

export interface AgentClient {
  ask(input: AskInput): Promise<AskResult>;
  getMemory(input: MemoryQuery): Promise<MemorySearchResult>;
  requestAgent(input: AgentRequestInput): Promise<AgentRequestResult>;
}

export interface AskInput {
  question: string;
  conversationId?: string;
  useMemory?: boolean;
  useRag?: boolean;
}

export interface AskResult {
  answer: string;
  conversationId: string;
  sources: MemorySource[];
}

export interface MemorySource {
  id: string;
  sourceType: "conversation" | "soul_md" | "document" | "external_app";
  content: string;
  score?: number;
  metadata?: Record<string, unknown>;
}

export interface AgentRequestInput {
  targetAgentId: string;
  question: string;
  purpose: string;
  capabilityToken?: string;
}

export interface AgentRequestResult {
  targetAgentId: string;
  question: string;
  status: "stubbed_until_phase_4";
}

export interface LlmClient {
  complete(input: { prompt: string }): Promise<string>;
}
