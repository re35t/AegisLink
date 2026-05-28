import type { VectorSearchResult, VectorStore } from "@aegislink/storage-core";
import type { EmbeddingProvider } from "./embedding.js";

export class RagRetriever {
  constructor(
    private readonly vectorStore: VectorStore,
    private readonly embeddings: EmbeddingProvider,
  ) {}

  async retrieve(input: {
    agentId: string;
    query: string;
    limit?: number;
    filter?: Record<string, unknown>;
  }): Promise<VectorSearchResult> {
    const queryEmbedding = await this.embeddings.embed(input.query);
    return this.vectorStore.search({
      queryEmbedding,
      limit: input.limit ?? 5,
      filter: {
        ...input.filter,
        agentId: input.agentId,
      },
    });
  }
}
