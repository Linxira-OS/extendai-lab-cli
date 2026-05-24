/**
 * @extendai/kernel — Question Tool
 *
 * Asks the user a question with selectable options.
 * Supports single/multi select, each option can have
 * a custom free-text input fallback.
 *
 * The TUI/Web UI renders this as an interactive prompt.
 * In CLI mode (no TUI), falls back to readline prompt.
 */

import * as readline from 'node:readline/promises';
import { stdin as input, stdout as output } from 'node:process';
import {
  type Tool,
  type ToolChunk,
} from './registry.js';

export interface QuestionOption {
  label: string;
  description?: string;
  /** Allow free-text custom input after this option. Default: true */
  custom?: boolean;
}

export interface QuestionHandler {
  (params: QuestionParams): Promise<QuestionResult>;
}

export interface QuestionParams {
  question: string;
  header?: string;
  options: QuestionOption[];
  multiple?: boolean;
}

export interface QuestionResult {
  selected: string[];
  custom?: string;
}

// ─── Default handler (CLI fallback) ───────────────────────

export async function askQuestion(params: QuestionParams): Promise<QuestionResult> {
  const rl = readline.createInterface({ input, output });

  try {
    console.log('');
    if (params.header) {
      console.log(`  ${params.header}`);
      console.log('');
    }
    console.log(`  ${params.question}`);
    console.log('');

    const selected: string[] = [];
    const maxIdx = params.options.length;

    if (params.multiple) {
      // Multi-select: user types comma-separated numbers
      for (let i = 0; i < params.options.length; i++) {
        const opt = params.options[i];
        const desc = opt.description ? ` — ${opt.description}` : '';
        console.log(`  [${i + 1}] ${opt.label}${desc}`);
      }
      console.log(`  [${maxIdx + 1}] ── Custom input ──`);

      const answer = await rl.question('  Enter numbers (comma-separated, e.g. "1,3"): ');
      const parts = answer.split(',').map((s) => parseInt(s.trim(), 10)).filter((n) => !isNaN(n));

      for (const num of parts) {
        if (num >= 1 && num <= maxIdx) {
          selected.push(params.options[num - 1].label);
        } else if (num === maxIdx + 1) {
          // Custom input
        }
      }
    } else {
      // Single select
      for (let i = 0; i < params.options.length; i++) {
        const opt = params.options[i];
        const desc = opt.description ? ` — ${opt.description}` : '';
        const customHint = opt.custom !== false ? ' (or type your own)' : '';
        console.log(`  ${i + 1}. ${opt.label}${desc}${customHint}`);
      }

      const answer = await rl.question('  Your choice: ');
      const num = parseInt(answer.trim(), 10);

      if (!isNaN(num) && num >= 1 && num <= maxIdx) {
        selected.push(params.options[num - 1].label);
      } else {
        // Treat as custom input
        if (answer.trim()) {
          selected.push(answer.trim());
        }
      }
    }

    // If nothing selected, ask for custom input
    if (selected.length === 0) {
      const custom = await rl.question('  Custom input (optional): ');
      return { selected: [], custom: custom.trim() || undefined };
    }

    return { selected, custom: undefined };
  } finally {
    rl.close();
  }
}

// ─── Tool definition ──────────────────────────────────────

export function createQuestionTool(handler?: QuestionHandler): Tool {
  const ask = handler ?? askQuestion;

  return {
    name: 'question',
    permission: { permission: 'question', pattern: '*', action: 'allow' },
    description: `Ask the user a question with selectable options.
Supports single choice (default) or multiple choice.
Each option may also accept custom free-text input.
Use when you need clarification, decisions, or preferences from the user.
DO NOT guess — if you are unsure, ask.`,
    parameters: {
      type: 'object',
      properties: {
        question: {
          type: 'string',
          description: 'The complete question to ask',
        },
        header: {
          type: 'string',
          description: 'Short label for the question group (max 30 chars)',
        },
        options: {
          type: 'array',
          description: 'Available choices (each option also allows custom free-text input)',
          items: {
            type: 'object',
            properties: {
              label: { type: 'string', description: 'Display text (1-5 words)' },
              description: { type: 'string', description: 'Optional explanation' },
              custom: { type: 'boolean', description: 'Allow custom input for this option (default: true)' },
            },
            required: ['label'],
          },
        },
        multiple: {
          type: 'boolean',
          description: 'Allow selecting multiple choices (default: false)',
        },
      },
      required: ['question', 'options'],
    },

    async *execute(params, _ctx): AsyncGenerator<ToolChunk> {
      const question = String(params.question ?? '');
      const header = params.header ? String(params.header) : undefined;
      const options = Array.isArray(params.options)
        ? params.options.map((o: any) => ({
            label: String(o.label ?? ''),
            description: o.description ? String(o.description) : undefined,
            custom: o.custom !== false,
          }))
        : [];
      const multiple = Boolean(params.multiple);

      if (!question || options.length === 0) {
        yield { type: 'error', error: 'Question and options are required' };
        return;
      }

      yield { type: 'progress', message: `Asking user: ${header || question.slice(0, 60)}...` };

      try {
        const result = await ask({ question, header, options, multiple });
        yield { type: 'text', content: JSON.stringify(result) };
        yield { type: 'done', result };
      } catch (e: unknown) {
        const msg = e instanceof Error ? e.message : String(e);
        yield { type: 'error', error: `Question failed: ${msg}` };
      }
    },
  };
}
