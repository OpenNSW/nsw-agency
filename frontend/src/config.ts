import { validateConfig } from './configs/types';
import type { UIConfig } from './configs/types';

declare const __BRANDING_CONFIG__: unknown;

function loadConfig(): UIConfig {
  return validateConfig(__BRANDING_CONFIG__, 'VITE_BRANDING_PATH');
}

export const appConfig = loadConfig();