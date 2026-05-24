/**
 * @extendai/kernel — Core type definitions
 */

export type Role = 'system' | 'user' | 'assistant';

/** A single message in the conversation */
export interface Message {
  role: Role;
  content: string;
}

/** Token usage reported by the provider API */
export interface Usage {
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
}

/** A single chunk from the streaming response */
export interface StreamChunk {
  type: 'content' | 'done' | 'error';
  content?: string;
  error?: string;
  usage?: Usage;
}

/** Provider-level configuration (one provider) */
export interface ProviderConfig {
  name: string;
  type: string;
  apiKey: string;
  baseUrl: string;
  model: string;
  maxTokens: number;
  contextLength: number;
  systemPrompt: string;
  temperature: number;
}

/** Full application configuration */
export interface AppConfig {
  provider: ProviderConfig;
}
