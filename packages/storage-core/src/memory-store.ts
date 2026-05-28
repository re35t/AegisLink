export type MemorySourceType = "conversation" | "soul_md" | "document" | "external_app";
export type MemoryVisibility = "private" | "owner" | "team" | "org" | "public";

export interface MemoryItem {
  id: string;
  agentId: string;
  content: string;
  sourceType: MemorySourceType;
  visibility: MemoryVisibility;
  metadata: Record<string, unknown>;
  createdAt: string;
}

export interface MemoryQuery {
  agentId: string;
  query: string;
  limit?: number;
  visibility?: MemoryVisibility[];
}

export interface ScoredMemoryItem extends MemoryItem {
  score?: number;
}

export interface MemorySearchResult {
  items: ScoredMemoryItem[];
}

export interface MemoryStore {
  append(item: MemoryItem): Promise<void>;
  search(query: MemoryQuery): Promise<MemorySearchResult>;
  getById(id: string): Promise<MemoryItem | null>;
}
