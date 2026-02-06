import { useMemo } from 'react'

interface CronExplainerProps {
  expression: string
  className?: string
}

// Parse cron expression and return human-readable description
function explainCron(expr: string): string {
  if (!expr || expr.trim() === '') {
    return 'No schedule defined'
  }

  const parts = expr.trim().split(/\s+/)
  if (parts.length < 5 || parts.length > 6) {
    return 'Invalid cron expression'
  }

  // Handle 6-part expressions (with seconds) by skipping the first part
  const [minute, hour, dayOfMonth, month, dayOfWeek] = parts.length === 6 
    ? parts.slice(1) 
    : parts

  try {
    const minuteDesc = parseMinute(minute)
    const hourDesc = parseHour(hour)
    const dayOfMonthDesc = parseDayOfMonth(dayOfMonth)
    const monthDesc = parseMonth(month)
    const dayOfWeekDesc = parseDayOfWeek(dayOfWeek)

    // Build human-readable string
    return buildDescription(minuteDesc, hourDesc, dayOfMonthDesc, monthDesc, dayOfWeekDesc)
  } catch {
    return 'Unable to parse cron expression'
  }
}

interface FieldDesc {
  type: 'every' | 'specific' | 'range' | 'step' | 'list'
  values?: number[]
  step?: number
  start?: number
  end?: number
}

function parseField(field: string, min: number, _max: number): FieldDesc {
  if (field === '*') {
    return { type: 'every' }
  }

  if (field.includes('/')) {
    const [range, step] = field.split('/')
    const stepNum = parseInt(step, 10)
    if (range === '*') {
      return { type: 'step', step: stepNum, start: min }
    }
    const start = parseInt(range, 10)
    return { type: 'step', step: stepNum, start }
  }

  if (field.includes('-')) {
    const [start, end] = field.split('-').map(n => parseInt(n, 10))
    return { type: 'range', start, end }
  }

  if (field.includes(',')) {
    const values = field.split(',').map(n => parseInt(n, 10))
    return { type: 'list', values }
  }

  const value = parseInt(field, 10)
  if (!isNaN(value)) {
    return { type: 'specific', values: [value] }
  }

  return { type: 'every' }
}

function parseMinute(field: string): FieldDesc {
  return parseField(field, 0, 59)
}

function parseHour(field: string): FieldDesc {
  return parseField(field, 0, 23)
}

function parseDayOfMonth(field: string): FieldDesc {
  return parseField(field, 1, 31)
}

function parseMonth(field: string): FieldDesc {
  // Handle month names
  const monthNames: Record<string, number> = {
    jan: 1, feb: 2, mar: 3, apr: 4, may: 5, jun: 6,
    jul: 7, aug: 8, sep: 9, oct: 10, nov: 11, dec: 12
  }
  let normalized = field.toLowerCase()
  Object.entries(monthNames).forEach(([name, num]) => {
    normalized = normalized.replace(new RegExp(name, 'g'), String(num))
  })
  return parseField(normalized, 1, 12)
}

function parseDayOfWeek(field: string): FieldDesc {
  // Handle day names
  const dayNames: Record<string, number> = {
    sun: 0, mon: 1, tue: 2, wed: 3, thu: 4, fri: 5, sat: 6
  }
  let normalized = field.toLowerCase()
  Object.entries(dayNames).forEach(([name, num]) => {
    normalized = normalized.replace(new RegExp(name, 'g'), String(num))
  })
  return parseField(normalized, 0, 6)
}

function formatTime(hour: number, minute: number): string {
  const h = hour % 12 || 12
  const ampm = hour < 12 ? 'AM' : 'PM'
  const m = minute.toString().padStart(2, '0')
  return `${h}:${m} ${ampm}`
}

function formatHour(hour: number): string {
  const h = hour % 12 || 12
  const ampm = hour < 12 ? 'AM' : 'PM'
  return `${h} ${ampm}`
}

const monthNames = ['', 'January', 'February', 'March', 'April', 'May', 'June', 
                    'July', 'August', 'September', 'October', 'November', 'December']
const dayNames = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday']

function ordinal(n: number): string {
  const s = ['th', 'st', 'nd', 'rd']
  const v = n % 100
  return n + (s[(v - 20) % 10] || s[v] || s[0])
}

