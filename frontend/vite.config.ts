import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react-swc'
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";
import fs from 'node:fs';
import { parse as parseYaml } from 'yaml';

function deepMerge(base: any, override: any): any {
  const result = { ...base };
  for (const key of Object.keys(override)) {
    if (
      override[key] && typeof override[key] === 'object' && !Array.isArray(override[key]) &&
      base[key] && typeof base[key] === 'object' && !Array.isArray(base[key])
    ) {
      result[key] = deepMerge(base[key], override[key]);
    } else {
      result[key] = override[key];
    }
  }
  return result;
}

// Load base config
const defaultYamlPath = path.resolve(import.meta.dirname, 'src/configs/default.yaml');
const defaultYaml = fs.readFileSync(defaultYamlPath, 'utf8');
let mergedConfig = parseYaml(defaultYaml);

// Load custom branding if provided
const brandingPath = process.env.VITE_BRANDING_PATH;
if (brandingPath) {
  const absolutePath = path.resolve(import.meta.dirname, brandingPath);
  if (fs.existsSync(absolutePath)) {
    const customYaml = fs.readFileSync(absolutePath, 'utf8');
    const customConfig = parseYaml(customYaml);
    mergedConfig = deepMerge(mergedConfig, customConfig);
    console.log(`[Vite] Loaded branding configuration from: ${absolutePath}`);
  } else {
    console.warn(`[Vite] Branding configuration file not found at: ${absolutePath}`);
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: process.env.VITE_PORT ? parseInt(process.env.VITE_PORT, 10) : 5174,
  },
  define: {
    __BRANDING_CONFIG__: JSON.stringify(mergedConfig),
  },
  resolve: {
    alias: {
      '@opennsw/ui': path.resolve(import.meta.dirname, '../../packages/ui/src'),
      '@opennsw/jsonforms-renderers': path.resolve(import.meta.dirname, '../../packages/jsonforms-renderers/src')
    }
  }
})
