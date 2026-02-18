import { useState, useEffect } from 'react'
import { localToday } from '../lib/format'

export function useLocalToday(): string {
  const [today, setToday] = useState(localToday)

  useEffect(() => {
    const interval = setInterval(() => {
      const now = localToday()
      setToday(prev => prev !== now ? now : prev)
    }, 60_000)
    return () => clearInterval(interval)
  }, [])

  return today
}
