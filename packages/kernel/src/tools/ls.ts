/**
 * @extendai/kernel — Ls tool
 *
 * Directory listing with file info (size, date, type).
 * Simpler alternative to read for browsing directory structure.
 *
 * Reference: Pi ls tool, OpenCode directory listing
 */

import { readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import type { Tool } from './registry.js';

export const lsTool: Tool = {
  name: 'ls',
  description: `List directory contents with file info.

Shows files and subdirectories with size and last modified date.
Useful for browsing the project structure quickly.`,
  permission: 'file.read',
  parameters: {
    type: 'object',
    properties: {
      path: {
        type: 'string',
        description: 'Directory path to list (defaults to current working directory)',
      },
      long: {
        type: 'boolean',
        description: 'Show detailed info (size, date) — default: true',
      },
      maxItems: {
        type: 'number',
        description: 'Maximum items to show (default: 200)',
      },
    },
  },
  execute: async function* (params) {
    const dirPath = params.path ? String(params.path) : process.cwd();
    const long = params.long !== false;
    const maxItems = Number(params.maxItems) || 200;

    try {
      const entries = readdirSync(dirPath, { withFileTypes: true });

      if (entries.length === 0) {
        yield { type: 'text', content: `(empty directory: ${dirPath})` };
        return;
      }

      const items = entries.slice(0, maxItems);

      if (long) {
        let output = `📁 ${dirPath}\n`;
        for (const entry of items) {
          const fullPath = join(dirPath, entry.name);
          try {
            const stat = statSync(fullPath);
            const size = entry.isDirectory() ? '<DIR>' : formatSize(stat.size);
            const date = stat.mtime.toISOString().slice(0, 16).replace('T', ' ');
            output += `${entry.isDirectory() ? 'd' : '-'} ${size.padStart(8)} ${date} ${entry.name}\n`;
          } catch {
            output += `?         ????-??-?? ??:?? ${entry.name}\n`;
          }
        }

        if (entries.length > maxItems) {
          output += `\n... (${entries.length - maxItems} more items)`;
        }

        yield { type: 'text', content: output };
      } else {
        const names = items.map(e => e.name + (e.isDirectory() ? '/' : ''));
        let content = names.join('\n');

        if (entries.length > maxItems) {
          content += `\n... (${entries.length - maxItems} more items)`;
        }

        yield { type: 'text', content };
      }
    } catch (e: unknown) {
      const err = e as Error;
      yield { type: 'error', error: `Error listing directory: ${err.message}` };
    }
  },
};

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
