/**
 * @extendai/kernel — Edit tool
 *
 * Exact string replacement editing on files. Supports:
 *   1. Single match (default): one replacement per file
 *   2. replaceAll: replace all occurrences
 *   3. Selective line-based: find the right context uniquely
 *
 * Reference: OpenCode edit.ts (711 lines), Pi edit.ts
 *
 * Edit strategy:
 *   - Reads the file
 *   - Finds oldString in content
 *   - If no match or multiple matches with single mode -> error with context
 *   - Replaces oldString with newString
 *   - Writes back
 *   - Shows diff of the change
 *
 * The caller should ensure a snapshot is taken before calling
 * this tool (via Session.saveCheckpoint with a snapshot hash).
 */

import { existsSync, readFileSync, writeFileSync } from 'node:fs';
import type { Tool } from './registry.js';

export const editTool: Tool = {
  name: 'edit',
  description: `Perform exact string replacements in files.

Rules:
  - oldString must be found exactly once in the file (or use replaceAll).
  - If oldString is found multiple times and replaceAll is false, the tool
    errors out with context to help identify the right match.
  - Provide sufficient surrounding context in oldString to make it unique.
  - Use replaceAll to replace every occurrence of oldString (e.g., renaming).

The diff is shown as [+added / -removed] line counts.`,
  permission: { permission: 'file.write', pattern: '<project>/**', action: 'allow' },
  parameters: {
    type: 'object',
    properties: {
      filePath: {
        type: 'string',
        description: 'Absolute path to the file to modify',
      },
      oldString: {
        type: 'string',
        description: 'The exact text to replace (must be found in file)',
      },
      newString: {
        type: 'string',
        description: 'The replacement text (must differ from oldString)',
      },
      replaceAll: {
        type: 'boolean',
        description: 'Replace all occurrences of oldString (default: false)',
      },
    },
    required: ['filePath', 'oldString', 'newString'],
  },
  execute: async function* (params) {
    const filePath = String(params.filePath || '');
    const oldString = String(params.oldString || '');
    const newString = String(params.newString || '');
    const replaceAll = params.replaceAll === true;

    if (!filePath) {
      yield { type: 'error', error: 'filePath is required' };
      return;
    }

    if (!oldString) {
      yield { type: 'error', error: 'oldString is required' };
      return;
    }

    if (oldString === newString) {
      yield { type: 'error', error: 'oldString and newString must differ' };
      return;
    }

    if (!existsSync(filePath)) {
      yield { type: 'error', error: `File not found: ${filePath}` };
      return;
    }

    try {
      const content = readFileSync(filePath, 'utf-8');

      // Count occurrences
      const occurrences = countOccurrences(content, oldString);

      if (occurrences === 0) {
        yield {
          type: 'error',
          error: `oldString not found in ${filePath}. Please check the exact content and whitespace.`,
        };
        return;
      }

      if (occurrences > 1 && !replaceAll) {
        // Show surrounding context to help disambiguate
        const lines = content.split('\n');
        let context = '';
        for (let i = 0; i < lines.length; i++) {
          if (lines[i].includes(oldString)) {
            const start = Math.max(0, i - 2);
            const end = Math.min(lines.length, i + 3);
            context += `\nMatch near line ${i + 1}:\n`;
            for (let j = start; j < end; j++) {
              context += `${j + 1}: ${lines[j]}\n`;
            }
          }
        }

        yield {
          type: 'error',
          error: `Found ${occurrences} matches of oldString in ${filePath}. Use replaceAll=true or provide more surrounding context.\n${context}`,
        };
        return;
      }

      // Perform replacement
      const newContent = replaceAll
        ? content.split(oldString).join(newString)
        : content.replace(oldString, newString);

      if (content === newContent) {
        yield { type: 'text', content: `[unchanged] ${filePath}` };
        return;
      }

      // Count line changes
      const oldLines = content.split('\n').length;
      const newLines = newContent.split('\n').length;
      const added = Math.max(0, newLines - oldLines);
      const removed = Math.max(0, oldLines - newLines);

      writeFileSync(filePath, newContent, 'utf-8');

      yield {
        type: 'text',
        content: `[${occurrences === 1 ? 'edited' : 'edited x' + occurrences}] ${filePath} [+${added} / -${removed}]`,
      };
    } catch (e: unknown) {
      const err = e as Error;
      yield { type: 'error', error: `Error editing file: ${err.message}` };
    }
  },
};

function countOccurrences(content: string, pattern: string): number {
  let count = 0;
  let idx = 0;
  while (true) {
    idx = content.indexOf(pattern, idx);
    if (idx === -1) break;
    count++;
    idx += pattern.length;
  }
  return count;
}
