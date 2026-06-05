import type { CapabilityClaim, SignedAgentMessage } from "@aegislink/protocol";

export type AgentStatus = "active" | "disabled" | "deleted";
export type AuditDecision = "allow" | "deny";
export type AgentRouteStatus = "queued" | "delivered";

export interface RegisteredAgent {
  id: string;
  ownerUserId: string;
  displayName: string;
  publicKeyPem: string;
  status: AgentStatus;
  metadata: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

export interface AgentCapability extends CapabilityClaim {
  id: string;
  issuerAgentId?: string;
  revokedAt?: string;
  createdAt: string;
}

export interface RoutedMessage {
  id: string;
  message: SignedAgentMessage;
  status: AgentRouteStatus;
  createdAt: string;
  deliveredAt?: string;
}

export interface AuditLogEntry {
  id: string;
  actorAgentId?: string;
  action: string;
  resource: string;
  decision: AuditDecision;
  reason: string;
  request: unknown;
  createdAt: string;
}

export interface ServerStateSnapshot {
  agents: RegisteredAgent[];
  capabilities: AgentCapability[];
  messages: RoutedMessage[];
  auditLogs: AuditLogEntry[];
}
