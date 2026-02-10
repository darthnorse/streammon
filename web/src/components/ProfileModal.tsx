import { useState } from 'react'
import { Link } from 'react-router-dom'
import { useAuth } from '../context/AuthContext'
import { api } from '../lib/api'
import { formInputClass } from '../lib/constants'
import { errorMessage } from '../lib/utils'
import { useModal } from '../hooks/useModal'
import { UserAvatar } from './UserAvatar'

type FormMsg = { type: 'success' | 'error'; text: string }

function FormFeedback({ msg }: { msg: FormMsg | null }) {
  if (!msg) return null
  return (
    <div className={`text-sm font-mono px-1 ${
      msg.type === 'success'
        ? 'text-green-600 dark:text-green-400'
        : 'text-red-500 dark:text-red-400'
    }`}>
      {msg.text}
    </div>
  )
}

interface ProfileModalProps {
  onClose: () => void
}

export function ProfileModal({ onClose }: ProfileModalProps) {
  const { user, refreshUser } = useAuth()
  const modalRef = useModal(onClose)

  const [email, setEmail] = useState(user?.email ?? '')
  const [emailSaving, setEmailSaving] = useState(false)
  const [emailMsg, setEmailMsg] = useState<FormMsg | null>(null)

  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [pwdSaving, setPwdSaving] = useState(false)
  const [pwdMsg, setPwdMsg] = useState<FormMsg | null>(null)

  if (!user) return null

  async function handleEmailSave(e: React.FormEvent) {
    e.preventDefault()
    setEmailSaving(true)
    setEmailMsg(null)
    try {
      await api.put('/api/me', { email })
      await refreshUser()
      setEmailMsg({ type: 'success', text: 'Email updated' })
    } catch (err) {
      setEmailMsg({ type: 'error', text: errorMessage(err) })
    } finally {
      setEmailSaving(false)
    }
  }

  async function handlePasswordChange(e: React.FormEvent) {
    e.preventDefault()
    setPwdMsg(null)

    if (newPassword !== confirmPassword) {
      setPwdMsg({ type: 'error', text: 'Passwords do not match' })
      return
    }

    setPwdSaving(true)
    try {
      await api.post('/api/me/password', {
        current_password: currentPassword,
        new_password: newPassword,
      })
      setPwdMsg({ type: 'success', text: 'Password changed' })
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err) {
      setPwdMsg({ type: 'error', text: errorMessage(err) })
    } finally {
      setPwdSaving(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={e => { if (e.target === e.currentTarget) onClose() }}
      role="dialog"
      aria-modal="true"
      aria-label="User Profile"
    >
      <div
        ref={modalRef}
        className="card w-full max-w-lg max-h-[90vh] overflow-y-auto p-0
                      lg:max-w-xl animate-slide-up"
      >
        <div className="flex items-center justify-between px-6 py-4
                        border-b border-border dark:border-border-dark">
          <h2 className="text-lg font-semibold">Profile</h2>
          <button
            onClick={onClose}
            aria-label="Close"
            className="text-muted dark:text-muted-dark hover:text-gray-800
                       dark:hover:text-gray-100 transition-colors text-xl leading-none"
          >
            &times;
          </button>
        </div>

        <div className="px-6 py-5 space-y-6">
          <div className="flex items-center gap-4">
            <UserAvatar name={user.name} thumbUrl={user.thumb_url} size="lg" />
            <div>
              <div className="text-lg font-semibold">{user.name}</div>
              <div className="text-sm text-muted dark:text-muted-dark capitalize">{user.role}</div>
              <Link
                to="/my-stats"
                onClick={onClose}
                className="text-sm text-accent-dim dark:text-accent hover:underline"
              >
                View My Stats
              </Link>
            </div>
          </div>

          {user.has_password && (
            <form onSubmit={handleEmailSave} className="space-y-3">
              <label htmlFor="profile-email" className="block text-sm font-medium">Email</label>
              <div className="flex gap-2">
                <input
                  id="profile-email"
                  type="email"
                  value={email}
                  onChange={e => { setEmail(e.target.value); setEmailMsg(null) }}
                  placeholder="your@email.com"
                  className={formInputClass}
                />
                <button
                  type="submit"
                  disabled={emailSaving || email === (user.email ?? '')}
                  className="px-4 py-2.5 text-sm font-semibold rounded-lg shrink-0
                             bg-accent text-gray-900 hover:bg-accent/90
                             disabled:opacity-50 transition-colors"
                >
                  {emailSaving ? 'Saving...' : 'Save'}
                </button>
              </div>
              <FormFeedback msg={emailMsg} />
            </form>
          )}

          {!user.has_password && user.email && (
            <div className="space-y-1">
              <div className="text-sm font-medium">Email</div>
              <div className="text-sm text-muted dark:text-muted-dark">{user.email}</div>
            </div>
          )}

          {user.has_password && (
            <form onSubmit={handlePasswordChange} className="space-y-3">
              <div className="text-sm font-medium">Change Password</div>
              <input
                type="password"
                value={currentPassword}
                onChange={e => { setCurrentPassword(e.target.value); setPwdMsg(null) }}
                placeholder="Current password"
                className={formInputClass}
                autoComplete="current-password"
              />
              <input
                type="password"
                value={newPassword}
                onChange={e => { setNewPassword(e.target.value); setPwdMsg(null) }}
                placeholder="New password (min 8 characters)"
                minLength={8}
                className={formInputClass}
                autoComplete="new-password"
              />
              <input
                type="password"
                value={confirmPassword}
                onChange={e => { setConfirmPassword(e.target.value); setPwdMsg(null) }}
                placeholder="Confirm new password"
                minLength={8}
                className={formInputClass}
                autoComplete="new-password"
              />
              <button
                type="submit"
                disabled={pwdSaving || !currentPassword || !newPassword || !confirmPassword}
                className="px-4 py-2.5 text-sm font-semibold rounded-lg
                           bg-accent text-gray-900 hover:bg-accent/90
                           disabled:opacity-50 transition-colors"
              >
                {pwdSaving ? 'Changing...' : 'Change Password'}
              </button>
              <FormFeedback msg={pwdMsg} />
            </form>
          )}
        </div>
      </div>
    </div>
  )
}
