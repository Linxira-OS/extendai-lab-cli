/**
 * @extendai/kernel — TodoWrite tool
 *
 * Task list management for tracking multi-step work progress.
 * The main agent uses this to break down complex tasks and
 * track completion status.
 *
 * Reference: OpenCode todowrite, Pi todo_write
 *
 * States: pending, in_progress, completed, cancelled
 */

import type { Tool } from './registry.js';

type TodoStatus = 'pending' | 'in_progress' | 'completed' | 'canceled';

interface TodoItem {
  content: string;
  status: TodoStatus;
  priority: 'high' | 'medium' | 'low';
}

export const todoWriteTool: Tool = {
  name: 'todowrite',
  description: `Create and maintain a structured task list for the current coding session.

Use this when:
  - The task requires 3+ distinct steps
  - The work is non-trivial and benefits from planning
  - User provides multiple tasks or asks for a plan

Rules:
  - Keep exactly one 'in_progress' task at a time
  - Mark tasks as 'completed' only after the work is actually verified
  - If blocked, add a follow-up task describing the blocker
  - Items should be specific and actionable`,
  permission: 'file.read',
  parameters: {
    type: 'object',
    properties: {
      todos: {
        type: 'array',
        description: 'The updated list of all tasks with their status',
        items: {
          type: 'object',
          properties: {
            content: { type: 'string', description: 'Brief description of the task' },
            status: { type: 'string', description: 'Current status: pending, in_progress, completed, canceled', enum: ['pending', 'in_progress', 'completed', 'canceled'] },
            priority: { type: 'string', description: 'Priority: high, medium, low', enum: ['high', 'medium', 'low'] },
          },
          required: ['content', 'status', 'priority'],
        },
      },
    },
    required: ['todos'],
  },
  execute: async function* (params) {
    const todos = params.todos as TodoItem[] | undefined;

    if (!todos || !Array.isArray(todos) || todos.length === 0) {
      yield { type: 'error', error: 'todos array is required and must not be empty' };
      return;
    }

    // Validate statuses
    const validStatuses = new Set<TodoStatus>(['pending', 'in_progress', 'completed', 'canceled']);
    for (const t of todos) {
      if (!validStatuses.has(t.status as TodoStatus)) {
        yield { type: 'error', error: `Invalid status: '${t.status}'. Must be one of: pending, in_progress, completed, canceled` };
        return;
      }
    }

    // Count in_progress items
    const inProgress = todos.filter(t => t.status === 'in_progress');
    if (inProgress.length > 1) {
      yield { type: 'text', content: '⚠️ Multiple tasks marked in_progress\n\n' + formatTodos(todos) };
      return;
    }

    yield { type: 'text', content: formatTodos(todos) };
  },
};

function formatTodos(todos: TodoItem[]): string {
  const statusIcons: Record<TodoStatus, string> = {
    pending: '○',
    in_progress: '●',
    completed: '✅',
    canceled: '✕',
  };

  const priorities = { high: ' 🔥', medium: '', low: ' ↓' };

  return todos
    .map(t => `${statusIcons[t.status]} ${t.content}${priorities[t.priority]}`)
    .join('\n');
}
