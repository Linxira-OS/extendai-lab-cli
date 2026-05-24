/**
 * @extendai/kernel — Session / context management
 *
 * Maintains message history with automatic context window trimming.
 * Approximates token usage at ~3.5 chars/token for estimation.
 * The provider's actual token count (from usage API) is more accurate.
 */

import type { Message } from './types.js';

const CHARS_PER_TOKEN = 3.5;

export class Session {
  private messages: Message[] = [];
  private contextLength: number;

  constructor(systemPrompt: string, contextLength: number) {
    this.contextLength = contextLength;
    this.messages.push({ role: 'system', content: systemPrompt });
  }

  /** Add a message and auto-trim if needed */
  addMessage(msg: Message): void {
    this.messages.push(msg);
    this.trimContext();
  }

  /** Get all current messages */
  getMessages(): Message[] {
    return [...this.messages];
  }

  /** Clear all messages except the system prompt */
  clear(): void {
    const system = this.messages[0];
    this.messages = system ? [system] : [];
  }

  /** Replace the entire message list (e.g., after a restore) */
  setMessages(messages: Message[]): void {
    this.messages = messages;
  }

  /** Estimated current token count */
  get estimatedTokens(): number {
    const totalChars = this.messages.reduce((sum, m) => sum + m.content.length, 0);
    return Math.ceil(totalChars / CHARS_PER_TOKEN);
  }

  /** Count of non-system messages */
  get messageCount(): number {
    return this.messages.length - 1;
  }

  private trimContext(): void {
    if (this.messages.length <= 1) return;

    let totalChars = this.messages.reduce((sum, m) => sum + m.content.length, 0);
    let estimatedTokens = Math.ceil(totalChars / CHARS_PER_TOKEN);

    // Remove oldest user/assistant pairs until we're under the limit
    // Always keep system prompt at index 0
    while (estimatedTokens > this.contextLength && this.messages.length > 1) {
      // Remove the oldest non-system message
      this.messages.splice(1, 1);
      totalChars = this.messages.reduce((sum, m) => sum + m.content.length, 0);
      estimatedTokens = Math.ceil(totalChars / CHARS_PER_TOKEN);
    }
  }
}
