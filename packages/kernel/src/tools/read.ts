/**
 * @extendai/kernel — Read tool
 *
 * File/directory reading with offset, limit, binary detection.
 * Reference: Pi read.ts, OpenCode read tool, CodeX file reading.
 *
 * Behaviour:
 *   - Files: reads text content with optional offset/limit
 *   - Directories: lists entries (no line numbers)
 *   - Binary detection: reads as text by default; marks binary if detected
 *   - Readable binary (PDF/DOCX/XLSX/PPTX): returns path for tool dispatch
 *   - File not found: returns clear error
 */

import { readFileSync, existsSync, statSync } from 'node:fs';
import { readdirSync } from 'node:fs';
import { extname, join } from 'node:path';
import type { Tool } from './registry.js';

const BINARY_EXTS = new Set([
  '.png', '.jpg', '.jpeg', '.gif', '.webp', '.svg', '.bmp', '.ico',
  '.woff', '.woff2', '.ttf', '.otf', '.eot',
  '.zip', '.tar', '.gz', '.7z', '.rar',
  '.exe', '.dll', '.so', '.dylib', '.wasm',
  '.o', '.a', '.lib', '.obj',
  '.pkl', '.h5', '.hdf5', '.npy', '.npz', '.parquet',
  '.db', '.sqlite',
]);

const DOCUMENT_EXTS = new Set([
  '.pdf', '.docx', '.xlsx', '.pptx',
]);

export const readTool: Tool = {
  name: 'read',
  description: `Read a file or directory from the local filesystem.

If the path does not exist, an error is returned.

Usage:
  - By default, returns up to 2000 lines from the start of a file.
  - Use offset to start from a specific line (1-indexed).
  - Use limit to control how many lines to read.
  - If reading a directory, lists entries with trailing / for subdirectories.
  - Binary files (.png, .pdf, .docx, etc.) are detected and reported.
  - For PDF/DOCX/XLSX/PPTX: returns the file path so a dedicated tool can handle it.`,
  permission: 'file.read',
  parameters: {
    type: 'object',
    properties: {
      filePath: {
        type: 'string',
        description: 'Absolute path to the file or directory to read',
      },
      offset: {
        type: 'number',
        description: 'Line number to start reading from (1-indexed, default 1)',
      },
      limit: {
        type: 'number',
        description: 'Maximum number of lines to read (default 2000 for files)',
      },
    },
    required: ['filePath'],
  },
  execute: async function* (params, ctx) {
    const filePath = String(params.filePath || '');

    if (!filePath) {
      yield { type: 'error', error: 'filePath is required' };
      return;
    }

    if (!existsSync(filePath)) {
      yield { type: 'error', error: `Path not found: ${filePath}` };
      return;
    }

    const stat = statSync(filePath);

    // ── Directory listing ──
    if (stat.isDirectory()) {
      const entries = readdirSync(filePath, { withFileTypes: true });

      let output = '';
      for (const entry of entries) {
        output += entry.name + (entry.isDirectory() ? '/' : '') + '\n';
      }

      yield { type: 'text', content: output.trim() || '(empty directory)' };
      return;
    }

    // ── Binary detection ──
    const ext = extname(filePath).toLowerCase();
    if (BINARY_EXTS.has(ext)) {
      yield {
        type: 'text',
        content: `[Binary file: ${filePath} (${formatSize(stat.size)})]`,
      };
      return;
    }

    // ── Document formats (needs dedicated tool) ──
    if (DOCUMENT_EXTS.has(ext)) {
      yield {
        type: 'text',
        content: `[Document: ${filePath} (${formatSize(stat.size)}) — use dedicated tool to extract text]`,
      };
      return;
    }

    // ── Text file reading ──
    const offset = Number(params.offset) || 1;
    const defaultLimit = 2000;
    const limit = params.limit !== undefined ? Number(params.limit) : defaultLimit;

    try {
      const content = readFileSync(filePath, 'utf-8');
      const lines = content.split('\n');

      if (offset > lines.length && offset > 1) {
        yield { type: 'error', error: `Offset ${offset} exceeds file length (${lines.length} lines)` };
        return;
      }

      const start = Math.max(0, offset - 1);
      const end = Math.min(lines.length, start + limit);
      const slice = lines.slice(start, end);

      let output = '';
      for (let i = 0; i < slice.length; i++) {
        output += `${start + i + 1}: ${slice[i]}\n`;
      }

      // Truncation notice
      if (end < lines.length) {
        output += `\n... (showing ${limit} of ${lines.length} lines, use offset=${end + 1} to continue)`;
      }

      yield { type: 'text', content: output };
    } catch (e: unknown) {
      const err = e as Error;
      yield { type: 'error', error: `Error reading file: ${err.message}` };
    }
  },
};

function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
}
