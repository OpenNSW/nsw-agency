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
          <Outlet />
          <footer className="mt-auto pt-8 text-right text-sm text-gray-500">{t('footer.poweredBy')}</footer>
        </main>
      </div>
    </div>
  )
}
