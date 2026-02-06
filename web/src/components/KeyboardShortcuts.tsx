import { useEffect, useCallback, useState } from 'react'
import { useNavigate, useLocation } from 'react-router-dom'

export function useKeyboardShortcuts() {
  const navigate = useNavigate()
  const location = useLocation()

  const handleKeyDown = useCallback((event: KeyboardEvent) => {
    // Don't trigger shortcuts when typing in inputs
    const target = event.target as HTMLElement
    if (
      target.tagName === 'INPUT' ||
      target.tagName === 'TEXTAREA' ||
      target.tagName === 'SELECT' ||
      target.isContentEditable
    ) {
      // Allow Escape to blur inputs
      if (event.key === 'Escape') {
        target.blur()
      }
      return
    }

    const key = event.key.toLowerCase()
    const ctrl = event.ctrlKey || event.metaKey
    const shift = event.shiftKey

    // Navigation shortcuts
    if (!ctrl && !shift) {
      switch (key) {
        case 'g':
          // Wait for next key
          const handleNextKey = (e: KeyboardEvent) => {
            e.preventDefault()
            switch (e.key.toLowerCase()) {
              case 'd': navigate('/'); break
              case 'j': navigate('/jobs'); break
              case 'e': navigate('/executions'); break
              case 'w': navigate('/workflows'); break
              case 'c': navigate('/cluster'); break
              case 's': navigate('/settings'); break
            }
            document.removeEventListener('keydown', handleNextKey)
          }
          document.addEventListener('keydown', handleNextKey, { once: true })
          setTimeout(() => document.removeEventListener('keydown', handleNextKey), 1000)
          return

        case 'n':
          // New job
          if (location.pathname === '/jobs' || location.pathname.startsWith('/jobs')) {
            event.preventDefault()
            navigate('/jobs/new')
          }
          return

        case '/':
          // Focus search
          event.preventDefault()
          const searchInput = document.querySelector('input[placeholder*="Search"]') as HTMLInputElement
          if (searchInput) {
            searchInput.focus()
          }
          return

        case '?':
          // Show shortcuts help
          event.preventDefault()
          window.dispatchEvent(new CustomEvent('toggle-shortcuts-help'))
          return

        case 'escape':
          // Deselect, close modals
          window.dispatchEvent(new CustomEvent('escape-pressed'))
          return
      }
    }

    // Ctrl/Cmd shortcuts
    if (ctrl && !shift) {
      switch (key) {
        case 'k':
          // Command palette / quick search
          event.preventDefault()
          const searchBox = document.querySelector('input[placeholder*="Search"]') as HTMLInputElement
          if (searchBox) {
            searchBox.focus()
            searchBox.select()
          }
          return
      }
    }
  }, [navigate, location.pathname])

  useEffect(() => {
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [handleKeyDown])
}

// Keyboard shortcuts help modal
export function KeyboardShortcutsHelp({ isOpen, onClose }: { isOpen: boolean; onClose: () => void }) {
  if (!isOpen) return null

  const shortcuts = [
    { category: 'Navigation', items: [
      { keys: ['g', 'd'], description: 'Go to Dashboard' },
      { keys: ['g', 'j'], description: 'Go to Jobs' },
      { keys: ['g', 'e'], description: 'Go to Executions' },
      { keys: ['g', 'w'], description: 'Go to Workflows' },
      { keys: ['g', 'c'], description: 'Go to Cluster' },
      { keys: ['g', 's'], description: 'Go to Settings' },
    ]},
    { category: 'List Navigation', items: [
      { keys: ['j'], description: 'Move down in list' },
      { keys: ['k'], description: 'Move up in list' },
      { keys: ['Enter'], description: 'Open selected item' },
      { keys: ['Home'], description: 'Jump to first item' },
      { keys: ['End'], description: 'Jump to last item' },
    ]},
    { category: 'Actions', items: [
      { keys: ['n'], description: 'New job (when on Jobs page)' },
      { keys: ['/'], description: 'Focus search' },
      { keys: ['⌘', 'k'], description: 'Quick search' },
      { keys: ['Esc'], description: 'Clear selection / Close modal' },
    ]},
    { category: 'Help', items: [
      { keys: ['?'], description: 'Show this help' },
    ]},
  ]

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50" onClick={onClose}>
      <div 
        className="bg-white dark:bg-gray-800 rounded-lg shadow-xl w-full max-w-lg mx-4"
        onClick={e => e.stopPropagation()}
      >
        <div className="px-6 py-4 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
          <h3 className="text-lg font-medium text-gray-900 dark:text-white">Keyboard Shortcuts</h3>
          <button onClick={onClose} className="text-gray-400 hover:text-gray-600 dark:hover:text-gray-300">
            ✕
          </button>
        </div>
        <div className="px-6 py-4 max-h-96 overflow-auto">
          {shortcuts.map((group) => (
            <div key={group.category} className="mb-6 last:mb-0">
              <h4 className="text-sm font-medium text-gray-500 dark:text-gray-400 mb-2">
                {group.category}
              </h4>
              <div className="space-y-2">
                {group.items.map((item, i) => (
                  <div key={i} className="flex justify-between items-center">
                    <span className="text-sm text-gray-700 dark:text-gray-300">{item.description}</span>
                    <div className="flex gap-1">
                      {item.keys.map((key, j) => (
                        <span key={j}>
                          <kbd className="px-2 py-1 text-xs font-semibold text-gray-800 bg-gray-100 border border-gray-300 rounded dark:bg-gray-700 dark:text-gray-200 dark:border-gray-600">
                            {key}
                          </kbd>
                          {j < item.keys.length - 1 && <span className="mx-0.5 text-gray-400">then</span>}
                        </span>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
        <div className="px-6 py-3 bg-gray-50 dark:bg-gray-900 border-t border-gray-200 dark:border-gray-700 text-center">
          <span className="text-xs text-gray-500 dark:text-gray-400">
            Press <kbd className="px-1 py-0.5 text-xs bg-gray-200 dark:bg-gray-700 rounded">?</kbd> to toggle this help
          </span>
        </div>
      </div>
    </div>
  )
}

// Hook to manage keyboard shortcuts help visibility
export function useKeyboardShortcutsHelp() {
  const [showHelp, setShowHelp] = useState(false)

  useEffect(() => {
    const handleToggle = () => setShowHelp(prev => !prev)
    const handleEscape = () => setShowHelp(false)
    
    window.addEventListener('toggle-shortcuts-help', handleToggle)
    window.addEventListener('escape-pressed', handleEscape)
    
    return () => {
      window.removeEventListener('toggle-shortcuts-help', handleToggle)
      window.removeEventListener('escape-pressed', handleEscape)
    }
  }, [])

  return { showHelp, setShowHelp }
}
