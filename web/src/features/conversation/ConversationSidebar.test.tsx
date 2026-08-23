import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { Conversation } from "../../api/client";
import { ConversationSidebar } from "./ConversationSidebar";

const conversations: Conversation[] = [
  {
    id: "conversation-1",
    agentId: "aegis",
    title: "Plan the agent runtime",
    createdAt: "2026-08-23T00:00:00Z",
    updatedAt: "2026-08-23T00:00:00Z",
  },
  {
    id: "conversation-2",
    agentId: "aegis",
    title: "Review memory design",
    createdAt: "2026-08-23T00:00:00Z",
    updatedAt: "2026-08-23T00:00:00Z",
  },
];

describe("ConversationSidebar", () => {
  it("filters conversations without losing the selected navigation action", () => {
    const onSelect = vi.fn();
    render(
      <ConversationSidebar
        conversations={conversations}
        activeConversationId="conversation-1"
        loading={false}
        failed={false}
        creating={false}
        loggingOut={false}
        agentName="Aegis"
        modelLabel="deepseek-v4-flash"
        onCreate={vi.fn()}
        onLogout={vi.fn()}
        onRetry={vi.fn()}
        onSelect={onSelect}
      />,
    );

    const search = screen.getByRole("searchbox", {
      name: "Search conversations",
    });
    fireEvent.change(search, { target: { value: "memory" } });

    expect(
      screen.queryByRole("button", { name: "Plan the agent runtime" }),
    ).not.toBeInTheDocument();
    const result = screen.getByRole("button", {
      name: "Review memory design",
    });
    fireEvent.click(result);
    expect(onSelect).toHaveBeenCalledWith("conversation-2");
  });
});
