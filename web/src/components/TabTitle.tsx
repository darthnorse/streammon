import { useEffect } from 'react'
import { useSessions } from '../context/SessionsContext'

const BASE_TITLE = 'StreamMon'

export function TabTitle() {
  const { sessions } = useSessions()
  const count = sessions.length

  useEffect(() => {
    document.title = count > 0
      ? `${BASE_TITLE} | ${count} stream${count === 1 ? '' : 's'}`
      : BASE_TITLE
    return () => { document.title = BASE_TITLE }
  }, [count])

  return null
}
