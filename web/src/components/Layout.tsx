import { Outlet } from 'react-router-dom'
import { Sidebar } from './Sidebar'
import { MobileNav } from './MobileNav'
import { ThemeToggle } from './ThemeToggle'

export function Layout() {
  return (
    <div className="flex min-h-screen scanlines">
      <Sidebar />

      <div className="flex-1 flex flex-col min-w-0">
        <header className="lg:hidden flex items-center justify-between px-4 h-14
                          border-b border-border dark:border-border-dark
                          bg-panel dark:bg-panel-dark sticky top-0 z-40">
          <span className="text-accent font-mono font-semibold text-sm tracking-widest uppercase">
            StreamMon
          </span>
          <ThemeToggle />
        </header>

        <main className="flex-1 p-4 md:p-6 lg:p-8 pb-20 lg:pb-8">
          <Outlet />
        </main>
      </div>

      <MobileNav />
    </div>
  )
}
