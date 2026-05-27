import { BellIcon, MagnifyingGlassIcon } from '@radix-ui/react-icons'
import { SignedIn, SignedOut, SignInButton, useAsgardeo } from '@asgardeo/react'
import { appConfig } from '../../config'
import { useState, useEffect, useRef } from 'react'

export function TopBar() {
  const { signOut, getDecodedIdToken } = useAsgardeo() as unknown as {
    signOut: (options?: unknown, callback?: (url: string) => void) => Promise<unknown>
    getDecodedIdToken: () => Promise<unknown>
  }

  const [userInfo, setUserInfo] = useState<{ name?: string; email?: string } | null>(null)
  const [isMenuOpen, setIsMenuOpen] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    getDecodedIdToken()
      .then((decoded) => {
        const payload =
          (decoded as { decodedIDTokenPayload?: any })?.decodedIDTokenPayload ??
          (decoded as { payload?: any })?.payload ??
          decoded
        const name = payload?.name ?? `${payload?.given_name ?? ''} ${payload?.family_name ?? ''}`.trim()
        setUserInfo({
          name: name || payload?.username || 'User',
          email: payload?.email || '',
        })
      })
      .catch(() => {})
  }, [getDecodedIdToken])

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsMenuOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => {
      document.removeEventListener('mousedown', handleClickOutside)
    }
  }, [])

  const getInitials = (name: string) => {
    if (!name) return 'U'
    const parts = name.split(' ')
    if (parts.length >= 2) {
      return (parts[0][0] + parts[1][0]).toUpperCase()
    }
    return name.slice(0, 2).toUpperCase()
  }

  const handleSignOut = async () => {
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
  }

  return (
    <header className="fixed top-0 left-0 right-0 z-50 h-16 bg-white border-b border-gray-200 flex items-center justify-between px-6">
      {/* Logo + App Name */}
      <div className="flex items-center gap-3">
        {appConfig.branding.systemLogoUrl && (
          <img
            src={appConfig.branding.systemLogoUrl}
            alt={appConfig.branding.portalName}
            className="h-8 w-auto object-contain"
          />
        )}
        <span className="text-xl font-bold text-gray-900">{appConfig.branding.portalName}</span>
      </div>

      {/* Right Side Actions */}
      <div className="flex items-center gap-4">
        {/* Search */}
        <div className="relative">
          <MagnifyingGlassIcon className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400" />
          <input
            type="text"
            placeholder="Search..."
            className="w-64 pl-9 pr-4 py-2 text-sm bg-gray-50 border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          />
        </div>

        {/* Notifications */}
        <button className="relative p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded-lg transition-colors">
          <BellIcon className="w-5 h-5" />
          <span className="absolute top-1.5 right-1.5 w-2 h-2 bg-red-500 rounded-full"></span>
        </button>

        {/* User */}
        <div className="flex items-center gap-3 pl-3 border-l border-gray-200">
          <SignedIn>
            <div className="relative" ref={menuRef}>
              <button
                onClick={() => setIsMenuOpen(!isMenuOpen)}
                className="flex items-center justify-center w-9 h-9 rounded-full bg-blue-600 text-white font-semibold text-sm cursor-pointer shadow-sm hover:bg-blue-700 active:scale-95 transition-all"
              >
                {userInfo ? getInitials(userInfo.name || '') : 'U'}
              </button>

              {isMenuOpen && (
                <div className="absolute right-0 mt-2 w-56 bg-white border border-gray-100 rounded-xl shadow-lg py-2 z-50">
                  <div className="px-4 py-2 border-b border-gray-50">
                    <p className="text-sm font-semibold text-gray-900 truncate">{userInfo?.name}</p>
                    <p className="text-xs text-gray-500 truncate">{userInfo?.email}</p>
                  </div>
                  <button
                    onClick={() => {
                      setIsMenuOpen(false)
                      void handleSignOut()
                    }}
                    className="w-full text-left px-4 py-2.5 text-sm text-red-600 hover:bg-red-50 hover:text-red-700 cursor-pointer flex items-center gap-2 transition-colors"
                  >
                    Sign Out
                  </button>
                </div>
              )}
            </div>
          </SignedIn>
          <SignedOut>
            <SignInButton />
          </SignedOut>
        </div>
      </div>
    </header>
  )
}
