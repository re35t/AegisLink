export interface TextChunk {
  id: string;
  text: string;
  metadata: Record<string, unknown>;
}

export interface ChunkTextInput {
  documentId: string;
  text: string;
  chunkSize?: number;
  overlap?: number;
  metadata?: Record<string, unknown>;
}

export function chunkText(input: ChunkTextInput): TextChunk[] {
  const chunkSize = input.chunkSize ?? 800;
  const overlap = input.overlap ?? 120;
  const normalized = input.text.replace(/\s+/g, " ").trim();

  if (!normalized) {
    return [];
  }

  const chunks: TextChunk[] = [];
  let start = 0;
  let index = 0;

  while (start < normalized.length) {
    const end = Math.min(start + chunkSize, normalized.length);
    const text = normalized.slice(start, end).trim();

    if (text) {
      chunks.push({
        id: `${input.documentId}:${index}`,
        text,
        metadata: {
          ...input.metadata,
          documentId: input.documentId,
          chunkIndex: index,
        },
      });
    }

    if (end === normalized.length) {
      break;
    }

    start = Math.max(0, end - overlap);
    index += 1;
  }

  return chunks;
}
