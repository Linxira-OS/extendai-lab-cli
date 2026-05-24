/**
 * @extendai/cli — Interactive chat loop
 *
 * readline-based terminal interface with:
 *   • Streaming output
 *   • Slash commands (/undo, /snapshot, /fork, etc.)
 *   • Dialog history (up/down arrows, last 10)
 *   • Session auto-naming after 3 turns
 *   • Undo checkpoints before each user message
 *   • Snapshot integration
 */

import * as readline from 'node:readline/promises';
import * as readlineSync from 'node:readline';
import { stdin as input, stdout as output } from 'node:process';
import type { AppConfig, WorktreeInfo } from '@extendai/kernel';
import {
  Session,
  streamCompletion,
  takeSnapshot,
  listSnapshots,
  snapshotDiff,
} from '@extendai/kernel';

// ─── Dialog history ────────────────────────────────────────

const MAX_HISTORY = 10;
const dialogHistory: string[] = [];
let historyIndex = -1;

// ─── Main chat loop ────────────────────────────────────────

export async function startChat(config: AppConfig, worktree: WorktreeInfo): Promise<void> {
  const session = new Session(
    config.provider.systemPrompt,
    config.provider.contextLength,
    { branch: worktree.branch, worktreeId: worktree.id },
  );

  const rl = readline.createInterface({
    input,
    output,
    terminal: true,
  });

  // Override readline history — we manage our own
  (rl as any).history = [];
  (rl as any).historyIndex = -1;

  // Capture up/down for custom dialog history
  input.on('keypress', (_str: string, key: { name: string; ctrl: boolean }) => {
    if (key.name === 'up') {
      if (historyIndex < dialogHistory.length - 1) {
        historyIndex++;
        // Clear current line and write history item
        readlineSync.clearLine(output, 0);
        readlineSync.cursorTo(output, 0);
        output.write(`> ${dialogHistory[dialogHistory.length - 1 - historyIndex]}`);
      }
    } else if (key.name === 'down') {
      if (historyIndex > 0) {
        historyIndex--;
        readlineSync.clearLine(output, 0);
        readlineSync.cursorTo(output, 0);
        output.write(`> ${dialogHistory[dialogHistory.length - 1 - historyIndex]}`);
      } else if (historyIndex === 0) {
        historyIndex = -1;
        readlineSync.clearLine(output, 0);
        readlineSync.cursorTo(output, 0);
        output.write('> ');
      }
    }
  });

  const cleanup = () => {
    rl.close();
    process.exit(0);
  };
  process.on('SIGINT', cleanup);
  process.on('SIGTERM', cleanup);

  // Banner
  const model = config.provider.model;
  const ctx = (config.provider.contextLength / 1000).toFixed(0);
  const outMax = (config.provider.maxTokens / 1000).toFixed(0);
  const keyHint = config.provider.apiKey
    ? `***${config.provider.apiKey.slice(-4)}`
    : '(not set)';

  console.log('');
  console.log('  ╔══════════════════════════════════════════════╗');
  console.log('  ║         ExtendAI Lab — Interactive CLI       ║');
  console.log('  ╠══════════════════════════════════════════════╣');
  console.log(`  ║  Model:   ${model.padEnd(34)}║`);
  console.log(`  ║  Context: ${ctx}K tokens                     ║`);
  console.log(`  ║  Output:  ${outMax}K max tokens                ║`);
  console.log(`  ║  API Key: ${keyHint.padEnd(34)}║`);
  if (worktree.isGit) {
    console.log(`  ║  Branch:  ${worktree.label.padEnd(34)}║`);
  }
  console.log('  ╠══════════════════════════════════════════════╣');
  console.log('  ║  Type /help for commands                    ║');
  console.log('  ╚══════════════════════════════════════════════╝');
  console.log('');

  // ─── Main loop ────────────────────────────────────────

  while (true) {
    const prompt = `[${session.displayName}] > `;
    const rawInput = await rl.question(prompt);
    const input_ = rawInput.trim();
    if (!input_) continue;

    // ── Slash commands ─────────────────────────────────
    if (input_.startsWith('/')) {
      const handled = await handleCommand(input_, session, rl, config, worktree);
      if (handled) continue;
    }

    // Save to dialog history
    if (dialogHistory.length >= MAX_HISTORY) {
      dialogHistory.shift();
    }
    dialogHistory.push(input_);
    historyIndex = -1;

    // Save undo checkpoint BEFORE processing
    session.saveCheckpoint(input_.slice(0, 40));

    // ── User message ───────────────────────────────────
    session.addMessage({ role: 'user', content: input_ });

    // Auto-name after 3 turns
    session.tryAutoName({ branch: worktree.branch, worktreeId: worktree.id });

    // Stream the response
    console.log('');
    let fullResponse = '';

    try {
      const messages = session.getMessages();

      for await (const chunk of streamCompletion(messages, config.provider)) {
        if (chunk.type === 'content' && chunk.content) {
          output.write(chunk.content);
          fullResponse += chunk.content;
        } else if (chunk.type === 'error') {
          console.error(`\n  Error: ${chunk.error}`);
        } else if (chunk.type === 'done') {
          if (chunk.usage) {
            console.log('');
            console.log(
              `  [usage: ${chunk.usage.promptTokens}↑ ${chunk.usage.completionTokens}↓ total ${chunk.usage.totalTokens}]`,
            );
          }
        }
      }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : String(e);
      console.error(`\n  Error: ${msg}`);
    }

    if (fullResponse) {
      session.addMessage({ role: 'assistant', content: fullResponse });
    }

    console.log('');
  }
}

