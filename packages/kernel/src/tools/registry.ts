/**
 * @extendai/kernel — Tool Registry
 *
 * Central registry for all tools. Tools are self-describing:
 * each provides name, description, JSON Schema parameters,
 * and an execute() generator.
 *
 * The registry can produce OpenAI-style tool definitions
 * for LLM function calling.
 */

import type { Session } from '../session.js';
import type { WorktreeInfo } from '../worktree.js';
import type { AppConfig } from '../types.js';
import type { PermissionSpec } from './permission.js';

// ─── Types ────────────────────────────────────────────────

export interface ToolParameterSchema {
  type: 'object';
  properties: Record<string, SchemaProperty>;
  required?: string[];
}

export interface SchemaProperty {
  type: string;
  description?: string;
  enum?: string[];
  default?: unknown;
  items?: SchemaProperty;
  properties?: Record<string, SchemaProperty>;
  required?: string[];
}

export type ToolChunk =
  | { type: 'text'; content: string }
  | { type: 'error'; error: string }
  | { type: 'progress'; message: string }
  | { type: 'done'; result?: unknown };

export interface ToolContext {
  session: Session;
  worktree: WorktreeInfo;
  config: AppConfig;
  signal?: AbortSignal;
}

export interface ToolDefinition {
  name: string;
  description: string;
  parameters: ToolParameterSchema;
}

export interface Tool {
  name: string;
  description: string;
  parameters: ToolParameterSchema;
  /** 权限声明：字符串（如 "file.read"）或规则数组 */
  permission: PermissionSpec;
  execute(params: Record<string, unknown>, ctx: ToolContext): AsyncGenerator<ToolChunk>;
}

// ─── Registry ─────────────────────────────────────────────

export class ToolRegistry {
  private tools = new Map<string, Tool>();

  register(tool: Tool): void {
    this.tools.set(tool.name, tool);
  }

  unregister(name: string): boolean {
    return this.tools.delete(name);
  }

  get(name: string): Tool | undefined {
    return this.tools.get(name);
  }

  list(): Tool[] {
    return Array.from(this.tools.values());
  }

  /** Format tools as OpenAI-compatible function definitions for LLM tool calling */
  toOpenAITools(): Array<{
    type: 'function';
    function: {
      name: string;
      description: string;
      parameters: ToolParameterSchema;
    };
  }> {
    return this.list().map((tool) => ({
      type: 'function' as const,
      function: {
        name: tool.name,
        description: tool.description,
        parameters: tool.parameters,
      },
    }));
  }

  /** Execute a tool by name, returns async generator */
  async *execute(
    name: string,
    params: Record<string, unknown>,
    ctx: ToolContext,
  ): AsyncGenerator<ToolChunk> {
    const tool = this.tools.get(name);
    if (!tool) {
      yield { type: 'error', error: `Unknown tool: ${name}` };
      return;
    }
    yield* tool.execute(params, ctx);
  }
}
