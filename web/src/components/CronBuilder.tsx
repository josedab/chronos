import { useState, useEffect, useCallback } from 'react'

interface CronBuilderProps {
  value: string
  onChange: (value: string) => void
}

type CronMode = 'simple' | 'advanced'
type SimpleSchedule = 'every-minute' | 'hourly' | 'daily' | 'weekly' | 'monthly' | 'custom'

interface ParsedCron {
  minute: string
  hour: string
  dayOfMonth: string
  month: string
  dayOfWeek: string
}

const DAYS_OF_WEEK = [
  { value: '0', label: 'Sunday' },
  { value: '1', label: 'Monday' },
  { value: '2', label: 'Tuesday' },
  { value: '3', label: 'Wednesday' },
  { value: '4', label: 'Thursday' },
  { value: '5', label: 'Friday' },
  { value: '6', label: 'Saturday' },
]

const MONTHS = [
  { value: '1', label: 'January' },
  { value: '2', label: 'February' },
  { value: '3', label: 'March' },
  { value: '4', label: 'April' },
  { value: '5', label: 'May' },
  { value: '6', label: 'June' },
  { value: '7', label: 'July' },
  { value: '8', label: 'August' },
  { value: '9', label: 'September' },
  { value: '10', label: 'October' },
  { value: '11', label: 'November' },
  { value: '12', label: 'December' },
]

const PRESET_SCHEDULES: { value: SimpleSchedule; label: string; cron: string }[] = [
  { value: 'every-minute', label: 'Every minute', cron: '* * * * *' },
  { value: 'hourly', label: 'Every hour', cron: '0 * * * *' },
  { value: 'daily', label: 'Daily at midnight', cron: '0 0 * * *' },
  { value: 'weekly', label: 'Weekly on Sunday', cron: '0 0 * * 0' },
  { value: 'monthly', label: 'Monthly on the 1st', cron: '0 0 1 * *' },
  { value: 'custom', label: 'Custom schedule', cron: '' },
]

function parseCron(cron: string): ParsedCron {
  const parts = cron.trim().split(/\s+/)
  return {
    minute: parts[0] || '*',
    hour: parts[1] || '*',
    dayOfMonth: parts[2] || '*',
    month: parts[3] || '*',
    dayOfWeek: parts[4] || '*',
  }
}

function buildCron(parsed: ParsedCron): string {
  return `${parsed.minute} ${parsed.hour} ${parsed.dayOfMonth} ${parsed.month} ${parsed.dayOfWeek}`
}

