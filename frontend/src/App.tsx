import type { ReactNode } from 'react'
import { Routes, Route, Navigate } from 'react-router-dom'
import { Layout } from './components/Layout'
import { ConsignmentListScreen } from './features/consignment/ConsignmentListScreen'
import { ApplicationListScreen } from './features/application/ApplicationListScreen'
import { ApplicationDetailScreen } from './features/application/ApplicationDetailScreen'
import { appConfig } from './config.ts'
import { useEffect } from 'react'
import { SignedOut } from './components/Auth'
import { LoginScreen } from './screens/LoginScreen'
import { useAuthContext } from './hooks/useAuthContext'
import { UnauthorizedScreen } from './screens/UnauthorizedScreen'
import { uploadFile, getDownloadUrl } from './features/application/service'
import { UploadProvider } from '@opennsw/jsonforms-renderers'

function UploadWrapper({ children }: { children: ReactNode }) {
  return (
    <UploadProvider onUpload={uploadFile} getDownloadUrl={getDownloadUrl}>
      {children}
    </UploadProvider>
  )
}

function ProtectedLayout() {
  const { isSignedIn, isLoading, isAuthorized, isResolvingOrg } = useAuthContext()

  if (isLoading || (isSignedIn && (isResolvingOrg || isAuthorized === null))) return null
  if (!isSignedIn) return <Navigate to="/login" replace />
  if (isAuthorized === false) return <UnauthorizedScreen />

  return (
    <UploadWrapper>
      <Layout />
    </UploadWrapper>
  )
}

function App() {
  useEffect(() => {
    document.title = `${appConfig.branding.portalName || appConfig.branding.appName} | ${appConfig.branding.systemName}`

    if (appConfig.branding.favicon) {
      const link = (document.querySelector("link[rel~='icon']") as HTMLLinkElement) ?? document.createElement('link')
      link.rel = 'icon'
      link.href = appConfig.branding.favicon
      document.head.appendChild(link)
    }
  }, [])

  return (
    <Routes>
      <Route
        path="/login"
        element={
          <SignedOut fallback={<Navigate to="/" replace />}>
            <LoginScreen />
          </SignedOut>
        }
      />

      <Route element={<ProtectedLayout />}>
        <Route path="/" element={<Navigate to="/consignments" replace />} />
        <Route path="/consignments" element={<ConsignmentListScreen />} />
        <Route path="/consignments/:consignmentId/tasks" element={<ApplicationListScreen />} />
        <Route path="/consignments/:consignmentId" element={<ApplicationDetailScreen />} />
      </Route>

      <Route path="*" element={<Navigate to="/login" replace />} />
    </Routes>
  )
}

export default App
