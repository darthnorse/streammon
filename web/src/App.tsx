import { Routes, Route } from 'react-router-dom'
import { Layout } from './components/Layout'
import { Dashboard } from './pages/Dashboard'
import { History } from './pages/History'

function SettingsPlaceholder() {
  return <div><h1 className="text-2xl font-semibold">Settings</h1></div>
}

function NotFound() {
  return (
    <div className="card p-12 text-center">
      <div className="text-4xl mb-3 opacity-30">?</div>
      <h1 className="text-xl font-semibold mb-1">Page not found</h1>
      <p className="text-sm text-muted dark:text-muted-dark">
        The page you're looking for doesn't exist.
      </p>
    </div>
  )
}

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Dashboard />} />
        <Route path="/history" element={<History />} />
        <Route path="/settings" element={<SettingsPlaceholder />} />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  )
}
