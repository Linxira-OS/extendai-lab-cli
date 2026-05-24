/**
 * @extendai/kernel — Configuration loading
 *
 * Priority (high → low):
 *   1. Environment variables (EXTENDAI_*)
 *   2. User config file (~/.extendai/config.json)
 *   3. Hardcoded defaults
 */

import { existsSync, readFileSync, mkdirSync, writeFileSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';
import type { AppConfig } from './types.js';

/** Default configuration matching user requirements */
const DEFAULT_CONFIG: AppConfig = {
  provider: {
    name: 'default',
    type: 'openai',
    apiKey: '',
    baseUrl: 'https://api.openai.com/v1',
    model: 'gpt-4o',
    maxTokens: 128_000,
    contextLength: 200_000,
    systemPrompt: 'You are a helpful AI assistant.',
    temperature: 0.7,
  },
};

const CONFIG_DIR = join(homedir(), '.extendai');
const CONFIG_PATH = join(CONFIG_DIR, 'config.json');

/**
 * Load configuration with override chain:
 * defaults ← config file ← environment variables
 */
export function loadConfig(): AppConfig {
  const config = deepClone(DEFAULT_CONFIG);

  // Layer 1: Config file
  if (existsSync(CONFIG_PATH)) {
    try {
      const raw = readFileSync(CONFIG_PATH, 'utf-8');
      const fileConfig = JSON.parse(raw);
      deepMerge(config, fileConfig);
    } catch (e) {
      console.error(`Warning: Failed to parse ${CONFIG_PATH}: ${e}`);
    }
  }

  // Layer 2: Environment variables
  if (process.env.EXTENDAI_API_KEY) {
    config.provider.apiKey = process.env.EXTENDAI_API_KEY;
  }
  if (process.env.EXTENDAI_BASE_URL) {
    config.provider.baseUrl = process.env.EXTENDAI_BASE_URL;
  }
  if (process.env.EXTENDAI_MODEL) {
    config.provider.model = process.env.EXTENDAI_MODEL;
  }

  return config;
}

/**
 * Save configuration to ~/.extendai/config.json.
 * API key from env is not written back to file.
 */
export function saveConfig(config: AppConfig): void {
  if (!existsSync(CONFIG_DIR)) {
    mkdirSync(CONFIG_DIR, { recursive: true });
  }

  const save = deepClone(config);
  // Don't persist env-var sourced API key
  if (process.env.EXTENDAI_API_KEY) {
    save.provider.apiKey = '';
  }

  writeFileSync(CONFIG_PATH, JSON.stringify(save, null, 2), 'utf-8');
}

// ─── Helpers ─────────────────────────────────────────────

function deepClone<T>(obj: T): T {
  return JSON.parse(JSON.stringify(obj));
}

function deepMerge(target: Record<string, any>, source: Record<string, any>): void {
  for (const [key, value] of Object.entries(source)) {
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      if (!target[key]) target[key] = {};
      deepMerge(target[key], value);
    } else {
      target[key] = value;
    }
  }
}
