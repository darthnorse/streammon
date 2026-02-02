import { Routes, Route } from 'react-router-dom'
import { Layout } from './components/Layout'
import { Dashboard } from './pages/Dashboard'
import { History } from './pages/History'

function SettingsPlaceholder() {
  return <div><h1 className="text-2xl font-semibold">Settings</h1></div>
}

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Dashboard />} />
        <Route path="/history" element={<History />} />
        <Route path="/settings" element={<SettingsPlaceholder />} />
      </Route>
    </Routes>
  )
}
