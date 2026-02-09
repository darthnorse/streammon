import { NavLink, useNavigate, useLocation } from 'react-router-dom'
import { ThemeToggle } from './ThemeToggle'
import { UserAvatar } from './UserAvatar'
import { navLinks } from '../lib/constants'
import { useAuth } from '../context/AuthContext'
import {
  LayoutDashboard,
  History,
  BarChart3,
  Library,
  Users,
  ShieldAlert,
  Settings,
  Film,
} from 'lucide-react'

const iconMap: Record<string, React.ComponentType<{ className?: string }>> = {
  LayoutDashboard,
  History,
  BarChart3,
  Library,
  Users,
  ShieldAlert,
  Settings,
  Film,
}

interface SidebarProps {
  onOpenProfile: () => void
}

export function Sidebar({ onOpenProfile }: SidebarProps) {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()

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
        {navLinks
          .filter(link => !link.adminOnly || user?.role === 'admin')
          .map(link => {
            const Icon = iconMap[link.icon]
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

      <div className="px-3 py-4 border-t border-border dark:border-border-dark">
        {user && (
          <div className="mb-3">
            <button
              onClick={onOpenProfile}
              className="flex items-center gap-2.5 w-full rounded-lg px-2 py-1.5
                         hover:bg-surface dark:hover:bg-surface-dark transition-colors text-left"
            >
              <UserAvatar name={user.name} thumbUrl={user.thumb_url} />
              <span className="text-sm truncate">{user.name}</span>
            </button>
            <button
              onClick={logout}
              className="text-xs text-muted dark:text-muted-dark hover:text-foreground
                         dark:hover:text-foreground-dark transition-colors mt-1 ml-2"
            >
              Sign out
            </button>
          </div>
        )}
        <ThemeToggle />
      </div>
    </aside>
  )
}
