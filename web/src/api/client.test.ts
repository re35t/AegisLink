import { afterEach, describe, expect, it, vi } from "vitest";

import { APIError, api } from "./client";

describe("API client", () => {
  afterEach(() => vi.restoreAllMocks());

  it("preserves the server error contract", async () => {
    const fetch = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          code: "active_run_exists",
          message: "busy",
          requestId: "req-1",
        }),
        {
          status: 409,
          headers: { "Content-Type": "application/json" },
        },
      ),
    );
    await expect(api.sendMessage("conversation", "hello")).rejects.toEqual(
      new APIError("active_run_exists", "busy", "req-1", 409),
    );
    expect(fetch).toHaveBeenCalledWith(
      "/api/v1/conversations/conversation/messages",
      expect.objectContaining({ credentials: "include" }),
    );
  });
});
