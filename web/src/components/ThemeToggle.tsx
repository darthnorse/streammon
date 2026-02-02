import { useState, useEffect } from 'react'

type Theme = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'streammon-theme'

function getSystemTheme(): 'light' | 'dark' {
  if (typeof window === 'undefined') return 'dark'
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function applyTheme(theme: Theme) {
  const resolved = theme === 'system' ? getSystemTheme() : theme
  document.documentElement.classList.toggle('dark', resolved === 'dark')
}

export function ThemeToggle() {
  const [theme, setTheme] = useState<Theme>(() => {
    const stored = localStorage.getItem(STORAGE_KEY) as Theme | null
    return stored || 'system'
  })

  useEffect(() => {
    applyTheme(theme)
    localStorage.setItem(STORAGE_KEY, theme)
  }, [theme])

  useEffect(() => {
    if (theme !== 'system') return
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = () => applyTheme('system')
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [theme])

  function cycle() {
    setTheme(prev => {
      if (prev === 'system') return 'light'
      if (prev === 'light') return 'dark'
      return 'system'
    })
  }

  const icons: Record<Theme, string> = {
    system: '◐',
    light: '☀',
    dark: '☾',
  }

  return (
    <button
      onClick={cycle}
      aria-label={`Toggle theme (${theme})`}
      className="p-2 rounded-lg text-muted hover:text-gray-800 dark:hover:text-gray-100
                 hover:bg-gray-100 dark:hover:bg-white/5 transition-colors"
    >
      <span className="text-lg">{icons[theme]}</span>
    </button>
  )
}
