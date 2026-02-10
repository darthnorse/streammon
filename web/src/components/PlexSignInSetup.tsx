import { PlexAuthFlow } from './PlexAuthFlow'
import { plexBtnClass } from '../lib/constants'
import type { User } from '../types'

interface PlexSignInSetupProps {
  onSuccess: (user: User) => void
}

export function PlexSignInSetup({ onSuccess }: PlexSignInSetupProps) {
  return (
    <PlexAuthFlow
      onSuccess={onSuccess}
      endpoint="/api/setup/plex"
      buttonClassName={plexBtnClass}
      loadingMessage="Creating admin account..."
      autoStart
    />
  )
}
