import { useAuth } from 'react-oidc-context'
import { getExpectedOuHandle } from '../runtimeConfig'

interface UseAuthContextResult {
  isSignedIn: boolean
  isLoading: boolean
  isAuthorized: boolean | null
  isResolvingOrg: boolean
}

export function useAuthContext(): UseAuthContextResult {
  const auth = useAuth()

  const isSignedIn = auth.isAuthenticated
  const isLoading = auth.isLoading

  let isAuthorized: boolean | null = null

  if (isSignedIn && auth.user) {
    try {
      const expectedOu = getExpectedOuHandle()
      const profile = auth.user.profile as Record<string, unknown>
      const ouHandle = typeof profile.ouHandle === 'string' ? profile.ouHandle : undefined
      isAuthorized = ouHandle === expectedOu
    } catch (e) {
      console.error('Error verifying organization handle:', e)
      isAuthorized = false
    }
  }

  return {
    isSignedIn,
    isLoading,
    isAuthorized,
    isResolvingOrg: false,
  }
}