function describeCron(cron: string): string {
  const parts = cron.trim().split(/\s+/)
  if (parts.length !== 5) return 'Invalid cron expression'

  const [minute, hour, dayOfMonth, month, dayOfWeek] = parts

  // Handle common shortcuts
  if (cron.startsWith('@')) {
    const shortcuts: Record<string, string> = {
      '@yearly': 'Once a year at midnight on January 1st',
      '@annually': 'Once a year at midnight on January 1st',
      '@monthly': 'Once a month at midnight on the 1st',
      '@weekly': 'Once a week at midnight on Sunday',
      '@daily': 'Once a day at midnight',
      '@midnight': 'Once a day at midnight',
      '@hourly': 'Once an hour at the beginning of the hour',
    }
    return shortcuts[cron] || cron
  }

  // Handle @every syntax
  if (cron.startsWith('@every')) {
    return `Every ${cron.replace('@every ', '')}`
  }

  // Build description
  const descriptions: string[] = []

  // Minute
  if (minute === '*') {
    descriptions.push('Every minute')
  } else if (minute.startsWith('*/')) {
    descriptions.push(`Every ${minute.slice(2)} minutes`)
  } else if (minute.includes(',')) {
    descriptions.push(`At minutes ${minute}`)
  } else if (minute.includes('-')) {
    descriptions.push(`Minutes ${minute}`)
  } else {
    descriptions.push(`At minute ${minute}`)
  }

  // Hour
  if (hour !== '*') {
    if (hour.startsWith('*/')) {
      descriptions.push(`every ${hour.slice(2)} hours`)
    } else if (hour.includes(',')) {
      descriptions.push(`at hours ${hour}`)
    } else if (hour.includes('-')) {
      const [start, end] = hour.split('-')
      descriptions.push(`between ${formatHour(start)} and ${formatHour(end)}`)
    } else {
      descriptions.push(`at ${formatHour(hour)}`)
    }
  }

  // Day of month
  if (dayOfMonth !== '*') {
    if (dayOfMonth.startsWith('*/')) {
      descriptions.push(`every ${dayOfMonth.slice(2)} days`)
    } else {
      descriptions.push(`on day ${dayOfMonth} of the month`)
    }
  }

  // Month
  if (month !== '*') {
    const monthName = MONTHS.find(m => m.value === month)?.label || month
    descriptions.push(`in ${monthName}`)
  }

  // Day of week
  if (dayOfWeek !== '*') {
    if (dayOfWeek.includes(',')) {
      const days = dayOfWeek.split(',').map(d => DAYS_OF_WEEK.find(day => day.value === d)?.label || d)
      descriptions.push(`on ${days.join(', ')}`)
    } else if (dayOfWeek.includes('-')) {
      const [start, end] = dayOfWeek.split('-')
      const startDay = DAYS_OF_WEEK.find(d => d.value === start)?.label || start
      const endDay = DAYS_OF_WEEK.find(d => d.value === end)?.label || end
      descriptions.push(`${startDay} through ${endDay}`)
    } else {
      const day = DAYS_OF_WEEK.find(d => d.value === dayOfWeek)?.label || dayOfWeek
      descriptions.push(`on ${day}`)
    }
  }

  return descriptions.join(' ') || 'Every minute'
}

function formatHour(hour: string): string {
  const h = parseInt(hour, 10)
  if (isNaN(h)) return hour
  if (h === 0) return '12:00 AM'
  if (h === 12) return '12:00 PM'
  if (h > 12) return `${h - 12}:00 PM`
  return `${h}:00 AM`
}

function getNextRuns(cron: string, count: number = 5): Date[] {
  const now = new Date()
  const runs: Date[] = []
  const parsed = parseCron(cron)

  // Simple next-run calculation for standard cron expressions
  for (let i = 0; i < count && runs.length < count; i++) {
    const nextRun = new Date(now.getTime() + (i + 1) * 60000)
    
    // For "every minute" just show next minutes
    if (cron === '* * * * *') {
      nextRun.setSeconds(0, 0)
      runs.push(nextRun)
      continue
    }

    // For hourly, show next hours
    if (parsed.minute !== '*' && parsed.hour === '*') {
      const targetMinute = parseInt(parsed.minute, 10)
      if (!isNaN(targetMinute)) {
        const run = new Date(now)
        run.setMinutes(targetMinute, 0, 0)
        if (run <= now) run.setHours(run.getHours() + 1)
        for (let j = 0; j < count; j++) {
          const r = new Date(run.getTime() + j * 3600000)
          if (r > now) runs.push(r)
          if (runs.length >= count) break
        }
        break
      }
    }

    // For daily at specific time
    if (parsed.minute !== '*' && parsed.hour !== '*' && parsed.dayOfMonth === '*' && parsed.dayOfWeek === '*') {
      const targetHour = parseInt(parsed.hour, 10)
      const targetMinute = parseInt(parsed.minute, 10)
      if (!isNaN(targetHour) && !isNaN(targetMinute)) {
        const run = new Date(now)
        run.setHours(targetHour, targetMinute, 0, 0)
        if (run <= now) run.setDate(run.getDate() + 1)
        for (let j = 0; j < count; j++) {
          const r = new Date(run.getTime() + j * 86400000)
          runs.push(r)
        }
        break
      }
    }
  }

  return runs.slice(0, count)
}

