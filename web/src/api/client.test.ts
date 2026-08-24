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

  it("uploads Skill bundles as multipart without overriding the boundary", async () => {
    const fetch = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ name: "portable-skill" }), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const bundle = new File(["---\nname: portable-skill\n---\n"], "SKILL.md", {
      type: "text/markdown",
    });
    await api.importSkill("agent-one", bundle, "1.0.0");
    const [, init] = fetch.mock.calls[0];
    expect(init?.body).toBeInstanceOf(FormData);
    expect(new Headers(init?.headers).has("Content-Type")).toBe(false);
    const form = init?.body as FormData;
    expect(form.get("bundle")).toBe(bundle);
    expect(form.get("version")).toBe("1.0.0");
  });
});
