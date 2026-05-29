import { useCallback } from 'react'
import { useAsgardeo } from '@asgardeo/react'

export function useSignOutHandler(): () => void {
  const { signOut } = useAsgardeo() as unknown as { signOut: () => Promise<unknown> }

  return useCallback(() => {
    void (async () => {
      try {
        await signOut()
      } catch (error) {
        console.warn('[Auth] Server-side logout failed (expected with Thunder IDP). Forcing local logout.', error)
      } finally {
        // Force Local Logout Execution
        for (const key of Object.keys(sessionStorage)) {
          if (key.startsWith('asgardeo') || key.startsWith('instance_')) {
            sessionStorage.removeItem(key)
          }
        }
        for (const key of Object.keys(localStorage)) {
          if (key.startsWith('asgardeo') || key.startsWith('instance_')) {
            localStorage.removeItem(key)
          }
        }

        const appUrl = import.meta.env.VITE_APP_URL || window.location.origin
        window.location.href = appUrl
      }
    })()
  }, [signOut])
}
