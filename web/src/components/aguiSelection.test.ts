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
});
