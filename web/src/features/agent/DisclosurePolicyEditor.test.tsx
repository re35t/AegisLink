import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { AgentProfile } from "../../api/client";
import { DisclosurePolicyEditor } from "./DisclosurePolicyEditor";

describe("DisclosurePolicyEditor", () => {
  afterEach(cleanup);

  it("collects an explicit per-item public disclosure change", () => {
    const onSave = vi.fn();
    render(
      <DisclosurePolicyEditor
        profile={profileFixture()}
        pending={false}
        error=""
        conflict={false}
        onReload={vi.fn()}
        onSave={onSave}
      />,
    );

    fireEvent.change(
      screen.getByRole("combobox", { name: "Aegis visibility" }),
      {
        target: { value: "public" },
      },
    );
    fireEvent.click(screen.getByRole("checkbox", { name: "Agent Card" }));
    fireEvent.click(screen.getByRole("button", { name: "Save disclosure" }));

    expect(onSave).toHaveBeenCalledWith([
      expect.objectContaining({
        subjectType: "identity",
        subjectId: "agent-one",
        policy: expect.objectContaining({
          visibility: "public",
          channels: expect.arrayContaining(["runtime-context", "agent-card"]),
        }),
      }),
    ]);
  });

  it("requires an audience for restricted disclosure", () => {
    render(
      <DisclosurePolicyEditor
        profile={profileFixture()}
        pending={false}
        error=""
        conflict={false}
        onReload={vi.fn()}
        onSave={vi.fn()}
      />,
    );

    fireEvent.change(
      screen.getByRole("combobox", { name: "Aegis visibility" }),
      {
        target: { value: "restricted" },
      },
    );
    expect(
      screen.getByRole("button", { name: "Save disclosure" }),
    ).toBeDisabled();
    fireEvent.change(screen.getByRole("textbox", { name: "Audiences" }), {
      target: { value: "team-security" },
    });
    expect(
      screen.getByRole("button", { name: "Save disclosure" }),
    ).toBeEnabled();
  });
});

function profileFixture(): AgentProfile {
  const timestamp = "2026-08-25T00:00:00Z";
  return {
    agentId: "agent-one",
    version: 1,
    contextRevision: 1,
    identity: {
      id: "agent-one",
      name: "Aegis",
      description: "Personal Agent",
      avatarUrl: "",
      humanLinked: true,
      disclosure: {
        visibility: "private",
        channels: ["runtime-context"],
        indexable: false,
        audiences: [],
      },
    },
    capabilities: [],
    endpoints: [],
    confirmedFacts: [],
    impressions: [],
    pendingFactCount: 0,
    createdAt: timestamp,
    updatedAt: timestamp,
  };
}
