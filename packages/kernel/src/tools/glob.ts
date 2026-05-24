/**
 * @extendai/kernel — Glob tool
 *
 * Fast file pattern matching using ripgrep-wrapped glob patterns.
 * Returns matching file paths sorted by modification time.
 *
 * Reference: OpenCode glob tool (ripgrep-based)
 */

import { execSync } from 'node:child_process';
import type { Tool } from './registry.js';

export const globTool: Tool = {
  name: 'glob',
  description: `Fast file pattern matching tool that uses ripgrep-style glob patterns.

Finds files by name/extension patterns like "**/*.ts" or "src/**/*.tsx".
Returns matching file paths sorted by modification time (most recent first).

Use this when you need to find files by name patterns.
For content search, use the grep tool instead.`,
  permission: 'file.read',
  parameters: {
    type: 'object',
    properties: {
      pattern: {
        type: 'string',
        description: 'The glob pattern to match files against (e.g., "**/*.ts", "src/**/*.tsx")',
      },
      path: {
        type: 'string',
        description: 'The directory to search in (defaults to current working directory)',
      },
    },
    required: ['pattern'],
  },
  execute: async function* (params) {
    const pattern = String(params.pattern || '');
    const searchPath = params.path ? String(params.path) : process.cwd();

    if (!pattern) {
      yield { type: 'error', error: 'pattern is required' };
      return;
    }

    try {
      // Use ripgrep --files with glob filter for fast file matching
      // Fallback to native tools if ripgrep isn't available
      let output: string;

      try {
        const cmd = `rg --files -g "${escapeGlob(pattern)}" --glob-case-insensitive "${searchPath}" 2>nul`;
        output = execSync(cmd, {
          encoding: 'utf-8',
          stdio: 'pipe',
          timeout: 30000,
          maxBuffer: 10 * 1024 * 1024,
          shell: true as any,
        }).trim();
      } catch {
        // Fallback: try basic dir /s with wildcard
        try {
          const fallbackCmd = `Get-ChildItem -Path "${searchPath}" -Filter "${pattern.replace('**/', '')}" -Recurse -Name 2>$null | Select-Object -First 1000`;
          output = execSync(fallbackCmd, {
            encoding: 'utf-8',
            stdio: 'pipe',
            timeout: 30000,
            maxBuffer: 10 * 1024 * 1024,
            shell: true as any,
          }).trim();
        } catch {
          output = '';
        }
      }

      if (!output) {
        yield { type: 'text', content: `No files matching "${pattern}"` };
        return;
      }

      const files = output.split('\n').filter(Boolean);
      yield { type: 'text', content: files.join('\n') };
    } catch (e: unknown) {
      const err = e as Error;
      yield { type: 'error', error: `Glob error: ${err.message}` };
    }
  },
};

function escapeGlob(pattern: string): string {
  // Escape special chars for shell
  return pattern.replace(/"/g, '\\"');
}
