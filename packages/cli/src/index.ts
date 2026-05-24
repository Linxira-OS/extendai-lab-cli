#!/usr/bin/env node

/**
 * @extendai/cli — CLI Entry Point
 *
 * Wires together kernel, config, worktree, snapshots, and interactive chat.
 * Usage:
 *   extendai                    Start interactive chat
 *   extendai --help             Show help
 *   extendai --version          Show version
 *   extendai --model <name>     Start with specific model
 *   extendai --init-git         Initialize git repo + .gitignore for current dir
 */

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, resolve } from 'node:path';
import {
  loadConfig,
  saveConfig,
  detectWorktree,
  ensureGit,
  ensureGitIgnore,
  SnapshotService,
} from '@extendai/kernel';
import { startChat } from './chat.js';

function getVersion(): string {
  try {
    const distPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../package.json');
    return JSON.parse(readFileSync(distPath, 'utf-8')).version;
  } catch {
    try {
      const srcPath = resolve(dirname(fileURLToPath(import.meta.url)), '../package.json');
      return JSON.parse(readFileSync(srcPath, 'utf-8')).version;
    } catch {
      return '0.0.1';
    }
  }
}

async function main(): Promise<void> {
  const args = process.argv.slice(2);
  const version = getVersion();
  const cwd = process.cwd();

  // ── Help ──────────────────────────────────────────────

  if (args.includes('--help') || args.includes('-h')) {
    console.log(`
  ExtendAI Lab CLI v${version} — AI assistant in your terminal

  Usage:
    extendai                     Start interactive chat
    extendai --help              Show this help
    extendai --version           Show version
    extendai --model <name>      Start with specific model
    extendai --init              Create default config at ~/.extendai/config.json
    extendai --init-git          Init git repo + .gitignore in current directory

  Configuration (priority: env > config file > defaults):
    EXTENDAI_API_KEY   API key (required for remote; localhost/LAN IPs auto-skip)
    EXTENDAI_BASE_URL  API base URL (default: https://api.openai.com/v1)
    EXTENDAI_MODEL     Model name (default: gpt-4o)

  Local providers:
    LM Studio:  set EXTENDAI_BASE_URL=http://192.168.x.x:1234/v1, key = model ID
    Ollama:     set EXTENDAI_BASE_URL=http://localhost:11434/v1, key = ollama

  Config file: ~/.extendai/config.json
    `);
    process.exit(0);
  }

  // ── Version ───────────────────────────────────────────

  if (args.includes('--version') || args.includes('-v')) {
    console.log(`extendai v${version}`);
    process.exit(0);
  }

  // ── Init config ───────────────────────────────────────

  if (args.includes('--init')) {
    const config = loadConfig();
    saveConfig(config);
    console.log('');
    console.log('  Default config written to ~/.extendai/config.json');
    console.log('  Set an API key and run:');
    console.log('    $env:EXTENDAI_API_KEY="sk-..."  # PowerShell');
    console.log('    extendai');
    console.log('');
    process.exit(0);
  }

  // ── Init git ──────────────────────────────────────────

  if (args.includes('--init-git')) {
    const created = ensureGit(cwd);
    ensureGitIgnore(cwd);
    console.log('');
    if (created) {
      console.log('  Git repository initialized.');
    } else {
      console.log('  Already a git repository.');
    }
    console.log('  .extendai/ added to .gitignore (snapshot data excluded from project git).');
    console.log('');
    process.exit(0);
  }

  // ── Load configuration ────────────────────────────────

  const config = loadConfig();

  const modelIdx = args.indexOf('--model');
  if (modelIdx !== -1 && args[modelIdx + 1]) {
    config.provider.model = args[modelIdx + 1];
  }

  // ── Validate API key (skip for local/LAN providers) ──
  const isLocalProvider = isPrivateUrl(config.provider.baseUrl);

  if (!config.provider.apiKey && !isLocalProvider) {
    console.error('');
    console.error('  Error: API key not set.');
    console.error('');
    console.error('  Set the EXTENDAI_API_KEY environment variable:');
    console.error('    $env:EXTENDAI_API_KEY="sk-..."  (PowerShell)');
    console.error('    export EXTENDAI_API_KEY="sk-..." (bash)');
    console.error('');
    console.error('  Or create a config file:');
    console.error('    extendai --init');
    console.error('    Then edit ~/.extendai/config.json and add your apiKey');
    console.error('');
    console.error('  Local provider (Ollama/LM Studio)? Set the model as API key:');
    console.error('    $env:EXTENDAI_API_KEY = $env:EXTENDAI_MODEL    # LM Studio: key = model ID');
    console.error('    $env:EXTENDAI_API_KEY = "ollama"               # Ollama: any dummy value');
    console.error('');
    process.exit(1);
  }

  // ── Detect worktree & init snapshots ──────────────────

  const worktree = detectWorktree(cwd);

  if (worktree.isGit) {
    try {
      new SnapshotService(worktree.root, worktree.id ?? 'default').init();
      ensureGitIgnore(worktree.root);
    } catch {
      // Non-fatal: snapshots are optional
    }
  }

  // ── Start chat ────────────────────────────────────────

  await startChat(config, worktree);
}

/**
 * Check if a URL points to a local/private network address.
 * Covers: localhost, 127.x, 10.x, 192.168.x, 172.16-31.x, ::1, 0.0.0.0
 */
function isPrivateUrl(url: string): boolean {
  const hostname = url
    .replace(/^https?:\/\//, '')
    .replace(/\/.*$/, '')
    .split(':')[0]
    .toLowerCase();

  // Named local addresses
  if (hostname === 'localhost' || hostname === '0.0.0.0' || hostname === '::1' || hostname === '[::1]') {
    return true;
  }

  // IPv4 private ranges
  if (/^(127\.\d{1,3}\.)/.test(hostname)) return true;
  if (/^(10\.\d{1,3}\.)/.test(hostname)) return true;
  if (/^(192\.168\.)/.test(hostname)) return true;
  if (/^(172\.(1[6-9]|2\d|3[01])\.)/.test(hostname)) return true;

  // Unix socket
  if (hostname.endsWith('.sock')) return true;

  return false;
}

main().catch((e) => {
  console.error('Fatal error:', e);
  process.exit(1);
});
