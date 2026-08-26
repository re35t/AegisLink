import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { InstructionsEditor } from "./AgentProfilePage";

describe("InstructionsEditor", () => {
  afterEach(() => cleanup());

  it("preserves a local draft and saves normalized private instructions", () => {
    const save = vi.fn();
    render(
      <InstructionsEditor
        instructions={{
          agentId: "agent-one",
          systemPrompt: "Be concise.",
          version: 1,
          updatedAt: "2026-08-26T00:00:00Z",
        }}
        pending={false}
        error=""
        conflict={false}
        onReload={vi.fn()}
        onSave={save}
      />,
    );

    fireEvent.change(screen.getByRole("textbox", { name: "System prompt" }), {
      target: { value: "  Answer directly.  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Save instructions" }));

    expect(save).toHaveBeenCalledWith("Answer directly.");
  });

  it("offers an explicit reload after a version conflict", () => {
    const reload = vi.fn();
    render(
      <InstructionsEditor
        instructions={{
          agentId: "agent-one",
          systemPrompt: "Be concise.",
          version: 3,
          updatedAt: "2026-08-26T00:00:00Z",
        }}
        pending={false}
        error="These instructions changed elsewhere."
        conflict
        onReload={reload}
        onSave={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Reload" }));
    expect(reload).toHaveBeenCalledOnce();
  });
});
