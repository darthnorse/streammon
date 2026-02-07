import { ReactNode } from 'react'
import { Navigate } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { LoadingScreen } from './LoadingScreen'

export function AuthGuard({ children }: { children: ReactNode }) {
  const { user, loading, setupRequired } = useAuth()

  if (loading) {
    return <LoadingScreen />
  }

  if (setupRequired) {
    return <Navigate to="/setup" replace />
  }

  if (!user) {
    return <Navigate to="/login" replace />
  }

  return <>{children}</>
}
