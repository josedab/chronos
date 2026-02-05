import { useTheme } from './ThemeProvider'

export default function ThemeToggle() {
  const { theme, setTheme } = useTheme()

  const options = [
    { value: 'light', icon: '☀️', label: 'Light' },
    { value: 'dark', icon: '🌙', label: 'Dark' },
    { value: 'system', icon: '💻', label: 'System' },
  ] as const

  return (
    <div className="relative">
      <div className="flex items-center gap-1 bg-gray-100 dark:bg-gray-800 rounded-lg p-1">
        {options.map((option) => (
          <button
            key={option.value}
            onClick={() => setTheme(option.value)}
            className={`px-2 py-1 rounded-md text-sm transition-colors ${
              theme === option.value
                ? 'bg-white dark:bg-gray-700 shadow text-gray-900 dark:text-white'
                : 'text-gray-600 dark:text-gray-400 hover:text-gray-900 dark:hover:text-white'
            }`}
            title={option.label}
          >
            {option.icon}
          </button>
        ))}
      </div>
    </div>
  )
}
