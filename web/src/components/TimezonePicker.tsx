import { useState, useRef, useEffect, useMemo } from 'react'

interface TimezonePickerProps {
  value: string
  onChange: (timezone: string) => void
  className?: string
}

// Common timezones grouped by region
const TIMEZONES = {
  'Popular': [
    'UTC',
    'America/New_York',
    'America/Chicago',
    'America/Denver',
    'America/Los_Angeles',
    'Europe/London',
    'Europe/Paris',
    'Europe/Berlin',
    'Asia/Tokyo',
    'Asia/Shanghai',
    'Asia/Singapore',
    'Australia/Sydney',
  ],
  'Americas': [
    'America/New_York',
    'America/Chicago',
    'America/Denver',
    'America/Los_Angeles',
    'America/Phoenix',
    'America/Anchorage',
    'America/Toronto',
    'America/Vancouver',
    'America/Mexico_City',
    'America/Sao_Paulo',
    'America/Buenos_Aires',
    'America/Lima',
    'America/Bogota',
  ],
  'Europe': [
    'Europe/London',
    'Europe/Paris',
    'Europe/Berlin',
    'Europe/Amsterdam',
    'Europe/Brussels',
    'Europe/Madrid',
    'Europe/Rome',
    'Europe/Vienna',
    'Europe/Warsaw',
    'Europe/Moscow',
    'Europe/Istanbul',
    'Europe/Athens',
  ],
  'Asia': [
    'Asia/Tokyo',
    'Asia/Shanghai',
    'Asia/Hong_Kong',
    'Asia/Singapore',
    'Asia/Seoul',
    'Asia/Taipei',
    'Asia/Bangkok',
    'Asia/Jakarta',
    'Asia/Manila',
    'Asia/Dubai',
    'Asia/Kolkata',
    'Asia/Karachi',
  ],
  'Pacific': [
    'Pacific/Auckland',
    'Pacific/Fiji',
    'Pacific/Honolulu',
    'Pacific/Guam',
  ],
  'Australia': [
    'Australia/Sydney',
    'Australia/Melbourne',
    'Australia/Brisbane',
    'Australia/Perth',
    'Australia/Adelaide',
  ],
  'Africa': [
    'Africa/Cairo',
    'Africa/Lagos',
    'Africa/Johannesburg',
    'Africa/Nairobi',
    'Africa/Casablanca',
  ],
}

function getTimezoneOffset(tz: string): string {
  try {
    const now = new Date()
    const formatter = new Intl.DateTimeFormat('en-US', {
      timeZone: tz,
      timeZoneName: 'shortOffset',
    })
    const parts = formatter.formatToParts(now)
    const offset = parts.find(p => p.type === 'timeZoneName')?.value || ''
    return offset.replace('GMT', 'UTC')
  } catch {
    return ''
  }
}

function getCurrentTimeInZone(tz: string): string {
  try {
    const now = new Date()
    return now.toLocaleTimeString('en-US', {
      timeZone: tz,
      hour: '2-digit',
      minute: '2-digit',
      hour12: true,
    })
  } catch {
    return ''
  }
}

