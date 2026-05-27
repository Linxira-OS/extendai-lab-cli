/**
 * Tests for @extendai/kernel — Calculator tool
 */
import { describe, it, expect } from "vitest";
import { calculatorTool } from "./calculator.js";

// Mock context (calculator doesn't use it, but the type requires it)
const mockCtx = {} as any;

async function evalCalc(expression: string) {
  const results: string[] = [];
  const gen = calculatorTool.execute({ expression }, mockCtx);
  for await (const chunk of gen) {
    if (chunk.type === "text") results.push(chunk.content!);
    if (chunk.type === "error") results.push(`ERROR: ${chunk.error}`);
  }
  return results.join("");
}

describe("calculatorTool", () => {
  it("has correct metadata", () => {
    expect(calculatorTool.name).toBe("calculator");
    expect(calculatorTool.permission).toBe("file.read");
  });

  describe("basic arithmetic", () => {
    it("evaluates addition", async () => {
      expect(await evalCalc("2 + 3")).toBe("= 5");
    });

    it("evaluates subtraction", async () => {
      expect(await evalCalc("10 - 4")).toBe("= 6");
    });

    it("evaluates multiplication", async () => {
      expect(await evalCalc("6 * 7")).toBe("= 42");
    });

    it("evaluates division", async () => {
      expect(await evalCalc("15 / 3")).toBe("= 5");
    });

    it("evaluates modulo", async () => {
      expect(await evalCalc("10 % 3")).toBe("= 1");
    });

    it("evaluates power", async () => {
      expect(await evalCalc("2 ** 10")).toBe("= 1024");
    });

    it("evaluates parentheses", async () => {
      expect(await evalCalc("(2 + 3) * 4")).toBe("= 20");
    });
  });

  describe("math functions", () => {
    it("evaluates sqrt", async () => {
      expect(await evalCalc("sqrt(144)")).toBe("= 12");
    });

    it("evaluates abs", async () => {
      expect(await evalCalc("abs(-42)")).toBe("= 42");
    });

    it("evaluates floor", async () => {
      expect(await evalCalc("floor(3.7)")).toBe("= 3");
    });

    it("evaluates ceil", async () => {
      expect(await evalCalc("ceil(3.2)")).toBe("= 4");
    });

    it("evaluates round", async () => {
      expect(await evalCalc("round(3.5)")).toBe("= 4");
    });

    it("evaluates pi", async () => {
      const result = await evalCalc("pi");
      expect(result).toContain("3.14159");
    });

    it("evaluates trig functions", async () => {
      const result = await evalCalc("sin(pi/2)");
      expect(result).toContain("1");
    });
  });

  describe("security", () => {
    it("rejects empty expression", async () => {
      expect(await evalCalc("")).toBe("ERROR: expression is required");
    });

    it("rejects expressions with disallowed characters", async () => {
      // Quotes are not in the allowed character set
      expect(await evalCalc('"hello"')).toBe(
        "ERROR: Expression contains disallowed characters"
      );
    });

    it("rejects expressions referencing undefined variables", async () => {
      // process.exit() passes regex (letters + parens + dot) but throws in sandbox
      const result = await evalCalc("process.exit()");
      expect(result).toContain("ERROR:");
    });
  });

  describe("edge cases", () => {
    it("handles division by zero", async () => {
      const result = await evalCalc("1 / 0");
      expect(result).toContain("Infinity");
    });

    it("handles complex expressions", async () => {
      const result = await evalCalc("sqrt(2**2 + 3**2)");
      expect(result).toContain("3.60555");
    });
  });
});
