import { readFile } from "node:fs/promises";
import { createServer, type IncomingMessage, type ServerResponse } from "node:http";
import { extname, join } from "node:path";
import { fileURLToPath } from "node:url";
import type { SignedAgentMessage } from "@aegislink/protocol";
import { AegisLinkServer, HttpError, type RegisterAgentInput, ValidationError } from "./aegislink-server.js";
import type { ChatService } from "./chat-service.js";
import type { AgentStatus } from "./types.js";

export interface CreateHttpServerOptions {
  service: AegisLinkServer;
  chat?: ChatService;
  publicDir?: string;
}

const defaultPublicDir = fileURLToPath(new URL("../public", import.meta.url));

export function createAegisLinkHttpServer(options: CreateHttpServerOptions) {
  const publicDir = options.publicDir ?? defaultPublicDir;

  return createServer(async (request, response) => {
    try {
      await handleRequest(options.service, options.chat, publicDir, request, response);
    } catch (error) {
      sendError(response, error);
    }
  });
}

async function handleRequest(
  service: AegisLinkServer,
  chat: ChatService | undefined,
  publicDir: string,
  request: IncomingMessage,
  response: ServerResponse,
): Promise<void> {
  const url = new URL(request.url ?? "/", "http://localhost");
  if (url.pathname === "/healthz") {
    sendJson(response, 200, { ok: true });
    return;
  }

  if (url.pathname.startsWith("/api/")) {
    await handleApi(service, chat, request, response, url);
    return;
  }

  await serveStatic(publicDir, url.pathname, response);
}

async function handleApi(
  service: AegisLinkServer,
  chat: ChatService | undefined,
  request: IncomingMessage,
  response: ServerResponse,
  url: URL,
): Promise<void> {
  if (request.method === "POST" && url.pathname === "/api/chat") {
    if (!chat) {
      throw new HttpError(501, "chat_service_not_configured");
    }
    const body = assertObject(await readJson(request));
    sendJson(response, 200, await chat.ask({
      question: assertString(body.question, "question"),
      agentId: optionalString(body.agentId),
      conversationId: optionalString(body.conversationId),
      useMemory: optionalBoolean(body.useMemory, "useMemory"),
    }));
    return;
  }

  if (request.method === "GET" && url.pathname === "/api/agents") {
    sendJson(response, 200, {
      agents: service.listAgents({
        query: url.searchParams.get("query") ?? undefined,
        ownerUserId: url.searchParams.get("ownerUserId") ?? undefined,
      }),
    });
    return;
  }

  if (request.method === "POST" && url.pathname === "/api/agents") {
    const body = await readJson(request);
    sendJson(response, 201, { agent: service.registerAgent(assertRegisterAgentInput(body)) });
    return;
  }

  const agentStatusMatch = url.pathname.match(/^\/api\/agents\/([^/]+)\/status$/);
  if (request.method === "PATCH" && agentStatusMatch) {
    const body = assertObject(await readJson(request));
    const status = assertAgentStatus(body.status);
    sendJson(response, 200, { agent: service.updateAgentStatus(decodeURIComponent(agentStatusMatch[1] ?? ""), status) });
    return;
  }

  if (request.method === "GET" && url.pathname === "/api/capabilities") {
    sendJson(response, 200, { capabilities: service.listCapabilities(url.searchParams.get("agentId") ?? undefined) });
    return;
  }

  if (request.method === "POST" && url.pathname === "/api/capabilities") {
    const body = assertObject(await readJson(request));
    sendJson(response, 201, { capability: service.issueCapability({
      issuerAgentId: optionalString(body.issuerAgentId),
      subjectAgentId: assertString(body.subjectAgentId, "subjectAgentId"),
      audienceAgentId: optionalString(body.audienceAgentId),
      actions: assertStringArray(body.actions, "actions"),
      resources: assertStringArray(body.resources, "resources"),
      scope: assertScope(body.scope),
      expiresAt: assertString(body.expiresAt, "expiresAt"),
    }) });
    return;
  }

  const revokeCapabilityMatch = url.pathname.match(/^\/api\/capabilities\/([^/]+)\/revoke$/);
  if (request.method === "POST" && revokeCapabilityMatch) {
    sendJson(response, 200, { capability: service.revokeCapability(decodeURIComponent(revokeCapabilityMatch[1] ?? "")) });
    return;
  }

  if (request.method === "GET" && url.pathname === "/api/messages") {
    sendJson(response, 200, { messages: service.listMessages(url.searchParams.get("agentId") ?? undefined) });
    return;
  }

  if (request.method === "POST" && url.pathname === "/api/messages/route") {
    const body = assertObject(await readJson(request)) as unknown as SignedAgentMessage;
    sendJson(response, 202, { message: service.routeMessage(body) });
    return;
  }

  if (request.method === "GET" && url.pathname === "/api/audit") {
    sendJson(response, 200, { auditLogs: service.listAuditLogs() });
    return;
  }

  if (request.method === "GET" && url.pathname === "/api/snapshot") {
    sendJson(response, 200, service.snapshot());
    return;
  }

  throw new HttpError(404, "route_not_found");
}

