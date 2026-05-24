/**
 * @extendai/kernel — Bash/Shell Tool
 *
 * Cross-platform shell execution:
 *   - Windows: PowerShell 7+ (pwsh) > powershell.exe > git bash
 *   - Linux/macOS: /bin/bash
 *   - Fallback: /bin/sh
 *
 * Features:
 *   - Timeout with automatic SIGKILL
 *   - Output truncation (max 2000 lines)
 *   - Large output redirected to temp file
 */

import { execSync, spawn } from 'node:child_process';
import { writeFileSync, unlinkSync, mkdtempSync, realpathSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import {
  type Tool,
  type ToolChunk,
  type ToolContext,
} from './registry.js';

const MAX_OUTPUT_LINES = 2000;
const DEFAULT_TIMEOUT = 120_000; // 2 minutes
const MAX_OUTPUT_SIZE = 51200; // 50KB before truncation

// ─── Platform detection ───────────────────────────────────

interface ShellConfig {
  shell: string;
  shellArgs: string[];
  platform: string;
}

function detectShell(): ShellConfig {
  const plat = process.platform;

  if (plat === 'win32') {
    // Try PowerShell 7+ first, fall back to Windows PowerShell, then git bash
    try {
      execSync('where pwsh', { stdio: 'ignore' });
      return { shell: 'pwsh', shellArgs: ['-NoProfile', '-Command', '-'], platform: 'windows' };
    } catch {
      // pwsh not found, try powershell.exe
    }
    try {
      execSync('where powershell.exe', { stdio: 'ignore' });
      return { shell: 'powershell.exe', shellArgs: ['-NoProfile', '-Command', '-'], platform: 'windows' };
    } catch {
      // Windows PowerShell not found, try git bash
    }
    try {
      execSync('where bash', { stdio: 'ignore' });
      return { shell: 'bash', shellArgs: ['--norc', '--noprofile', '-c'], platform: 'windows' };
    } catch {
      return { shell: 'pwsh', shellArgs: ['-NoProfile', '-Command', '-'], platform: 'windows' };
    }
  }

  if (plat === 'linux' || plat === 'darwin') {
    return { shell: '/bin/bash', shellArgs: ['--norc', '--noprofile', '-c'], platform: plat };
  }

  return { shell: '/bin/sh', shellArgs: ['-c'], platform: 'unknown' };
}

// ─── Execution ────────────────────────────────────────────

async function* runCommand(
  command: string,
  ctx: ToolContext,
  timeout: number,
  workdir?: string,
): AsyncGenerator<ToolChunk> {
  const shell = detectShell();
  const cwd = workdir || process.cwd();

  yield { type: 'progress', message: `[${shell.platform}] ${command.slice(0, 80)}${command.length > 80 ? '...' : ''}` };

  // For simple commands, use execSync which is more reliable
  // For complex/long ones, use spawn with streaming
  const isShortCommand = command.length < 200 && !command.includes('\n');

  if (isShortCommand) {
    try {
      const result = execSync(command, {
        cwd,
        encoding: 'utf-8',
        timeout,
        maxBuffer: 10 * 1024 * 1024, // 10MB
        windowsHide: true,
      } as any);

      // Handle output line limiting
      const lines = result.split('\n');
      if (lines.length > MAX_OUTPUT_LINES) {
        const head = lines.slice(0, 50).join('\n');
        const tail = lines.slice(-50).join('\n');
        yield {
          type: 'text',
          content:
            `${head}\n... (${lines.length - 100} lines truncated)\n${tail}`,
        };
      } else if (result.length > MAX_OUTPUT_SIZE) {
        yield {
          type: 'text',
          content: result.slice(0, MAX_OUTPUT_SIZE) +
            `\n... (output truncated at ${MAX_OUTPUT_SIZE} chars)`,
        };
      } else {
        yield { type: 'text', content: result || '(empty output)' };
      }
    } catch (e: unknown) {
      const err = e as Error & { stdout?: string; stderr?: string; status?: number };
      const msg = err.stderr || err.message || String(e);
      if (err.stdout) {
        yield { type: 'text', content: err.stdout };
      }
      yield { type: 'error', error: `Exit code ${err.status ?? -1}: ${msg.slice(0, 1000)}` };
      return;
    }
  } else {
    // Long-running command - use spawn with streaming
    const tempDir = mkdtempSync(join(tmpdir(), 'extendai-'));
    const outFile = join(tempDir, 'output.txt');
    const errFile = join(tempDir, 'error.txt');

    try {
      const child = spawn(shell.shell, [...shell.shellArgs, command], {
        cwd,
        shell: true,
        windowsHide: true,
        env: { ...process.env },
        stdio: ['ignore', 'pipe', 'pipe'],
      });

      const outChunks: Buffer[] = [];
      const errChunks: Buffer[] = [];

      child.stdout!.on('data', (chunk: Buffer) => {
        outChunks.push(chunk);
        // Flush to temp file if accumulating
        if (outChunks.length * chunk.length > MAX_OUTPUT_SIZE) {
          writeFileSync(outFile, Buffer.concat(outChunks));
        }
      });

      child.stderr!.on('data', (chunk: Buffer) => {
        errChunks.push(chunk);
      });

      const exitCode = await new Promise<number>((resolve) => {
        const timer = setTimeout(() => {
          child.kill('SIGKILL');
          resolve(-1);
        }, timeout);

        child.on('exit', (code) => {
          clearTimeout(timer);
          resolve(code ?? -1);
        });

        child.on('error', () => {
          clearTimeout(timer);
          resolve(-2);
        });
      });

      // Collect output
      const fullOut = Buffer.concat(outChunks).toString('utf-8');
      const fullErr = Buffer.concat(errChunks).toString('utf-8');

      if (fullOut) {
        const lines = fullOut.split('\n');
        if (lines.length > MAX_OUTPUT_LINES) {
          const head = lines.slice(0, 50).join('\n');
          const tail = lines.slice(-50).join('\n');
          yield {
            type: 'text',
            content: `${head}\n... (${lines.length - 100} lines truncated, see temp file: ${outFile})\n${tail}`,
          };
        } else {
          yield { type: 'text', content: fullOut };
        }
      }

      if (exitCode !== 0) {
        const errMsg = fullErr || `Process exited with code ${exitCode}`;
        yield { type: 'error', error: errMsg.slice(0, 2000) };
        return;
      }
    } finally {
      // Clean up temp files if they're small, leave if large for manual inspection
      try {
        unlinkSync(outFile);
      } catch { /* ignore */ }
      try {
        unlinkSync(errFile);
      } catch { /* ignore */ }
      try {
        unlinkSync(tempDir);
      } catch { /* ignore */ }
    }
  }

  yield { type: 'done' };
}

// ─── Tool definition ──────────────────────────────────────

export const bashTool: Tool = {
  name: 'bash',
  description: `Execute shell commands. Cross-platform: PowerShell on Windows, bash on Linux/macOS.
Use for running scripts, building projects, git operations, and any terminal tasks.
Include a clear 5-10 word description of the command for logging.`,
  permission: 'shell',
  parameters: {
    type: 'object',
    properties: {
      command: {
        type: 'string',
        description: 'The shell command to execute',
      },
      description: {
        type: 'string',
        description: 'Brief 5-10 word description of what this command does',
      },
      timeout: {
        type: 'number',
        description: 'Timeout in milliseconds (default: 120000)',
        default: DEFAULT_TIMEOUT,
      },
      workdir: {
        type: 'string',
        description: 'Working directory (default: current project root)',
      },
    },
    required: ['command'],
  },

  async *execute(params, ctx): AsyncGenerator<ToolChunk> {
    const command = String(params.command ?? '');
    const timeout = Number(params.timeout) || DEFAULT_TIMEOUT;
    const workdir = params.workdir ? String(params.workdir) : undefined;

    if (!command.trim()) {
      yield { type: 'error', error: 'Empty command' };
      return;
    }

    yield* runCommand(command, ctx, timeout, workdir);
  },
};
