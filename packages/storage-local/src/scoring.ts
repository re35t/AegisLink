import { tokenize } from "@aegislink/rag-core";

export function lexicalScore(query: string, candidate: string): number {
  const queryTokens = new Set(tokenize(query));
  const candidateTokens = new Set(tokenize(candidate));

  if (queryTokens.size === 0 || candidateTokens.size === 0) {
    return 0;
  }

  let hits = 0;
  for (const token of queryTokens) {
    if (candidateTokens.has(token)) {
      hits += 1;
    }
  }

  return hits / Math.sqrt(queryTokens.size * candidateTokens.size);
}
