import type { RunAgentInput } from "@ag-ui/client";

import type { SelectedMention } from "./MentionCatalog";

export function withAegisSelection(
  input: RunAgentInput,
  selection?: SelectedMention,
): RunAgentInput {
  if (!selection) return input;
  return {
    ...input,
    forwardedProps: {
      ...input.forwardedProps,
      aegislink: {
        selection: { mentionId: selection.id, action: selection.action },
      },
    },
  };
}
