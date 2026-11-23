// localStorage can throw (private mode, disabled storage, quota), so every access
// degrades to "nothing stored" rather than breaking the page.
export function safeGetItem(key: string): string | null {
  try {
    return localStorage.getItem(key)
  } catch {
    return null
  }
}

export function safeSetItem(key: string, value: string): boolean {
  try {
    localStorage.setItem(key, value)
    return true
  } catch {
    return false
  }
}

export function safeRemoveItem(key: string): boolean {
  try {
    localStorage.removeItem(key)
    return true
  } catch {
    return false
  }
}
