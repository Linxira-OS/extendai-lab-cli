/**
 * @extendai/kernel — Tool System
 *
 * Exports:
 *   - ToolRegistry: register/list/execute tools
 *   - built-in tools: bash, browser, question
 *   - types: Tool, ToolContext, ToolChunk, ToolDefinition
 */

export * from './registry.js';
export * from './bash.js';
export * from './browser.js';
export * from './question.js';
export * from './permission.js';
export * from './dangerous.js';
export * from './approval.js';

import { ToolRegistry } from './registry.js';
import { bashTool } from './bash.js';
import { browserTool } from './browser.js';
import { createQuestionTool } from './question.js';

/**
 * Create the default tool registry with all built-in tools.
 */
export function createDefaultRegistry(): ToolRegistry {
  const registry = new ToolRegistry();
  registry.register(bashTool);
  registry.register(browserTool);
  registry.register(createQuestionTool());
  return registry;
}
