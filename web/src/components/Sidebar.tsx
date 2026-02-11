import { useState, useRef, useEffect } from 'react'
import { NavLink, useNavigate, useLocation } from 'react-router-dom'
import { ThemeToggle } from './ThemeToggle'
import { UserAvatar } from './UserAvatar'
import { visibleNavLinks, navIconMap } from '../lib/constants'
import { useAuth } from '../context/AuthContext'
import { useFetch } from '../hooks/useFetch'
import type { VersionInfo } from '../types'

interface SidebarProps {
  onOpenProfile: () => void
}

export function Sidebar({ onOpenProfile }: SidebarProps) {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [showMenu, setShowMenu] = useState(false)
  const menuRef = useRef<HTMLDivElement>(null)
  const { data: versionInfo } = useFetch<VersionInfo>('/api/version')

  useEffect(() => {
    if (!showMenu) return
    function handleClick(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setShowMenu(false)
      }
    }
    document.addEventListener('mousedown', handleClick)
    return () => document.removeEventListener('mousedown', handleClick)
  }, [showMenu])

  const handleNavClick = (to: string) => (e: React.MouseEvent) => {
    // Always navigate to root of section when sidebar link is clicked
    if (location.pathname !== to) {
      e.preventDefault()
      navigate(to)
    }
  }

  return (
    <aside className="hidden lg:flex flex-col w-60 h-screen sticky top-0
                      border-r border-border dark:border-border-dark
                      bg-panel dark:bg-panel-dark">
      <div className="flex items-center gap-2.5 px-5 h-16 border-b border-border dark:border-border-dark">
        <img src="/android-chrome-192x192.png" alt="" className="w-7 h-7" />
        <span className="text-accent font-mono font-semibold text-sm tracking-widest uppercase">
          StreamMon
        </span>
      </div>

      <nav className="flex-1 px-3 py-4 space-y-1">
        {visibleNavLinks(user?.role).map(link => {
            const Icon = navIconMap[link.icon]
            return (
              <NavLink
                key={link.to}
                to={link.to}
                end={link.to === '/'}
                onClick={handleNavClick(link.to)}
                className={({ isActive }) =>
                  `nav-item ${isActive ? 'active' : ''}`
                }
              >
                {Icon && <Icon className="w-5 h-5" />}
                {link.label}
              </NavLink>
            )
          })}
      </nav>

      {versionInfo?.update_available && user?.role === 'admin' && versionInfo.release_url && (
        <div className="px-3 pb-2">
          <a
            href={versionInfo.release_url}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center justify-center gap-1.5 rounded-lg px-2.5 py-1.5
                       bg-gradient-to-r from-accent/15 to-accent/5
                       dark:from-accent/20 dark:to-accent/5
                       border border-accent/25 hover:border-accent/40
                       transition-colors group"
          >
            <span className="relative flex h-2 w-2 shrink-0">
              <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-accent opacity-60" />
              <span className="relative inline-flex h-2 w-2 rounded-full bg-accent" />
            </span>
            <span className="text-xs font-mono text-accent-dim dark:text-accent group-hover:underline">
              v{versionInfo.latest_version} available
            </span>
          </a>
        </div>
      )}

      <div className="px-3 py-4 border-t border-border dark:border-border-dark">
        {user && (
          <div className="relative flex items-center" ref={menuRef}>
            <button
              onClick={() => setShowMenu(prev => !prev)}
              className="flex items-center gap-2.5 flex-1 min-w-0 rounded-lg px-2 py-1.5
                         hover:bg-surface dark:hover:bg-surface-dark transition-colors text-left"
            >
              <UserAvatar name={user.name} thumbUrl={user.thumb_url} />
              <span className="text-sm truncate">{user.name}</span>
            </button>
            <ThemeToggle />
            {showMenu && (
              <div className="absolute bottom-full left-0 right-0 mb-1 rounded-lg border
                              border-border dark:border-border-dark bg-panel dark:bg-panel-dark
                              shadow-lg overflow-hidden">
                <button
                  onClick={() => { setShowMenu(false); onOpenProfile() }}
                  className="w-full px-3 py-2 text-sm text-left hover:bg-surface
                             dark:hover:bg-surface-dark transition-colors"
                >
                  Profile
                </button>
                <button
                  onClick={() => { setShowMenu(false); logout() }}
                  className="w-full px-3 py-2 text-sm text-left hover:bg-surface
                             dark:hover:bg-surface-dark transition-colors
                             text-red-500 dark:text-red-400"
                >
                  Sign out
                </button>
              </div>
            )}
          </div>
        )}
        {versionInfo && (
          <div className="mt-2 text-center">
            <span className="text-xs font-mono text-muted dark:text-muted-dark">
              {versionInfo.version === 'dev' ? 'dev' : `v${versionInfo.version}`}
            </span>
          </div>
        )}
      </div>
    </aside>
  )
}
