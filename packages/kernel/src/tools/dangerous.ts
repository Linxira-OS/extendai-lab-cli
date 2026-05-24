/**
 * @extendai/kernel — Dangerous Tool Detector
 *
 * Detects dangerous commands and file operations before execution.
 * Cross-platform: POSIX shell, PowerShell, CMD.
 *
 * Reference:
 *   - CodeX: shell-command/src/command_safety/is_dangerous_command.rs
 *   - CodeX: shell-command/src/command_safety/windows_dangerous_commands.rs
 *   - OpenCode shell.ts: FILES Set for file operation detection
 */

// ─── 危险命令模式 ────────────────────────────────────────

export const DANGEROUS_PATTERNS_POSIX: RegExp[] = [
  /\brm\s+(-rf?|--recursive\b)/i,                 // rm -rf
  /\bsudo\s+(rm|dd|mkfs|fdisk|shutdown|reboot)/i, // sudo 高风险
  /\bdd\s+if=/,                                    // dd 写盘
  /\bmkfs\b/,                                      // 格式化
  /\bfdisk\b/,                                     // 分区
  /\bkill\s+-9\b/,                                 // 强制杀进程
  /\bchmod\s+777\b/,                               // 777 权限
  /\bchown\s+[^:]+:/,                              // 所有者修改
  /\bshred\b/,                                     // 安全擦除
  /\bmv\s+\/.*\s+\//,                              // 跨目录 mv
];

export const DANGEROUS_PATTERNS_POWERSHELL: RegExp[] = [
  /Remove-Item\s+-Force/i,                         // 强制删除
  /ri\s+-Force/i,                                  // ri -Force
  /rm\s+-Force/i,                                  // rm -Force
  /Remove-Item\s+-Recurse/i,                       // 递归删除
  /Format-Volume/i,                                // 格式化
  /Clear-Content/i,                                // 清空内容
  /Start-Process\s+['"]?https?:\/\//i,             // URL 启动（潜在恶意）
];

export const DANGEROUS_PATTERNS_CMD: RegExp[] = [
  /del\s+\/f/i,                                    // 强制删除
  /erase\s+\/f/i,                                  // 强制擦除
  /rd\s+\/s\s+\/q/i,                               // 静默递归删除目录
  /format\s+\w+:/,                                  // 格式化
];

// ─── 安全命令白名单 ──────────────────────────────────────

export const SAFE_COMMANDS_POSIX = new Set([
  'cat', 'ls', 'echo', 'cd', 'pwd', 'which',
  'head', 'tail', 'wc', 'sort', 'uniq', 'cut', 'tr', 'tee',
  'grep', 'rg', 'find', 'locate',
  'diff', 'cmp',
  'date', 'cal', 'env', 'printenv',
  'dirname', 'basename', 'realpath', 'readlink',
  'id', 'whoami', 'who', 'uname', 'hostname',
  'ps', 'top', 'htop',
  'du', 'df', 'stat', 'file',
  'true', 'false', 'sleep', 'yes',
  'npm ls', 'npm list', 'npm view',
  'git status', 'git log', 'git diff', 'git show', 'git branch',
  'pip list', 'pip show',
  'cargo check', 'cargo tree',
]);

export const SAFE_COMMANDS_WINDOWS = new Set([
  'dir', 'type', 'echo', 'cd', 'where',
  'find', 'findstr', 'more', 'sort',
  'date', 'time', 'ver', 'set',
  'help',
]);

// ─── 文件操作集（参考 OpenCode shell.ts） ────────────────

export const FILE_OPS_POSIX = new Set([
  'rm', 'cp', 'mv', 'mkdir', 'touch', 'chmod', 'chown',
  'cat', '>', '>>',
]);

export const FILE_OPS_WINDOWS = new Set([
  'copy', 'xcopy', 'move', 'ren', 'rename',
  'del', 'erase', 'rd', 'rmdir', 'md', 'mkdir',
  'type',
]);

// ─── 检测结果 ────────────────────────────────────────────

export interface SafetyAssessment {
  level: 'safe' | 'suspicious' | 'dangerous';
  reason?: string;
  matchedPattern?: RegExp;
}

export interface FileOperation {
  type: 'read' | 'write' | 'delete' | 'move' | 'create';
  path: string;
  pattern: string;
}

// ─── 检测器 ────────────────────────────────────────────────

export class DangerousDetector {
  /**
   * 评估命令安全等级
   */
  assess(command: string): SafetyAssessment {
    // 检查 POSIX 危险模式
    for (const pat of DANGEROUS_PATTERNS_POSIX) {
      if (pat.test(command)) {
        return { level: 'dangerous', reason: `POSIX dangerous pattern: ${pat}`, matchedPattern: pat };
      }
    }

    // 检查 PowerShell 危险模式
    for (const pat of DANGEROUS_PATTERNS_POWERSHELL) {
      if (pat.test(command)) {
        return { level: 'dangerous', reason: `PowerShell dangerous pattern: ${pat}`, matchedPattern: pat };
      }
    }

    // 检查 CMD 危险模式
    for (const pat of DANGEROUS_PATTERNS_CMD) {
      if (pat.test(command)) {
        return { level: 'dangerous', reason: `CMD dangerous pattern: ${pat}`, matchedPattern: pat };
      }
    }

    // 检查是否是安全命令
    const firstWord = command.trim().split(/\s+/)[0]?.toLowerCase() ?? '';
    if (SAFE_COMMANDS_POSIX.has(firstWord) || SAFE_COMMANDS_WINDOWS.has(firstWord)) {
      return { level: 'safe', reason: `Known safe command: ${firstWord}` };
    }

    // 检查是否包含文件写入重定向
    if (/>>?\s+/.test(command) && !/>>?\s+\/dev\/null/.test(command)) {
      return { level: 'suspicious', reason: 'File write redirect detected' };
    }

    // 未知命令 → 可疑
    return { level: 'suspicious', reason: 'Unknown command, potential risk' };
  }

  /**
   * 提取命令中的文件操作
   * 简化版：检测 rm/cp/mv 等操作及其路径参数
   */
  detectFileOperations(command: string): FileOperation[] {
    const ops: FileOperation[] = [];
    const tokens = command.trim().split(/\s+/);

    if (tokens.length === 0) return ops;

    const cmd = tokens[0]?.toLowerCase();

    // rm → delete
    if (cmd === 'rm' || cmd === 'del' || cmd === 'erase') {
      const paths = tokens.slice(1).filter(t => !t.startsWith('-'));
      for (const p of paths) {
        ops.push({ type: 'delete', path: p, pattern: command });
      }
    }

    // cp/mv → move/create
    if (cmd === 'cp' || cmd === 'copy' || cmd === 'xcopy') {
      const paths = tokens.slice(1).filter(t => !t.startsWith('-'));
      if (paths.length >= 2) {
        ops.push({ type: 'write', path: paths[paths.length - 1]!, pattern: command });
      }
    }
    if (cmd === 'mv' || cmd === 'move' || cmd === 'ren' || cmd === 'rename') {
      const paths = tokens.slice(1).filter(t => !t.startsWith('-'));
      if (paths.length >= 2) {
        ops.push({ type: 'move', path: `${paths[0]} → ${paths[1]}`, pattern: command });
      }
    }

    // mkdir/md → create
    if (cmd === 'mkdir' || cmd === 'md') {
      const paths = tokens.slice(1).filter(t => !t.startsWith('-'));
      for (const p of paths) {
        ops.push({ type: 'create', path: p, pattern: command });
      }
    }

    return ops;
  }
}
