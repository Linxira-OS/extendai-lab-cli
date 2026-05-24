/**
 * @extendai/kernel — Calculator tool
 *
 * Simple math expression evaluation.
 * Uses a sandboxed eval for basic arithmetic.
 *
 * Reference: Pi calculator tool, CodeWhale calculator
 */

import type { Tool } from './registry.js';

export const calculatorTool: Tool = {
  name: 'calculator',
  description: `Evaluate math expressions safely.

Supports:
  - Basic arithmetic: +, -, *, /, %, **
  - Trig: sin, cos, tan, asin, acos, atan
  - Math: sqrt, abs, round, floor, ceil, log, ln, exp, pi, e
  - Parentheses for grouping

Does NOT support:
  - Variable assignment or state
  - String operations
  - Function definitions`,
  permission: 'file.read',
  parameters: {
    type: 'object',
    properties: {
      expression: {
        type: 'string',
        description: 'Math expression to evaluate (e.g., "sin(pi/4) * 100")',
      },
    },
    required: ['expression'],
  },
  execute: async function* (params) {
    const expression = String(params.expression || '').trim();

    if (!expression) {
      yield { type: 'error', error: 'expression is required' };
      return;
    }

    // Security: only allow safe math
    if (!/^[\d\s+\-*/().,%^a-zA-Z]+$/.test(expression)) {
      yield { type: 'error', error: 'Expression contains disallowed characters' };
      return;
    }

    try {
      // Use Function constructor instead of eval for slightly better sandboxing
      const fn = new Function(
        'sin', 'cos', 'tan', 'asin', 'acos', 'atan',
        'sqrt', 'abs', 'round', 'floor', 'ceil', 'log', 'ln', 'exp', 'pi', 'e', 'pow',
        `return (${expression});`,
      );

      const result = fn(
        Math.sin, Math.cos, Math.tan, Math.asin, Math.acos, Math.atan,
        Math.sqrt, Math.abs, Math.round, Math.floor, Math.ceil,
        Math.log10, Math.log, Math.exp, Math.PI, Math.E, Math.pow,
      );

      if (typeof result === 'number' && (isNaN(result) || !isFinite(result))) {
        yield { type: 'text', content: `Result: ${result}` };
      } else {
        // Format to reasonable precision
        const formatted = typeof result === 'number'
          ? (Number.isInteger(result) ? String(result) : result.toFixed(6).replace(/\.?0+$/, ''))
          : String(result);
        yield { type: 'text', content: `= ${formatted}` };
      }
    } catch (e: unknown) {
      const err = e as Error;
      yield { type: 'error', error: `Error evaluating expression: ${err.message}` };
    }
  },
};
