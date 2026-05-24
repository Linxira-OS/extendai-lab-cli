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

export type {
  SnapshotPatch,
  SnapshotRecord,
} from './snapshot.js';

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
  SnapshotService,
} from './snapshot.js';

export {
  generateSessionName,
  generateSessionId,
  formatSessionName,
} from './namer.js';

export {
  ToolRegistry,
  createDefaultRegistry,
  bashTool,
  browserTool,
  createQuestionTool,
  askQuestion,
  PermissionGuard,
  DangerousDetector,
  ApprovalGate,
} from './tools/index.js';
export type {
  Tool,
  ToolContext,
  ToolChunk,
  ToolDefinition,
  PermissionSpec,
  PermissionRule,
  PermissionResult,
  PermissionAction,
  SafetyAssessment,
  FileOperation,
  ApprovalPrompt,
  ApprovalDecision,
  ApprovalOption,
  ApprovalOptionKind,
  AutoAcceptMode,
  QuestionParams,
  QuestionResult,
  QuestionOption,
  QuestionHandler,
} from './tools/index.js';
