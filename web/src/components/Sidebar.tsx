import { NavLink, useNavigate, useLocation } from 'react-router-dom'
import { ThemeToggle } from './ThemeToggle'
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

export function Sidebar() {
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
      <div className="flex items-center gap-2 px-5 h-16 border-b border-border dark:border-border-dark">
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
            <div className="text-xs text-muted dark:text-muted-dark truncate mb-1">
              {user.name}
            </div>
            <button
              onClick={logout}
              className="text-xs text-muted dark:text-muted-dark hover:text-foreground dark:hover:text-foreground-dark transition-colors"
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
