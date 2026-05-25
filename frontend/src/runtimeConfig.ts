type RuntimeConfigValue = string | undefined

type RuntimeConfigMap = Record<string, RuntimeConfigValue>

declare global {
  interface Window {
    __APP_CONFIG__?: RuntimeConfigMap
  }
}

function resolveRuntimeConfig(): RuntimeConfigMap {
  if (typeof window === 'undefined') {
    return {}
  }

  return window.__APP_CONFIG__ ?? {}
}

export function getEnv(name: string, fallback?: string): string | undefined {
  const runtimeValue = resolveRuntimeConfig()[name]
  if (runtimeValue && runtimeValue.trim() !== '') {
    return runtimeValue
  }

  const buildValue = (import.meta.env as Record<string, string | undefined>)[name]
  if (buildValue && buildValue.trim() !== '') {
    return buildValue
  }

  return fallback
}

export function getRequiredEnv(name: string): string {
  const value = getEnv(name)
  if (!value || value.trim() === '') {
    throw new Error(`Missing required environment variable: ${name}`)
  }

  return value
}

export function getExpectedOuHandle(): string {
  return getRequiredEnv('VITE_IDP_EXPECTED_OU_HANDLE')
}
