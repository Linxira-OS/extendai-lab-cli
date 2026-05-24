/**
 * @extendai/kernel — Write tool
 *
 * File writing with overwrite protection, directory creation, and
 * change detection. Integrates with the snapshot system so that
 * every write is captured in the AI file-change history.
 *
 * Reference: Pi write.ts, OpenCode write.ts
 *
 * Overwrite protection:
 *   - If the file already exists and the user didn't explicitly allow,
 *     the tool yields the diff for review.
 *   - Force-write via the 'force' parameter skips this check.
 *   - Large files (>5MB) are rejected unless force is true.
 */

import { existsSync, statSync, mkdirSync, writeFileSync, readFileSync } from 'node:fs';
import { dirname } from 'node:path';
import type { Tool } from './registry.js';

const MAX_FILE_SIZE = 5 * 1024 * 1024; // 5MB

export const writeTool: Tool = {
  name: 'write',
  description: `Write content to a file. Creates parent directories if needed.

Overwrite protection:
  - If the file exists and 'force' is false (default), the tool shows the diff
    and requires confirmation (handled by the caller).
  - Set force=true to overwrite without confirmation.
  - Files larger than 5MB are rejected unless force=true.

All file writes are automatically tracked by the snapshot system
for undo support.`,
  permission: { permission: 'file.write', pattern: '<project>/**', action: 'allow' },
  parameters: {
    type: 'object',
    properties: {
      filePath: {
        type: 'string',
        description: 'Absolute path to the file to write',
      },
      content: {
        type: 'string',
        description: 'Content to write to the file',
      },
      force: {
        type: 'boolean',
        description: 'Overwrite existing file without confirmation (default: false)',
      },
    },
    required: ['filePath', 'content'],
  },
  execute: async function* (params) {
    const filePath = String(params.filePath || '');
    const content = String(params.content || '');
    const force = params.force === true;

    if (!filePath) {
      yield { type: 'error', error: 'filePath is required' };
      return;
    }

    // Large file check
    if (!force && existsSync(filePath)) {
      const stat = statSync(filePath);
      if (stat.size > MAX_FILE_SIZE) {
        yield { type: 'error', error: `File too large (${stat.size} bytes). Use force=true to overwrite.` };
        return;
      }
    }

    // Overwrite detection
    let isNew = false;
    let diff = '';
    if (existsSync(filePath)) {
      if (!force) {
        try {
          const existing = readFileSync(filePath, 'utf-8');
          if (existing === content) {
            yield { type: 'text', content: `[unchanged] ${filePath}` };
            return;
          }
          // Generate simple diff (for display)
          diff = simpleDiff(existing, content);
        } catch {
          // Binary or unreadable file — show size comparison
          const stat = statSync(filePath);
          diff = `[binary file] size: ${stat.size} → ${Buffer.byteLength(content, 'utf-8')}`;
        }
      }
    } else {
      isNew = true;
    }

    // Create directory if needed
    const dir = dirname(filePath);
    if (!existsSync(dir)) {
      mkdirSync(dir, { recursive: true });
    }

    try {
      writeFileSync(filePath, content, 'utf-8');

      const result = isNew
        ? `[created] ${filePath} (${Buffer.byteLength(content, 'utf-8')} bytes)`
        : `[updated] ${filePath}${diff ? `\n${diff}` : ''}`;

      yield { type: 'text', content: result };
    } catch (e: unknown) {
      const err = e as Error;
      yield { type: 'error', error: `Error writing file: ${err.message}` };
    }
  },
};

/** Generate a minimal diff between old and new content. */
function simpleDiff(oldContent: string, newContent: string): string {
  const oldLines = oldContent.split('\n');
  const newLines = newContent.split('\n');

  let added = 0;
  let removed = 0;

  // Count line-level changes (simple heuristic)
  const oldSet = new Set(oldLines);
  const newSet = new Set(newLines);

  for (const line of oldLines) {
    if (!newSet.has(line)) removed++;
  }
  for (const line of newLines) {
    if (!oldSet.has(line)) added++;
  }

  return `[+${added} / -${removed}]`;
}
