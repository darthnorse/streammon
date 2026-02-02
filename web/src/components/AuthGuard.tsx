import { ReactNode } from 'react'
import { useAuth } from '../context/AuthContext'

export function AuthGuard({ children }: { children: ReactNode }) {
  const { user, loading } = useAuth()

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-muted dark:text-muted-dark">Loading...</div>
      </div>
    )
  }

  if (!user) {
    window.location.href = '/auth/login'
    return null
  }

  return <>{children}</>
}
