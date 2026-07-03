// Applies the stored theme before first paint to prevent a flash of the
// wrong theme. Served as a static, same-origin script (rather than inline
// in index.html) so the app can run under a strict script-src 'self' CSP
// without 'unsafe-inline'.
(function () {
  var stored = localStorage.getItem('streammon-theme')
  var theme = stored || 'system'
  var isDark = theme === 'dark' || (theme === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches)
  if (isDark) document.documentElement.classList.add('dark')
})()
