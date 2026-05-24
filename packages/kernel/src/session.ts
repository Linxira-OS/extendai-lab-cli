/**
 * @extendai/kernel — Session tree & undo
 *
 * Session lifecycle:
 *   1. Created with system prompt → unnamed
 *   2. After 3 turns (user+assistant pairs) → auto-named (HHmm-branch)
 *   3. Checkpoints saved BEFORE each user message (with snapshot hash)
 *   4. /undo restores the session to the previous checkpoint
 *   5. If snapshot hash present, /undo ALSO rolls back file changes
 *
 * Multiple sessions form a tree: a new session can fork from any
 * existing session, inheriting the messages up to that point.
 *
 * Reference: OpenCode session/revert.ts — links messages to snapshot commits.
 * When undo is called, we return the checkpoint so the caller knows which
 * snapshot patches to revert and which file changes to roll back.
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
  /** Undo checkpoint stack (FILO) — each holds messages + optional snapshot hash */
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

  // ── Snapshot-linked undo ────────────────────────────────

  /**
   * Save an undo checkpoint BEFORE processing a user message.
   *
   * @param snapshotHash — optional tree hash from SnapshotService.track()
   *                       capturing file state before this turn.
   *                       Pass null/undefined for checkpoints that don't
   *                       modify files (e.g. pure conversation).
   * @param label — optional human-readable label
   */
  saveCheckpoint(snapshotHash?: string, label: string = ''): void {
    this.checkpoints.push({
      id: ++this.checkpointSeq,
      messages: this.messages.map(m => ({ ...m })),
      timestamp: new Date().toISOString(),
      snapshotHash: snapshotHash || null,
      label,
    });
  }

  /**
   * Undo: restore the most recent checkpoint.
   *
   * Returns the checkpoint info so the caller knows:
   *   - Which messages to restore (already done internally)
   *   - Which snapshot hash to revert files to (if any)
   *
   * If the checkpoint has a snapshotHash, the caller should call
   * SnapshotService.restore(snapshotHash) to roll back file changes.
   *
   * Returns null if no checkpoint exists.
   */
  undo(): UndoCheckpoint | null {
    const cp = this.checkpoints.pop();
    if (!cp) return null;

    // Restore messages to checkpoint state
    this.messages = cp.messages;

    // Recalculate turn count
    let count = 0;
    for (const m of cp.messages) {
      if (m.role === 'assistant') count++;
    }
    this.meta.turnCount = count;
    this.meta.updatedAt = new Date().toISOString();

    return cp;
  }

  /** Get the most recent undo checkpoint without consuming it. */
  peekCheckpoint(): UndoCheckpoint | null {
    return this.checkpoints[this.checkpoints.length - 1] ?? null;
  }

  /** Number of available undo steps */
  get undoCount(): number {
    return this.checkpoints.length;
  }

  /** Get all undo snapshots for display (date + label). */
  get undoHistory(): Array<{ id: number; timestamp: string; label: string; hasSnapshot: boolean }> {
    return this.checkpoints.map(cp => ({
      id: cp.id,
      timestamp: cp.timestamp,
      label: cp.label,
      hasSnapshot: !!cp.snapshotHash,
    }));
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
