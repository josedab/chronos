import { Routes, Route, Link, useLocation } from 'react-router-dom'
import JobList from './pages/JobList'
import JobDetail from './pages/JobDetail'
import JobCreate from './pages/JobCreate'
import JobEdit from './pages/JobEdit'
import Dashboard from './pages/Dashboard'
import Workflows from './pages/Workflows'
import ExecutionHistory from './pages/ExecutionHistory'
import ClusterStatus from './pages/ClusterStatus'
import Settings from './pages/Settings'
import Templates from './pages/Templates'
import { ThemeProvider } from './components/ThemeProvider'
import ThemeToggle from './components/ThemeToggle'
import { ToastProvider } from './components/Toast'
import { FavoritesProvider } from './components/Favorites'
import { TourProvider } from './components/OnboardingTour'
import CommandPalette from './components/CommandPalette'
import Breadcrumbs from './components/Breadcrumbs'
import { useKeyboardShortcuts, KeyboardShortcutsHelp, useKeyboardShortcutsHelp } from './components/KeyboardShortcuts'

function AppContent() {
  const location = useLocation()
  
  // Enable keyboard shortcuts
  useKeyboardShortcuts()
  const { showHelp, setShowHelp } = useKeyboardShortcutsHelp()

  const navItems = [
    { path: '/', label: 'Dashboard' },
    { path: '/jobs', label: 'Jobs' },
    { path: '/executions', label: 'Executions' },
    { path: '/workflows', label: 'Workflows' },
    { path: '/templates', label: 'Templates' },
    { path: '/cluster', label: 'Cluster' },
    { path: '/settings', label: 'Settings' },
  ]

  return (
    <div className="min-h-screen bg-gray-100 dark:bg-gray-900 transition-colors">
      {/* Header */}
      <header className="bg-white dark:bg-gray-800 shadow">
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex justify-between h-16">
            <div className="flex">
              <div className="flex-shrink-0 flex items-center">
                <h1 className="text-xl font-bold text-indigo-600 dark:text-indigo-400">Chronos</h1>
              </div>
              <nav className="ml-10 flex space-x-8">
                {navItems.map((item) => (
                  <Link
                    key={item.path}
                    to={item.path}
                    className={`inline-flex items-center px-1 pt-1 border-b-2 text-sm font-medium ${
                      location.pathname === item.path ||
                      (item.path !== '/' && location.pathname.startsWith(item.path))
                        ? 'border-indigo-500 text-gray-900 dark:text-white'
                        : 'border-transparent text-gray-500 dark:text-gray-400 hover:border-gray-300 hover:text-gray-700 dark:hover:text-gray-200'
                    }`}
                  >
                    {item.label}
                  </Link>
                ))}
              </nav>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => setShowHelp(true)}
                className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 p-2"
                title="Keyboard shortcuts (?)"
              >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                </svg>
                <span className="sr-only">Keyboard shortcuts</span>
              </button>
              <ThemeToggle />
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="max-w-7xl mx-auto py-6 sm:px-6 lg:px-8">
        {/* Breadcrumbs - show on all pages except dashboard */}
        {location.pathname !== '/' && (
          <Breadcrumbs className="mb-4 px-4 sm:px-0" />
        )}
        
        <Routes>
          <Route path="/" element={<Dashboard />} />
          <Route path="/jobs" element={<JobList />} />
          <Route path="/jobs/new" element={<JobCreate />} />
          <Route path="/jobs/:id" element={<JobDetail />} />
          <Route path="/jobs/:id/edit" element={<JobEdit />} />
          <Route path="/executions" element={<ExecutionHistory />} />
          <Route path="/workflows" element={<Workflows />} />
          <Route path="/templates" element={<Templates />} />
          <Route path="/cluster" element={<ClusterStatus />} />
          <Route path="/settings" element={<Settings />} />
        </Routes>
      </main>
      
      {/* Command Palette */}
      <CommandPalette />
      
      {/* Keyboard Shortcuts Help Modal */}
      <KeyboardShortcutsHelp isOpen={showHelp} onClose={() => setShowHelp(false)} />
    </div>
  )
}

function App() {
  return (
    <ThemeProvider>
      <ToastProvider>
        <FavoritesProvider>
          <TourProvider>
            <AppContent />
          </TourProvider>
        </FavoritesProvider>
      </ToastProvider>
    </ThemeProvider>
  )
}

export default App
