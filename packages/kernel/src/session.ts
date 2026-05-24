/**
 * @extendai/kernel — Session tree & undo
 *
 * Session lifecycle:
 *   1. Created with system prompt → unnamed
 *   2. After 3 turns (user+assistant pairs) → auto-named (HHmm-branch)
 *   3. Undo checkpoints created before each user message
 *   4. /undo restores session to the previous checkpoint
 *
 * Multiple sessions form a tree: a new session can fork from any
 * existing session, inheriting the messages up to that point.
 */

import type { Message, UndoCheckpoint, SessionMeta } from './types.js';
import {
  generateSessionName,
  generateSessionId,
  formatSessionName,
  type NamerOptions,
} from './namer.js';

const CHARS_PER_TOKEN = 3.5;

export class Session {
  private messages: Message[] = [];
  private contextLength: number;
  private systemPrompt: string;

  // ── Tree/undo state ────────────────────────────────────

  /** Session identity (immutable after construction) */
  readonly meta: SessionMeta;
  /** Child sessions forked from this one */
  readonly children: Session[] = [];
  /** Undo checkpoint stack (FILO) */
  private checkpoints: UndoCheckpoint[] = [];
  /** Counter for checkpoint IDs */
  private checkpointSeq = 0;

  constructor(systemPrompt: string, contextLength: number, opts?: NamerOptions) {
    this.systemPrompt = systemPrompt;
    this.contextLength = contextLength;
    this.messages.push({ role: 'system', content: systemPrompt });

    this.meta = {
      name: '',
      id: generateSessionId(opts?.worktreeId || ''),
      parentId: null,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      turnCount: 0,
    };
  }

  // ── Message management ──────────────────────────────────

  addMessage(msg: Message): void {
    this.messages.push(msg);

    if (msg.role === 'assistant') {
      this.meta.turnCount++;
      this.meta.updatedAt = new Date().toISOString();
    }
  }

  getMessages(): Message[] {
    return [...this.messages];
  }

  clear(): void {
    const sys = this.messages[0];
    this.messages = sys ? [sys] : [];
    this.meta.turnCount = 0;
    this.meta.updatedAt = new Date().toISOString();
  }

  setMessages(msgs: Message[]): void {
    this.messages = msgs;

    // Recalculate turn count
    let count = 0;
    for (const m of msgs) {
      if (m.role === 'assistant') count++;
    }
    this.meta.turnCount = count;
    this.meta.updatedAt = new Date().toISOString();
  }

  get estimatedTokens(): number {
    const total = this.messages.reduce((s, m) => s + m.content.length, 0);
    return Math.ceil(total / CHARS_PER_TOKEN);
  }

  get messageCount(): number {
    return this.messages.length - 1; // minus system
  }

  /** Attempt auto-naming. Returns the name or null. */
  tryAutoName(opts: NamerOptions): string | null {
    const name = generateSessionName(this.meta.turnCount, opts);
    if (name) {
      this.meta.name = name;
    }
    return name;
  }

  get displayName(): string {
    return formatSessionName(this.meta.name || null, this.meta.turnCount);
  }

  // ── Undo ────────────────────────────────────────────────

  /**
   * Save an undo checkpoint BEFORE processing a user message.
   * This captures the current message state, so /undo can roll back to it.
   */
  saveCheckpoint(label: string = ''): void {
    this.checkpoints.push({
      id: ++this.checkpointSeq,
      messages: this.messages.map(m => ({ ...m })),
      timestamp: new Date().toISOString(),
      label,
    });
  }

  /**
   * Undo: restore the most recent checkpoint (removing the last user+assistant pair).
   * Returns the restored messages or null if no checkpoint exists.
   */
  undo(): Message[] | null {
    const cp = this.checkpoints.pop();
    if (!cp) return null;

    this.messages = cp.messages;

    // Recalculate turn count
    let count = 0;
    for (const m of cp.messages) {
      if (m.role === 'assistant') count++;
    }
    this.meta.turnCount = count;
    this.meta.updatedAt = new Date().toISOString();

    return this.getMessages();
  }

  /** Number of available undo steps */
  get undoCount(): number {
    return this.checkpoints.length;
  }

  // ── Fork (session tree) ─────────────────────────────────

  /**
   * Fork a child session from this one.
   * The child inherits all current messages and can branch independently.
   */
  fork(contextLength?: number): Session {
    const child = new Session(this.systemPrompt, contextLength ?? this.contextLength);
    // Copy all messages (deep clone)
    child.setMessages(this.messages.map(m => ({ ...m })));
    // Link parent
    (child.meta as any).parentId = this.meta.id;
    this.children.push(child);
    return child;
  }

  // ── Trim context ────────────────────────────────────────

  private trimContext(): void {
    if (this.messages.length <= 1) return;

    let total = this.messages.reduce((s, m) => s + m.content.length, 0);
    let est = Math.ceil(total / CHARS_PER_TOKEN);

    while (est > this.contextLength && this.messages.length > 1) {
      this.messages.splice(1, 1);
      total = this.messages.reduce((s, m) => s + m.content.length, 0);
      est = Math.ceil(total / CHARS_PER_TOKEN);
    }
  }
}
