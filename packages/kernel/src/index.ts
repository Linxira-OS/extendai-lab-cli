/**
 * @extendai/kernel — Core Engine
 *
 * Session tree, context management, undo, provider abstraction,
 * configuration loading, worktree detection, snapshot system,
 * and session naming.
 *
 * Every component is a replaceable proxy — plugins can
 * invasively swap implementations at runtime.
 */

export type {
  Role,
  Message,
  Usage,
  StreamChunk,
  ProviderConfig,
  AppConfig,
  SessionMeta,
  UndoCheckpoint,
  SnapshotInfo,
  WorktreeInfo,
} from './types.js';

export {
  streamCompletion,
} from './provider.js';

export {
  Session,
} from './session.js';

export {
  loadConfig,
  saveConfig,
} from './config.js';

export {
  detectWorktree,
  isInsideWorktree,
  ensureGit,
  ensureGitIgnore,
} from './worktree.js';

export {
  initSnapshotRepo,
  takeSnapshot,
  revertToSnapshot,
  listSnapshots,
  snapshotDiff,
  snapshotDiffBetween,
} from './snapshot.js';

export {
  generateSessionName,
  generateSessionId,
  formatSessionName,
} from './namer.js';
