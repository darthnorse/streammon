import { Routes, Route, Navigate } from 'react-router-dom'
import { AuthProvider } from './context/AuthContext'
import { AuthGuard } from './components/AuthGuard'
import { ErrorBoundary } from './components/ErrorBoundary'
import { Layout } from './components/Layout'
import { Dashboard } from './pages/Dashboard'
import { Users } from './pages/Users'
import { History } from './pages/History'
import { UserDetail } from './pages/UserDetail'
import { Settings } from './pages/Settings'
import { Statistics } from './pages/Statistics'
import { Libraries } from './pages/Libraries'
import { Rules } from './pages/Rules'
import { Discover } from './pages/Discover'
import { Calendar } from './pages/Calendar'
import { DiscoverAll } from './pages/DiscoverAll'
import { Setup } from './pages/Setup'
import { Login } from './pages/Login'
import { EmptyState } from './components/EmptyState'
import { useAuth } from './context/AuthContext'

function NotFound() {
  return <EmptyState icon="?" title="Page not found" description="The page you're looking for doesn't exist." />
}

function MyStats() {
  const { user } = useAuth()
  if (!user) return null
  return <UserDetail userName={user.name} />
}

function AdminRoute({ children }: { children: React.ReactNode }) {
  const { user } = useAuth()
  if (user?.role !== 'admin') return <Navigate to="/discover" replace />
  return <>{children}</>
}

export default function App() {
  return (
    <AuthProvider>
      <ErrorBoundary>
        <Routes>
          {/* Public routes (no auth required) */}
          <Route path="/setup" element={<Setup />} />
          <Route path="/login" element={<Login />} />

          {/* Protected routes */}
          <Route element={<AuthGuard><Layout /></AuthGuard>}>
            <Route path="/" element={<AdminRoute><Dashboard /></AdminRoute>} />
            <Route path="/discover" element={<Discover />} />
            <Route path="/discover/*" element={<DiscoverAll />} />
            <Route path="/requests" element={<Navigate to="/discover" replace />} />
            <Route path="/calendar" element={<Calendar />} />
            <Route path="/users" element={<AdminRoute><Users /></AdminRoute>} />
            <Route path="/users/:name" element={<UserDetail />} />
            <Route path="/my-stats" element={<MyStats />} />
            <Route path="/history" element={<AdminRoute><History /></AdminRoute>} />
            <Route path="/statistics" element={<AdminRoute><Statistics /></AdminRoute>} />
            <Route path="/library" element={<AdminRoute><Libraries /></AdminRoute>} />
            <Route path="/rules" element={<AdminRoute><Rules /></AdminRoute>} />
            <Route path="/settings" element={<AdminRoute><Settings /></AdminRoute>} />
            <Route path="*" element={<NotFound />} />
          </Route>
        </Routes>
      </ErrorBoundary>
    </AuthProvider>
  )
}
