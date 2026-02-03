import { Routes, Route } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import { AuthGuard } from './components/AuthGuard'
import { ErrorBoundary } from './components/ErrorBoundary'
import { Layout } from './components/Layout'
import { Dashboard } from './pages/Dashboard'
import { History } from './pages/History'
import { UserDetail } from './pages/UserDetail'
import { Settings } from './pages/Settings'
import { EmptyState } from './components/EmptyState'

function NotFound() {
  return <EmptyState icon="?" title="Page not found" description="The page you're looking for doesn't exist." />
}

export default function App() {
  return (
    <AuthProvider>
      <AuthGuard>
        <ErrorBoundary>
          <Routes>
            <Route element={<Layout />}>
              <Route path="/" element={<Dashboard />} />
              <Route path="/history" element={<History />} />
              <Route path="/users/:name" element={<UserDetail />} />
              <Route path="/settings" element={<Settings />} />
              <Route path="*" element={<NotFound />} />
            </Route>
          </Routes>
        </ErrorBoundary>
      </AuthGuard>
    </AuthProvider>
  )
}
