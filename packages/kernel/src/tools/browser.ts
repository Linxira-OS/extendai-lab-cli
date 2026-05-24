/**
 * @extendai/kernel — Browser Tool
 *
 * Lightweight browser automation via fetch + HTML parsing.
 * For full browser automation (JS rendering, screenshots),
 * Playwright or Puppeteer is recommended as an external MCP.
 *
 * Execution modes:
 *   1. Simple fetch (default) — GET page, return HTML text
 *   2. Playwright fallback — if @playwright/browser is available
 *   3. External MCP — recommended for full browser testing
 */

import {
  type Tool,
  type ToolChunk,
} from './registry.js';

// ─── Simple HTTP fetch ────────────────────────────────────

async function* fetchPage(
  url: string,
  _ctx: any,
): AsyncGenerator<ToolChunk> {
  try {
    const response = await fetch(url, {
      headers: {
        'User-Agent': 'ExtendAI-Lab-CLI/0.1 (+https://github.com/extendai-lab)',
        'Accept': 'text/html,application/xhtml+xml',
      },
      redirect: 'follow',
      signal: AbortSignal.timeout(30_000),
    });

    if (!response.ok) {
      yield { type: 'error', error: `HTTP ${response.status}: ${response.statusText}` };
      return;
    }

    const html = await response.text();
    // Strip HTML tags for readability
    const text = html
      .replace(/<script[^>]*>[\s\S]*?<\/script>/gi, '')
      .replace(/<style[^>]*>[\s\S]*?<\/style>/gi, '')
      .replace(/<[^>]+>/g, ' ')
      .replace(/&[a-z]+;/g, ' ')
      .replace(/\s+/g, ' ')
      .trim()
      .slice(0, 10000); // Limit to 10K chars

    yield {
      type: 'text',
      content: `Title: ${extractTitle(html)}\nURL: ${response.url}\n\n${text}`,
    };
    yield { type: 'done', result: { url: response.url, status: response.status } };
  } catch (e: unknown) {
    const msg = e instanceof Error ? e.message : String(e);
    yield { type: 'error', error: `Fetch failed: ${msg}` };
  }
}

function extractTitle(html: string): string {
  const match = html.match(/<title[^>]*>([^<]+)<\/title>/i);
  return match ? match[1].trim() : '(no title)';
}

// ─── Tool definition ──────────────────────────────────────

export const browserTool: Tool = {
  name: 'browser',
  permission: 'network',
  description: `Fetch and read web pages. Supports HTTP GET navigation and content extraction.
For full browser testing (JavaScript rendering, screenshots, form filling),
use the Playwright MCP server instead.

Usage:
  browser(url: "https://example.com") — fetch page content
  browser(url: "https://example.com", action: "screenshot") — not yet supported, use Playwright
`,
  parameters: {
    type: 'object',
    properties: {
      url: {
        type: 'string',
        description: 'The URL to navigate to',
      },
      action: {
        type: 'string',
        description: 'Action to perform: "fetch" (default), "screenshot" (requires Playwright MCP)',
        enum: ['fetch', 'screenshot'],
        default: 'fetch',
      },
    },
    required: ['url'],
  },

  async *execute(params, ctx): AsyncGenerator<ToolChunk> {
    const url = String(params.url ?? '');
    const action = String(params.action ?? 'fetch');

    if (!url) {
      yield { type: 'error', error: 'URL is required' };
      return;
    }

    if (action === 'screenshot') {
      yield {
        type: 'error',
        error: 'Screenshot requires Playwright MCP. Install with: npx @playwright/mcp',
      };
      return;
    }

    yield { type: 'progress', message: `Fetching ${url}...` };
    yield* fetchPage(url, ctx);
  },
};
