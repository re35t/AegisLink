import { randomUUID } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import type { MemoryItem, MemoryQuery, MemorySearchResult, MemoryStore } from "@aegislink/storage-core";
import { lexicalScore } from "./scoring.js";

export class SoulMdStore implements MemoryStore {
  constructor(
    private readonly filePath: string,
    private readonly agentId: string,
  ) {}

  async append(item: MemoryItem): Promise<void> {
    await mkdir(dirname(this.filePath), { recursive: true });
    const current = await this.readSoul();
    const line = `\n- ${item.content.replace(/\n+/g, " ").trim()}`;
    await writeFile(this.filePath, `${current.trimEnd()}${line}\n`, "utf8");
  }

  async search(query: MemoryQuery): Promise<MemorySearchResult> {
    const items = await this.readItems();
    const allowedVisibility = query.visibility ? new Set(query.visibility) : null;
    const matches = items
      .filter((item) => item.agentId === query.agentId)
      .filter((item) => !allowedVisibility || allowedVisibility.has(item.visibility))
      .map((item) => ({ ...item, score: lexicalScore(query.query, item.content) }))
      .filter((item) => item.score > 0 || query.query.trim() === "")
      .sort((a, b) => (b.score ?? 0) - (a.score ?? 0))
      .slice(0, query.limit ?? 10);

    return { items: matches };
  }

  async getById(id: string): Promise<MemoryItem | null> {
    const items = await this.readItems();
    return items.find((item) => item.id === id) ?? null;
  }

  private async readItems(): Promise<MemoryItem[]> {
    const raw = await this.readSoul();
    return raw
      .split("\n")
      .map((line) => line.trim())
      .filter((line) => line.startsWith("- "))
      .map((line, index) => ({
        id: `soul:${index}:${randomUUID()}`,
        agentId: this.agentId,
        content: line.slice(2).trim(),
        sourceType: "soul_md" as const,
        visibility: "private" as const,
        metadata: { path: this.filePath, line: index + 1 },
        createdAt: new Date(0).toISOString(),
      }));
  }

  private async readSoul(): Promise<string> {
    try {
      return await readFile(this.filePath, "utf8");
    } catch (error) {
      if (isNotFound(error)) {
        return `# ${this.agentId} soul\n`;
      }
      throw error;
    }
  }
}

function isNotFound(error: unknown): boolean {
  return error instanceof Error && "code" in error && error.code === "ENOENT";
}
