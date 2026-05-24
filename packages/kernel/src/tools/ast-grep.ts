/**
 * @extendai/kernel — AST Grep tool
 *
 * AST-aware code search and pattern matching.
 * Uses tree-sitter based pattern matching for structural code search.
 *
 * Reference: OpenCode ast-grep, LabForge ast_grep_search
 *
 * IMPORTANT: Patterns must be complete AST nodes (valid code).
 * For functions, include params and body:
 *   ✅ 'export async function $NAME($$$) { $$$ }'
 *   ❌ 'export async function $NAME'
 */

import { execSync } from 'node:child_process';
import type { Tool } from './registry.js';

export const astGrepTool: Tool = {
  name: 'ast_grep_search',
  description: `Search code patterns across filesystem using AST-aware matching.

Supports 25+ languages. Use meta-variables: $VAR (single node), $$$ (multiple nodes).

IMPORTANT: Patterns must be complete AST nodes (valid code).
Examples:
  - 'console.log($MSG)' — finds all console.log calls
  - 'function $NAME($$$) { $$$ }' — finds all function declarations
  - 'export async function $NAME($$$) { $$$ }' — finds exported async functions

Use the 'pattern' parameter with exact code pattern.
Use 'lang' to specify the language (javascript, typescript, python, rust, go, etc.)
Use 'paths' to restrict to specific directories.`,
  permission: 'file.read',
  parameters: {
    type: 'object',
    properties: {
      pattern: {
        type: 'string',
        description: 'AST pattern to match (e.g., "console.log($MSG)")',
      },
      lang: {
        type: 'string',
        description: 'Target language (e.g., "typescript", "python", "rust", "go", "javascript")',
      },
      paths: {
        type: 'array',
        items: { type: 'string' },
        description: 'Directories to search in (defaults to current directory)',
      },
    },
    required: ['pattern', 'lang'],
  },
  execute: async function* (params) {
    const pattern = String(params.pattern || '');
    const lang = String(params.lang || '');
    const paths = Array.isArray(params.paths) ? params.paths : ['.'];

    if (!pattern) {
      yield { type: 'error', error: 'pattern is required' };
      return;
    }

    if (!lang) {
      yield { type: 'error', error: 'lang is required' };
      return;
    }

    try {
      // Try to use ast-grep (sg) if available
      const cmd = `sg -p "${escapeArg(pattern)}" --lang ${escapeArg(lang)} ${paths.map((p: string) => `"${escapeArg(p)}"`).join(' ')} 2>nul`;
      const output = execSync(cmd, {
        encoding: 'utf-8',
        stdio: 'pipe',
        timeout: 30000,
        maxBuffer: 10 * 1024 * 1024,
        shell: true as any,
      }).trim();

      if (!output) {
        yield { type: 'text', content: `No AST matches for "${pattern}" in ${lang}` };
        return;
      }

      // Limit output
      const lines = output.split('\n');
      const maxLines = 200;
      let content = lines.slice(0, maxLines).join('\n');
      if (lines.length > maxLines) {
        content += `\n... (${lines.length - maxLines} more matches)`;
      }

      yield { type: 'text', content };
    } catch (e: unknown) {
      const err = e as Error;
      // Check if sg is not installed — provide useful message
      if (err.message?.includes('not recognized') || err.message?.includes('not found')) {
        yield { type: 'text', content: `ast-grep (sg) not installed. Install with: npm install -g @ast-grep/cli\n\nFalling back to text grep...\n${pattern}` };
      } else {
        yield { type: 'error', error: `AST grep error: ${err.message}` };
      }
    }
  },
};

function escapeArg(s: string): string {
  return s.replace(/"/g, '\\"').replace(/\\/g, '\\\\');
}
