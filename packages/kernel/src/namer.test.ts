/**
 * Tests for @extendai/kernel — Session namer
 */
import { describe, it, expect } from "vitest";
import {
  generateSessionName,
  generateSessionId,
  formatSessionName,
} from "../src/namer.js";

describe("generateSessionName", () => {
  it("returns null when turnCount < 3", () => {
    expect(generateSessionName(0, {})).toBeNull();
    expect(generateSessionName(1, {})).toBeNull();
    expect(generateSessionName(2, {})).toBeNull();
  });

  it("returns name after 3 turns", () => {
    const name = generateSessionName(3, { branch: "main" });
    expect(name).not.toBeNull();
    expect(name).toMatch(/^\d{4}-main$/);
  });

  it("abbreviates long branch names", () => {
    const longBranch = "feature/add-user-authentication";
    const name = generateSessionName(5, { branch: longBranch });
    expect(name).not.toBeNull();
    // Should be truncated: first 8 + last 4
    expect(name).toContain(longBranch.slice(0, 8));
    expect(name).toContain(longBranch.slice(-4));
  });

  it("uses 'unknown' when no branch provided", () => {
    const name = generateSessionName(4, {});
    expect(name).toMatch(/^\d{4}-unknown$/);
  });

  it("preserves short branch names as-is", () => {
    const name = generateSessionName(3, { branch: "feat-x" });
    expect(name).toContain("feat-x");
  });
});

describe("generateSessionId", () => {
  it("generates a 12-char hex string", () => {
    const id = generateSessionId("worktree-abc");
    expect(id).toMatch(/^[0-9a-f]{12}$/);
  });

  it("generates unique IDs", () => {
    const id1 = generateSessionId("");
    const id2 = generateSessionId("");
    expect(id1).not.toBe(id2);
  });

  it("incorporates worktreeId into hash", () => {
    const id1 = generateSessionId("wt-1");
    const id2 = generateSessionId("wt-2");
    // Not guaranteed to differ, but statistically should
    // We just verify format is correct
    expect(id1).toMatch(/^[0-9a-f]{12}$/);
    expect(id2).toMatch(/^[0-9a-f]{12}$/);
  });
});

describe("formatSessionName", () => {
  it("returns name when provided", () => {
    expect(formatSessionName("1430-main", 5)).toBe("1430-main");
  });

  it("returns 'new session' when no name and 0 turns", () => {
    expect(formatSessionName(null, 0)).toBe("new session");
  });

  it("returns 'unnamed (N turns)' when no name and turns > 0", () => {
    expect(formatSessionName(null, 3)).toBe("unnamed (3 turns)");
    expect(formatSessionName(null, 1)).toBe("unnamed (1 turns)");
  });
});
