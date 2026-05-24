/**
 * @extendai/kernel — Git worktree detection & session isolation
 *
 * Each git worktree gets its own session namespace.
 * Non-git directories still get isolated by absolute path hash.
 */

import { execSync } from 'node:child_process';
import { realpathSync, existsSync, readFileSync, writeFileSync } from 'node:fs';
import { createHash } from 'node:crypto';

export interface WorktreeInfo {
  /** Globally unique hash for session isolation */
  id: string;
  /** Absolute path to the worktree root */
  root: string;
  /** Git branch name, or '(detached)' / '(no git)' */
  branch: string;
  /** Whether this is inside a git repository */
  isGit: boolean;
  /** Current commit short hash, or '' */
  commit: string;
  /** Human-readable label for display */
  label: string;
}

/**
 * Detect the current git worktree (or fallback to absolute-path isolation).
 * This should be called once at startup and cached.
 */
export function detectWorktree(cwd: string = process.cwd()): WorktreeInfo {
  // Try to get git info
  try {
    const root = execSync('git rev-parse --show-toplevel', {
      cwd,
      encoding: 'utf-8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim();

    const realRoot = realpathSync(root);

    const branch = execSync('git rev-parse --abbrev-ref HEAD', {
      cwd,
      encoding: 'utf-8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim();

    const commit = execSync('git rev-parse --short HEAD', {
      cwd,
      encoding: 'utf-8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim();

    const id = makeId(realRoot);
    const label = branch === 'HEAD' ? `(detached) ${commit}` : branch;

    return { id, root: realRoot, branch, isGit: true, commit, label };
  } catch {
    // Not a git repo — fall back to absolute path isolation
    const abs = realpathSync(cwd);
    const id = makeId(abs);
    return { id, root: abs, branch: '(no git)', isGit: false, commit: '', label: abs.split(/[/\\]/).pop() || 'unknown' };
  }
}

/**
 * Generate a short, unique hash from an absolute path.
 * 16 hex chars = 64 bits, collision probability negligible.
 */
function makeId(absPath: string): string {
  return createHash('sha256').update(absPath.toLowerCase()).digest('hex').slice(0, 16);
}

/**
 * Check if a specific path is inside the current worktree.
 */
export function isInsideWorktree(path: string, worktree: WorktreeInfo): boolean {
  try {
    const real = realpathSync(path);
    return real.startsWith(worktree.root);
  } catch {
    return false;
  }
}

/**
 * Initialize a git repository at the given path if one doesn't exist.
 * Returns true if a new repo was created, false if already a repo.
 */
export function ensureGit(path: string): boolean {
  if (existsSync(`${path}/.git`)) {
    return false; // Already a git repo
  }

  execSync('git init', { cwd: path, encoding: 'utf-8', stdio: 'ignore' });
  return true;
}

/**
 * Add .extendai/ to the project's .gitignore if it's a git repo.
 */
export function ensureGitIgnore(path: string): void {
  if (!existsSync(`${path}/.git`)) return;

  const gitignorePath = `${path}/.gitignore`;
  const entry = '\n# ExtendAI Lab\n.extendai/\n';

  if (existsSync(gitignorePath)) {
    const content = readFileSync(gitignorePath, 'utf-8');
    if (content.includes('.extendai/')) return; // Already there
    writeFileSync(gitignorePath, content + entry, 'utf-8');
  } else {
    writeFileSync(gitignorePath, entry, 'utf-8');
  }
}
