import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { Theme } from '@radix-ui/themes'
import { ThunderIDProvider } from '@thunderid/react'
import '@radix-ui/themes/styles.css'
import './index.css'
import App from './App.tsx'
import { getEnv, getRequiredEnv } from './runtimeConfig'
import { initAppConfig } from './config.ts'

const APP_URL = getEnv('VITE_APP_URL', window.location.origin)
const CLIENT_ID = getRequiredEnv('VITE_IDP_CLIENT_ID')
const IDP_BASE_URL = getRequiredEnv('VITE_IDP_BASE_URL')
// organizationHandle is mandatory when a custom domain is used (i.e. the base URL
// does not follow the /t/{orgHandle} pattern). Re-uses the OU handle env var since
// both values identify the same agency organisation in ThunderID.
const IDP_ORG_HANDLE = getEnv('VITE_IDP_EXPECTED_OU_HANDLE')
const rawScopes = getEnv('VITE_IDP_SCOPES')
const IDP_SCOPES = rawScopes
  ? rawScopes.split(',').map((scope: string) => scope.trim())
  : ['openid', 'profile', 'email', 'ou']

void initAppConfig().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <ThunderIDProvider
        clientId={CLIENT_ID}
        baseUrl={IDP_BASE_URL}
        organizationHandle={IDP_ORG_HANDLE}
        signInUrl={APP_URL}
        afterSignInUrl={APP_URL}
        afterSignOutUrl={APP_URL}
        scopes={IDP_SCOPES}
        storage="sessionStorage"
        tokenLifecycle={{ refreshToken: { autoRefresh: true } }}
      >
        <Theme scaling="110%">
          <BrowserRouter>
            <App />
          </BrowserRouter>
        </Theme>
      </ThunderIDProvider>
    </StrictMode>,
  )
})
