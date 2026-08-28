import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { api, type Agent } from "../../api/client";
import { AgentSetupPage } from "./AgentSetupPage";

describe("AgentSetupPage", () => {
  it("saves the public identity before entering the workspace", async () => {
    const configure = vi
      .spyOn(api, "configureAgent")
      .mockResolvedValue({} as never);
    const configured = vi.fn();
    const queryClient = new QueryClient({
      defaultOptions: { mutations: { retry: false } },
    });
    const agent: Agent = {
      id: "agent-one",
      name: "Aegis",
      description: "Personal Agent",
      createdAt: "2026-08-28T00:00:00Z",
      updatedAt: "2026-08-28T00:00:00Z",
    };

    render(
      <QueryClientProvider client={queryClient}>
        <AgentSetupPage
          agent={agent}
          onConfigured={configured}
          onSignedOut={vi.fn()}
        />
      </QueryClientProvider>,
    );

    fireEvent.change(screen.getByRole("textbox", { name: "Agent name" }), {
      target: { value: "Atlas" },
    });
    fireEvent.change(
      screen.getByRole("textbox", {
        name: "What is this Agent mainly good at?",
      }),
      { target: { value: "Go systems and API design" } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Save profile and register Agent" }),
    );

    await waitFor(() => expect(configured).toHaveBeenCalledOnce());
    expect(configure).toHaveBeenCalledWith("agent-one", {
      name: "Atlas",
      primaryFocus: "Go systems and API design",
    });
  });
});
