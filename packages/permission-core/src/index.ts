import type { CapabilityClaim } from "@aegislink/protocol";

export type PermissionDecision = "allow" | "deny";

export interface PermissionCheckInput {
  capability: CapabilityClaim;
  action: string;
  resource: string;
  now?: Date;
}

export interface PermissionCheckResult {
  decision: PermissionDecision;
  reason: string;
}

export function evaluateCapability(input: PermissionCheckInput): PermissionCheckResult {
  const now = input.now ?? new Date();
  if (new Date(input.capability.expiresAt).getTime() <= now.getTime()) {
    return { decision: "deny", reason: "capability_expired" };
  }
  if (!input.capability.actions.includes(input.action)) {
    return { decision: "deny", reason: "action_not_granted" };
  }
  if (!input.capability.resources.includes(input.resource) && !input.capability.resources.includes("*")) {
    return { decision: "deny", reason: "resource_not_granted" };
  }
  return { decision: "allow", reason: "capability_allows" };
}
