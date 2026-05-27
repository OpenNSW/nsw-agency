import { BellIcon, MagnifyingGlassIcon } from '@radix-ui/react-icons'
import { SignedIn, SignedOut, SignInButton, UserDropdown, useThunderID } from '@thunderid/react'
import { appConfig } from '../../config'

export function TopBar() {
  const { signOut } = useThunderID() as unknown as {
    signOut: (options?: unknown, callback?: (url: string) => void) => Promise<unknown>
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
            <UserDropdown
              onSignOut={() => {
                void handleSignOut()
              }}
            />
          </SignedIn>
          <SignedOut>
            <SignInButton />
          </SignedOut>
        </div>
      </div>
    </header>
  )
}
