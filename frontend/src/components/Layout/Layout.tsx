import { Outlet } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Sidebar } from './Sidebar'
import { TopBar } from './TopBar'
import { useState } from 'react'

export function Layout() {
  const { t } = useTranslation()
  const [isSidebarExpanded, setIsSidebarExpanded] = useState(() => {
    const savedState = localStorage.getItem('sidebarExpanded')
    return savedState !== null ? savedState === 'true' : true
  })
  const sidebarWidth = isSidebarExpanded ? 256 : 80

  const handleToggleSidebar = () => {
    setIsSidebarExpanded((prev) => {
      const newState = !prev
      localStorage.setItem('sidebarExpanded', String(newState))
      return newState
    })
  }

  return (
    <div className="min-h-screen bg-gray-50">
      <TopBar />

      <div className="flex">
        <Sidebar isExpanded={isSidebarExpanded} onToggle={handleToggleSidebar} />

        <main
          style={{ marginLeft: `${sidebarWidth}px`, width: `calc(100% - ${sidebarWidth}px)` }}
          className="flex min-h-[calc(100vh-64px)] flex-col transition-all duration-300 mt-16 p-8"
        >
          {/* Plain block wrapper: flex items with mx-auto stop stretching, which
              would shrink pages using max-w-* mx-auto to fit-content width. */}
          <div className="w-full">
            <Outlet />
          </div>
          <footer className="mt-auto pt-8 text-right text-sm text-gray-500">
            <a
              href="https://github.com/OpenNSW"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-gray-700 hover:underline"
            >
              {t('footer.poweredBy')}
            </a>
          </footer>
        </main>
      </div>
    </div>
  )
}
