import { createPrivateKey, createPublicKey, sign, verify } from "node:crypto";

export function canonicalJson(value: unknown): string {
  if (value === null || typeof value !== "object") {
    return JSON.stringify(value);
  }

  if (Array.isArray(value)) {
    return `[${value.map((item) => canonicalJson(item)).join(",")}]`;
  }

  const entries = Object.entries(value as Record<string, unknown>).sort(([left], [right]) =>
    left.localeCompare(right),
  );
  return `{${entries.map(([key, entryValue]) => `${JSON.stringify(key)}:${canonicalJson(entryValue)}`).join(",")}}`;
}

export function signPayload(input: { privateKeyPem: string; payload: unknown }): string {
  const privateKey = createPrivateKey(input.privateKeyPem);
  const signature = sign(null, Buffer.from(canonicalJson(input.payload)), privateKey);
  return signature.toString("base64url");
}

export function verifyPayload(input: { publicKeyPem: string; payload: unknown; signature: string }): boolean {
  const publicKey = createPublicKey(input.publicKeyPem);
  return verify(
    null,
    Buffer.from(canonicalJson(input.payload)),
    publicKey,
    Buffer.from(input.signature, "base64url"),
  );
}
