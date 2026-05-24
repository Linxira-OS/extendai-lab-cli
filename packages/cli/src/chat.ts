/**
 * @extendai/cli — Interactive chat loop
 *
 * readline-based terminal interface with streaming output,
 * slash commands, and session management.
 */

import * as readline from 'node:readline/promises';
import { stdin as input, stdout as output } from 'node:process';
import type { AppConfig } from '@extendai/kernel';
import { Session, streamCompletion } from '@extendai/kernel';

// ─── Main chat loop ───────────────────────────────────────

export async function startChat(config: AppConfig): Promise<void> {
  const session = new Session(
    config.provider.systemPrompt,
    config.provider.contextLength,
  );

  const rl = readline.createInterface({
    input,
    output,
    terminal: true,
  });

  // Register graceful exit
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
  console.log('  ╠══════════════════════════════════════════════╣');
  console.log('  ║  Type /help for commands                    ║');
  console.log('  ╚══════════════════════════════════════════════╝');
  console.log('');

  // ─── Main loop ───────────────────────────────────────

  while (true) {
    const rawInput = await rl.question('> ');
    const input_ = rawInput.trim();
    if (!input_) continue;

    // ── Slash commands ─────────────────────────────────

    if (input_.startsWith('/')) {
      const handled = await handleCommand(input_, session, rl, config);
      if (handled) continue;
    }

    // ── User message ───────────────────────────────────

    session.addMessage({ role: 'user', content: input_ });

    // Stream the response
    console.log('');
    let fullResponse = '';

    try {
      // Get trimmed message list for this request
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

// ─── Command handler ──────────────────────────────────────

async function handleCommand(
  input: string,
  session: Session,
  _rl: readline.Interface,
  config: AppConfig,
): Promise<boolean> {
  const parts = input.slice(1).split(/\s+/);
  const cmd = parts[0].toLowerCase();

  switch (cmd) {
    case 'help':
      console.log('');
      console.log('  Commands:');
      console.log('    /help              Show this help');
      console.log('    /clear             Clear conversation history');
      console.log('    /model  <name>     Switch model (e.g. /model gpt-4o-mini)');
      console.log('    /temp   <n>        Set temperature (e.g. /temp 0.5)');
      console.log('    /config            Show current configuration');
      console.log('    /exit              Exit (or Ctrl+C)');
      console.log('');
      return true;

    case 'clear':
      session.clear();
      console.log('  Conversation cleared.\n');
      return true;

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
