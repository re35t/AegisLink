export interface VectorRecord {
  id: string;
  text: string;
  embedding: number[];
  metadata: Record<string, unknown>;
}

export interface VectorSearchInput {
  queryEmbedding: number[];
  limit: number;
  filter?: Record<string, unknown>;
}

export interface VectorSearchResult {
  matches: Array<VectorRecord & { score: number }>;
}

export interface VectorStore {
  upsert(records: VectorRecord[]): Promise<void>;
  search(input: VectorSearchInput): Promise<VectorSearchResult>;
  deleteByFilter(filter: Record<string, unknown>): Promise<void>;
}
