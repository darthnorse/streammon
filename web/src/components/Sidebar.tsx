import { NavLink } from 'react-router-dom'
import { ThemeToggle } from './ThemeToggle'
import { navLinks } from '../lib/constants'

export function Sidebar() {
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
        {navLinks.map(link => (
          <NavLink
            key={link.to}
            to={link.to}
            end={link.to === '/'}
            className={({ isActive }) =>
              `nav-item ${isActive ? 'active' : ''}`
            }
          >
            <span className="text-base">{link.icon}</span>
            {link.label}
          </NavLink>
        ))}
      </nav>

      <div className="px-3 py-4 border-t border-border dark:border-border-dark">
        <ThemeToggle />
      </div>
    </aside>
  )
}
