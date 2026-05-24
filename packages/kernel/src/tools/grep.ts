/**
 * @extendai/kernel — Grep tool
 *
 * Fast content search using ripgrep.
 * Searches file contents using regular expressions.
 * Supports full regex syntax and file pattern filtering.
 *
 * Reference: OpenCode grep tool, Pi grep tool
 */

import { execSync } from 'node:child_process';
import type { Tool } from './registry.js';

export const grepTool: Tool = {
  name: 'grep',
  description: `Fast content search tool using ripgrep.

Searches file contents using regular expressions.
Supports full regex syntax (e.g., "log.*Error", "function\\s+\\w+").
Filter files by pattern with the include parameter (e.g., "*.ts", "*.{ts,tsx}").

Returns matching file paths and line numbers, sorted by modification time.
For file name searches, use the glob tool instead.`,
  permission: 'file.read',
  parameters: {
    type: 'object',
    properties: {
      pattern: {
        type: 'string',
        description: 'The regex pattern to search for in file contents',
      },
      path: {
        type: 'string',
        description: 'The directory to search in (defaults to current working directory)',
      },
      include: {
        type: 'string',
        description: 'File glob pattern to filter (e.g., "*.ts", "*.{ts,tsx}")',
      },
    },
    required: ['pattern'],
  },
  execute: async function* (params) {
    const pattern = String(params.pattern || '');
    const searchPath = params.path ? String(params.path) : process.cwd();
    const include = params.include ? String(params.include) : '';

    if (!pattern) {
      yield { type: 'error', error: 'pattern is required' };
      return;
    }

    try {
      let cmd = `rg -n --no-heading "${escapeArg(pattern)}" "${searchPath}" 2>nul`;

      if (include) {
        cmd = `rg -n --no-heading -g "${escapeArg(include)}" "${escapeArg(pattern)}" "${searchPath}" 2>nul`;
      }

      const output = execSync(cmd, {
        encoding: 'utf-8',
        stdio: 'pipe',
        timeout: 30000,
        maxBuffer: 10 * 1024 * 1024,
        shell: true as any,
      }).trim();

      if (!output) {
        yield { type: 'text', content: `No matches for "${pattern}"` };
        return;
      }

      // Truncate if too many results
      const lines = output.split('\n');
      const maxResults = 500;

      if (lines.length > maxResults) {
        yield {
          type: 'text',
          content: `${lines.slice(0, maxResults).join('\n')}\n\n... (${lines.length - maxResults} more results, pattern too broad)`,
        };
      } else {
        yield { type: 'text', content: output };
      }
    } catch (e: unknown) {
      const err = e as Error;
      yield { type: 'error', error: `Grep error: ${err.message}` };
    }
  },
};

function escapeArg(s: string): string {
  return s.replace(/"/g, '\\"').replace(/\\/g, '\\\\');
}
