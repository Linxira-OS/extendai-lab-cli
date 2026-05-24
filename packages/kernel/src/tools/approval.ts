/**
 * @extendai/kernel — Approval Gate
 *
 * User confirmation system for tool calls.
 * Supports multi-level approval caching for auto-accept.
 *
 * Reference:
 *   - Cline: src/core/task/tools/autoApprove.ts (YOLO + granular settings)
 *   - copilot-cli: permission prompts with once/session/location scopes
 *   - oh-my-pi: #acpPermissionDecisions (allow_once/allow_always)
 *   - CodeX: ApprovalStore + with_cached_approval
 */

// ─── 批准选项 ────────────────────────────────────────────

export type ApprovalOptionKind =
  | 'allow_once'
  | 'allow_session'
  | 'allow_location'
  | 'allow_always'
  | 'reject_once'
  | 'reject_always'
  | 'reject_with_feedback';

export interface ApprovalOption {
  kind: ApprovalOptionKind;
  label: string;
  description?: string;
}

export interface ApprovalPrompt {
  title: string;
  detail: string;
  severity: 'info' | 'warning' | 'danger';
  options: ApprovalOption[];
}

export interface ApprovalDecision {
  kind: 'allow' | 'reject';
  cacheKey?: string;
  cacheScope?: 'once' | 'session' | 'location' | 'always';
  feedback?: string;
}

// ─── Auto-Accept 模式 ────────────────────────────────────

export type AutoAcceptMode = 'off' | 'safe_only' | 'all_in_workspace' | 'yolo';

// ─── 批准缓存键 ─────────────────────────────────────────

export interface ApprovalCacheKey {
  permission: string;
  pattern: string;
  command?: string;
}

function hashKey(key: ApprovalCacheKey): string {
  return `${key.permission}::${key.pattern}::${key.command ?? ''}`;
}

// ─── 批准缓存存储 ───────────────────────────────────────

interface CacheEntry {
  decision: 'allow' | 'reject';
  scope: 'session' | 'location' | 'always';
  createdAt: number;
}

// ─── 批准门 ────────────────────────────────────────────────

export class ApprovalGate {
  // Session 缓存（内存，session 内有效）
  private sessionCache = new Map<string, CacheEntry>();

  // Auto-accept 模式
  private autoAcceptMode: AutoAcceptMode = 'off';

  constructor() {
    this.sessionCache = new Map();
  }

  /** 设置自动批准模式 */
  setAutoAcceptMode(mode: AutoAcceptMode): void {
    this.autoAcceptMode = mode;
  }

  /** 获取当前自动批准模式 */
  getAutoAcceptMode(): AutoAcceptMode {
    return this.autoAcceptMode;
  }

  /**
   * 是否需要用户批准
   * @returns null = 不需要批准（自动通过/拒绝），否则返回需要展示的提示
   */
  needsApproval(params: {
    permission: string;
    pattern: string;
    command?: string;
    severity: 'info' | 'warning' | 'danger';
  }): ApprovalPrompt | null {
    const key: ApprovalCacheKey = {
      permission: params.permission,
      pattern: params.pattern,
      command: params.command,
    };

    // 1. 检查缓存
    const cached = this.checkCache(key);
    if (cached) {
      return null; // 缓存命中，不需要提示
    }

    // 2. 自动批准模式检查
    if (this.autoAcceptMode === 'yolo') {
      return null; // YOLO 模式：全部自动过
    }

    if (this.autoAcceptMode === 'all_in_workspace' && params.severity !== 'danger') {
      return null; // 工作区自动模式：非危险操作直接过
    }

    if (this.autoAcceptMode === 'safe_only' && params.severity === 'info') {
      return null; // 安全模式：info 级别直接过
    }

    // 3. 需要用户批准，构造提示
    return this.buildPrompt(params);
  }

  /**
   * 处理用户批准决策
   */
  applyDecision(decision: ApprovalDecision, cacheKey?: ApprovalCacheKey): void {
    if (!cacheKey) return;

    // 只有 session/location/always 范围的才缓存
    if (decision.cacheScope && decision.cacheScope !== 'once') {
      const entry: CacheEntry = {
        decision: decision.kind,
        scope: decision.cacheScope,
        createdAt: Date.now(),
      };

      switch (decision.cacheScope) {
        case 'session':
          this.sessionCache.set(hashKey(cacheKey), entry);
          break;
        case 'location':
          // TODO: 持久化到本地配置（按 worktree hash）
          this.sessionCache.set(hashKey(cacheKey), entry);
          break;
        case 'always':
          // TODO: 持久化到全局配置
          this.sessionCache.set(hashKey(cacheKey), entry);
          break;
      }
    }
  }

  /** 撤销所有自动批准 */
  resetAllApprovals(): void {
    this.sessionCache.clear();
  }

  /** 撤销指定权限的缓存 */
  resetApproval(permission: string, pattern: string): void {
    for (const [k] of this.sessionCache) {
      if (k.startsWith(`${permission}::`)) {
        this.sessionCache.delete(k);
      }
    }
  }

  // ─── 私有方法 ─────────────────────────────────────────

  private checkCache(key: ApprovalCacheKey): CacheEntry | undefined {
    return this.sessionCache.get(hashKey(key));
  }

  private buildPrompt(params: {
    permission: string;
    pattern: string;
    command?: string;
    severity: 'info' | 'warning' | 'danger';
  }): ApprovalPrompt {
    const title = params.command
      ? `Shell 要执行: ${params.command.slice(0, 80)}${params.command.length > 80 ? '...' : ''}`
      : `${params.permission} 请求: ${params.pattern}`;

    const options: ApprovalOption[] = [];

    if (params.severity === 'danger') {
      options.push(
        { kind: 'allow_once', label: '允许一次', description: '仅这次允许' },
        { kind: 'reject_once', label: '拒绝', description: '拒绝这次操作' },
        { kind: 'reject_with_feedback', label: '拒绝并反馈', description: '拒绝并给 AI 修改建议' },
      );
    } else {
      options.push(
        { kind: 'allow_once', label: '允许一次' },
        { kind: 'allow_session', label: '本次会话允许' },
        { kind: 'allow_location', label: '此位置始终允许' },
        { kind: 'allow_always', label: '始终允许此操作' },
        { kind: 'reject_once', label: '拒绝' },
        { kind: 'reject_with_feedback', label: '拒绝并反馈' },
      );
    }

    return {
      title,
      detail: `权限类型: ${params.permission}\n模式: ${params.pattern}`,
      severity: params.severity,
      options,
    };
  }
}
