import type { VectorStore } from "@aegislink/storage-core";
import { chunkText } from "./chunker.js";
import type { EmbeddingProvider } from "./embedding.js";

export interface RagIndexer {
  indexDocument(input: IndexDocumentInput): Promise<number>;
}

export interface IndexDocumentInput {
  documentId: string;
  agentId: string;
  text: string;
  metadata?: Record<string, unknown>;
}

export class DefaultRagIndexer implements RagIndexer {
  constructor(
    private readonly vectorStore: VectorStore,
    private readonly embeddings: EmbeddingProvider,
  ) {}

  async indexDocument(input: IndexDocumentInput): Promise<number> {
    const chunks = chunkText({
      documentId: input.documentId,
      text: input.text,
      metadata: {
        ...input.metadata,
        agentId: input.agentId,
        sourceType: "document",
      },
    });

    const records = await Promise.all(
      chunks.map(async (chunk) => ({
        id: chunk.id,
        text: chunk.text,
        embedding: await this.embeddings.embed(chunk.text),
        metadata: chunk.metadata,
      })),
    );

    await this.vectorStore.upsert(records);
    return records.length;
  }
}
