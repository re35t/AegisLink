import type { VectorRecord, VectorSearchInput, VectorSearchResult, VectorStore } from "@aegislink/storage-core";

export class ChromaVectorStore implements VectorStore {
  private readonly records = new Map<string, VectorRecord>();

  async upsert(records: VectorRecord[]): Promise<void> {
    for (const record of records) {
      this.records.set(record.id, record);
    }
  }

  async search(input: VectorSearchInput): Promise<VectorSearchResult> {
    const matches = Array.from(this.records.values())
      .filter((record) => matchesFilter(record.metadata, input.filter))
      .map((record) => ({
        ...record,
        score: cosineSimilarity(input.queryEmbedding, record.embedding),
      }))
      .sort((a, b) => b.score - a.score)
      .slice(0, input.limit);

    return { matches };
  }

  async deleteByFilter(filter: Record<string, unknown>): Promise<void> {
    for (const [id, record] of this.records) {
      if (matchesFilter(record.metadata, filter)) {
        this.records.delete(id);
      }
    }
  }
}

function matchesFilter(metadata: Record<string, unknown>, filter?: Record<string, unknown>): boolean {
  if (!filter) {
    return true;
  }

  return Object.entries(filter).every(([key, value]) => metadata[key] === value);
}

function cosineSimilarity(left: number[], right: number[]): number {
  const length = Math.min(left.length, right.length);
  let dot = 0;
  let leftMagnitude = 0;
  let rightMagnitude = 0;

  for (let index = 0; index < length; index += 1) {
    dot += left[index] * right[index];
    leftMagnitude += left[index] * left[index];
    rightMagnitude += right[index] * right[index];
  }

  if (leftMagnitude === 0 || rightMagnitude === 0) {
    return 0;
  }

  return dot / (Math.sqrt(leftMagnitude) * Math.sqrt(rightMagnitude));
}
