import { NavLink } from 'react-router-dom'
import { navLinks } from '../lib/constants'
import {
  LayoutDashboard,
  History,
  BarChart3,
  Library,
  Users,
  ShieldAlert,
  Settings,
} from 'lucide-react'

const iconMap: Record<string, React.ComponentType<{ className?: string }>> = {
  LayoutDashboard,
  History,
  BarChart3,
  Library,
  Users,
  ShieldAlert,
  Settings,
}

export function MobileNav() {
  return (
    <nav className="lg:hidden fixed bottom-0 left-0 right-0 z-50
                    bg-panel dark:bg-panel-dark
                    border-t border-border dark:border-border-dark
                    pb-[env(safe-area-inset-bottom)]">
      <div className="flex items-center justify-around h-16">
        {navLinks.map(link => {
          const Icon = iconMap[link.icon]
          return (
            <NavLink
              key={link.to}
              to={link.to}
              end={link.to === '/'}
              className={({ isActive }) =>
                `flex flex-col items-center gap-1 px-3 py-2 min-w-[64px]
                 text-xs font-medium transition-colors
                 ${isActive
                   ? 'text-accent-dim dark:text-accent'
                   : 'text-muted dark:text-muted-dark'}`
              }
            >
              {Icon && <Icon className="w-5 h-5" />}
              {link.label}
            </NavLink>
          )
        })}
      </div>
    </nav>
  )
}
