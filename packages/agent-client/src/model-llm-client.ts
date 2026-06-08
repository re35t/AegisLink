import type { LlmClient } from "./types.js";

export interface ModelLlmClientOptions {
  apiKey: string;
  model: string;
  baseUrl?: string;
  timeoutMs?: number;
  fetch?: typeof fetch;
}

interface ChatCompletionResponse {
  choices?: Array<{
    message?: {
      content?: unknown;
    };
  }>;
}

export class ModelProviderError extends Error {
  constructor(
    message: string,
    public readonly statusCode?: number,
  ) {
    super(message);
  }
}

export class ModelLlmClient implements LlmClient {
  private readonly baseUrl: string;
  private readonly timeoutMs: number;
  private readonly fetchFn: typeof fetch;

  constructor(private readonly options: ModelLlmClientOptions) {
    if (!options.apiKey.trim()) {
      throw new ModelProviderError("MODEL_API_KEY is required");
    }
    if (!options.model.trim()) {
      throw new ModelProviderError("MODEL_NAME is required");
    }
    this.baseUrl = trimTrailingSlash(options.baseUrl ?? "https://api.openai.com/v1");
    this.timeoutMs = options.timeoutMs ?? 30_000;
    this.fetchFn = options.fetch ?? globalThis.fetch;
  }

  async complete(input: { prompt: string; temperature?: number; maxTokens?: number }): Promise<string> {
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), this.timeoutMs);

    try {
      const response = await this.fetchFn(`${this.baseUrl}/chat/completions`, {
        method: "POST",
        headers: {
          authorization: `Bearer ${this.options.apiKey}`,
          "content-type": "application/json",
        },
        body: JSON.stringify({
          model: this.options.model,
          messages: [{ role: "user", content: input.prompt }],
          temperature: input.temperature ?? 0.3,
          max_tokens: input.maxTokens,
        }),
        signal: controller.signal,
      });

      const body = await readJson(response);
      if (!response.ok) {
        throw new ModelProviderError(providerErrorMessage(body) ?? `model provider returned ${response.status}`, response.status);
      }

      const content = (body as ChatCompletionResponse).choices?.[0]?.message?.content;
      const text = normalizeContent(content);
      if (!text) {
        throw new ModelProviderError("model provider response did not include assistant content", response.status);
      }
      return text;
    } catch (error) {
      if (error instanceof ModelProviderError) {
        throw error;
      }
      if (error instanceof Error && error.name === "AbortError") {
        throw new ModelProviderError("model provider request timed out");
      }
      throw new ModelProviderError(error instanceof Error ? error.message : "model provider request failed");
    } finally {
      clearTimeout(timeout);
    }
  }
}

async function readJson(response: Response): Promise<unknown> {
  try {
    return await response.json();
  } catch {
    return {};
  }
}

function normalizeContent(content: unknown): string {
  if (typeof content === "string") {
    return content.trim();
  }
  if (Array.isArray(content)) {
    return content
      .map((part) => {
        if (typeof part === "string") {
          return part;
        }
        if (part && typeof part === "object" && "text" in part && typeof part.text === "string") {
          return part.text;
        }
        return "";
      })
      .join("")
      .trim();
  }
  return "";
}

function providerErrorMessage(body: unknown): string | undefined {
  if (!body || typeof body !== "object" || !("error" in body)) {
    return undefined;
  }
  const error = body.error;
  if (typeof error === "string") {
    return error;
  }
  if (error && typeof error === "object" && "message" in error && typeof error.message === "string") {
    return error.message;
  }
  return undefined;
}

function trimTrailingSlash(value: string): string {
  return value.replace(/\/+$/, "");
}
