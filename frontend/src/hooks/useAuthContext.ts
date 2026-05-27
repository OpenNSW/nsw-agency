import { useEffect, useState } from 'react'
import { useAsgardeo } from '@asgardeo/react'
import { getExpectedOuHandle } from '../runtimeConfig'

interface UseAuthContextResult {
  isSignedIn: boolean
  isLoading: boolean
  /**
   * null  = not yet resolved (initial state, only while effect hasn't run)
   * true  = signed in and OU handle matches
   * false = signed in but OU handle mismatched, missing, or resolution failed
   */
  isAuthorized: boolean | null
  isResolvingOrg: boolean
}

export function useAuthContext(): UseAuthContextResult {
  const { isSignedIn, isLoading, getDecodedIdToken } = useAsgardeo()
  const [isAuthorized, setIsAuthorized] = useState<boolean | null>(null)
  const [isResolvingOrg, setIsResolvingOrg] = useState(false)

  useEffect(() => {
    let isMounted = true

    const resolveOuHandle = async () => {
      if (isLoading) {
        return
      }

      if (!isSignedIn) {
        if (isMounted) {
          // Reset to null so state is clean for next sign-in attempt
          setIsAuthorized(null)
          setIsResolvingOrg(false)
        }
        return
      }

      setIsResolvingOrg(true)
      try {
        const decodedIdToken = await getDecodedIdToken()
        if (!isMounted) {
          return
        }

        const payload =
          (decodedIdToken as { decodedIDTokenPayload?: unknown })?.decodedIDTokenPayload ??
          (decodedIdToken as { payload?: unknown })?.payload ??
          decodedIdToken

        const ouHandle = (payload as { ouHandle?: unknown })?.ouHandle
        const expectedOu = getExpectedOuHandle()

        setIsAuthorized(typeof ouHandle === 'string' && ouHandle === expectedOu)
      } catch (err) {
        if (!isMounted) {
          return
        }

        // Set to false (not null) so ProtectedLayout renders <UnauthorizedScreen>
        // instead of a permanent blank page. null is reserved for "not yet resolved".
        console.error('[useAuthContext] Failed to resolve OU handle:', err)
        setIsAuthorized(false)
      } finally {
        if (isMounted) {
          setIsResolvingOrg(false)
        }
      }
    }

    void resolveOuHandle()

    return () => {
      isMounted = false
    }
  }, [getDecodedIdToken, isLoading, isSignedIn])

  return {
    isSignedIn,
    isLoading,
    isAuthorized,
    isResolvingOrg,
  }
}
