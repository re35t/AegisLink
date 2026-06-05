import { randomUUID } from "node:crypto";
import { describe, expect, it } from "vitest";
import { signPayload, generateAgentIdentity } from "@aegislink/crypto-identity";
import type { CapabilityClaim, SignedAgentMessage } from "@aegislink/protocol";
import { AegisLinkServer, unsignedMessage } from "../apps/server/src/aegislink-server.js";

describe("AegisLink server backend", () => {
  it("registers agents, issues capabilities, verifies signatures, routes messages, and audits allows", () => {
    const service = new AegisLinkServer();
    const senderIdentity = generateAgentIdentity("local-agent");
    const targetIdentity = generateAgentIdentity("remote-agent");

    service.registerAgent({
      id: senderIdentity.agentId,
      ownerUserId: "user-1",
      displayName: "Local Agent",
      publicKeyPem: senderIdentity.publicKeyPem,
    });
    service.registerAgent({
      id: targetIdentity.agentId,
      ownerUserId: "user-2",
      displayName: "Remote Agent",
      publicKeyPem: targetIdentity.publicKeyPem,
    });

    const capability = service.issueCapability({
      issuerAgentId: senderIdentity.agentId,
      subjectAgentId: senderIdentity.agentId,
      audienceAgentId: targetIdentity.agentId,
      actions: ["agent.message.ask"],
      resources: [`agent:${targetIdentity.agentId}`],
      scope: "team",
      expiresAt: new Date(Date.now() + 60_000).toISOString(),
    });
    const message = signedMessage(senderIdentity.privateKeyPem, {
      type: "agent.ask",
      senderAgentId: senderIdentity.agentId,
      targetAgentId: targetIdentity.agentId,
      capability,
      payload: { question: "Can you help?", purpose: "test" },
    });

    const routed = service.routeMessage(message);

    expect(routed.status).toBe("delivered");
    expect(service.listMessages(senderIdentity.agentId)).toHaveLength(1);
    expect(service.listAuditLogs().some((entry) => entry.action === "agent.message.ask" && entry.decision === "allow")).toBe(true);
  });

  it("rejects messages when capability resources do not cover the target", () => {
    const service = new AegisLinkServer();
    const senderIdentity = generateAgentIdentity("local-agent");
    const targetIdentity = generateAgentIdentity("remote-agent");

    service.registerAgent({
      id: senderIdentity.agentId,
      ownerUserId: "user-1",
      displayName: "Local Agent",
      publicKeyPem: senderIdentity.publicKeyPem,
    });
    service.registerAgent({
      id: targetIdentity.agentId,
      ownerUserId: "user-2",
      displayName: "Remote Agent",
      publicKeyPem: targetIdentity.publicKeyPem,
    });

    const capability: CapabilityClaim = {
      subjectAgentId: senderIdentity.agentId,
      audienceAgentId: targetIdentity.agentId,
      actions: ["agent.message.ask"],
      resources: ["agent:someone-else"],
      scope: "team",
      expiresAt: new Date(Date.now() + 60_000).toISOString(),
    };
    const message = signedMessage(senderIdentity.privateKeyPem, {
      type: "agent.ask",
      senderAgentId: senderIdentity.agentId,
      targetAgentId: targetIdentity.agentId,
      capability,
      payload: { question: "Can you help?", purpose: "test" },
    });

    expect(() => service.routeMessage(message)).toThrow("resource_not_granted");
    expect(service.listMessages()).toHaveLength(0);
    expect(service.listAuditLogs().some((entry) => entry.decision === "deny" && entry.reason === "resource_not_granted")).toBe(true);
  });
});

function signedMessage(
  privateKeyPem: string,
  input: Pick<SignedAgentMessage, "type" | "senderAgentId" | "targetAgentId" | "capability" | "payload">,
): SignedAgentMessage {
  const message: SignedAgentMessage = {
    messageId: randomUUID(),
    type: input.type,
    senderAgentId: input.senderAgentId,
    targetAgentId: input.targetAgentId,
    issuedAt: new Date().toISOString(),
    expiresAt: new Date(Date.now() + 60_000).toISOString(),
    nonce: randomUUID(),
    capability: input.capability,
    payload: input.payload,
    signature: "",
  };

  return {
    ...message,
    signature: signPayload({ privateKeyPem, payload: unsignedMessage(message) }),
  };
}
