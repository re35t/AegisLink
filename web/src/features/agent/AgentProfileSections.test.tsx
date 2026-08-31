import { QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  api,
  type AgentProfile,
  type AgentPublication,
  type FactCandidate,
} from "../../api/client";
import { queryClient } from "../../app/queryClient";
import { FactsSection, PublicationSection } from "./AgentProfileSections";

describe("AgentProfileSections", () => {
  afterEach(() => {
    cleanup();
    queryClient.clear();
    vi.restoreAllMocks();
  });

  it("lets the owner edit namespace, key, and JSON before confirming a Fact", async () => {
    const candidate: FactCandidate = {
      id: "candidate-one",
      subject: "agent",
      namespace: "preferences",
      key: "communicationStyle",
      value: { tone: "concise" },
      rationale: "Observed repeatedly",
      confidence: 0.9,
      version: 1,
      status: "pending",
      proposedAt: "2026-08-25T00:00:00Z",
      sourceImpressionIds: ["impression-one"],
    };
    vi.spyOn(api, "listAgentFactCandidates").mockResolvedValue([candidate]);
    const confirm = vi
      .spyOn(api, "confirmAgentFactCandidate")
      .mockResolvedValue(profileFixture());

    renderWithQuery(
      <FactsSection agentId="agent-one" profile={profileFixture()} />,
    );

    fireEvent.change(
      await screen.findByRole("textbox", { name: "Namespace" }),
      {
        target: { value: "communication" },
      },
    );
    fireEvent.change(screen.getByRole("textbox", { name: "Key" }), {
      target: { value: "responseStyle" },
    });
    fireEvent.change(
      screen.getByRole("textbox", {
        name: "preferences.communicationStyle JSON",
      }),
      { target: { value: '{"tone":"direct","detail":"compact"}' } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Confirm" }));

    await waitFor(() =>
      expect(confirm).toHaveBeenCalledWith("agent-one", "candidate-one", {
        expectedVersion: 1,
        expectedCandidateVersion: 1,
        subject: "agent",
        namespace: "communication",
        key: "responseStyle",
        value: { tone: "direct", detail: "compact" },
      }),
    );
  });

  it("revokes an active AgentFacts query token", async () => {
    vi.spyOn(api, "getAgentPublication").mockResolvedValue(
      publicationFixture(),
    );
    const revoke = vi
      .spyOn(api, "revokeAgentAccessToken")
      .mockResolvedValue(undefined);

    renderWithQuery(<PublicationSection agentId="agent-one" />);

    fireEvent.click(await screen.findByRole("button", { name: "Revoke" }));
    await waitFor(() =>
      expect(revoke).toHaveBeenCalledWith("agent-one", "token-one"),
    );
  });
});

function renderWithQuery(element: ReactNode) {
  return render(
    <QueryClientProvider client={queryClient}>{element}</QueryClientProvider>,
  );
}

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
    pendingFactCount: 1,
    createdAt: timestamp,
    updatedAt: timestamp,
  };
}

function publicationFixture(): AgentPublication {
  return {
    agentId: "agent-one",
    revision: 1,
    enabled: false,
    hostname: "",
    hostnameStatus: "unconfigured",
    dnsChallenge: "",
    ttlSeconds: 86400,
    signingKey: { available: false, status: "unavailable" },
    agentFactsReady: false,
    agentCard: {
      draft: {
        supportedInterfaces: [],
        capabilities: {},
        defaultInputModes: ["text/plain"],
        defaultOutputModes: ["text/plain"],
        name: "Aegis",
        description: "Personal Agent",
        skills: [],
        version: "draft-1",
      },
      readiness: {
        publishable: false,
        blockers: ["general-a2a-publication-disabled"],
      },
    },
    tokens: [
      {
        id: "token-one",
        label: "QA token",
        audience: "team-security",
        scopes: ["agent-facts:query"],
        createdAt: "2026-08-25T00:00:00Z",
        expiresAt: "2026-11-23T00:00:00Z",
        revoked: false,
      },
    ],
  };
}
