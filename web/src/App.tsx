import { Routes, Route } from 'react-router-dom'
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
import { Requests } from './pages/Requests'
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
            <Route path="/" element={<Dashboard />} />
            <Route path="/requests/discover/*" element={<DiscoverAll />} />
            <Route path="/requests" element={<Requests />} />
            <Route path="/users" element={<Users />} />
            <Route path="/users/:name" element={<UserDetail />} />
            <Route path="/my-stats" element={<MyStats />} />
            <Route path="/history" element={<History />} />
            <Route path="/statistics" element={<Statistics />} />
            <Route path="/library" element={<Libraries />} />
            <Route path="/rules" element={<Rules />} />
            <Route path="/settings" element={<Settings />} />
            <Route path="*" element={<NotFound />} />
          </Route>
        </Routes>
      </ErrorBoundary>
    </AuthProvider>
  )
}
