import { getRequiredEnv } from '../runtimeConfig'

interface RequestConfig {
  url: string
  method?: string
  headers?: Record<string, string>
  params?: Record<string, string | number | boolean | undefined | null>
  data?: unknown
  attachToken?: boolean
  signal?: AbortSignal
}

export const http = {
  request: async (config: RequestConfig) => {
    let url = config.url
    if (config.params) {
      const searchParams = new URLSearchParams()
      for (const [key, value] of Object.entries(config.params)) {
        if (value !== undefined && value !== null) {
          searchParams.append(key, String(value))
        }
      }
      const queryString = searchParams.toString()
      if (queryString) {
        url += (url.includes('?') ? '&' : '?') + queryString
      }
    }

    const headers = { ...config.headers }

    if (config.attachToken) {
      try {
        const idpBaseUrl = getRequiredEnv('VITE_IDP_BASE_URL')
        const clientId = getRequiredEnv('VITE_IDP_CLIENT_ID')
        const storageKey = `oidc.user:${idpBaseUrl}:${clientId}`
        const oidcUserStr = sessionStorage.getItem(storageKey)
        if (oidcUserStr) {
          const oidcUser = JSON.parse(oidcUserStr) as Record<string, unknown> | null
          if (oidcUser && typeof oidcUser.access_token === 'string') {
            headers['Authorization'] = `Bearer ${oidcUser.access_token}`
          }
        }
      } catch (e) {
        console.error('Error attaching OIDC access token to request:', e)
      }
    }

    if (config.data && !headers['Content-Type']) {
      headers['Content-Type'] = 'application/json'
    }

    const response = await fetch(url, {
      method: config.method || 'GET',
      headers,
      body: config.data ? JSON.stringify(config.data) : undefined,
      signal: config.signal,
    })

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }

    const contentType = response.headers.get('content-type')
    const data = (
      contentType && contentType.includes('application/json') ? await response.json() : await response.text()
    ) as unknown

    return { data }
  },
}
