/**
 * @extendai/kernel — Core type definitions
 */

export type Role = 'system' | 'user' | 'assistant';

export interface Message {
  role: Role;
  content: string;
}

export interface Usage {
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
}

export interface StreamChunk {
  type: 'content' | 'done' | 'error';
  content?: string;
  error?: string;
  usage?: Usage;
}

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

export interface AppConfig {
  provider: ProviderConfig;
}

// ─── Session tree ─────────────────────────────────────────

export interface SessionMeta {
  /** Auto-generated name: "HHmm-branch" after 3 turns */
  name: string;
  /** Worktree-bound unique ID */
  id: string;
  /** Parent session ID (for tree navigation) */
  parentId: string | null;
  /** Creation timestamp */
  createdAt: string;
  /** Last activity timestamp */
  updatedAt: string;
  /** How many turns (user+assistant pairs) so far */
  turnCount: number;
}

/** A checkpoint for undo — saves messages before processing a user input */
export interface UndoCheckpoint {
  id: number;
  messages: Message[];
  timestamp: string;
  label: string;
}

// ─── Snapshot ─────────────────────────────────────────────

export interface SnapshotInfo {
  hash: string;
  date: string;
  message: string;
  files: number;
}

// ─── Worktree ─────────────────────────────────────────────

export interface WorktreeInfo {
  id: string;
  root: string;
  branch: string;
  isGit: boolean;
  commit: string;
  label: string;
}
