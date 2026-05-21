import { validateConfig } from './configs/types'
import type { UIConfig } from './configs/types'

import { getEnv } from './runtimeConfig'

export let appConfig: UIConfig

export async function initAppConfig(): Promise<void> {
  const brandingName = getEnv('VITE_BRANDING_NAME') || 'default'
  const brandingPath = `/configs/${brandingName}.branding.json`

  try {
    const response = await fetch(brandingPath)
    if (!response.ok) {
      throw new Error(`Failed to fetch branding config from ${brandingPath}: ${response.statusText}`)
    }
    const data = (await response.json()) as unknown
    appConfig = validateConfig(data, brandingName)
  } catch (error) {
    console.warn(`[Config] Failed to load dynamic branding from ${brandingPath}, falling back to default...`, error)
    try {
      const defaultResponse = await fetch('/configs/default.branding.json')
      if (!defaultResponse.ok) {
        throw new Error(`Failed to fetch default branding config: ${defaultResponse.statusText}`)
      }
      const defaultData = (await defaultResponse.json()) as unknown
      appConfig = validateConfig(defaultData, 'default')
    } catch (fallbackError) {
      console.error('[Config] Critical error: Failed to load fallback default branding.', fallbackError)
      // Provide a hardcoded emergency config as a final safety fallback to keep the app working
      appConfig = {
        branding: {
          systemName: 'NSW',
          appName: 'NSW Agency Officer Portal',
          portalName: 'NSW Agency Portal',
          description: 'A unified digital platform enabling regulatory consignments.',
        },
      }
    }
  }
}
