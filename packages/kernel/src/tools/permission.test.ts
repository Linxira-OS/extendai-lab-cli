/**
 * Tests for @extendai/kernel — Permission Guard
 */
import { describe, it, expect, beforeEach } from "vitest";
import { PermissionGuard } from "./permission.js";
import type { Tool } from "./registry.js";

// Mock tools for testing
const mockFileReadTool: Tool = {
  name: "read",
  description: "Read file",
  permission: "file.read",
  parameters: { type: "object", properties: {} },
  execute: async function* () {},
};

const mockFileWriteTool: Tool = {
  name: "write",
  description: "Write file",
  permission: "file.write",
  parameters: { type: "object", properties: {} },
  execute: async function* () {},
};

const mockShellTool: Tool = {
  name: "bash",
  description: "Run shell command",
  permission: "shell",
  parameters: { type: "object", properties: {} },
  execute: async function* () {},
};

const mockDestructiveTool: Tool = {
  name: "rm",
  description: "Remove files",
  permission: "destructive",
  parameters: { type: "object", properties: {} },
  execute: async function* () {},
};

const mockNetworkTool: Tool = {
  name: "websearch",
  description: "Web search",
  permission: "network",
  parameters: { type: "object", properties: {} },
  execute: async function* () {},
};

describe("PermissionGuard", () => {
  let guard: PermissionGuard;

  beforeEach(() => {
    guard = new PermissionGuard();
  });

  describe("built-in rules", () => {
    it("allows file.read for project paths", () => {
      const result = guard.check(mockFileReadTool, { path: "src/index.ts" });
      expect(result.action).toBe("allow");
    });

    it("allows file.write for project paths", () => {
      const result = guard.check(mockFileWriteTool, { path: "src/new.ts" });
      expect(result.action).toBe("allow");
    });

    it("denies file.write for .git directory", () => {
      const result = guard.check(mockFileWriteTool, { path: ".git/config" });
      expect(result.action).toBe("deny");
    });

    it("denies file.write for /etc", () => {
      const result = guard.check(mockFileWriteTool, { path: "/etc/passwd" });
      expect(result.action).toBe("deny");
    });

    it("denies file.write for /usr", () => {
      const result = guard.check(mockFileWriteTool, { path: "/usr/bin/ls" });
      expect(result.action).toBe("deny");
    });

    it("asks for shell commands", () => {
      const result = guard.check(mockShellTool, { command: "ls -la" });
      expect(result.action).toBe("ask");
    });

    it("denies destructive operations", () => {
      const result = guard.check(mockDestructiveTool, { command: "rm -rf /" });
      expect(result.action).toBe("deny");
    });

    it("allows network operations", () => {
      const result = guard.check(mockNetworkTool, { url: "https://example.com" });
      expect(result.action).toBe("allow");
    });
  });

  describe("user rules", () => {
    it("user rules take precedence over built-in rules", () => {
      // Built-in allows file.read in project, but user denies it
      guard.addRule({
        permission: "file.read",
        pattern: "secret/**",
        action: "deny",
      });

      const result = guard.check(mockFileReadTool, { path: "secret/key.pem" });
      expect(result.action).toBe("deny");
    });

    it("loadRules replaces rules correctly", () => {
      guard.loadRules([
        { permission: "shell", pattern: "*", action: "allow" },
      ]);

      const result = guard.check(mockShellTool, { command: "anything" });
      expect(result.action).toBe("allow");
    });
  });

  describe("reset", () => {
    it("reset() restores built-in rules", () => {
      guard.addRule({
        permission: "shell",
        pattern: "*",
        action: "allow",
      });
      guard.reset();

      const result = guard.check(mockShellTool, { command: "test" });
      expect(result.action).toBe("ask"); // back to built-in default
    });
  });

  describe("no match", () => {
    it("returns ask when no rule matches", () => {
      const unknownTool: Tool = {
        name: "unknown",
        description: "Unknown tool",
        permission: "unknown.permission",
        parameters: { type: "object", properties: {} },
        execute: async function* () {},
      };

      const result = guard.check(unknownTool, {});
      expect(result.action).toBe("ask");
      expect(result.reason).toBe("No matching rule");
    });
  });

  describe("listRules", () => {
    it("returns copy of rules (immutable)", () => {
      const rules = guard.listRules();
      rules.push({ permission: "test", pattern: "*", action: "allow" });
      expect(guard.listRules().length).not.toBe(rules.length);
    });
  });
});
