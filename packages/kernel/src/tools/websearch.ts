/**
 * @extendai/kernel — WebSearch tool
 *
 * Web search and URL fetching.
 * Searches the web for recent information or fetches specific URLs.
 *
 * Reference: OpenCode websearch/webfetch, Pi web_search
 *
 * Two modes:
 *   1. Search: query + optional numResults → web search results
 *   2. Fetch: url → fetch and extract content from a specific page
 */

import { execSync } from 'node:child_process';
import type { Tool } from './registry.js';

export const webSearchTool: Tool = {
  name: 'web_search',
  description: `Search the web for information or fetch content from a specific URL.

Two modes:
  1. Search mode (use 'query'): searches the web and returns results
     - Returns clean text content from top search results
     - Use for current events, documentation lookups, recent APIs
  2. Fetch mode (use 'url'): fetches and extracts content from a specific page
     - Returns the page content as markdown text
     - Use for reading documentation, blog posts, articles

The current year is 2026.`,
  permission: 'network',
  parameters: {
    type: 'object',
    properties: {
      query: {
        type: 'string',
        description: 'Web search query (for search mode)',
      },
      url: {
        type: 'string',
        description: 'URL to fetch (for fetch mode — uses webfetch)',
      },
      numResults: {
        type: 'number',
        description: 'Number of search results to return (default: 5, max: 10)',
      },
    },
  },
  execute: async function* (params) {
    const query = params.query ? String(params.query).trim() : '';
    const url = params.url ? String(params.url).trim() : '';

    if (!query && !url) {
      yield { type: 'error', error: 'Either query or url is required' };
      return;
    }

    // ── Mode 1: URL fetch ──
    if (url) {
      try {
        // Try using curl for URL fetching
        const cmd = `curl -sL --max-time 15 "${escapeArg(url)}" 2>nul`;
        const output = execSync(cmd, {
          encoding: 'utf-8',
          stdio: 'pipe',
          timeout: 20000,
          maxBuffer: 5 * 1024 * 1024,
          shell: true as any,
        });

        // Strip HTML tags for readability
        const text = output
          .replace(/<script[^>]*>[\s\S]*?<\/script>/gi, '')
          .replace(/<style[^>]*>[\s\S]*?<\/style>/gi, '')
          .replace(/<[^>]+>/g, '')
          .replace(/\n{3,}/g, '\n\n')
          .trim()
          .slice(0, 10000);

        yield { type: 'text', content: text || '(empty page)' };
      } catch (e: unknown) {
        const err = e as Error;
        yield { type: 'error', error: `Fetch error: ${err.message}` };
      }
      return;
    }

    // ── Mode 2: Web search ──
    try {
      const num = Math.min(Math.max(1, Number(params.numResults) || 5), 10);

      // Try using curl to hit a search API
      // Fallback: use a simple search via DuckDuckGo HTML
      const searchUrl = `https://html.duckduckgo.com/html/?q=${encodeURIComponent(query)}`;
      const cmd = `curl -sL --max-time 15 "${searchUrl}" 2>nul`;
      const output = execSync(cmd, {
        encoding: 'utf-8',
        stdio: 'pipe',
        timeout: 20000,
        maxBuffer: 5 * 1024 * 1024,
        shell: true as any,
      });

      // Extract search result links and titles from DuckDuckGo HTML
      const results: Array<{ title: string; url: string; snippet: string }> = [];
      const linkRegex = /<a[^>]+class="result__a"[^>]*href="([^"]*)"[^>]*>(.*?)<\/a>/gi;
      const snippetRegex = /<a[^>]+class="result__snippet"[^>]*>(.*?)<\/a>/gi;

      let match;
      while ((match = linkRegex.exec(output)) !== null && results.length < num) {
        const url2 = match[1].replace(/\/\/duckduckgo\.com\/l\/\?uddg=/, '').replace(/&rut=.*$/, '');
        results.push({
          title: decodeHtml(match[2]),
          url: decodeURIComponent(url2),
          snippet: '',
        });
      }

      // Reset index for snippets
      snippetRegex.lastIndex = 0;
      let snippetIdx = 0;
      while ((match = snippetRegex.exec(output)) !== null && snippetIdx < results.length) {
        results[snippetIdx].snippet = decodeHtml(match[1]);
        snippetIdx++;
      }

      if (results.length === 0) {
        yield { type: 'text', content: `No results found for "${query}"` };
        return;
      }

      const formatted = results
        .map((r, i) => `${i + 1}. ${r.title}\n   ${r.snippet}\n   ${r.url}`)
        .join('\n\n');

      yield { type: 'text', content: formatted };
    } catch (e: unknown) {
      const err = e as Error;
      yield { type: 'error', error: `Search error: ${err.message}` };
    }
  },
};

function decodeHtml(html: string): string {
  return html
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#x27;/g, "'")
    .replace(/&#x2F;/g, '/');
}

function escapeArg(s: string): string {
  return s.replace(/"/g, '\\"');
}
