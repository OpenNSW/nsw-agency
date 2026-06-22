import { UserManager, WebStorageStateStore } from 'oidc-client-ts'
import { getEnv, getRequiredEnv } from '@/runtimeConfig'

const rawScopes = getEnv('VITE_IDP_SCOPES')
const scope = rawScopes
  ? rawScopes
      .split(',')
      .map((s) => s.trim())
      .join(' ')
  : 'openid profile email ou'

export const userManager = new UserManager({
  authority: getRequiredEnv('VITE_IDP_BASE_URL'),
  client_id: getRequiredEnv('VITE_IDP_CLIENT_ID'),
  redirect_uri: getEnv('VITE_APP_URL') ?? window.location.origin,
  post_logout_redirect_uri: getEnv('VITE_APP_URL') ?? window.location.origin,
  scope,
  userStore: new WebStorageStateStore({ store: window.sessionStorage }),
  automaticSilentRenew: true,
})
