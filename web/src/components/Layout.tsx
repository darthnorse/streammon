import { useState, useMemo } from 'react'
import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { MobileNav } from './MobileNav'
import { ThemeToggle } from './ThemeToggle'
import { ProfileModal } from './ProfileModal'
import { UserAvatar } from './UserAvatar'
import { useAuth } from '../context/AuthContext'
import { useFetch } from '../hooks/useFetch'
import type { IntegrationStatus } from '../lib/constants'
import type { OverseerrRequestCount } from '../types'

export function Layout() {
  const { user } = useAuth()
  const [showProfile, setShowProfile] = useState(false)
  const { data: sonarrStatus } = useFetch<{ configured: boolean }>('/api/sonarr/configured')
  const { data: overseerrStatus } = useFetch<{ configured: boolean }>('/api/overseerr/configured')
  const { data: guestResp } = useFetch<{ settings: Record<string, boolean> }>('/api/settings/guest')
  const guestSettings = guestResp?.settings

  const sonarrConfigured = sonarrStatus?.configured ?? false
  const overseerrConfigured = overseerrStatus?.configured ?? false
  const isAdmin = user?.role === 'admin'
  const { data: requestCounts } = useFetch<OverseerrRequestCount>(
    overseerrConfigured && isAdmin ? '/api/overseerr/requests/count' : null,
  )
  const pendingCount = requestCounts?.pending ?? 0

  const integrationsLoaded = sonarrStatus !== null && overseerrStatus !== null && guestResp !== null
  const integrations = useMemo<IntegrationStatus | undefined>(() => {
    if (!integrationsLoaded) return undefined
    return {
      sonarr: sonarrConfigured,
      overseerr: overseerrConfigured,
      discover: isAdmin || (guestSettings?.show_discover ?? true),
      profile: isAdmin || (guestSettings?.visible_profile ?? true),
      calendar: sonarrConfigured && (isAdmin || (guestSettings?.show_calendar ?? true)),
    }
  }, [integrationsLoaded, sonarrConfigured, overseerrConfigured, guestSettings, isAdmin])

  return (
    <div className="flex min-h-screen scanlines">
      <Sidebar onOpenProfile={() => setShowProfile(true)} integrations={integrations} pendingCount={pendingCount} />

      <div className="flex-1 flex flex-col min-w-0">
        <header className="lg:hidden flex items-center justify-between px-4 h-14
                          border-b border-border dark:border-border-dark
                          bg-panel dark:bg-panel-dark sticky top-0 z-40">
          <div className="flex items-center gap-2.5">
            <img src="/android-chrome-192x192.png" alt="" className="w-7 h-7" />
            <span className="text-accent font-mono font-semibold text-sm tracking-widest uppercase">
              StreamMon
            </span>
          </div>
          <div className="flex items-center gap-2">
            <ThemeToggle />
            {user && (
              <button
                onClick={() => setShowProfile(true)}
                className="shrink-0"
                aria-label="Open profile"
              >
                <UserAvatar name={user.name} thumbUrl={user.thumb_url} />
              </button>
            )}
          </div>
        </header>

        <main className="flex-1 p-4 md:p-6 lg:p-8 pb-20 lg:pb-8">
          <Outlet />
        </main>
      </div>

      <MobileNav integrations={integrations} pendingCount={pendingCount} />

      {showProfile && <ProfileModal onClose={() => setShowProfile(false)} />}
    </div>
  )
}
