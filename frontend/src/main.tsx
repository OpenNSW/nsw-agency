// Hook global fetch to intercept Asgardeo endpoints that are not supported by the lightweight Go IDP (Thunder)
(function interceptAsgardeoUnsupportedEndpoints() {
  const originalFetch = window.fetch;

  window.fetch = async function (input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
    const url = typeof input === 'string' ? input : (input instanceof Request ? input.url : input.toString());

    if (url) {
      // 1. Intercept branding preference resolution
      if (url.includes('/api/server/v1/branding-preference/resolve')) {
        console.warn("Intercepted Asgardeo branding fetch. Returning mock 200 OK.");
        return new Response(
          JSON.stringify({
            theme: {
              activeTheme: "default",
              displayName: "Default Theme"
            }
          }),
          {
            status: 200,
            headers: {
              "Content-Type": "application/json",
              "Access-Control-Allow-Origin": "*",
              "Access-Control-Allow-Methods": "GET, OPTIONS",
              "Access-Control-Allow-Headers": "*"
            }
          }
        );
      }

      // 2. Intercept B2B Organization resolution to prevent blank screens on post-login redirect
      if (url.includes('/api/users/v1/me/organizations')) {
        console.warn("Intercepted Asgardeo organizations fetch. Returning mock empty organizations list.");
        return new Response(
          JSON.stringify({
            organizations: []
          }),
          {
            status: 200,
            headers: {
              "Content-Type": "application/json",
              "Access-Control-Allow-Origin": "*",
              "Access-Control-Allow-Methods": "GET, OPTIONS",
              "Access-Control-Allow-Headers": "*"
            }
          }
        );
      }
    }

    return originalFetch(input, init);
  };
})();

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
}

const AgencyAsgardeoProvider = AsgardeoProvider as unknown as (props: AgencyAsgardeoProviderProps) => ReactElement

const normalizeIdpPlatform = (value: string): 'AsgardeoV2' | 'Asgardeo' | 'IdentityServer' | 'Unknown' => {
  if (value === 'AsgardeoV2' || value === 'Asgardeo' || value === 'IdentityServer' || value === 'Unknown') {
    return value
  }

  return 'AsgardeoV2'
}

const APP_URL = getEnv('VITE_APP_URL', window.location.origin)
const CLIENT_ID = getRequiredEnv('VITE_IDP_CLIENT_ID')
const IDP_BASE_URL = getRequiredEnv('VITE_IDP_BASE_URL')
const IDP_PLATFORM = normalizeIdpPlatform(getEnv('VITE_IDP_PLATFORM', 'AsgardeoV2')!)
const rawScopes = getEnv('VITE_IDP_SCOPES')
const IDP_SCOPES = rawScopes
  ? rawScopes.split(',').map((scope: string) => scope.trim())
  : ['openid', 'profile', 'email', 'ou']

void initAppConfig().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <AgencyAsgardeoProvider
        clientId={CLIENT_ID}
        baseUrl={IDP_BASE_URL}
        platform={IDP_PLATFORM}
        signInUrl={APP_URL}
        afterSignInUrl={APP_URL}
        afterSignOutUrl={APP_URL}
        scopes={IDP_SCOPES}
        storage="sessionStorage"
        periodicTokenRefresh
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