// ─── Command handler ───────────────────────────────────────

async function handleCommand(
  input: string,
  session: Session,
  _rl: readline.Interface,
  config: AppConfig,
  worktree: WorktreeInfo,
): Promise<boolean> {
  const parts = input.slice(1).split(/\s+/);
  const cmd = parts[0].toLowerCase();

  switch (cmd) {
    case 'help':
      console.log('');
      console.log('  Commands:');
      console.log('    /help              Show this help');
      console.log('    /clear             Clear conversation history');
      console.log('    /undo              Undo last exchange (rollback messages)');
      console.log('    /snapshot          Take a file snapshot (git-based)');
      console.log('    /snapshots         List recent snapshots');
      console.log('    /model  <name>     Switch model (e.g. /model gpt-4o-mini)');
      console.log('    /temp   <n>        Set temperature (e.g. /temp 0.5)');
      console.log('    /config            Show current configuration');
      console.log('    /fork              Fork a child session (branch)');
      console.log('    /session           Show session info');
      console.log('    /exit              Exit (or Ctrl+C)');
      console.log('');
      return true;

    case 'clear':
      session.clear();
      console.log('  Conversation cleared.\n');
      return true;

    case 'undo': {
      const result = session.undo();
      if (result) {
        console.log(`  Undone. ${session.undoCount} checkpoint(s) remaining.\n`);
      } else {
        console.log('  Nothing to undo.\n');
      }
      return true;
    }

    case 'snapshot': {
      if (!worktree.isGit) {
        console.log('  Snapshots require a git repository. Run: extendai --init-git\n');
        return true;
      }
      const msg = parts.slice(1).join(' ') || session.displayName;
      try {
        const hash = takeSnapshot(worktree.root, session.meta.id, msg);
        if (hash) {
          console.log(`  Snapshot taken: ${hash}\n`);
        } else {
          console.log('  No changes to snapshot.\n');
        }
      } catch (e: unknown) {
        const err = e instanceof Error ? e.message : String(e);
        console.log(`  Snapshot failed: ${err}\n`);
      }
      return true;
    }

    case 'snapshots': {
      if (!worktree.isGit) {
        console.log('  No snapshots (not a git repository).\n');
        return true;
      }
      const snaps = listSnapshots(worktree.root, 10);
      if (snaps.length === 0) {
        console.log('  No snapshots yet.\n');
      } else {
        console.log('');
        for (const s of snaps) {
          console.log(`  ${s.hash}  ${s.date.slice(0, 19)}  ${s.message.slice(0, 60)}`);
        }
        console.log('');
      }
      return true;
    }

    case 'model':
      if (parts[1]) {
        config.provider.model = parts[1];
        console.log(`  Model switched to: ${parts[1]}\n`);
      } else {
        console.log(`  Current model: ${config.provider.model}\n`);
      }
      return true;

    case 'temp':
      if (parts[1]) {
        const t = parseFloat(parts[1]);
        if (!isNaN(t) && t >= 0 && t <= 2) {
          config.provider.temperature = t;
          console.log(`  Temperature set to: ${t}\n`);
        } else {
          console.log('  Temperature must be between 0 and 2.\n');
        }
      } else {
        console.log(`  Current temperature: ${config.provider.temperature}\n`);
      }
      return true;

    case 'config':
      console.log('');
      console.log(`  Model:          ${config.provider.model}`);
      console.log(`  Base URL:       ${config.provider.baseUrl}`);
      console.log(
        `  API Key:        ${config.provider.apiKey ? `***${config.provider.apiKey.slice(-4)}` : '(not set)'}`,
      );
      console.log(`  Context:        ${(config.provider.contextLength / 1000).toFixed(0)}K tokens`);
      console.log(`  Max output:     ${(config.provider.maxTokens / 1000).toFixed(0)}K tokens`);
      console.log(`  Temperature:    ${config.provider.temperature}`);
      console.log(`  System prompt:  ${config.provider.systemPrompt.slice(0, 60)}...`);
      console.log(`  Messages:       ${session.messageCount}`);
      console.log(`  Est. tokens:    ${session.estimatedTokens}`);
      console.log(`  Undo steps:     ${session.undoCount}`);
      console.log(`  Branch:         ${worktree.label}`);
      console.log('');
      return true;

    case 'fork': {
      const child = session.fork(config.provider.contextLength);
      console.log(`  Forked session: ${child.displayName} (${child.meta.id})\n`);
      return true;
    }

    case 'session':
      console.log('');
      console.log(`  Name:       ${session.displayName}`);
      console.log(`  ID:         ${session.meta.id}`);
      console.log(`  Turns:      ${session.meta.turnCount}`);
      console.log(`  Messages:   ${session.messageCount}`);
      console.log(`  Est. tokens: ${session.estimatedTokens}`);
      console.log(`  Undo steps:  ${session.undoCount}`);
      console.log(`  Children:    ${session.children.length}`);
      console.log(`  Created:    ${session.meta.createdAt}`);
      console.log('');
      return true;

    case 'exit':
    case 'quit':
      process.exit(0);

    default:
      console.log(`  Unknown command: /${cmd}. Type /help for available commands.\n`);
      return true;
  }

  return false;
}
