import { randomUUID } from "node:crypto";
import { verifyPayload } from "@aegislink/crypto-identity";
import { evaluateCapability } from "@aegislink/permission-core";
import type { AgentMessageType, CapabilityClaim, SignedAgentMessage } from "@aegislink/protocol";
import type { AgentCapability, AuditLogEntry, RegisteredAgent, RoutedMessage, ServerStateSnapshot } from "./types.js";

export interface RegisterAgentInput {
  id?: string;
  ownerUserId: string;
  displayName: string;
  publicKeyPem: string;
  metadata?: Record<string, unknown>;
}

export interface IssueCapabilityInput {
  issuerAgentId?: string;
  subjectAgentId: string;
  audienceAgentId?: string;
  actions: string[];
  resources: string[];
  scope: CapabilityClaim["scope"];
  expiresAt: string;
}

export interface AgentSearchInput {
  query?: string;
  ownerUserId?: string;
  status?: RegisteredAgent["status"];
}

export class AegisLinkServer {
  private readonly agents = new Map<string, RegisteredAgent>();
  private readonly capabilities = new Map<string, AgentCapability>();
  private readonly routedMessages: RoutedMessage[] = [];
  private readonly auditLogs: AuditLogEntry[] = [];

  registerAgent(input: RegisterAgentInput): RegisteredAgent {
    const now = new Date().toISOString();
    const id = input.id ?? randomUUID();
    if (this.agents.has(id)) {
      throw new ConflictError(`agent '${id}' already exists`);
    }

    const agent: RegisteredAgent = {
      id,
      ownerUserId: input.ownerUserId,
      displayName: input.displayName,
      publicKeyPem: input.publicKeyPem,
      status: "active",
      metadata: input.metadata ?? {},
      createdAt: now,
      updatedAt: now,
    };
    this.agents.set(agent.id, agent);
    this.audit({
      actorAgentId: agent.id,
      action: "agent.register",
      resource: `agent:${agent.id}`,
      decision: "allow",
      reason: "agent_registered",
      request: { ownerUserId: agent.ownerUserId, displayName: agent.displayName },
    });
    return agent;
  }

  listAgents(input: AgentSearchInput = {}): RegisteredAgent[] {
    const query = input.query?.trim().toLowerCase();
    return [...this.agents.values()]
      .filter((agent) => (input.status ? agent.status === input.status : agent.status !== "deleted"))
      .filter((agent) => (input.ownerUserId ? agent.ownerUserId === input.ownerUserId : true))
      .filter((agent) => {
        if (!query) {
          return true;
        }
        return (
          agent.id.toLowerCase().includes(query) ||
          agent.displayName.toLowerCase().includes(query) ||
          JSON.stringify(agent.metadata).toLowerCase().includes(query)
        );
      })
      .sort((left, right) => left.createdAt.localeCompare(right.createdAt));
  }

  getAgent(agentId: string): RegisteredAgent {
    const agent = this.agents.get(agentId);
    if (!agent || agent.status === "deleted") {
      throw new NotFoundError(`agent '${agentId}' was not found`);
    }
    return agent;
  }

  updateAgentStatus(agentId: string, status: RegisteredAgent["status"]): RegisteredAgent {
    const agent = this.getAgent(agentId);
    const updated = { ...agent, status, updatedAt: new Date().toISOString() };
    this.agents.set(agentId, updated);
    this.audit({
      actorAgentId: agentId,
      action: "agent.status.update",
      resource: `agent:${agentId}`,
      decision: "allow",
      reason: "agent_status_updated",
      request: { status },
    });
    return updated;
  }

  issueCapability(input: IssueCapabilityInput): AgentCapability {
    this.assertActiveAgent(input.subjectAgentId);
    if (input.issuerAgentId) {
      this.assertActiveAgent(input.issuerAgentId);
    }
    if (input.audienceAgentId) {
      this.assertActiveAgent(input.audienceAgentId);
    }
    if (new Date(input.expiresAt).getTime() <= Date.now()) {
      throw new ValidationError("capability expiresAt must be in the future");
    }
    if (input.actions.length === 0 || input.resources.length === 0) {
      throw new ValidationError("capability requires at least one action and one resource");
    }

    const capability: AgentCapability = {
      id: randomUUID(),
      issuerAgentId: input.issuerAgentId,
      subjectAgentId: input.subjectAgentId,
      audienceAgentId: input.audienceAgentId,
      actions: [...new Set(input.actions)],
      resources: [...new Set(input.resources)],
      scope: input.scope,
      expiresAt: input.expiresAt,
      createdAt: new Date().toISOString(),
    };
    this.capabilities.set(capability.id, capability);
    this.audit({
      actorAgentId: input.issuerAgentId,
      action: "capability.issue",
      resource: `capability:${capability.id}`,
      decision: "allow",
      reason: "capability_issued",
      request: capability,
    });
    return capability;
  }

  listCapabilities(agentId?: string): AgentCapability[] {
    return [...this.capabilities.values()]
      .filter((capability) => (agentId ? capability.subjectAgentId === agentId || capability.audienceAgentId === agentId : true))
      .sort((left, right) => left.createdAt.localeCompare(right.createdAt));
  }

