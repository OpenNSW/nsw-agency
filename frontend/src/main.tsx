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
        afterSignInUrl={`${APP_URL}/consignments`}
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