function buildDescription(
  minute: FieldDesc,
  hour: FieldDesc,
  dayOfMonth: FieldDesc,
  month: FieldDesc,
  dayOfWeek: FieldDesc
): string {
  const parts: string[] = []

  // Special common patterns
  if (minute.type === 'every' && hour.type === 'every' && 
      dayOfMonth.type === 'every' && month.type === 'every' && dayOfWeek.type === 'every') {
    return 'Every minute'
  }

  // Every N minutes
  if (minute.type === 'step' && hour.type === 'every' && 
      dayOfMonth.type === 'every' && month.type === 'every' && dayOfWeek.type === 'every') {
    return `Every ${minute.step} minute${minute.step !== 1 ? 's' : ''}`
  }

  // Every hour at specific minute
  if (minute.type === 'specific' && hour.type === 'every' && 
      dayOfMonth.type === 'every' && month.type === 'every' && dayOfWeek.type === 'every') {
    const m = minute.values![0]
    return m === 0 ? 'Every hour, on the hour' : `Every hour at ${m} minute${m !== 1 ? 's' : ''} past`
  }

  // Specific time
  if (minute.type === 'specific' && hour.type === 'specific') {
    const time = formatTime(hour.values![0], minute.values![0])
    parts.push(`At ${time}`)
  } else if (minute.type === 'specific' && hour.type === 'every') {
    parts.push(`At ${minute.values![0]} minutes past every hour`)
  } else if (minute.type === 'every' && hour.type === 'specific') {
    parts.push(`Every minute during ${formatHour(hour.values![0])} hour`)
  } else if (minute.type === 'step') {
    parts.push(`Every ${minute.step} minutes`)
  } else if (hour.type === 'step') {
    parts.push(`Every ${hour.step} hours`)
  } else if (hour.type === 'range' && minute.type === 'specific') {
    parts.push(`At ${minute.values![0]} minutes past each hour from ${formatHour(hour.start!)} to ${formatHour(hour.end!)}`)
  } else if (hour.type === 'list' && minute.type === 'specific') {
    const hours = hour.values!.map(h => formatHour(h)).join(', ')
    parts.push(`At ${minute.values![0]} minutes past ${hours}`)
  }

  // Day of week
  if (dayOfWeek.type === 'specific') {
    const day = dayNames[dayOfWeek.values![0]]
    parts.push(`on ${day}`)
  } else if (dayOfWeek.type === 'list') {
    const days = dayOfWeek.values!.map(d => dayNames[d])
    if (days.length === 5 && !dayOfWeek.values!.includes(0) && !dayOfWeek.values!.includes(6)) {
      parts.push('on weekdays')
    } else if (days.length === 2 && dayOfWeek.values!.includes(0) && dayOfWeek.values!.includes(6)) {
      parts.push('on weekends')
    } else {
      parts.push(`on ${days.join(', ')}`)
    }
  } else if (dayOfWeek.type === 'range') {
    parts.push(`from ${dayNames[dayOfWeek.start!]} to ${dayNames[dayOfWeek.end!]}`)
  }

  // Day of month
  if (dayOfMonth.type === 'specific') {
    parts.push(`on the ${ordinal(dayOfMonth.values![0])}`)
  } else if (dayOfMonth.type === 'list') {
    const days = dayOfMonth.values!.map(d => ordinal(d)).join(', ')
    parts.push(`on the ${days}`)
  } else if (dayOfMonth.type === 'range') {
    parts.push(`from the ${ordinal(dayOfMonth.start!)} to the ${ordinal(dayOfMonth.end!)}`)
  }

  // Month
  if (month.type === 'specific') {
    parts.push(`in ${monthNames[month.values![0]]}`)
  } else if (month.type === 'list') {
    const months = month.values!.map(m => monthNames[m]).join(', ')
    parts.push(`in ${months}`)
  } else if (month.type === 'range') {
    parts.push(`from ${monthNames[month.start!]} to ${monthNames[month.end!]}`)
  }

  if (parts.length === 0) {
    return 'Custom schedule'
  }

  return parts.join(' ')
}

// Calculate next N run times
function getNextRuns(expr: string, count: number = 5): Date[] {
  if (!expr || expr.trim() === '') return []
  
  const parts = expr.trim().split(/\s+/)
  if (parts.length < 5) return []

  const runs: Date[] = []
  const now = new Date()
  let current = new Date(now)
  
  // Simple implementation for common patterns
  const [minute, hour, dayOfMonth, month, dayOfWeek] = parts.length === 6 
    ? parts.slice(1) 
    : parts

  for (let i = 0; i < 1000 && runs.length < count; i++) {
    current = new Date(current.getTime() + 60000) // Add 1 minute
    
    if (matchesCron(current, minute, hour, dayOfMonth, month, dayOfWeek)) {
      runs.push(new Date(current))
    }
  }

  return runs
}

function matchesCron(
  date: Date,
  minute: string,
  hour: string,
  dayOfMonth: string,
  month: string,
  dayOfWeek: string
): boolean {
  return matchField(date.getMinutes(), minute, 0, 59) &&
         matchField(date.getHours(), hour, 0, 23) &&
         matchField(date.getDate(), dayOfMonth, 1, 31) &&
         matchField(date.getMonth() + 1, month, 1, 12) &&
         matchField(date.getDay(), dayOfWeek, 0, 6)
}

function matchField(value: number, field: string, min: number, _max: number): boolean {
  if (field === '*') return true
  
  if (field.includes('/')) {
    const [range, step] = field.split('/')
    const stepNum = parseInt(step, 10)
    const start = range === '*' ? min : parseInt(range, 10)
    return (value - start) % stepNum === 0 && value >= start
  }

  if (field.includes('-')) {
    const [start, end] = field.split('-').map(n => parseInt(n, 10))
    return value >= start && value <= end
  }

  if (field.includes(',')) {
    const values = field.split(',').map(n => parseInt(n, 10))
    return values.includes(value)
  }

  return parseInt(field, 10) === value
}

export default function CronExplainer({ expression, className = '' }: CronExplainerProps) {
  const explanation = useMemo(() => explainCron(expression), [expression])
  const nextRuns = useMemo(() => getNextRuns(expression, 3), [expression])

  const isValid = !explanation.includes('Invalid') && !explanation.includes('Unable')

  return (
    <div className={`text-sm ${className}`}>
      <div className={`flex items-center gap-2 ${isValid ? 'text-gray-600 dark:text-gray-400' : 'text-red-600 dark:text-red-400'}`}>
        <svg className="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        <span className="font-medium">{explanation}</span>
      </div>
      
      {isValid && nextRuns.length > 0 && (
        <div className="mt-2 ml-6 text-xs text-gray-500 dark:text-gray-500">
          <span className="font-medium">Next runs:</span>
          <ul className="mt-1 space-y-0.5">
            {nextRuns.map((run, i) => (
              <li key={i} className="flex items-center gap-1">
                <span className="text-gray-400">→</span>
                {run.toLocaleString()}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}

export { explainCron, getNextRuns }
