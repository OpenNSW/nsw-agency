import { useCallback } from 'react'
import { useThunderID } from '@thunderid/react'

type SignOutFn = (options?: unknown, callback?: (url: string) => void) => Promise<unknown>

export function useSignOutHandler(): () => void {
  const { signOut } = useThunderID() as unknown as { signOut: SignOutFn }

  return useCallback(() => {
    void (async () => {
      try {
        const signOutResult = await signOut(undefined, (redirectUrl: string) => {
          if (redirectUrl) {
            window.location.assign(redirectUrl)
          }
        })

        if (typeof signOutResult === 'string' && signOutResult) {
          window.location.assign(signOutResult)
        }
      } catch {
        // Let the SDK configuration drive sign-out redirects.
      }
    })()
  }, [signOut])
}