async function serveStatic(publicDir: string, pathname: string, response: ServerResponse): Promise<void> {
  const normalizedPath = pathname === "/" ? "/index.html" : pathname;
  if (normalizedPath.includes("..")) {
    throw new HttpError(400, "invalid_static_path");
  }

  try {
    const filePath = join(publicDir, normalizedPath);
    const body = await readFile(filePath);
    response.writeHead(200, { "content-type": contentTypeFor(filePath) });
    response.end(body);
  } catch {
    const body = await readFile(join(publicDir, "index.html"));
    response.writeHead(200, { "content-type": "text/html; charset=utf-8" });
    response.end(body);
  }
}

async function readJson(request: IncomingMessage): Promise<unknown> {
  const chunks: Buffer[] = [];
  for await (const chunk of request) {
    chunks.push(Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk));
  }
  if (chunks.length === 0) {
    return {};
  }
  try {
    return JSON.parse(Buffer.concat(chunks).toString("utf8"));
  } catch {
    throw new ValidationError("request body must be valid JSON");
  }
}

function sendJson(response: ServerResponse, statusCode: number, body: unknown): void {
  response.writeHead(statusCode, { "content-type": "application/json; charset=utf-8" });
  response.end(JSON.stringify(body, null, 2));
}

function sendError(response: ServerResponse, error: unknown): void {
  const statusCode = error instanceof HttpError ? error.statusCode : 500;
  const message = error instanceof Error ? error.message : "internal_server_error";
  sendJson(response, statusCode, { error: message });
}

function assertObject(value: unknown): Record<string, unknown> {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new ValidationError("request body must be an object");
  }
  return value as Record<string, unknown>;
}

function assertRegisterAgentInput(value: unknown): RegisterAgentInput {
  const body = assertObject(value);
  return {
    id: optionalString(body.id),
    ownerUserId: assertString(body.ownerUserId, "ownerUserId"),
    displayName: assertString(body.displayName, "displayName"),
    publicKeyPem: assertString(body.publicKeyPem, "publicKeyPem"),
    metadata: assertMetadata(body.metadata),
  };
}

function assertString(value: unknown, field: string): string {
  if (typeof value !== "string" || value.trim().length === 0) {
    throw new ValidationError(`${field} must be a non-empty string`);
  }
  return value;
}

function optionalString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim().length > 0 ? value : undefined;
}

function optionalBoolean(value: unknown, field: string): boolean | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (typeof value !== "boolean") {
    throw new ValidationError(`${field} must be a boolean`);
  }
  return value;
}

function assertStringArray(value: unknown, field: string): string[] {
  if (!Array.isArray(value) || value.some((item) => typeof item !== "string" || item.trim().length === 0)) {
    throw new ValidationError(`${field} must be an array of strings`);
  }
  return value;
}

function assertScope(value: unknown) {
  if (value === "self" || value === "team" || value === "org" || value === "public") {
    return value;
  }
  throw new ValidationError("scope must be self, team, org, or public");
}

function assertAgentStatus(value: unknown): AgentStatus {
  if (value === "active" || value === "disabled" || value === "deleted") {
    return value;
  }
  throw new ValidationError("status must be active, disabled, or deleted");
}

function assertMetadata(value: unknown): Record<string, unknown> | undefined {
  if (value === undefined) {
    return undefined;
  }
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    throw new ValidationError("metadata must be an object");
  }
  return value as Record<string, unknown>;
}

function contentTypeFor(filePath: string): string {
  switch (extname(filePath)) {
    case ".css":
      return "text/css; charset=utf-8";
    case ".js":
      return "text/javascript; charset=utf-8";
    case ".json":
      return "application/json; charset=utf-8";
    default:
      return "text/html; charset=utf-8";
  }
}
