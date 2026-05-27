import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router-dom'
import { Theme } from '@radix-ui/themes'
import { AuthProvider } from 'react-oidc-context'
import { WebStorageStateStore } from 'oidc-client-ts'
import '@radix-ui/themes/styles.css'
import './index.css'
import App from './App.tsx'
import { getEnv, getRequiredEnv } from './runtimeConfig'
import { initAppConfig } from './config.ts'

const APP_URL = getEnv('VITE_APP_URL', window.location.origin)
const CLIENT_ID = getRequiredEnv('VITE_IDP_CLIENT_ID')
const IDP_BASE_URL = getRequiredEnv('VITE_IDP_BASE_URL')

const rawScopes = getEnv('VITE_IDP_SCOPES')
const IDP_SCOPES = rawScopes
  ? rawScopes.split(',').map((scope: string) => scope.trim())
  : ['openid', 'profile', 'email', 'ou']

const oidcConfig = {
  authority: IDP_BASE_URL,
  client_id: CLIENT_ID,
  redirect_uri: APP_URL,
  post_logout_redirect_uri: APP_URL,
  scope: IDP_SCOPES.join(' '),
  userStore: new WebStorageStateStore({ store: window.sessionStorage }),
  automaticSilentRenew: true,
  onSigninCallback: () => {
    window.history.replaceState({}, document.title, window.location.pathname)
  },
}

void initAppConfig().then(() => {
  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <AuthProvider {...oidcConfig}>
        <Theme scaling="110%">
          <BrowserRouter>
            <App />
          </BrowserRouter>
        </Theme>
      </AuthProvider>
    </StrictMode>,
  )
})
