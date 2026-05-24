/**
 * @extendai/kernel — OpenAI-compatible provider with streaming
 *
 * Supports any OpenAI-compatible API endpoint (OpenAI, Anthropic via proxy,
 * Ollama, vLLM, Together, Groq, etc.)
 */

import type { Message, StreamChunk, ProviderConfig } from './types.js';

/**
 * Stream a chat completion from an OpenAI-compatible API.
 *
 * Usage:
 * ```ts
 * for await (const chunk of streamCompletion(messages, config)) {
 *   if (chunk.type === 'content') process.stdout.write(chunk.content);
 * }
 * ```
 */
export async function* streamCompletion(
  messages: Message[],
  config: ProviderConfig,
  signal?: AbortSignal,
): AsyncGenerator<StreamChunk> {
  const baseUrl = config.baseUrl.replace(/\/+$/, '');
  const url = `${baseUrl}/chat/completions`;

  const response = await fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${config.apiKey}`,
    },
    body: JSON.stringify({
      model: config.model,
      messages,
      max_tokens: config.maxTokens,
      temperature: config.temperature,
      stream: true,
    }),
    signal,
  });

  if (!response.ok) {
    let errorBody = '';
    try {
      errorBody = await response.text();
    } catch {
      errorBody = 'Unknown error';
    }
    yield { type: 'error', error: `API error ${response.status}: ${errorBody.slice(0, 500)}` };
    return;
  }

  const reader = response.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  let usage: Usage | undefined;

  while (true) {
    const { done, value } = await reader.read();
    if (done) break;

    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';

    for (const line of lines) {
      const trimmed = line.trim();
      if (!trimmed || !trimmed.startsWith('data: ')) continue;

      const data = trimmed.slice(6); // Remove 'data: ' prefix
      if (data === '[DONE]') {
        // Final chunk — flush any remaining usage
        yield { type: 'done', usage };
        return;
      }

      try {
        const parsed = JSON.parse(data);
        const choice = parsed.choices?.[0];

        // Accumulate usage from the last chunk (some providers include it here)
        if (parsed.usage) {
          usage = {
            promptTokens: parsed.usage.prompt_tokens ?? 0,
            completionTokens: parsed.usage.completion_tokens ?? 0,
            totalTokens: parsed.usage.total_tokens ?? 0,
          };
        }

        if (choice?.delta?.content) {
          yield { type: 'content', content: choice.delta.content };
        }

        // Finish reasons
        if (choice?.finish_reason === 'stop' || choice?.finish_reason === 'length') {
          yield { type: 'done', usage };
          return;
        }
      } catch {
        // Skip malformed JSON lines (some providers send non-JSON after [DONE])
      }
    }
  }

  yield { type: 'done', usage };
}

interface Usage {
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
}
