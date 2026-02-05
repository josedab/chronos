import { useState, useCallback, useEffect, useRef } from 'react'

interface UseListNavigationOptions<T> {
  items: T[]
  onSelect?: (item: T, index: number) => void
  onEnter?: (item: T, index: number) => void
  enabled?: boolean
}

interface UseListNavigationResult {
  selectedIndex: number
  setSelectedIndex: (index: number) => void
  handleKeyDown: (event: KeyboardEvent) => void
}

/**
 * Hook for j/k keyboard navigation in list pages.
 * 
 * Usage:
 * ```tsx
 * const { selectedIndex, setSelectedIndex } = useListNavigation({
 *   items: jobs,
 *   onEnter: (job) => navigate(`/jobs/${job.id}`),
 * })
 * ```
 */
export function useListNavigation<T>({
  items,
  onSelect,
  onEnter,
  enabled = true,
}: UseListNavigationOptions<T>): UseListNavigationResult {
  const [selectedIndex, setSelectedIndex] = useState(-1)
  const itemsRef = useRef(items)
  
  // Keep items ref up to date
  useEffect(() => {
    itemsRef.current = items
  }, [items])

  // Reset selection when items change significantly
  useEffect(() => {
    if (selectedIndex >= items.length) {
      setSelectedIndex(items.length > 0 ? items.length - 1 : -1)
    }
  }, [items.length, selectedIndex])

  const handleKeyDown = useCallback((event: KeyboardEvent) => {
    if (!enabled || items.length === 0) return

    // Don't trigger when typing in inputs
    const target = event.target as HTMLElement
    if (
      target.tagName === 'INPUT' ||
      target.tagName === 'TEXTAREA' ||
      target.tagName === 'SELECT' ||
      target.isContentEditable
    ) {
      return
    }

    const key = event.key.toLowerCase()

    switch (key) {
      case 'j': // Move down
        event.preventDefault()
        setSelectedIndex(prev => {
          const newIndex = prev < items.length - 1 ? prev + 1 : prev
          if (onSelect && newIndex !== prev) {
            onSelect(items[newIndex], newIndex)
          }
          return newIndex
        })
        break

      case 'k': // Move up
        event.preventDefault()
        setSelectedIndex(prev => {
          const newIndex = prev > 0 ? prev - 1 : 0
          if (onSelect && newIndex !== prev) {
            onSelect(items[newIndex], newIndex)
          }
          return newIndex
        })
        break

      case 'enter': // Select current item
        if (selectedIndex >= 0 && selectedIndex < items.length && onEnter) {
          event.preventDefault()
          onEnter(items[selectedIndex], selectedIndex)
        }
        break

      case 'home': // Go to first item
        event.preventDefault()
        setSelectedIndex(0)
        if (onSelect && items.length > 0) {
          onSelect(items[0], 0)
        }
        break

      case 'end': // Go to last item
        event.preventDefault()
        const lastIndex = items.length - 1
        setSelectedIndex(lastIndex)
        if (onSelect && items.length > 0) {
          onSelect(items[lastIndex], lastIndex)
        }
        break
    }
  }, [enabled, items, selectedIndex, onSelect, onEnter])

  // Register global keyboard handler
  useEffect(() => {
    if (!enabled) return

    document.addEventListener('keydown', handleKeyDown)
    return () => document.removeEventListener('keydown', handleKeyDown)
  }, [enabled, handleKeyDown])

  return {
    selectedIndex,
    setSelectedIndex,
    handleKeyDown,
  }
}

/**
 * Helper component to scroll selected item into view.
 */
export function useScrollIntoView(selectedIndex: number, containerRef: React.RefObject<HTMLElement>) {
  useEffect(() => {
    if (selectedIndex < 0 || !containerRef.current) return

    const container = containerRef.current
    const selectedElement = container.querySelector(`[data-index="${selectedIndex}"]`) as HTMLElement
    
    if (selectedElement) {
      selectedElement.scrollIntoView({
        behavior: 'smooth',
        block: 'nearest',
      })
    }
  }, [selectedIndex, containerRef])
}
