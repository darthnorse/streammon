import { useState } from 'react'
import { NavLink, useNavigate, useLocation } from 'react-router-dom'
import { MoreHorizontal, X } from 'lucide-react'
import { visibleNavLinks, navIconMap, resolveNavLabel } from '../lib/constants'
import type { IntegrationStatus } from '../lib/constants'
import { useAuth } from '../context/AuthContext'

const navPanelBase = `lg:hidden fixed bottom-0 left-0 right-0 z-50
  bg-panel dark:bg-panel-dark border-t border-border dark:border-border-dark
  pb-[env(safe-area-inset-bottom)] transition-transform duration-200`

const navItemBase = `flex flex-col items-center gap-1 px-3 py-2 min-w-[64px]
  text-xs font-medium transition-colors`

const navItemActive = 'text-accent-dim dark:text-accent'
const navItemInactive = 'text-muted dark:text-muted-dark'

interface MobileNavProps {
  integrations: IntegrationStatus
}

export function MobileNav({ integrations }: MobileNavProps) {
  const [showMore, setShowMore] = useState(false)
  const navigate = useNavigate()
  const location = useLocation()
  const { user } = useAuth()

  const filteredLinks = visibleNavLinks(user?.role, integrations)
  const primaryLinks = filteredLinks.slice(0, 4)
  const moreLinks = filteredLinks.slice(4)

  const handleNavClick = (to: string, closeMenu = false) => (e: React.MouseEvent) => {
    e.preventDefault()
    if (closeMenu) setShowMore(false)
    if (location.pathname !== to || closeMenu) navigate(to)
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

      <div className={`${navPanelBase} ${showMore ? 'translate-y-full' : 'translate-y-0'}`}>
        <nav className="flex items-center justify-around h-16">
          {primaryLinks.map(link => {
            const Icon = navIconMap[link.icon]
            const label = resolveNavLabel(link, integrations)
            return (
              <NavLink
                key={link.to}
                to={link.to}
                end={link.to === '/'}
                onClick={handleNavClick(link.to)}
                className={({ isActive }) =>
                  `${navItemBase} ${isActive ? navItemActive : navItemInactive}`
                }
              >
                {Icon && <Icon className="w-5 h-5" />}
                {label}
              </NavLink>
            )
          })}
          {moreLinks.length > 0 && (
            <button
              onClick={() => setShowMore(true)}
              className={`${navItemBase} ${isMoreActive ? navItemActive : navItemInactive}`}
            >
              <MoreHorizontal className="w-5 h-5" />
              More
            </button>
          )}
        </nav>
      </div>

      <div className={`${navPanelBase} ${showMore ? 'translate-y-0' : 'translate-y-full'}`}>
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
            const Icon = navIconMap[link.icon]
            const isActive = location.pathname.startsWith(link.to)
            const label = resolveNavLabel(link, integrations)
            return (
              <a
                key={link.to}
                href={link.to}
                onClick={handleNavClick(link.to, true)}
                className={`${navItemBase} ${isActive ? navItemActive : navItemInactive}`}
              >
                {Icon && <Icon className="w-5 h-5" />}
                {label}
              </a>
            )
          })}
        </nav>
      </div>
    </>
  )
}
