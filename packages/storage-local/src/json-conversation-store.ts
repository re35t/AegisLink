import { randomUUID } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import type { MemoryItem, MemoryQuery, MemorySearchResult, MemoryStore } from "@aegislink/storage-core";
import { lexicalScore } from "./scoring.js";

export class JsonConversationStore implements MemoryStore {
  constructor(private readonly filePath: string) {}

  async append(item: MemoryItem): Promise<void> {
    await mkdir(dirname(this.filePath), { recursive: true });
    const normalized: MemoryItem = {
      ...item,
      id: item.id || randomUUID(),
      metadata: item.metadata ?? {},
    };
    const existing = await this.readRaw();
    await writeFile(this.filePath, `${existing}${JSON.stringify(normalized)}\n`, "utf8");
  }

  async search(query: MemoryQuery): Promise<MemorySearchResult> {
    const items = await this.readAll();
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
    const items = await this.readAll();
    return items.find((item) => item.id === id) ?? null;
  }

  private async readRaw(): Promise<string> {
    try {
      return await readFile(this.filePath, "utf8");
    } catch (error) {
      if (isNotFound(error)) {
        return "";
      }
      throw error;
    }
  }

  private async readAll(): Promise<MemoryItem[]> {
    const raw = await this.readRaw();
    return raw
      .split("\n")
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line) => JSON.parse(line) as MemoryItem);
  }
}

function isNotFound(error: unknown): boolean {
  return error instanceof Error && "code" in error && error.code === "ENOENT";
}