export default function CronBuilder({ value, onChange }: CronBuilderProps) {
  const [mode, setMode] = useState<CronMode>('simple')
  const [simpleSchedule, setSimpleSchedule] = useState<SimpleSchedule>('custom')
  const [parsed, setParsed] = useState<ParsedCron>(parseCron(value))
  const [customMinute, setCustomMinute] = useState('0')
  const [customHour, setCustomHour] = useState('9')
  const [selectedDays, setSelectedDays] = useState<string[]>([])

  // Detect if current value matches a preset
  useEffect(() => {
    const preset = PRESET_SCHEDULES.find(p => p.cron === value)
    if (preset && preset.value !== 'custom') {
      setSimpleSchedule(preset.value)
    } else {
      setSimpleSchedule('custom')
    }
    setParsed(parseCron(value))
  }, [value])

  const handlePresetChange = useCallback((preset: SimpleSchedule) => {
    setSimpleSchedule(preset)
    const schedule = PRESET_SCHEDULES.find(p => p.value === preset)
    if (schedule && schedule.cron) {
      onChange(schedule.cron)
    }
  }, [onChange])

  const handleCustomTimeChange = useCallback((minute: string, hour: string) => {
    setCustomMinute(minute)
    setCustomHour(hour)
    const newCron = buildCron({
      minute,
      hour,
      dayOfMonth: parsed.dayOfMonth,
      month: parsed.month,
      dayOfWeek: selectedDays.length > 0 ? selectedDays.join(',') : '*',
    })
    onChange(newCron)
  }, [onChange, parsed, selectedDays])

  const handleDayToggle = useCallback((day: string) => {
    const newDays = selectedDays.includes(day)
      ? selectedDays.filter(d => d !== day)
      : [...selectedDays, day].sort()
    setSelectedDays(newDays)
    const newCron = buildCron({
      minute: customMinute,
      hour: customHour,
      dayOfMonth: '*',
      month: parsed.month,
      dayOfWeek: newDays.length > 0 ? newDays.join(',') : '*',
    })
    onChange(newCron)
  }, [onChange, selectedDays, customMinute, customHour, parsed.month])

  const handleAdvancedChange = useCallback((field: keyof ParsedCron, fieldValue: string) => {
    const newParsed = { ...parsed, [field]: fieldValue }
    setParsed(newParsed)
    onChange(buildCron(newParsed))
  }, [onChange, parsed])

  const nextRuns = getNextRuns(value)

  return (
    <div className="space-y-4">
      {/* Mode Toggle */}
      <div className="flex gap-2">
        <button
          type="button"
          onClick={() => setMode('simple')}
          className={`px-3 py-1.5 text-sm rounded-md ${
            mode === 'simple'
              ? 'bg-indigo-100 text-indigo-700 font-medium'
              : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
          }`}
        >
          Simple
        </button>
        <button
          type="button"
          onClick={() => setMode('advanced')}
          className={`px-3 py-1.5 text-sm rounded-md ${
            mode === 'advanced'
              ? 'bg-indigo-100 text-indigo-700 font-medium'
              : 'bg-gray-100 text-gray-600 hover:bg-gray-200'
          }`}
        >
          Advanced
        </button>
      </div>

      {mode === 'simple' ? (
        <div className="space-y-4">
          {/* Preset Selector */}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Schedule Type
            </label>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              {PRESET_SCHEDULES.map((preset) => (
                <button
                  key={preset.value}
                  type="button"
                  onClick={() => handlePresetChange(preset.value)}
                  className={`px-3 py-2 text-sm rounded-md border ${
                    simpleSchedule === preset.value
                      ? 'border-indigo-500 bg-indigo-50 text-indigo-700'
                      : 'border-gray-300 hover:border-gray-400'
                  }`}
                >
                  {preset.label}
                </button>
              ))}
            </div>
          </div>

          {/* Custom Time Picker */}
          {simpleSchedule === 'custom' && (
            <div className="space-y-4 p-4 bg-gray-50 rounded-lg">
              <div className="grid grid-cols-2 gap-4">
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Hour
                  </label>
                  <select
                    value={customHour}
                    onChange={(e) => handleCustomTimeChange(customMinute, e.target.value)}
                    className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                  >
                    {Array.from({ length: 24 }, (_, i) => (
                      <option key={i} value={i.toString()}>
                        {formatHour(i.toString())}
                      </option>
                    ))}
                  </select>
                </div>
                <div>
                  <label className="block text-sm font-medium text-gray-700 mb-1">
                    Minute
                  </label>
                  <select
                    value={customMinute}
                    onChange={(e) => handleCustomTimeChange(e.target.value, customHour)}
                    className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 sm:text-sm"
                  >
                    {Array.from({ length: 60 }, (_, i) => (
                      <option key={i} value={i.toString()}>
                        :{i.toString().padStart(2, '0')}
                      </option>
                    ))}
                  </select>
                </div>
              </div>

              {/* Day of Week Selector */}
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-2">
                  Days of Week (leave empty for every day)
                </label>
                <div className="flex flex-wrap gap-2">
                  {DAYS_OF_WEEK.map((day) => (
                    <button
                      key={day.value}
                      type="button"
                      onClick={() => handleDayToggle(day.value)}
                      className={`px-3 py-1.5 text-sm rounded-md border ${
                        selectedDays.includes(day.value)
                          ? 'border-indigo-500 bg-indigo-100 text-indigo-700'
                          : 'border-gray-300 hover:border-gray-400'
                      }`}
                    >
                      {day.label.slice(0, 3)}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          )}
        </div>
      ) : (
        /* Advanced Mode */
        <div className="space-y-3">
          <div className="grid grid-cols-5 gap-2">
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">Minute</label>
              <input
                type="text"
                value={parsed.minute}
                onChange={(e) => handleAdvancedChange('minute', e.target.value)}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm font-mono"
                placeholder="*"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">Hour</label>
              <input
                type="text"
                value={parsed.hour}
                onChange={(e) => handleAdvancedChange('hour', e.target.value)}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm font-mono"
                placeholder="*"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">Day</label>
              <input
                type="text"
                value={parsed.dayOfMonth}
                onChange={(e) => handleAdvancedChange('dayOfMonth', e.target.value)}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm font-mono"
                placeholder="*"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">Month</label>
              <input
                type="text"
                value={parsed.month}
                onChange={(e) => handleAdvancedChange('month', e.target.value)}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm font-mono"
                placeholder="*"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-gray-500 mb-1">Weekday</label>
              <input
                type="text"
                value={parsed.dayOfWeek}
                onChange={(e) => handleAdvancedChange('dayOfWeek', e.target.value)}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500 text-sm font-mono"
                placeholder="*"
              />
            </div>
          </div>
          <p className="text-xs text-gray-500">
            Use * for any, */N for every N, N-M for ranges, or comma-separated values
          </p>
        </div>
      )}

      {/* Current Expression Display */}
      <div className="bg-gray-900 text-gray-100 rounded-lg p-4">
        <div className="flex justify-between items-center mb-2">
          <span className="text-xs text-gray-400">Cron Expression</span>
          <code className="text-lg font-mono text-green-400">{value}</code>
        </div>
        <p className="text-sm text-gray-300">
          📅 {describeCron(value)}
        </p>
      </div>

      {/* Next Runs Preview */}
      {nextRuns.length > 0 && (
        <div className="border border-gray-200 rounded-lg p-3">
          <h4 className="text-xs font-medium text-gray-500 mb-2">Next scheduled runs</h4>
          <ul className="space-y-1">
            {nextRuns.map((run, i) => (
              <li key={i} className="text-sm text-gray-600 font-mono">
                {run.toLocaleString()}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
