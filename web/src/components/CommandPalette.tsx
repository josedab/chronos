import { useState, useEffect, useRef, useMemo, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { getJobs } from '../api/client'

interface CommandItem {
  id: string
  title: string
  description?: string
  icon: JSX.Element
  action: () => void
  keywords?: string[]
  category: 'navigation' | 'actions' | 'jobs' | 'settings'
}

export default function CommandPalette() {
  const [isOpen, setIsOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [selectedIndex, setSelectedIndex] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)
  const navigate = useNavigate()

  // Fetch jobs for quick navigation
  const { data: jobsData } = useQuery({
    queryKey: ['jobs'],
    queryFn: getJobs,
    enabled: isOpen,
  })

  // Define available commands
  const commands = useMemo<CommandItem[]>(() => {
    const baseCommands: CommandItem[] = [
      // Navigation
      {
        id: 'nav-dashboard',
        title: 'Go to Dashboard',
        icon: <HomeIcon />,
        action: () => navigate('/'),
        keywords: ['home', 'overview'],
        category: 'navigation',
      },
      {
        id: 'nav-jobs',
        title: 'Go to Jobs',
        icon: <ListIcon />,
        action: () => navigate('/jobs'),
        keywords: ['list', 'all'],
        category: 'navigation',
      },
      {
        id: 'nav-executions',
        title: 'Go to Executions',
        icon: <ClockIcon />,
        action: () => navigate('/executions'),
        keywords: ['history', 'runs'],
        category: 'navigation',
      },
      {
        id: 'nav-cluster',
        title: 'Go to Cluster Status',
        icon: <ServerIcon />,
        action: () => navigate('/cluster'),
        keywords: ['nodes', 'health'],
        category: 'navigation',
      },
      {
        id: 'nav-settings',
        title: 'Go to Settings',
        icon: <CogIcon />,
        action: () => navigate('/settings'),
        keywords: ['config', 'preferences'],
        category: 'navigation',
      },
      {
        id: 'nav-templates',
        title: 'Go to Templates',
        icon: <TemplateIcon />,
        action: () => navigate('/templates'),
        keywords: ['presets'],
        category: 'navigation',
      },
      // Actions
      {
        id: 'action-create-job',
        title: 'Create New Job',
        description: 'Create a new scheduled job',
        icon: <PlusIcon />,
        action: () => navigate('/jobs/new'),
        keywords: ['add', 'new'],
        category: 'actions',
      },
      {
        id: 'action-toggle-theme',
        title: 'Toggle Dark Mode',
        description: 'Switch between light and dark theme',
        icon: <MoonIcon />,
        action: () => {
          document.documentElement.classList.toggle('dark')
          localStorage.setItem(
            'theme',
            document.documentElement.classList.contains('dark') ? 'dark' : 'light'
          )
        },
        keywords: ['theme', 'light', 'dark'],
        category: 'settings',
      },
      {
        id: 'action-shortcuts',
        title: 'Show Keyboard Shortcuts',
        description: 'View all available shortcuts',
        icon: <KeyboardIcon />,
        action: () => {
          document.dispatchEvent(new CustomEvent('toggle-shortcuts-help'))
        },
        keywords: ['help', 'keys', 'hotkeys'],
        category: 'settings',
      },
    ]

    // Add job commands from fetched data
    const jobCommands: CommandItem[] = (jobsData?.jobs || []).slice(0, 10).map((job) => ({
      id: `job-${job.id}`,
      title: job.name,
      description: job.description || `View job: ${job.name}`,
      icon: <BriefcaseIcon />,
      action: () => navigate(`/jobs/${job.id}`),
      keywords: [job.name.toLowerCase()],
      category: 'jobs' as const,
    }))

    return [...baseCommands, ...jobCommands]
  }, [navigate, jobsData])

  // Filter commands based on search
  const filteredCommands = useMemo(() => {
    if (!search.trim()) return commands

    const query = search.toLowerCase()
    return commands.filter((cmd) => {
      const titleMatch = cmd.title.toLowerCase().includes(query)
      const descMatch = cmd.description?.toLowerCase().includes(query)
      const keywordMatch = cmd.keywords?.some((k) => k.includes(query))
      return titleMatch || descMatch || keywordMatch
    })
  }, [commands, search])

  // Group commands by category
  const groupedCommands = useMemo(() => {
    const groups: Record<string, CommandItem[]> = {}
    filteredCommands.forEach((cmd) => {
      if (!groups[cmd.category]) groups[cmd.category] = []
      groups[cmd.category].push(cmd)
    })
    return groups
  }, [filteredCommands])

  const categoryOrder = ['actions', 'navigation', 'jobs', 'settings']
  const categoryLabels: Record<string, string> = {
    navigation: 'Navigation',
    actions: 'Actions',
    jobs: 'Jobs',
    settings: 'Settings',
  }

  // Open/close with Cmd+K
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        setIsOpen((prev) => !prev)
      }
      if (e.key === 'Escape' && isOpen) {
        setIsOpen(false)
      }
    }
    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [isOpen])

  // Focus input when opened
  useEffect(() => {
    if (isOpen) {
      setSearch('')
      setSelectedIndex(0)
      setTimeout(() => inputRef.current?.focus(), 0)
    }
  }, [isOpen])

  // Handle keyboard navigation
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      const flatCommands = filteredCommands
      
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSelectedIndex((prev) => (prev + 1) % flatCommands.length)
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIndex((prev) => (prev - 1 + flatCommands.length) % flatCommands.length)
      } else if (e.key === 'Enter') {
        e.preventDefault()
        const cmd = flatCommands[selectedIndex]
        if (cmd) {
          cmd.action()
          setIsOpen(false)
        }
      }
    },
    [filteredCommands, selectedIndex]
  )

  // Scroll selected item into view
  useEffect(() => {
    const selected = listRef.current?.querySelector('[data-selected="true"]')
    selected?.scrollIntoView({ block: 'nearest' })
  }, [selectedIndex])

  if (!isOpen) return null

  let commandIndex = 0

  return (
    <div 
      className="fixed inset-0 z-50 bg-black/50 flex items-start justify-center pt-[15vh]"
      onClick={() => setIsOpen(false)}
    >
      <div 
        className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-lg overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Search input */}
        <div className="flex items-center px-4 border-b border-gray-200 dark:border-gray-700">
          <svg className="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
          </svg>
          <input
            ref={inputRef}
            type="text"
            value={search}
            onChange={(e) => {
              setSearch(e.target.value)
              setSelectedIndex(0)
            }}
            onKeyDown={handleKeyDown}
            placeholder="Type a command or search..."
            className="flex-1 px-3 py-4 bg-transparent border-0 outline-none text-gray-900 dark:text-white placeholder-gray-400"
          />
          <kbd className="hidden sm:inline-flex items-center px-2 py-1 text-xs text-gray-400 bg-gray-100 dark:bg-gray-700 rounded">
            ESC
          </kbd>
        </div>

        {/* Command list */}
        <div ref={listRef} className="max-h-80 overflow-y-auto p-2">
          {filteredCommands.length === 0 ? (
            <div className="py-8 text-center text-gray-500">
              No commands found
            </div>
          ) : (
            categoryOrder.map((category) => {
              const cmds = groupedCommands[category]
              if (!cmds || cmds.length === 0) return null

              return (
                <div key={category} className="mb-2">
                  <div className="px-2 py-1 text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">
                    {categoryLabels[category]}
                  </div>
                  {cmds.map((cmd) => {
                    const idx = commandIndex++
                    const isSelected = idx === selectedIndex

                    return (
                      <button
                        key={cmd.id}
                        data-selected={isSelected}
                        onClick={() => {
                          cmd.action()
                          setIsOpen(false)
                        }}
                        onMouseEnter={() => setSelectedIndex(idx)}
                        className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg text-left transition-colors ${
                          isSelected
                            ? 'bg-indigo-50 dark:bg-indigo-900/30 text-indigo-700 dark:text-indigo-300'
                            : 'text-gray-700 dark:text-gray-300 hover:bg-gray-50 dark:hover:bg-gray-700'
                        }`}
                      >
                        <span className="flex-shrink-0 w-8 h-8 flex items-center justify-center rounded-md bg-gray-100 dark:bg-gray-700">
                          {cmd.icon}
                        </span>
                        <div className="flex-1 min-w-0">
                          <div className="font-medium truncate">{cmd.title}</div>
                          {cmd.description && (
                            <div className="text-xs text-gray-500 dark:text-gray-400 truncate">
                              {cmd.description}
                            </div>
                          )}
                        </div>
                        {isSelected && (
                          <kbd className="text-xs text-gray-400 bg-gray-100 dark:bg-gray-700 px-1.5 py-0.5 rounded">
                            ↵
                          </kbd>
                        )}
                      </button>
                    )
                  })}
                </div>
              )
            })
          )}
        </div>

        {/* Footer */}
        <div className="px-4 py-2 border-t border-gray-200 dark:border-gray-700 flex items-center justify-between text-xs text-gray-500">
          <div className="flex items-center gap-4">
            <span className="flex items-center gap-1">
              <kbd className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">↑</kbd>
              <kbd className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">↓</kbd>
              to navigate
            </span>
            <span className="flex items-center gap-1">
              <kbd className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">↵</kbd>
              to select
            </span>
          </div>
          <span className="flex items-center gap-1">
            <kbd className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">⌘</kbd>
            <kbd className="px-1.5 py-0.5 bg-gray-100 dark:bg-gray-700 rounded">K</kbd>
            to toggle
          </span>
        </div>
      </div>
    </div>
  )
}

// Icon components
function HomeIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
    </svg>
  )
}

function ListIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 10h16M4 14h16M4 18h16" />
    </svg>
  )
}

function ClockIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  )
}

function ServerIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01" />
    </svg>
  )
}

function CogIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
    </svg>
  )
}

function TemplateIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 5a1 1 0 011-1h14a1 1 0 011 1v2a1 1 0 01-1 1H5a1 1 0 01-1-1V5zM4 13a1 1 0 011-1h6a1 1 0 011 1v6a1 1 0 01-1 1H5a1 1 0 01-1-1v-6zM16 13a1 1 0 011-1h2a1 1 0 011 1v6a1 1 0 01-1 1h-2a1 1 0 01-1-1v-6z" />
    </svg>
  )
}

function PlusIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4v16m8-8H4" />
    </svg>
  )
}

function MoonIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
    </svg>
  )
}

function KeyboardIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707" />
    </svg>
  )
}

function BriefcaseIcon() {
  return (
    <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 13.255A23.931 23.931 0 0112 15c-3.183 0-6.22-.62-9-1.745M16 6V4a2 2 0 00-2-2h-4a2 2 0 00-2 2v2m4 6h.01M5 20h14a2 2 0 002-2V8a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
    </svg>
  )
}
