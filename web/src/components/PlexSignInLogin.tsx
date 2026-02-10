import { PlexAuthFlow } from './PlexAuthFlow'
import { plexBtnClass } from '../lib/constants'
import type { User } from '../types'

interface PlexSignInLoginProps {
  onSuccess: (user: User) => void
}

export function PlexSignInLogin({ onSuccess }: PlexSignInLoginProps) {
  return (
    <PlexAuthFlow
      onSuccess={onSuccess}
      endpoint="/auth/plex/login"
      buttonClassName={plexBtnClass + ' w-full'}
      centered
      autoStart
    />
  )
}
