import { createHash } from "node:crypto";

export interface EmbeddingProvider {
  embed(text: string): Promise<number[]>;
}

export class HashEmbeddingProvider implements EmbeddingProvider {
  constructor(private readonly dimensions = 64) {}

  async embed(text: string): Promise<number[]> {
    const vector = Array.from({ length: this.dimensions }, () => 0);
    const tokens = tokenize(text);

    for (const token of tokens) {
      const digest = createHash("sha256").update(token).digest();
      const bucket = digest[0] % this.dimensions;
      const sign = digest[1] % 2 === 0 ? 1 : -1;
      vector[bucket] += sign;
    }

    const magnitude = Math.sqrt(vector.reduce((sum, value) => sum + value * value, 0));
    return magnitude === 0 ? vector : vector.map((value) => value / magnitude);
  }
}

export function tokenize(text: string): string[] {
  return text
    .toLowerCase()
    .split(/[^\p{L}\p{N}_]+/u)
    .map((token) => token.trim())
    .filter(Boolean);
}
