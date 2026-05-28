import { useEffect, useState } from 'react'
import { useThunderID } from '@thunderid/react'
import { getExpectedOuHandle } from '../runtimeConfig'

interface UseAuthContextResult {
  isSignedIn: boolean
  isLoading: boolean
  isAuthorized: boolean | null
  isResolvingOrg: boolean
}

export function useAuthContext(): UseAuthContextResult {
  const { isSignedIn, isLoading, getDecodedIdToken } = useThunderID()
  const [isAuthorized, setIsAuthorized] = useState<boolean | null>(null)
  const [isResolvingOrg, setIsResolvingOrg] = useState(false)
  const expectedOu = getExpectedOuHandle()
  useEffect(() => {
    let isMounted = true

    const resolveOuHandle = async () => {
      if (isLoading) return

      if (!isSignedIn) {
        if (isMounted) {
          setIsAuthorized(null)
          setIsResolvingOrg(false)
        }
        return
      }
      if (isMounted) setIsResolvingOrg(true)
      try {
        const decodedToken = await getDecodedIdToken()
        if (!isMounted) return

        // Thunder issues `ouHandle` when the `ou` scope is requested.
        // Narrow from `any` explicitly via a typeof guard.
        // If `ouHandle` is absent the token was issued without the `ou` scope —
        // deny access so the server configuration is fixed rather than silently bypassed.
        const ouHandle = typeof decodedToken?.ouHandle === 'string' ? decodedToken.ouHandle : undefined
        setIsAuthorized(ouHandle === expectedOu)
      } catch {
        if (isMounted) setIsAuthorized(false)
      } finally {
        if (isMounted) setIsResolvingOrg(false)
      }
    }

    void resolveOuHandle()

    return () => {
      isMounted = false
    }
  }, [getDecodedIdToken, isLoading, isSignedIn])

  return { isSignedIn, isLoading, isAuthorized, isResolvingOrg }
}