function formatTimezoneName(tz: string): string {
  return tz.replace(/_/g, ' ').replace(/\//g, ' / ')
}

export default function TimezonePicker({ value, onChange, className = '' }: TimezonePickerProps) {
  const [isOpen, setIsOpen] = useState(false)
  const [search, setSearch] = useState('')
  const [selectedRegion, setSelectedRegion] = useState<keyof typeof TIMEZONES>('Popular')
  const dropdownRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)

  // Close dropdown on outside click
  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false)
      }
    }
    document.addEventListener('mousedown', handleClickOutside)
    return () => document.removeEventListener('mousedown', handleClickOutside)
  }, [])

  // Filter timezones by search
  const filteredTimezones = useMemo(() => {
    if (!search.trim()) {
      return TIMEZONES[selectedRegion]
    }
    const query = search.toLowerCase()
    const all = Object.values(TIMEZONES).flat()
    const unique = [...new Set(all)]
    return unique.filter(tz => 
      tz.toLowerCase().includes(query) ||
      formatTimezoneName(tz).toLowerCase().includes(query)
    )
  }, [search, selectedRegion])

  const currentOffset = value ? getTimezoneOffset(value) : ''
  const currentTime = value ? getCurrentTimeInZone(value) : ''

  return (
    <div className={`relative ${className}`} ref={dropdownRef}>
      <div 
        className="flex items-center gap-2 cursor-pointer"
        onClick={() => {
          setIsOpen(!isOpen)
          setTimeout(() => inputRef.current?.focus(), 0)
        }}
      >
        <div className="flex-1 relative">
          <input
            ref={inputRef}
            type="text"
            value={isOpen ? search : value || 'Select timezone...'}
            onChange={(e) => setSearch(e.target.value)}
            onFocus={() => setIsOpen(true)}
            placeholder="Search timezones..."
            className="w-full px-3 py-2 pr-20 border border-gray-300 dark:border-gray-600 rounded-md shadow-sm 
                       focus:ring-indigo-500 focus:border-indigo-500 
                       dark:bg-gray-700 dark:text-white text-sm"
          />
          {value && !isOpen && (
            <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-gray-500 dark:text-gray-400">
              {currentOffset} • {currentTime}
            </span>
          )}
        </div>
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation()
            onChange('')
            setSearch('')
          }}
          className="p-2 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300"
          title="Clear"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      {isOpen && (
        <div className="absolute z-50 mt-1 w-full bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 rounded-md shadow-lg">
          {/* Region tabs */}
          {!search.trim() && (
            <div className="flex flex-wrap border-b border-gray-200 dark:border-gray-700 p-1 gap-1">
              {(Object.keys(TIMEZONES) as Array<keyof typeof TIMEZONES>).map(region => (
                <button
                  key={region}
                  type="button"
                  onClick={() => setSelectedRegion(region)}
                  className={`px-2 py-1 text-xs rounded ${
                    selectedRegion === region
                      ? 'bg-indigo-100 dark:bg-indigo-900 text-indigo-700 dark:text-indigo-300'
                      : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-gray-700'
                  }`}
                >
                  {region}
                </button>
              ))}
            </div>
          )}

          {/* Timezone list */}
          <div className="max-h-60 overflow-y-auto">
            {filteredTimezones.length === 0 ? (
              <div className="p-3 text-sm text-gray-500 text-center">
                No timezones found
              </div>
            ) : (
              filteredTimezones.map(tz => {
                const offset = getTimezoneOffset(tz)
                const time = getCurrentTimeInZone(tz)
                return (
                  <button
                    key={tz}
                    type="button"
                    onClick={() => {
                      onChange(tz)
                      setSearch('')
                      setIsOpen(false)
                    }}
                    className={`w-full px-3 py-2 text-left text-sm flex items-center justify-between hover:bg-gray-50 dark:hover:bg-gray-700 ${
                      value === tz ? 'bg-indigo-50 dark:bg-indigo-900/30' : ''
                    }`}
                  >
                    <div>
                      <span className="font-medium text-gray-900 dark:text-white">
                        {formatTimezoneName(tz)}
                      </span>
                      <span className="ml-2 text-gray-500 dark:text-gray-400 text-xs">
                        {offset}
                      </span>
                    </div>
                    <span className="text-xs text-gray-400 dark:text-gray-500">
                      {time}
                    </span>
                  </button>
                )
              })
            )}
          </div>

          {/* Current browser timezone hint */}
          <div className="border-t border-gray-200 dark:border-gray-700 p-2">
            <button
              type="button"
              onClick={() => {
                const browserTz = Intl.DateTimeFormat().resolvedOptions().timeZone
                onChange(browserTz)
                setSearch('')
                setIsOpen(false)
              }}
              className="w-full px-3 py-1.5 text-xs text-indigo-600 dark:text-indigo-400 hover:bg-indigo-50 dark:hover:bg-indigo-900/30 rounded flex items-center gap-2"
            >
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17.657 16.657L13.414 20.9a1.998 1.998 0 01-2.827 0l-4.244-4.243a8 8 0 1111.314 0z" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 11a3 3 0 11-6 0 3 3 0 016 0z" />
              </svg>
              Use browser timezone ({Intl.DateTimeFormat().resolvedOptions().timeZone})
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
