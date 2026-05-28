import type { MemoryItem, MemoryQuery, MemorySearchResult, MemoryStore } from "@aegislink/storage-core";

export class CompositeMemoryStore implements MemoryStore {
  constructor(private readonly stores: MemoryStore[]) {}

  async append(item: MemoryItem): Promise<void> {
    const [primary] = this.stores;
    if (!primary) {
      throw new Error("CompositeMemoryStore requires at least one backing store.");
    }
    await primary.append(item);
  }

  async search(query: MemoryQuery): Promise<MemorySearchResult> {
    const results = await Promise.all(this.stores.map((store) => store.search(query)));
    const items = results
      .flatMap((result) => result.items)
      .sort((a, b) => (b.score ?? 0) - (a.score ?? 0))
      .slice(0, query.limit ?? 10);

    return { items };
  }

  async getById(id: string): Promise<MemoryItem | null> {
    for (const store of this.stores) {
      const item = await store.getById(id);
      if (item) {
        return item;
      }
    }
    return null;
  }
}
