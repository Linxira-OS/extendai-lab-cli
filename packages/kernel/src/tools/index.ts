/**
 * @extendai/kernel — Tool System
 *
 * Exports:
 *   - ToolRegistry: register/list/execute tools
 *   - All built-in tools
 *   - All types
 *
 * Total tools (14):
 *   P0: bash, browser, question, read, write, edit, glob, grep
 *   P1: find, ls, todowrite, calculator, ast_grep, web_search
 */

export * from './registry.js';
export * from './bash.js';
export * from './browser.js';
export * from './question.js';
export * from './permission.js';
export * from './dangerous.js';
export * from './approval.js';
export * from './read.js';
export * from './write.js';
export * from './edit.js';
export * from './glob.js';
export * from './grep.js';
export * from './find.js';
export * from './ls.js';
export * from './todowrite.js';
export * from './calculator.js';
export * from './ast-grep.js';
export * from './websearch.js';

import { ToolRegistry } from './registry.js';
import { bashTool } from './bash.js';
import { browserTool } from './browser.js';
import { createQuestionTool } from './question.js';
import { readTool } from './read.js';
import { writeTool } from './write.js';
import { editTool } from './edit.js';
import { globTool } from './glob.js';
import { grepTool } from './grep.js';
import { findTool } from './find.js';
import { lsTool } from './ls.js';
import { todoWriteTool } from './todowrite.js';
import { calculatorTool } from './calculator.js';
import { astGrepTool } from './ast-grep.js';
import { webSearchTool } from './websearch.js';

/**
 * Create the default tool registry with all built-in tools.
 */
export function createDefaultRegistry(): ToolRegistry {
  const registry = new ToolRegistry();
  // P0 — core tools
  registry.register(bashTool);
  registry.register(browserTool);
  registry.register(createQuestionTool());
  registry.register(readTool);
  registry.register(writeTool);
  registry.register(editTool);
  registry.register(globTool);
  registry.register(grepTool);
  // P1 — utility tools
  registry.register(findTool);
  registry.register(lsTool);
  registry.register(todoWriteTool);
  registry.register(calculatorTool);
  registry.register(astGrepTool);
  registry.register(webSearchTool);
  return registry;
}
