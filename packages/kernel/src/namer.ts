/**
 * @extendai/kernel — Session naming
 *
 * Sessions are auto-named after the 3rd turn:
 *   "HHmm-branchHash"  (e.g., "1430-main" or "1430-feat-undo")
 *
 * Before naming, they display as "unnamed".
 */

import { createHash } from 'node:crypto';

const TURNS_BEFORE_NAMING = 3;

export interface NamerOptions {
  branch?: string;
  worktreeId?: string;
}

/**
 * Generate a session name after enough turns.
 * Format: HHmm-branchAbbrev
 */
export function generateSessionName(turnCount: number, opts: NamerOptions): string | null {
  if (turnCount < TURNS_BEFORE_NAMING) return null;

  const time = new Date();
  const hhmm = String(time.getHours()).padStart(2, '0') +
               String(time.getMinutes()).padStart(2, '0');

  const branch = opts.branch || 'unknown';
  const abbrev = branch.length > 12
    ? branch.slice(0, 8) + branch.slice(-4)
    : branch;

  return `${hhmm}-${abbrev}`;
}

/**
 * Generate a session ID scoped by worktree.
 */
export function generateSessionId(worktreeId: string): string {
  const time = Date.now().toString(36);
  const rand = Math.random().toString(36).slice(2, 6);
  const raw = `${worktreeId}-${time}-${rand}`;
  return createHash('sha256').update(raw).digest('hex').slice(0, 12);
}

/**
 * Format a session name for the display prompt.
 */
export function formatSessionName(name: string | null, turnCount: number): string {
  if (name) return name;
  if (turnCount === 0) return 'new session';
  return `unnamed (${turnCount} turns)`;
}
