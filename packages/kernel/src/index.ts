/**
 * @extendai/kernel — Core Engine
 *
 * Session tree, context management, provider abstraction,
 * configuration loading, and message types.
 *
 * Every component is a replaceable proxy — plugins can
 * invasively swap implementations at runtime.
 */

export type {
  Role,
  Message,
  Usage,
  StreamChunk,
  ProviderConfig,
  AppConfig,
} from './types.js';

export {
  streamCompletion,
} from './provider.js';

export {
  Session,
} from './session.js';

export {
  loadConfig,
  saveConfig,
} from './config.js';
