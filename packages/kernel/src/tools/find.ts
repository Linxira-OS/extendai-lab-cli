/**
 * @extendai/kernel — Find tool
 *
 * File finding by name or path pattern.
 * Simpler than glob — searches for files by name substring or exact name.
 *
 * Reference: Pi find tool
 */

import { execSync } from 'node:child_process';
import type { Tool } from './registry.js';

export const findTool: Tool = {
  name: 'find',
  description: `Find files by name or path pattern.

Searches for files and directories matching a name/pattern.
Simpler than glob — good for finding files when you know part of the name.
Uses ripgrep --files internally for speed.`,
  permission: 'file.read',
  parameters: {
    type: 'object',
    properties: {
      name: {
        type: 'string',
        description: 'File name or pattern to search for (e.g., "read", "*.tsx")',
      },
      path: {
        type: 'string',
        description: 'Directory to search in (defaults to current working directory)',
      },
      maxResults: {
        type: 'number',
        description: 'Maximum results to return (default: 50)',
      },
    },
    required: ['name'],
  },
  execute: async function* (params) {
    const name = String(params.name || '');
    const searchPath = params.path ? String(params.path) : process.cwd();
    const maxResults = Number(params.maxResults) || 50;

    if (!name) {
      yield { type: 'error', error: 'name is required' };
      return;
    }

    try {
      // Use ripgrep --files piped to findstr for name filtering
      const escapedPattern = name
        .replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
        .replace(/\\\*/g, '.*')
        .replace(/\\\?/g, '.');

      const cmd = `rg --files "${searchPath}" 2>nul | findstr /ri "${escapeArg(escapedPattern)}" 2>nul`;
      const output = execSync(cmd, {
        encoding: 'utf-8',
        stdio: 'pipe',
        timeout: 30000,
        maxBuffer: 10 * 1024 * 1024,
        shell: true as any,
      }).trim();

      const files = output.split('\n').filter(Boolean);

      if (files.length === 0) {
        yield { type: 'text', content: `No files matching "${name}"` };
        return;
      }

      const results = files.slice(0, maxResults);
      let content = results.join('\n');
      if (files.length > maxResults) {
        content += `\n\n... (${files.length - maxResults} more results)`;
      }

      yield { type: 'text', content };
    } catch (e: unknown) {
      const err = e as Error;
      yield { type: 'error', error: `Find error: ${err.message}` };
    }
  },
};

function escapeArg(s: string): string {
  return s.replace(/"/g, '\\"');
}
