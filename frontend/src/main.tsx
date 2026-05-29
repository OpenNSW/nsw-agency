// Intercept Asgardeo SDK calls to enterprise endpoints unsupported by the lightweight Thunder Go IDP.
// The SDK uses BOTH window.fetch AND legacy XMLHttpRequest depending on the call — we must patch both.
;(function interceptAsgardeoUnsupportedEndpoints() {
  const MOCKED_ENDPOINTS: Record<string, string> = {
    '/scim2/Me': '{}',
    '/api/users/v1/me/organizations': '{"count":0,"organizations":[]}',
    '/api/server/v1/branding-preference/resolve': '{"preference":{"theme":{"LIGHT":{},"DARK":{}}}}',
  }

  // --- Layer 1: Intercept window.fetch (used for some SDK calls) ---
  const _originalFetch = window.fetch
  window.fetch = async function (input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
    const url = typeof input === 'string' ? input : input instanceof Request ? input.url : input.toString()
    for (const [path, body] of Object.entries(MOCKED_ENDPOINTS)) {
      if (url.includes(path)) {
        console.warn(`[Fetch Interceptor] Mocking ${url}`)
        return new Response(body, {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        })
      }
    }
    return _originalFetch(input, init)
  }

  // --- Layer 2: Intercept XMLHttpRequest (used by Asgardeo SDK for /scim2/Me and /organizations) ---
  const _open = XMLHttpRequest.prototype.open
  const _send = XMLHttpRequest.prototype.send

  XMLHttpRequest.prototype.open = function (
    this: XMLHttpRequest & { _mockedResponse?: string },
    method: string,
    url: string | URL,
    ...rest: unknown[]
  ) {
    const urlStr = typeof url === 'string' ? url : url.toString()
    for (const [path, body] of Object.entries(MOCKED_ENDPOINTS)) {
      if (urlStr.includes(path)) {
        this._mockedResponse = body
        console.warn(`[XHR Interceptor] Mocking ${method} ${urlStr}`)
        return _open.call(this, 'GET', 'about:blank', true)
      }
    }
    return (_open as unknown as (...a: unknown[]) => void).call(this, method, url, ...rest)
  }

  XMLHttpRequest.prototype.send = function (
    this: XMLHttpRequest & { _mockedResponse?: string },
    body?: Document | XMLHttpRequestBodyInit | null,
  ) {
    if (this._mockedResponse !== undefined) {
      const mockBody = this._mockedResponse
      Object.defineProperty(this, 'readyState', { get: () => 4, configurable: true })
      Object.defineProperty(this, 'status', { get: () => 200, configurable: true })
      Object.defineProperty(this, 'responseText', { get: () => mockBody, configurable: true })
      Object.defineProperty(this, 'response', { get: () => mockBody, configurable: true })
      setTimeout(() => {
        this.dispatchEvent(new ProgressEvent('load', { loaded: mockBody.length, total: mockBody.length }))
        this.dispatchEvent(new ProgressEvent('loadend', { loaded: mockBody.length, total: mockBody.length }))
      }, 0)
      return
    }
    return _send.call(this, body)
  }
})()



import { StrictMode, type ComponentProps, type ReactElement } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { Theme } from '@radix-ui/themes'
import { AsgardeoProvider } from '@asgardeo/react'
import '@radix-ui/themes/styles.css'
import './index.css'
import App from './App.tsx'
import { getEnv, getRequiredEnv } from './runtimeConfig'
import { initAppConfig } from './config.ts'

type AgencyAsgardeoProviderProps = ComponentProps<typeof AsgardeoProvider> & {
  periodicTokenRefresh?: boolean
  endpoints?: Record<string, string>
}

const AgencyAsgardeoProvider = AsgardeoProvider as unknown as (props: AgencyAsgardeoProviderProps) => ReactElement

const APP_URL = getEnv('VITE_APP_URL', window.location.origin)
const CLIENT_ID = getRequiredEnv('VITE_IDP_CLIENT_ID')
const IDP_BASE_URL = getRequiredEnv('VITE_IDP_BASE_URL')
const rawScopes = getEnv('VITE_IDP_SCOPES')
const IDP_SCOPES = rawScopes
  ? rawScopes.split(',').map((scope: string) => scope.trim())
  : ['openid', 'profile', 'email', 'ou']

// Explicit OIDC endpoints force the Asgardeo SDK into standard compliance mode.
// Without idpPlatform: "IdentityServer", the SDK does NOT fire legacy enterprise WSO2
// hydration calls (/scim2/Me, /api/users/v1/me/organizations) that Thunder does not support.
// The issuer field resolves the JWT iss claim validation without needing idpPlatform as a workaround.
const IDP_ENDPOINTS = {
  authorizationEndpoint: `${IDP_BASE_URL}/oauth2/authorize`,
  tokenEndpoint: `${IDP_BASE_URL}/oauth2/token`,
  jwksUri: `${IDP_BASE_URL}/oauth2/jwks`,
  userinfoEndpoint: `${IDP_BASE_URL}/oauth2/userinfo`,
  issuer: `${IDP_BASE_URL}/oauth2/token`,
}

void initAppConfig().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <AgencyAsgardeoProvider
        clientId={CLIENT_ID}
        baseUrl={IDP_BASE_URL}
        signInUrl={APP_URL}
        afterSignInUrl={APP_URL}
        afterSignOutUrl={APP_URL}
        scopes={IDP_SCOPES}
        storage="sessionStorage"
        periodicTokenRefresh
        endpoints={IDP_ENDPOINTS}
      >
        <Theme scaling="110%">
          <BrowserRouter>
            <App />
          </BrowserRouter>
        </Theme>
      </AgencyAsgardeoProvider>
    </StrictMode>,
  )
})

