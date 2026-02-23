import { useAuth } from '../context/AuthContext'

export function EncryptionWarning() {
  const { encryptionMissing } = useAuth()

  if (!encryptionMissing) return null

  return (
    <div className="w-full max-w-md mb-4 p-3 rounded-lg bg-yellow-500/10 border border-yellow-500/30 text-sm text-yellow-200">
      <span className="font-semibold">Encryption not configured.</span>{' '}
      Set <code className="bg-black/20 px-1 rounded">TOKEN_ENCRYPTION_KEY</code> to
      encrypt secrets at rest. Generate one
      with: <code className="bg-black/20 px-1 rounded">openssl rand -base64 32</code>
    </div>
  )
}
