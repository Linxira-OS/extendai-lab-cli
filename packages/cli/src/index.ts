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
  initSnapshotRepo,
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
    EXTENDAI_API_KEY   API key (required)
    EXTENDAI_BASE_URL  API base URL (default: https://api.openai.com/v1)
    EXTENDAI_MODEL     Model name (default: gpt-4o)

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

  // ── Validate API key (skip for known local providers) ──
  const isLocalProvider =
    config.provider.baseUrl.includes('localhost') ||
    config.provider.baseUrl.includes('127.0.0.1') ||
    config.provider.baseUrl.includes('0.0.0.0');

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
    console.error('  Local provider detected? Set EXTENDAI_BASE_URL and EXTENDAI_API_KEY=ollama');
    console.error('');
    process.exit(1);
  }

  // ── Detect worktree & init snapshots ──────────────────

  const worktree = detectWorktree(cwd);

  if (worktree.isGit) {
    try {
      initSnapshotRepo(worktree.root);
      ensureGitIgnore(worktree.root);
    } catch {
      // Non-fatal: snapshots are optional
    }
  }

  // ── Start chat ────────────────────────────────────────

  await startChat(config, worktree);
}

main().catch((e) => {
  console.error('Fatal error:', e);
  process.exit(1);
});