  revokeCapability(capabilityId: string): AgentCapability {
    const capability = this.capabilities.get(capabilityId);
    if (!capability) {
      throw new NotFoundError(`capability '${capabilityId}' was not found`);
    }
    const updated = { ...capability, revokedAt: new Date().toISOString() };
    this.capabilities.set(capabilityId, updated);
    this.audit({
      actorAgentId: capability.issuerAgentId,
      action: "capability.revoke",
      resource: `capability:${capabilityId}`,
      decision: "allow",
      reason: "capability_revoked",
      request: { capabilityId },
    });
    return updated;
  }

  routeMessage(message: SignedAgentMessage): RoutedMessage {
    const resource = message.targetAgentId ? `agent:${message.targetAgentId}` : "agent:*";
    const action = actionForMessageType(message.type);
    const sender = this.getAgent(message.senderAgentId);
    const target = message.targetAgentId ? this.getAgent(message.targetAgentId) : undefined;

    try {
      this.assertMessageWindow(message);
      if (message.capability.subjectAgentId !== message.senderAgentId) {
        throw new PermissionError("capability_subject_mismatch");
      }
      if (target && message.capability.audienceAgentId && message.capability.audienceAgentId !== target.id) {
        throw new PermissionError("capability_audience_mismatch");
      }
      if (this.isCapabilityRevoked(message.capability)) {
        throw new PermissionError("capability_revoked");
      }

      const signaturePayload = unsignedMessage(message);
      if (!verifyPayload({ publicKeyPem: sender.publicKeyPem, payload: signaturePayload, signature: message.signature })) {
        throw new PermissionError("invalid_signature");
      }

      const decision = evaluateCapability({ capability: message.capability, action, resource });
      if (decision.decision === "deny") {
        throw new PermissionError(decision.reason);
      }

      const routed: RoutedMessage = {
        id: randomUUID(),
        message,
        status: "delivered",
        createdAt: new Date().toISOString(),
        deliveredAt: new Date().toISOString(),
      };
      this.routedMessages.push(routed);
      this.audit({
        actorAgentId: message.senderAgentId,
        action,
        resource,
        decision: "allow",
        reason: "message_routed",
        request: { messageId: message.messageId, type: message.type, targetAgentId: message.targetAgentId },
      });
      return routed;
    } catch (error) {
      this.audit({
        actorAgentId: message.senderAgentId,
        action,
        resource,
        decision: "deny",
        reason: error instanceof Error ? error.message : "message_rejected",
        request: { messageId: message.messageId, type: message.type, targetAgentId: message.targetAgentId },
      });
      throw error;
    }
  }

  listMessages(agentId?: string): RoutedMessage[] {
    return this.routedMessages
      .filter((entry) => (agentId ? entry.message.senderAgentId === agentId || entry.message.targetAgentId === agentId : true))
      .sort((left, right) => left.createdAt.localeCompare(right.createdAt));
  }

  listAuditLogs(): AuditLogEntry[] {
    return [...this.auditLogs].sort((left, right) => left.createdAt.localeCompare(right.createdAt));
  }

  snapshot(): ServerStateSnapshot {
    return {
      agents: this.listAgents({ status: undefined }),
      capabilities: this.listCapabilities(),
      messages: this.listMessages(),
      auditLogs: this.listAuditLogs(),
    };
  }

  private assertActiveAgent(agentId: string): void {
    const agent = this.getAgent(agentId);
    if (agent.status !== "active") {
      throw new PermissionError("agent_not_active");
    }
  }

  private assertMessageWindow(message: SignedAgentMessage): void {
    const now = Date.now();
    if (new Date(message.issuedAt).getTime() > now + 30_000) {
      throw new PermissionError("message_issued_in_future");
    }
    if (new Date(message.expiresAt).getTime() <= now) {
      throw new PermissionError("message_expired");
    }
  }

  private isCapabilityRevoked(claim: CapabilityClaim): boolean {
    return [...this.capabilities.values()].some((capability) => {
      if (!capability.revokedAt || capability.subjectAgentId !== claim.subjectAgentId) {
        return false;
      }
      return (
        capability.audienceAgentId === claim.audienceAgentId &&
        capability.scope === claim.scope &&
        sameSet(capability.actions, claim.actions) &&
        sameSet(capability.resources, claim.resources) &&
        capability.expiresAt === claim.expiresAt
      );
    });
  }

  private audit(input: Omit<AuditLogEntry, "id" | "createdAt">): void {
    this.auditLogs.push({
      id: randomUUID(),
      createdAt: new Date().toISOString(),
      ...input,
    });
  }
}

export function actionForMessageType(type: AgentMessageType): string {
  switch (type) {
    case "agent.ask":
      return "agent.message.ask";
    case "agent.notify":
      return "agent.message.notify";
    case "agent.discovery":
      return "agent.discovery";
    case "agent.response":
      return "agent.message.respond";
  }
}

export function unsignedMessage(message: SignedAgentMessage): Omit<SignedAgentMessage, "signature"> {
  const { signature: _signature, ...payload } = message;
  return payload;
}

function sameSet(left: string[], right: string[]): boolean {
  return left.length === right.length && left.every((item) => right.includes(item));
}

export class HttpError extends Error {
  constructor(
    public readonly statusCode: number,
    message: string,
  ) {
    super(message);
  }
}

export class ValidationError extends HttpError {
  constructor(message: string) {
    super(400, message);
  }
}

export class PermissionError extends HttpError {
  constructor(message: string) {
    super(403, message);
  }
}

export class NotFoundError extends HttpError {
  constructor(message: string) {
    super(404, message);
  }
}

export class ConflictError extends HttpError {
  constructor(message: string) {
    super(409, message);
  }
}
