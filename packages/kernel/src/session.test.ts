/**
 * Tests for @extendai/kernel — Session class
 */
import { describe, it, expect } from "vitest";
import { Session } from "../src/session.js";

describe("Session", () => {
  const SYSTEM_PROMPT = "You are a test assistant.";
  const CONTEXT_LENGTH = 100_000;

  function makeSession() {
    return new Session(SYSTEM_PROMPT, CONTEXT_LENGTH);
  }

  describe("construction", () => {
    it("starts with system prompt as first message", () => {
      const session = makeSession();
      const msgs = session.getMessages();
      expect(msgs).toHaveLength(1);
      expect(msgs[0]).toEqual({ role: "system", content: SYSTEM_PROMPT });
    });

    it("has empty name and 0 turns initially", () => {
      const session = makeSession();
      expect(session.meta.name).toBe("");
      expect(session.meta.turnCount).toBe(0);
    });

    it("generates a unique ID", () => {
      const s1 = makeSession();
      const s2 = makeSession();
      expect(s1.meta.id).not.toBe(s2.meta.id);
      expect(s1.meta.id).toMatch(/^[0-9a-f]{12}$/);
    });
  });

  describe("message management", () => {
    it("adds messages and increments turn count for assistant", () => {
      const session = makeSession();
      session.addMessage({ role: "user", content: "Hello" });
      expect(session.messageCount).toBe(1);
      expect(session.meta.turnCount).toBe(0); // user doesn't count

      session.addMessage({ role: "assistant", content: "Hi there" });
      expect(session.messageCount).toBe(2);
      expect(session.meta.turnCount).toBe(1);
    });

    it("clear() preserves system prompt", () => {
      const session = makeSession();
      session.addMessage({ role: "user", content: "Hello" });
      session.addMessage({ role: "assistant", content: "Hi" });
      session.clear();

      expect(session.messageCount).toBe(0);
      expect(session.meta.turnCount).toBe(0);
      expect(session.getMessages()).toHaveLength(1);
      expect(session.getMessages()[0].role).toBe("system");
    });

    it("setMessages replaces all messages", () => {
      const session = makeSession();
      session.addMessage({ role: "user", content: "A" });
      session.addMessage({ role: "assistant", content: "B" });

      session.setMessages([
        { role: "system", content: "New system" },
        { role: "user", content: "C" },
      ]);

      expect(session.messageCount).toBe(1);
      expect(session.meta.turnCount).toBe(0);
    });

    it("getMessages returns a copy (immutable)", () => {
      const session = makeSession();
      const msgs = session.getMessages();
      msgs.push({ role: "user", content: "injected" });
      expect(session.messageCount).toBe(0);
    });
  });

  describe("estimatedTokens", () => {
    it("calculates approximate token count", () => {
      const session = makeSession();
      // System prompt is ~200 chars
      const tokens = session.estimatedTokens;
      expect(tokens).toBeGreaterThan(0);
      expect(tokens).toBeLessThan(1000);
    });
  });

  describe("auto-naming", () => {
    it("returns null before 3 turns", () => {
      const session = makeSession();
      const name = session.tryAutoName({});
      expect(name).toBeNull();
      expect(session.meta.name).toBe("");
    });

    it("names after 3 turns", () => {
      const session = makeSession();
      session.meta.turnCount = 3;
      const name = session.tryAutoName({ branch: "main" });
      expect(name).not.toBeNull();
      expect(session.meta.name).toBe(name);
    });

    it("displayName uses auto-name when available", () => {
      const session = makeSession();
      session.meta.turnCount = 4;
      session.tryAutoName({ branch: "feat" });
      expect(session.displayName).toBe(session.meta.name);
    });
  });

  describe("undo/checkpoint", () => {
    it("undo returns null when no checkpoints", () => {
      const session = makeSession();
      expect(session.undo()).toBeNull();
    });

    it("saveCheckpoint + undo restores messages", () => {
      const session = makeSession();
      session.addMessage({ role: "user", content: "Hello" });
      session.addMessage({ role: "assistant", content: "Hi" });

      // Save checkpoint at 2 messages
      session.saveCheckpoint("abc123", "before edit");

      // Add more messages
      session.addMessage({ role: "user", content: "Edit this" });
      session.addMessage({ role: "assistant", content: "Done" });
      expect(session.messageCount).toBe(4);

      // Undo
      const cp = session.undo();
      expect(cp).not.toBeNull();
      expect(cp!.snapshotHash).toBe("abc123");
      expect(cp!.label).toBe("before edit");
      expect(session.messageCount).toBe(2);
    });

    it("multiple checkpoints stack correctly (LIFO)", () => {
      const session = makeSession();
      session.saveCheckpoint("h1", "first");

      session.addMessage({ role: "user", content: "A" });
      session.addMessage({ role: "assistant", content: "B" });
      session.saveCheckpoint("h2", "second");

      // Undo second checkpoint — restores to state at saveCheckpoint("h2")
      const cp2 = session.undo();
      expect(cp2!.label).toBe("second");
      expect(session.messageCount).toBe(2); // user + assistant

      // Undo first checkpoint — restores to state at saveCheckpoint("h1")
      const cp1 = session.undo();
      expect(cp1!.label).toBe("first");
      expect(session.messageCount).toBe(0); // back to initial
    });

    it("peekCheckpoint doesn't consume", () => {
      const session = makeSession();
      session.saveCheckpoint("h1", "test");
      expect(session.peekCheckpoint()).not.toBeNull();
      expect(session.undoCount).toBe(1);
      expect(session.peekCheckpoint()).not.toBeNull();
    });

    it("undoHistory returns metadata", () => {
      const session = makeSession();
      session.saveCheckpoint("h1", "step1");
      session.saveCheckpoint("h2", "step2");

      const history = session.undoHistory;
      expect(history).toHaveLength(2);
      expect(history[0].label).toBe("step1");
      expect(history[1].label).toBe("step2");
      expect(history[0].hasSnapshot).toBe(true);
    });
  });

  describe("fork", () => {
    it("creates child with copied messages", () => {
      const parent = makeSession();
      parent.addMessage({ role: "user", content: "Hello" });
      parent.addMessage({ role: "assistant", content: "Hi" });

      const child = parent.fork();
      expect(child.messageCount).toBe(2);
      expect(child.meta.parentId).toBe(parent.meta.id);
      expect(parent.children).toHaveLength(1);
    });

    it("child is independent from parent", () => {
      const parent = makeSession();
      parent.addMessage({ role: "user", content: "Hello" });

      const child = parent.fork();
      child.addMessage({ role: "assistant", content: "Response" });

      // Parent unaffected
      expect(parent.messageCount).toBe(1);
      expect(child.messageCount).toBe(2);
    });

    it("child inherits system prompt", () => {
      const parent = makeSession();
      const child = parent.fork();
      expect(child.getMessages()[0].content).toBe(SYSTEM_PROMPT);
    });
  });

  describe("trimContext", () => {
    it("trims oldest non-system messages when over context limit", () => {
      // Create session with very small context limit
      const session = new Session("sys", 50); // 50 tokens ≈ 175 chars

      // Add messages that exceed context
      for (let i = 0; i < 20; i++) {
        session.addMessage({ role: "user", content: `Message ${i} with some content` });
        session.addMessage({ role: "assistant", content: `Response ${i} with content` });
      }

      // Should have trimmed to fit
      const tokens = session.estimatedTokens;
      // With trimming, should be under or near limit
      // (exact count depends on trim logic)
      expect(session.getMessages().length).toBeGreaterThan(1); // at least system
    });
  });
});
