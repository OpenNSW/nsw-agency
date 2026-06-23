import { z } from 'zod'

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

const UIConfigSchema = z.object({
  branding: z.object({
    systemName: z.string().min(1),
    appName: z.string().min(1),
    logoUrl: z.string().optional(),
    systemLogoUrl: z.string().optional(),
    favicon: z.string().optional(),
    portalName: z.string().optional(),
    description: z.string().optional(),
    heroImageUrl: z.string().optional(),
    partnerLogos: z.array(z.object({ url: z.string(), alt: z.string() })).optional(),
  }),
  theme: z
    .object({
      fontFamily: z.string(),
      borderRadius: z.string(),
    })
    .optional(),
  features: z
    .object({
      preConsignment: z.boolean(),
      consignmentManagement: z.boolean(),
      reportingDashboard: z.boolean(),
    })
    .optional(),
})

export type UIConfig = z.infer<typeof UIConfigSchema>

function validateConfig(parsed: unknown, instanceId: string): UIConfig {
  const result = UIConfigSchema.safeParse(parsed)
  if (!result.success) {
    throw new Error(
      'Invalid configuration for ' +
        instanceId +
        ':\n' +
        result.error.issues.map((i) => '- ' + i.path.join('.') + ': ' + i.message).join('\n'),
    )
  }
  return result.data
}
