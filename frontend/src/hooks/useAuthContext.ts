import { useEffect, useState } from 'react'
import { useAsgardeo } from '@asgardeo/react'
import { getExpectedOuHandle } from '../runtimeConfig'

interface UseAuthContextResult {
  isSignedIn: boolean
  isLoading: boolean
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
      } catch {
        if (!isMounted) {
          return
        }

        setIsAuthorized(null)
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
