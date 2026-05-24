/**
 * @extendai/kernel — Permission Guard
 *
 * Rules engine for tool permission check.
 * Each tool declares a permission type; the guard matches
 * built-in + user rules to decide: allow / deny / ask.
 *
 * Reference: OpenCode permission/index.ts, CodeX execpolicy/
 */

import type { Tool } from './registry.js';

// ─── Types ────────────────────────────────────────────────

export type PermissionAction = 'allow' | 'deny' | 'ask';

export interface PermissionRule {
  permission: string;        // "file.read" | "file.write" | "shell" | "destructive"
  pattern: string;           // glob 匹配
  action: PermissionAction;  // 决策
}

export interface PermissionResult {
  action: PermissionAction;
  rule: PermissionRule | null;   // 匹配的规则
  reason?: string;
}

export type PermissionSpec = string | PermissionRule | PermissionRule[];
// 简单声明: "file.read" (等同于 { permission: "file.read", pattern: "*", action: "ask" })
// 单条规则: { permission: "question", pattern: "*", action: "allow" }
// 复杂声明: [{ permission: "file.write", pattern: "/etc/**", action: "ask" }]

// ─── 内置安全规则（安全底线，不可修改） ─────────────────

const BUILT_IN_RULES: PermissionRule[] = [
  // 项目内文件
  { permission: 'file.read',    pattern: '<project>/**',            action: 'allow' },
  { permission: 'file.write',   pattern: '<project>/**',            action: 'allow' },

  // 保护路径
  { permission: 'file.write',   pattern: '**/.git/**',              action: 'deny' },
  { permission: 'file.write',   pattern: '~/.ssh/**',               action: 'deny' },
  { permission: 'file.write',   pattern: '~/.config/extendai/**',   action: 'deny' },
  { permission: 'file.write',   pattern: '/etc/**',                 action: 'deny' },
  { permission: 'file.write',   pattern: '/usr/**',                 action: 'deny' },

  // 破坏性操作
  { permission: 'destructive',  pattern: '*',                       action: 'deny' },

  // Shell: 安全命令默认 allow（具体由 DangerousDetector 进一步判断）
  { permission: 'shell',        pattern: '*',                       action: 'ask' },

  // 网络
  { permission: 'network',      pattern: '*',                       action: 'allow' },
];

// ─── Glob 匹配（简化版） ─────────────────────────────────

function globMatch(pattern: string, target: string): boolean {
  // 支持: *, **, {a,b}
  if (pattern === '*' || pattern === '**/*' || pattern === '<project>/**') {
    return true; // 简化：项目内路径假设为 true
  }
  if (pattern.startsWith('~')) {
    const home = process.env.HOME || process.env.USERPROFILE || '';
    return target.includes(pattern.replace('~', home).replace('/**', ''));
  }
  if (pattern.includes('/**')) {
    const prefix = pattern.replace('/**', '');
    return target.startsWith(prefix);
  }
  // 通配符匹配
  const regexStr = pattern
    .replace(/\./g, '\\.')
    .replace(/\*/g, '.*')
    .replace(/\?/g, '.');
  return new RegExp(`^${regexStr}$`).test(target);
}

// ─── Permission Guard ─────────────────────────────────────

export class PermissionGuard {
  private rules: PermissionRule[] = [];

  constructor() {
    this.reset();
  }

  /** 重置为内置规则 */
  reset(): void {
    this.rules = [...BUILT_IN_RULES];
  }

  /** 添加用户规则（覆盖内置规则） */
  addRule(rule: PermissionRule): void {
    this.rules.unshift(rule); // 用户规则优先（unshift 到最前）
  }

  /** 加载用户配置规则 */
  loadRules(rules: PermissionRule[]): void {
    // 用户规则插入到内置规则之前，优先匹配
    this.rules = [...rules, ...BUILT_IN_RULES];
  }

  /** 解析 PermissionSpec */
  resolveSpec(spec: PermissionSpec): PermissionRule[] {
    if (typeof spec === 'string') {
      return [{ permission: spec, pattern: '*', action: 'ask' }];
    }
    if (Array.isArray(spec)) {
      return spec;
    }
    return [spec]; // 单条规则
  }

  /**
   * 检查工具调用是否允许
   * @param tool 调用的工具
   * @param params 工具参数（可能包含 path, command 等）
   * @returns 权限判定结果
   */
  check(tool: Tool, params: Record<string, unknown>): PermissionResult {
    const specs = this.resolveSpec(tool.permission);

    for (const spec of specs) {
      for (const rule of this.rules) {
        // 权限类型匹配
        if (rule.permission !== spec.permission && rule.permission !== '*') continue;

        // 模式匹配：取命令或路径参数
        const target = String(params.command ?? params.url ?? params.path ?? '');
        if (!globMatch(rule.pattern, target)) continue;

        return { action: rule.action, rule };
      }
    }

    // 无匹配 → 默认 ask
    return { action: 'ask', rule: null, reason: 'No matching rule' };
  }

  /** 列出所有规则（用于调试/展示） */
  listRules(): PermissionRule[] {
    return [...this.rules];
  }
}
