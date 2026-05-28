import { generateKeyPairSync, KeyObject } from "node:crypto";

export interface AgentIdentity {
  agentId: string;
  publicKeyPem: string;
  privateKeyPem: string;
}

export function generateAgentIdentity(agentId: string): AgentIdentity {
  const { publicKey, privateKey } = generateKeyPairSync("ed25519");
  return {
    agentId,
    publicKeyPem: publicKey.export({ type: "spki", format: "pem" }).toString(),
    privateKeyPem: privateKey.export({ type: "pkcs8", format: "pem" }).toString(),
  };
}

export function assertKeyObject(value: KeyObject): KeyObject {
  return value;
}
