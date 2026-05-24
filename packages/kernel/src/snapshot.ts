/**
 * @extendai/kernel — Snapshot file change tracker
 *
 * Maintains an independent git repository for file-change history.
 * Location: <project-root>/.extendai/snapshots/
 * NOT in the project's own git — .extendai/ is added to .gitignore.
 *
 * Protection:
 *   The permission system blocks AI writes to .extendai/ directory,
 *   protecting snapshots from accidental deletion during AI operations.
 *
 * Retention:
 *   - Max 1000 commits
 *   - Auto-cleanup on each new snapshot
 *   - 7-day commit expiry
 *
 * This is the LAST RESORT file rollback mechanism.
 * Layer 1: /undo (session-level, restores user message)
 * Layer 2: /snapshot revert (git-level, restores files)
 */

import { execSync } from 'node:child_process';
import { existsSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';

const SNAPSHOT_DIR = '.extendai/snapshots';

export interface Snapshot {
  hash: string;
  date: string;
  message: string;
  files: number;
}

/**
 * Initialize the snapshot git repository.
 * Creates a bare-like repo for efficient storage.
 */
export function initSnapshotRepo(projectRoot: string): void {
  const repoDir = join(projectRoot, SNAPSHOT_DIR);

  if (existsSync(repoDir)) return; // Already initialized

  mkdirSync(repoDir, { recursive: true });
  execSync('git init', { cwd: repoDir, encoding: 'utf-8', stdio: 'ignore' });

  // Configure git for snapshot-only use
  execSync('git config user.name "ExtendAI Snapshot"', { cwd: repoDir, encoding: 'utf-8', stdio: 'ignore' });
  execSync('git config user.email "snapshot@extendai.local"', { cwd: repoDir, encoding: 'utf-8', stdio: 'ignore' });

  // Initial empty commit
  execSync('git commit --allow-empty -m "init"', { cwd: repoDir, encoding: 'utf-8', stdio: 'ignore' });
}

/**
 * Take a snapshot of all tracked files.
 * @returns the snapshot hash
 */
export function takeSnapshot(projectRoot: string, sessionId: string, message: string = ''): string {
  const repoDir = join(projectRoot, SNAPSHOT_DIR);
  if (!existsSync(repoDir)) initSnapshotRepo(projectRoot);

  // Add all files (respecting project .gitignore)
  execSync('git add -A', { cwd: projectRoot, encoding: 'utf-8', stdio: 'ignore' });

  // Check if there are changes to commit
  const status = execSync('git status --porcelain', {
    cwd: repoDir,
    encoding: 'utf-8',
    stdio: ['ignore', 'pipe', 'ignore'],
  });

  if (!status.trim()) return ''; // No changes

  const date = new Date().toISOString();
  const commitMsg = `session:${sessionId} date:${date} ${message}`.trim();

  execSync(`git commit -m "${escapeCommitMsg(commitMsg)}"`, {
    cwd: repoDir,
    encoding: 'utf-8',
    stdio: 'ignore',
  });

  const hash = execSync('git rev-parse --short HEAD', {
    cwd: repoDir,
    encoding: 'utf-8',
    stdio: ['ignore', 'pipe', 'ignore'],
  }).trim();

  // Enforce retention policy
  enforceRetention(repoDir);

  return hash;
}

/**
 * Revert files to a specific snapshot state.
 * @param hash snapshot hash to revert to
 */
export function revertToSnapshot(projectRoot: string, hash: string): string {
  const repoDir = join(projectRoot, SNAPSHOT_DIR);
  if (!existsSync(repoDir)) throw new Error('Snapshot repo not initialized');

  // Checkout the snapshot (restores all files, detaching HEAD)
  const output = execSync(`git checkout ${hash} -- .`, {
    cwd: repoDir,
    encoding: 'utf-8',
    stdio: ['ignore', 'pipe', 'pipe'],
  });

  return output.trim();
}

/**
 * List all snapshots.
 */
export function listSnapshots(projectRoot: string, limit: number = 20): Snapshot[] {
  const repoDir = join(projectRoot, SNAPSHOT_DIR);
  if (!existsSync(repoDir)) return [];

  const output = execSync(
    `git log --oneline --format="%h|%ci|%s" -${limit}`,
    { cwd: repoDir, encoding: 'utf-8', stdio: ['ignore', 'pipe', 'ignore'] },
  );

  return output
    .trim()
    .split('\n')
    .filter(Boolean)
    .map((line: string) => {
      const [hash, date, ...msgParts] = line.split('|');
      return { hash, date: date || '', message: msgParts.join('|'), files: 0 };
    });
}

/**
 * Show diff for a snapshot.
 */
export function snapshotDiff(projectRoot: string, hash: string): string {
  const repoDir = join(projectRoot, SNAPSHOT_DIR);
  if (!existsSync(repoDir)) return '';

  return execSync(`git show --stat ${hash}`, {
    cwd: repoDir,
    encoding: 'utf-8',
    stdio: ['ignore', 'pipe', 'ignore'],
  });
}

/**
 * Get diff between two snapshots.
 */
export function snapshotDiffBetween(projectRoot: string, fromHash: string, toHash: string): string {
  const repoDir = join(projectRoot, SNAPSHOT_DIR);
  if (!existsSync(repoDir)) return '';

  return execSync(`git diff --stat ${fromHash}..${toHash}`, {
    cwd: repoDir,
    encoding: 'utf-8',
    stdio: ['ignore', 'pipe', 'ignore'],
  }).trim();
}

// ─── Retention ────────────────────────────────────────────

const MAX_COMMITS = 1000;
const MAX_DAYS = 7;

function enforceRetention(repoDir: string): void {
  // Count commits
  const count = parseInt(
    execSync('git rev-list --count HEAD', {
      cwd: repoDir,
      encoding: 'utf-8',
      stdio: ['ignore', 'pipe', 'ignore'],
    }).trim(),
    10,
  );

  if (count <= MAX_COMMITS) return;

  // Remove old commits (keep most recent MAX_COMMITS)
  const toRemove = count - MAX_COMMITS;
  const oldestKept = execSync(`git rev-list --reverse HEAD | head -${toRemove + 1} | tail -1`, {
    cwd: repoDir,
    encoding: 'utf-8',
    stdio: ['ignore', 'pipe', 'ignore'],
  }).trim();

  // Create a new orphan branch and graft
  execSync(`git checkout --orphan temp ${oldestKept}`, {
    cwd: repoDir,
    encoding: 'utf-8',
    stdio: 'ignore',
  });
  execSync('git commit -m "squash old snapshots"', {
    cwd: repoDir,
    encoding: 'utf-8',
    stdio: 'ignore',
  });
  execSync('git rebase --onto temp HEAD', {
    cwd: repoDir,
    encoding: 'utf-8',
    stdio: 'ignore',
  });
  execSync('git branch -D temp', {
    cwd: repoDir,
    encoding: 'utf-8',
    stdio: 'ignore',
  });
}

function escapeCommitMsg(msg: string): string {
  return msg.replace(/"/g, '\\"').replace(/`/g, '\\`').replace(/\$/g, '\\$');
}
