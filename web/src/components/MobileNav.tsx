import { useState } from 'react'
import { NavLink, useNavigate, useLocation } from 'react-router-dom'
import { navLinks } from '../lib/constants'
import {
  LayoutDashboard,
  History,
  BarChart3,
  Library,
  Users,
  ShieldAlert,
  Settings,
  MoreHorizontal,
  X,
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

const primaryLinks = navLinks.slice(0, 4)
const moreLinks = navLinks.slice(4)

export function MobileNav() {
  const [showMore, setShowMore] = useState(false)
  const navigate = useNavigate()
  const location = useLocation()

  const handleNavClick = (to: string) => (e: React.MouseEvent) => {
    if (location.pathname !== to) {
      e.preventDefault()
      navigate(to)
    }
  }

  const handleMoreNavClick = (to: string) => (e: React.MouseEvent) => {
    e.preventDefault()
    setShowMore(false)
    navigate(to)
  }

  const isMoreActive = moreLinks.some(link => location.pathname.startsWith(link.to))

  return (
    <>
      {showMore && (
        <div
          className="lg:hidden fixed inset-0 bg-black/50 z-40"
          onClick={() => setShowMore(false)}
        />
      )}

      <div
        className={`lg:hidden fixed bottom-0 left-0 right-0 z-50
                    bg-panel dark:bg-panel-dark
                    border-t border-border dark:border-border-dark
                    pb-[env(safe-area-inset-bottom)]
                    transition-transform duration-200
                    ${showMore ? 'translate-y-full' : 'translate-y-0'}`}
      >
        <nav className="flex items-center justify-around h-16">
          {primaryLinks.map(link => {
            const Icon = iconMap[link.icon]
            return (
              <NavLink
                key={link.to}
                to={link.to}
                end={link.to === '/'}
                onClick={handleNavClick(link.to)}
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
          <button
            onClick={() => setShowMore(true)}
            className={`flex flex-col items-center gap-1 px-3 py-2 min-w-[64px]
                       text-xs font-medium transition-colors
                       ${isMoreActive
                         ? 'text-accent-dim dark:text-accent'
                         : 'text-muted dark:text-muted-dark'}`}
          >
            <MoreHorizontal className="w-5 h-5" />
            More
          </button>
        </nav>
      </div>

      <div
        className={`lg:hidden fixed bottom-0 left-0 right-0 z-50
                    bg-panel dark:bg-panel-dark
                    border-t border-border dark:border-border-dark
                    pb-[env(safe-area-inset-bottom)]
                    transition-transform duration-200
                    ${showMore ? 'translate-y-0' : 'translate-y-full'}`}
      >
        <div className="flex items-center justify-between px-4 py-3 border-b border-border dark:border-border-dark">
          <span className="text-sm font-medium text-muted dark:text-muted-dark">More</span>
          <button
            onClick={() => setShowMore(false)}
            className="p-1 text-muted dark:text-muted-dark hover:text-gray-800 dark:hover:text-gray-200"
          >
            <X className="w-5 h-5" />
          </button>
        </div>
        <nav className="flex items-center justify-around h-16">
          {moreLinks.map(link => {
            const Icon = iconMap[link.icon]
            const isActive = location.pathname.startsWith(link.to)
            return (
              <a
                key={link.to}
                href={link.to}
                onClick={handleMoreNavClick(link.to)}
                className={`flex flex-col items-center gap-1 px-3 py-2 min-w-[64px]
                           text-xs font-medium transition-colors
                           ${isActive
                             ? 'text-accent-dim dark:text-accent'
                             : 'text-muted dark:text-muted-dark'}`}
              >
                {Icon && <Icon className="w-5 h-5" />}
                {link.label}
              </a>
            )
          })}
        </nav>
      </div>
    </>
  )
}
