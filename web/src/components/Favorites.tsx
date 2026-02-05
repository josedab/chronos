import { createContext, useContext, useState, useEffect, useCallback, ReactNode } from 'react'

interface FavoritesContextValue {
  favorites: string[]
  isFavorite: (jobId: string) => boolean
  toggleFavorite: (jobId: string) => void
  addFavorite: (jobId: string) => void
  removeFavorite: (jobId: string) => void
}

const STORAGE_KEY = 'chronos-favorite-jobs'

const FavoritesContext = createContext<FavoritesContextValue | null>(null)

export function useFavorites() {
  const context = useContext(FavoritesContext)
  if (!context) {
    throw new Error('useFavorites must be used within a FavoritesProvider')
  }
  return context
}

interface FavoritesProviderProps {
  children: ReactNode
}

export function FavoritesProvider({ children }: FavoritesProviderProps) {
  const [favorites, setFavorites] = useState<string[]>(() => {
    try {
      const stored = localStorage.getItem(STORAGE_KEY)
      return stored ? JSON.parse(stored) : []
    } catch {
      return []
    }
  })

  // Persist to localStorage
  useEffect(() => {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(favorites))
  }, [favorites])

  const isFavorite = useCallback((jobId: string) => {
    return favorites.includes(jobId)
  }, [favorites])

  const addFavorite = useCallback((jobId: string) => {
    setFavorites(prev => {
      if (prev.includes(jobId)) return prev
      return [...prev, jobId]
    })
  }, [])

  const removeFavorite = useCallback((jobId: string) => {
    setFavorites(prev => prev.filter(id => id !== jobId))
  }, [])

  const toggleFavorite = useCallback((jobId: string) => {
    setFavorites(prev => {
      if (prev.includes(jobId)) {
        return prev.filter(id => id !== jobId)
      }
      return [...prev, jobId]
    })
  }, [])

  return (
    <FavoritesContext.Provider value={{ favorites, isFavorite, toggleFavorite, addFavorite, removeFavorite }}>
      {children}
    </FavoritesContext.Provider>
  )
}

// Favorite star button component
interface FavoriteButtonProps {
  jobId: string
  className?: string
  size?: 'sm' | 'md'
}

export function FavoriteButton({ jobId, className = '', size = 'md' }: FavoriteButtonProps) {
  const { isFavorite, toggleFavorite } = useFavorites()
  const favorite = isFavorite(jobId)

  const sizeClasses = size === 'sm' ? 'w-4 h-4' : 'w-5 h-5'

  return (
    <button
      onClick={(e) => {
        e.preventDefault()
        e.stopPropagation()
        toggleFavorite(jobId)
      }}
      className={`text-gray-400 hover:text-yellow-500 focus:outline-none transition-colors ${className}`}
      title={favorite ? 'Remove from favorites' : 'Add to favorites'}
    >
      {favorite ? (
        <svg className={`${sizeClasses} text-yellow-500`} fill="currentColor" viewBox="0 0 24 24">
          <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
        </svg>
      ) : (
        <svg className={sizeClasses} fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
        </svg>
      )}
    </button>
  )
}
