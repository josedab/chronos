import { useState, useEffect, createContext, useContext, ReactNode } from 'react'

interface TourStep {
  target: string  // CSS selector
  title: string
  content: string
  placement?: 'top' | 'bottom' | 'left' | 'right'
}

const TOUR_STEPS: TourStep[] = [
  {
    target: '[data-tour="dashboard"]',
    title: 'Welcome to Chronos! 🎉',
    content: 'This is your dashboard. Here you can see an overview of all your scheduled jobs and recent executions.',
    placement: 'bottom',
  },
  {
    target: '[data-tour="create-job"]',
    title: 'Create Jobs',
    content: 'Click here to create a new scheduled job. You can set up cron schedules and webhook endpoints.',
    placement: 'bottom',
  },
  {
    target: '[data-tour="jobs-nav"]',
    title: 'Manage Jobs',
    content: 'View and manage all your jobs from the Jobs page. You can enable, disable, trigger, and delete jobs.',
    placement: 'bottom',
  },
  {
    target: '[data-tour="executions-nav"]',
    title: 'Execution History',
    content: 'Track all job executions here. See which jobs ran, when they ran, and whether they succeeded or failed.',
    placement: 'bottom',
  },
  {
    target: '[data-tour="theme-toggle"]',
    title: 'Dark Mode',
    content: 'Toggle between light and dark mode for comfortable viewing.',
    placement: 'bottom',
  },
  {
    target: '[data-tour="command-palette"]',
    title: 'Quick Navigation',
    content: 'Press ⌘K (or Ctrl+K) to open the command palette for quick navigation and actions.',
    placement: 'bottom',
  },
]

const STORAGE_KEY = 'chronos-tour-completed'

interface TourContextValue {
  isActive: boolean
  currentStep: number
  startTour: () => void
  endTour: () => void
  nextStep: () => void
  prevStep: () => void
  skipTour: () => void
  hasCompletedTour: boolean
}

const TourContext = createContext<TourContextValue | null>(null)

export function useTour() {
  const context = useContext(TourContext)
  if (!context) {
    throw new Error('useTour must be used within a TourProvider')
  }
  return context
}

interface TourProviderProps {
  children: ReactNode
}

export function TourProvider({ children }: TourProviderProps) {
  const [isActive, setIsActive] = useState(false)
  const [currentStep, setCurrentStep] = useState(0)
  const [hasCompletedTour, setHasCompletedTour] = useState(() => {
    return localStorage.getItem(STORAGE_KEY) === 'true'
  })

  const startTour = () => {
    setCurrentStep(0)
    setIsActive(true)
  }

  const endTour = () => {
    setIsActive(false)
    setHasCompletedTour(true)
    localStorage.setItem(STORAGE_KEY, 'true')
  }

  const nextStep = () => {
    if (currentStep < TOUR_STEPS.length - 1) {
      setCurrentStep(prev => prev + 1)
    } else {
      endTour()
    }
  }

  const prevStep = () => {
    if (currentStep > 0) {
      setCurrentStep(prev => prev - 1)
    }
  }

  const skipTour = () => {
    endTour()
  }

  // Auto-start tour for new users
  useEffect(() => {
    if (!hasCompletedTour) {
      const timer = setTimeout(() => {
        startTour()
      }, 1000)
      return () => clearTimeout(timer)
    }
  }, [hasCompletedTour])

  return (
    <TourContext.Provider value={{ 
      isActive, 
      currentStep, 
      startTour, 
      endTour, 
      nextStep, 
      prevStep, 
      skipTour,
      hasCompletedTour 
    }}>
      {children}
      {isActive && <TourOverlay />}
    </TourContext.Provider>
  )
}

function TourOverlay() {
  const { currentStep, nextStep, prevStep, skipTour } = useTour()
  const step = TOUR_STEPS[currentStep]
  const [position, setPosition] = useState({ top: 0, left: 0 })

  useEffect(() => {
    const target = document.querySelector(step.target)
    if (target) {
      const rect = target.getBoundingClientRect()
      const scrollTop = window.scrollY
      const scrollLeft = window.scrollX

      let top = rect.bottom + scrollTop + 10
      let left = rect.left + scrollLeft + rect.width / 2

      // Adjust for placement
      if (step.placement === 'top') {
        top = rect.top + scrollTop - 10
      } else if (step.placement === 'left') {
        left = rect.left + scrollLeft - 10
        top = rect.top + scrollTop + rect.height / 2
      } else if (step.placement === 'right') {
        left = rect.right + scrollLeft + 10
        top = rect.top + scrollTop + rect.height / 2
      }

      setPosition({ top, left })

      // Highlight the target
      target.classList.add('ring-2', 'ring-indigo-500', 'ring-offset-2', 'relative', 'z-50')

      return () => {
        target.classList.remove('ring-2', 'ring-indigo-500', 'ring-offset-2', 'relative', 'z-50')
      }
    }
  }, [step])

  return (
    <>
      {/* Backdrop */}
      <div 
        className="fixed inset-0 bg-black/50 z-40"
        onClick={skipTour}
      />

      {/* Tooltip */}
      <div 
        className="fixed z-50 w-80 bg-white dark:bg-gray-800 rounded-lg shadow-xl p-4 transform -translate-x-1/2"
        style={{ 
          top: position.top, 
          left: position.left,
        }}
      >
        {/* Arrow */}
        <div className="absolute -top-2 left-1/2 -translate-x-1/2 w-4 h-4 bg-white dark:bg-gray-800 rotate-45" />

        {/* Content */}
        <div className="relative">
          <div className="flex items-center justify-between mb-2">
            <h4 className="font-semibold text-gray-900 dark:text-white">
              {step.title}
            </h4>
            <span className="text-xs text-gray-500 dark:text-gray-400">
              {currentStep + 1} / {TOUR_STEPS.length}
            </span>
          </div>
          <p className="text-sm text-gray-600 dark:text-gray-300 mb-4">
            {step.content}
          </p>

          {/* Actions */}
          <div className="flex items-center justify-between">
            <button
              onClick={skipTour}
              className="text-sm text-gray-500 hover:text-gray-700 dark:hover:text-gray-300"
            >
              Skip tour
            </button>
            <div className="flex gap-2">
              {currentStep > 0 && (
                <button
                  onClick={prevStep}
                  className="px-3 py-1.5 text-sm text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded"
                >
                  Back
                </button>
              )}
              <button
                onClick={nextStep}
                className="px-3 py-1.5 text-sm text-white bg-indigo-600 hover:bg-indigo-700 rounded"
              >
                {currentStep === TOUR_STEPS.length - 1 ? 'Finish' : 'Next'}
              </button>
            </div>
          </div>
        </div>

        {/* Progress dots */}
        <div className="flex justify-center gap-1 mt-3">
          {TOUR_STEPS.map((_, i) => (
            <div
              key={i}
              className={`w-1.5 h-1.5 rounded-full transition-colors ${
                i === currentStep
                  ? 'bg-indigo-600'
                  : i < currentStep
                  ? 'bg-indigo-300'
                  : 'bg-gray-300 dark:bg-gray-600'
              }`}
            />
          ))}
        </div>
      </div>
    </>
  )
}

// Button to restart the tour
export function RestartTourButton({ className = '' }: { className?: string }) {
  const { startTour } = useTour()

  return (
    <button
      onClick={startTour}
      className={`text-sm text-indigo-600 dark:text-indigo-400 hover:underline flex items-center gap-1 ${className}`}
    >
      <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
      </svg>
      Take a tour
    </button>
  )
}
