/**
 * @extendai/kernel — Snapshot file change tracker
 *
 * Maintains an independent git repository for AI file-change history.
 * COMPLETELY SEPARATE from the project's own git — .extendai/ is in .gitignore.
 *
 * Architecture (OpenCode-style):
 *   - Snapshot repo stored at ~/.extendai/snapshots/<project-hash>/<worktree-hash>/
 *   - Uses `--git-dir <snapshot-repo> --work-tree <project-root>` to track project files
 *     while storing git metadata in the snapshot repo (outside the project).
 *   - track() -> git add -A + git write-tree -> returns tree hash (not commit hash)
 *   - patch(hash) -> computes what changed since a previous tree hash
 *   - revert(patches) -> git checkout hash -- file for each file in each patch
 *
 * Layered protection:
 *   Layer 1: /undo (session-level, restores messages + file changes)
 *   Layer 2: /snapshot revert (git-level, explicit file restore)
 * Permissions: AI cannot write to .extendai/ (handled by PermissionGuard)
 */

import { execSync } from 'node:child_process';
import { existsSync, mkdirSync, unlinkSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { homedir } from 'node:os';
import { join, relative } from 'node:path';

// ─── Types ────────────────────────────────────────────────

/** A patch describing file changes between two snapshot points. */
export interface SnapshotPatch {
  /** The "before" tree hash */
  hash: string;
  /** Changed file paths (absolute, project-root-relative stored in git) */
  files: string[];
}

/** Commit-style record for list/display */
export interface SnapshotRecord {
  hash: string;
  date: string;
  message: string;
  files: number;
}

// ─── SnapshotService ──────────────────────────────────────

export class SnapshotService {
  private gitdir: string;
  private worktree: string;
  private initialized = false;
  private readonly STORAGE_BASE = join(homedir(), '.extendai', 'snapshots');

  constructor(projectRoot: string, worktreeId: string) {
    const projectHash = createHash('sha256')
      .update(projectRoot.toLowerCase())
      .digest('hex')
      .slice(0, 16);

    this.gitdir = join(this.STORAGE_BASE, projectHash, worktreeId);
    this.worktree = projectRoot;
  }

  /** Full path to the snapshot git repo (for debugging) */
  get repoPath(): string {
    return this.gitdir;
  }

  // ── git helpers ────────────────────────────────────────

  private git(args: string[], opts?: { stdin?: string; trim?: boolean }): string {
    const cmd = ['git', '--git-dir', this.gitdir, '--work-tree', this.worktree, ...args];
    try {
      const result = execSync(cmd.join(' '), {
        cwd: this.worktree,
        encoding: 'utf-8',
        stdio: ['pipe', 'pipe', 'pipe'],
        input: opts?.stdin,
        maxBuffer: 10 * 1024 * 1024, // 10MB
      });
      return opts?.trim !== false ? result.trim() : result;
    } catch (e: unknown) {
      const err = e as Error & { stderr?: string; stdout?: string };
      throw new Error(`git snapshot error: ${err.stderr || err.message}`);
    }
  }

  /** Execute git in the snapshot dir only (no worktree) */
  private gitDir(args: string[]): string {
    const cmd = ['git', '--git-dir', this.gitdir, ...args];
    try {
      return execSync(cmd.join(' '), {
        encoding: 'utf-8',
        stdio: ['pipe', 'pipe', 'pipe'],
      }).trim();
    } catch (e: unknown) {
      const err = e as Error & { stderr?: string };
      throw new Error(`git snapshot error: ${err.stderr || err.message}`);
    }
  }

  // ── Lifecycle ──────────────────────────────────────────

  /**
   * Initialize the snapshot git repository.
   * Creates a bare-like repo at ~/.extendai/snapshots/<hash>/<worktree>/
   */
  init(): void {
    if (this.initialized) return;

    if (!existsSync(this.gitdir)) {
      mkdirSync(this.gitdir, { recursive: true });

      // git init with minimal config
      this.gitDir(['init', '--bare']);
      this.gitDir(['config', 'core.autocrlf', 'false']);
      this.gitDir(['config', 'core.longpaths', 'true']);
      this.gitDir(['config', 'core.symlinks', 'true']);
      this.gitDir(['config', 'core.fsmonitor', 'false']);
      this.gitDir(['config', 'user.name', 'ExtendAI Snapshot']);
      this.gitDir(['config', 'user.email', 'snapshot@extendai.local']);

      // Sync project's gitignore excludes into snapshot repo
      this.syncGitIgnore();
    }

    this.initialized = true;
  }

  /**
   * Copy project's .gitignore rules into snapshot repo's info/exclude
   * so that git ignore rules are respected even outside a normal git repo context.
   */
  private syncGitIgnore(): void {
    try {
      // Check project's own gitignore via check-ignore against its own .git
      const projectGitDir = join(this.worktree, '.git');
      const checkIgnorePath = join(this.worktree, '.gitignore');

      if (existsSync(projectGitDir)) {
        // Read project's exclude file
        const projectExclude = join(this.worktree, '.git', 'info', 'exclude');
        let content = '';

        if (existsSync(projectExclude)) {
          const fs = require('fs') as typeof import('fs');
          content = fs.readFileSync(projectExclude, 'utf-8');
        }

        // Also read .gitignore
        if (existsSync(checkIgnorePath)) {
          const fs = require('fs') as typeof import('fs');
          content += '\n' + fs.readFileSync(checkIgnorePath, 'utf-8');
        }

        // Write to snapshot repo's info/exclude
        const excludeDir = join(this.gitdir, 'info');
        if (!existsSync(excludeDir)) {
          mkdirSync(excludeDir, { recursive: true });
        }
        const fs = require('fs') as typeof import('fs');
        fs.writeFileSync(join(this.gitdir, 'info', 'exclude'), content, 'utf-8');
      }
    } catch {
      // Non-fatal: gitignore sync failure
    }
  }

  // ── Core Operations ────────────────────────────────────

  /**
   * Track current state of all project files.
   * Equivalent to git add -A + git write-tree.
   * Returns the tree hash (or empty string if no changes).
   *
   * Respects .gitignore via `--work-tree`'s natural behavior.
   */
  track(): string {
    this.init();

    // Stage all changes (respects .gitignore via --work-tree)
    this.git(['add', '-A']);

    // Check if anything changed
    const status = this.git(['status', '--porcelain']);
    if (!status) return '';

    // Write tree and return hash
    return this.git(['write-tree']);
  }

  /**
   * Compute what changed since a given tree hash.
   * @param hash - the "before" tree hash (from a previous track() call)
   * @returns SnapshotPatch with changed file list
   */
  patch(hash: string): SnapshotPatch {
    // diff --cached compares the index (staging) against a tree
    const output = this.git(['diff', '--cached', '--name-only', hash, '--', '.']);
    const files = output ? output.split('\n').filter(Boolean) : [];
    return { hash, files };
  }

  /**
   * Batch compute patches for multiple tree hashes.
   * More efficient than individual patch() calls.
   */
  patchBatch(hashes: string[]): SnapshotPatch[] {
    return hashes.map(h => this.patch(h)).filter(p => p.files.length > 0);
  }

  /**
   * Revert files to the state captured in the patches.
   * Uses `git checkout <hash> -- <file>` for each unique hash-file pair.
   * Files that didn't exist in the snapshot are deleted.
   */
  revert(patches: SnapshotPatch[]): void {
    // Group files by hash for efficient checkout
    const groups = new Map<string, string[]>();
    for (const p of patches) {
      for (const f of p.files) {
        const list = groups.get(p.hash) || [];
        list.push(f);
        groups.set(p.hash, list);
      }
    }

    // For each hash, checkout all files at once
    for (const [hash, files] of groups) {
      // Check which files exist in the tree
      const lsResult = this.gitDir(['ls-tree', hash, ...files]);
      const existingFiles = new Set(
        lsResult.split('\n')
          .filter(Boolean)
          .map(line => line.split('\t').pop()!)
      );

      // Files that exist in snapshot -> checkout
      const toCheckout = files.filter(f => existingFiles.has(f));
      if (toCheckout.length > 0) {
        this.git(['checkout', hash, '--', ...toCheckout]);
      }

      // Files that DON'T exist in snapshot -> delete
      const toDelete = files.filter(f => !existingFiles.has(f));
      for (const f of toDelete) {
        const absPath = join(this.worktree, f);
        try {
          if (existsSync(absPath)) unlinkSync(absPath);
        } catch {
          // File may already be gone
        }
      }
    }
  }

  /**
   * Restore ALL files to a specific tree hash.
   * Uses `git checkout <hash> -- .` — a full restore.
   */
  restore(hash: string): void {
    this.git(['checkout', hash, '--', '.']);
  }

  /**
   * Show diff between two snapshots (for display).
   * @param fromHash - earlier tree hash
   * @param toHash - later tree hash (default: current index)
   */
  diff(fromHash: string, toHash?: string): string {
    if (toHash) {
      return this.git(['diff', '--stat', `${fromHash}..${toHash}`]);
    }
    return this.git(['diff', '--stat', fromHash, '--', '.']);
  }

  /** Get diff with full content */
  diffFull(fromHash: string, toHash?: string): string {
    if (toHash) {
      return this.git(['diff', fromHash, toHash]);
    }
    return this.git(['diff', fromHash, '--', '.']);
  }

  // ── List / History ─────────────────────────────────────

  /**
   * List recent snapshot commits (for display).
   * Uses git log on the reflog to show recent tree operations.
   */
  listSnapshots(limit: number = 20): SnapshotRecord[] {
    try {
      const output = this.gitDir([
        'log', '--oneline', `-${limit}`,
        '--format=%h|%ci|%s',
        '--all',
      ]);
      if (!output) return [];

      return output.split('\n').filter(Boolean).map(line => {
        const [hash, date, ...msgParts] = line.split('|');
        return { hash, date: date || '', message: msgParts.join('|'), files: 0 };
      });
    } catch {
      return [];
    }
  }

  /** Get count of tracked snapshots. */
  get commitCount(): number {
    try {
      return parseInt(this.gitDir(['rev-list', '--count', '--all']), 10);
    } catch {
      return 0;
    }
  }
}
