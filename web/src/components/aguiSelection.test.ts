import { describe, expect, it } from "vitest";
import type { RunAgentInput } from "@ag-ui/client";

import { withAegisSelection } from "./aguiSelection";

describe("withAegisSelection", () => {
  it("forwards only the opaque mention id and action", () => {
    const input = {
      threadId: "thread-one",
      runId: "run-one",
      state: {},
      messages: [],
      tools: [
        {
          name: "untrusted_browser_tool",
          description: "ignore me",
          parameters: {},
        },
      ],
      context: [],
      forwardedProps: { existing: true },
    } as RunAgentInput;

    const result = withAegisSelection(input, {
      id: "opaque-mention",
      kind: "mcp-tool",
      action: "force-tool-once",
      label: "read_file",
      groupLabel: "GitHub",
    });

    expect(result.forwardedProps).toEqual({
      existing: true,
      aegislink: {
        selection: {
          mentionId: "opaque-mention",
          action: "force-tool-once",
        },
      },
    });
    expect(result.tools).toEqual(input.tools);
    expect(JSON.stringify(result.forwardedProps)).not.toContain("read_file");
    expect(JSON.stringify(result.forwardedProps)).not.toContain("GitHub");
  });

  it.each([
    ["skill", "skill:review", "use-skill-once"],
    ["discovery", "discovery:agent-search", "discover-once"],
  ] as const)(
    "forwards the %s capability as a typed action",
    (kind, id, action) => {
      const result = withAegisSelection({} as RunAgentInput, {
        id,
        kind,
        action,
        label: kind,
        groupLabel: "AegisLink",
      });

      expect(result.forwardedProps).toEqual({
        aegislink: { selection: { mentionId: id, action } },
      });
    },
  );
});
