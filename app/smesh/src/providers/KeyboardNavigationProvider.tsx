import { isTouchDevice } from '@/lib/utils'
import { Event } from 'nostr-tools'
import {
  createContext,
  ReactNode,
  RefObject,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState
} from 'react'
import modalManager from '@/services/modal-manager.service'
import { useScreenSize } from './ScreenSizeProvider'
import { useUserPreferences } from './UserPreferencesProvider'

// ============================================================================
// Abstract Navigation Intent System
// ============================================================================

/**
 * Navigation intents are abstract actions that the control plane emits.
 * Components decide what each intent means in their context.
 */
export type NavigationIntent =
  | 'up'
  | 'down'
  | 'left'
  | 'right'
  | 'activate'
  | 'back'
  | 'cancel'
  | 'pageUp'
  | 'nextAction'
  | 'prevAction'

/**
 * Navigation regions handle intents for a specific area of the UI.
 * Higher priority regions handle intents first.
 */
export interface NavigationRegion {
  id: string
  priority: number
  isActive: () => boolean
  handleIntent: (intent: NavigationIntent) => boolean // returns true if handled
}

// ============================================================================
// Legacy Types (for backward compatibility during migration)
// ============================================================================

export type TNavigationColumn = 0 | 1 | 2 // 0=sidebar, 1=primary, 2=secondary

export type TActionType = 'reply' | 'repost' | 'quote' | 'react' | 'zap'

export type TActionMode = {
  active: boolean
  selectedAction: TActionType | null
  noteEvent: Event | null
}

export type TItemMeta = {
  type: 'sidebar' | 'note' | 'settings'
  event?: Event
  onActivate?: () => void
}

type TRegisteredItem = {
  ref: RefObject<HTMLElement>
  meta?: TItemMeta
}

// ============================================================================
// Context Types
// ============================================================================

type TKeyboardNavigationContext = {
  // Intent system
  emitIntent: (intent: NavigationIntent) => void
  registerRegion: (region: NavigationRegion) => void
  unregisterRegion: (id: string) => void

  // Column focus (legacy, components should migrate to regions)
  activeColumn: TNavigationColumn
  setActiveColumn: (column: TNavigationColumn) => void

  // Item selection per column
  selectedIndex: Record<TNavigationColumn, number>
  setSelectedIndex: (column: TNavigationColumn, index: number) => void
  resetPrimarySelection: () => void
  offsetSelection: (column: TNavigationColumn, offset: number) => void
  clearColumn: (column: TNavigationColumn) => void

  // Registered items per column
  registerItem: (
    column: TNavigationColumn,
    index: number,
    ref: RefObject<HTMLElement>,
    meta?: TItemMeta
  ) => void
  unregisterItem: (column: TNavigationColumn, index: number) => void
  getItemCount: (column: TNavigationColumn) => number

  // Load more callback per column (for infinite scroll)
  registerLoadMore: (column: TNavigationColumn, callback: () => void) => void
  unregisterLoadMore: (column: TNavigationColumn) => void

  // Action mode
  actionMode: TActionMode
  enterActionMode: (noteEvent: Event) => void
  exitActionMode: () => void
  cycleAction: (direction?: 1 | -1) => void

  // Visual state
  isItemSelected: (column: TNavigationColumn, index: number) => boolean

  // Settings accordion
  openAccordionItem: string | null
  setOpenAccordionItem: (value: string | null) => void

  // Keyboard nav enabled
  isEnabled: boolean
  toggleKeyboardMode: () => void

  // Scroll utilities
  scrollToCenter: (element: HTMLElement) => void
}

const ACTIONS: TActionType[] = ['reply', 'repost', 'quote', 'react', 'zap']

const KeyboardNavigationContext = createContext<TKeyboardNavigationContext | undefined>(undefined)

export function useKeyboardNavigation() {
  const context = useContext(KeyboardNavigationContext)
  if (!context) {
    throw new Error('useKeyboardNavigation must be used within KeyboardNavigationProvider')
  }
  return context
}

/**
 * Hook for components to register as a navigation region.
 * The region handles intents and decides what they mean.
 */
export function useNavigationRegion(
  id: string,
  priority: number,
  isActive: () => boolean,
  handleIntent: (intent: NavigationIntent) => boolean,
  deps: React.DependencyList = []
) {
  const { registerRegion, unregisterRegion } = useKeyboardNavigation()

  useEffect(() => {
    const region: NavigationRegion = {
      id,
      priority,
      isActive,
      handleIntent
    }
    registerRegion(region)
    return () => unregisterRegion(id)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [id, priority, registerRegion, unregisterRegion, ...deps])
}

// Helper to check if an input element is focused
function isInputFocused(): boolean {
  const activeElement = document.activeElement
  if (!activeElement) return false
  const tagName = activeElement.tagName.toLowerCase()
  return (
    tagName === 'input' ||
    tagName === 'textarea' ||
    activeElement.getAttribute('contenteditable') === 'true'
  )
}

export function KeyboardNavigationProvider({
  children,
  secondaryStackLength,
  sidebarDrawerOpen,
  onBack,
  onCloseSecondary
}: {
  children: ReactNode
  secondaryStackLength: number
  sidebarDrawerOpen: boolean
  onBack?: () => void
  onCloseSecondary?: () => void
}) {
  const { isSmallScreen } = useScreenSize()
  const { enableSingleColumnLayout } = useUserPreferences()

  const [activeColumn, setActiveColumnState] = useState<TNavigationColumn>(1)
  const [selectedIndex, setSelectedIndexState] = useState<Record<TNavigationColumn, number>>({
    0: 0,
    1: 0,
    2: 0
  })
  const [actionMode, setActionMode] = useState<TActionMode>({
    active: false,
    selectedAction: null,
    noteEvent: null
  })
  const [openAccordionItem, setOpenAccordionItem] = useState<string | null>(null)
  const [isEnabled, setIsEnabled] = useState(false)

  // Track Escape presses for triple-Escape to disable keyboard mode
  const escapeTimestampsRef = useRef<number[]>([])
  const TRIPLE_ESCAPE_WINDOW = 800 // ms within which 3 escapes must occur

  // Refs to always have latest values (avoid stale closures)
  const activeColumnRef = useRef<TNavigationColumn>(activeColumn)
  const selectedIndexRef = useRef<Record<TNavigationColumn, number>>(selectedIndex)

  // Wrapper to update both state and ref
  const setActiveColumn = useCallback((column: TNavigationColumn) => {
    activeColumnRef.current = column
    setActiveColumnState(column)
  }, [])

  // Toggle keyboard navigation mode
  const toggleKeyboardMode = useCallback(() => {
    setIsEnabled((prev) => {
      const newEnabled = !prev
      if (newEnabled) {
        // When enabling, initialize selection to first item in active column
        const items = itemsRef.current[activeColumnRef.current]
        if (items.size > 0) {
          const firstIndex = Array.from(items.keys()).sort((a, b) => a - b)[0]
          if (firstIndex !== undefined) {
            setSelectedIndexState((prevState) => ({
              ...prevState,
              [activeColumnRef.current]: firstIndex
            }))
          }
        }
      }
      return newEnabled
    })
  }, [])

  // Navigation regions registry
  const regionsRef = useRef<Map<string, NavigationRegion>>(new Map())

  // Ref to hold the latest handleDefaultIntent function
  const handleDefaultIntentRef = useRef<(intent: NavigationIntent) => void>(() => {})

  // Item registration per column
  const itemsRef = useRef<Record<TNavigationColumn, Map<number, TRegisteredItem>>>({
    0: new Map(),
    1: new Map(),
    2: new Map()
  })

  // Load more callbacks per column (for infinite scroll)
  const loadMoreRef = useRef<Record<TNavigationColumn, (() => void) | null>>({
    0: null,
    1: null,
    2: null
  })

  // ============================================================================
  // Intent System
  // ============================================================================

  const registerRegion = useCallback((region: NavigationRegion) => {
    regionsRef.current.set(region.id, region)
  }, [])

  const unregisterRegion = useCallback((id: string) => {
    regionsRef.current.delete(id)
  }, [])

  const emitIntent = useCallback((intent: NavigationIntent) => {
    // Sort regions by priority (highest first)
    const regions = Array.from(regionsRef.current.values())
      .filter((r) => r.isActive())
      .sort((a, b) => b.priority - a.priority)

    // Let regions handle the intent in priority order
    for (const region of regions) {
      if (region.handleIntent(intent)) {
        return // Intent was handled
      }
    }

    // Fallback to default handling if no region handled it
    handleDefaultIntentRef.current(intent)
  }, [])

  // ============================================================================
  // Scroll to Center (or top if near edge)
  // ============================================================================

  const scrollToCenter = useCallback((element: HTMLElement) => {
    // Find the scrollable container (look for overflow-y: auto/scroll)
    // Stop at document.body - if we reach it, use window scrolling instead
    let scrollContainer: HTMLElement | null = element.parentElement
    while (scrollContainer && scrollContainer !== document.body && scrollContainer !== document.documentElement) {
      const style = window.getComputedStyle(scrollContainer)
      const overflowY = style.overflowY
      if (overflowY === 'auto' || overflowY === 'scroll') {
        // Verify this container is actually scrollable (has scrollable content)
        if (scrollContainer.scrollHeight > scrollContainer.clientHeight) {
          break
        }
      }
      scrollContainer = scrollContainer.parentElement
    }

    // If we reached body/documentElement, use window scrolling
    if (!scrollContainer || scrollContainer === document.body || scrollContainer === document.documentElement) {
      scrollContainer = null
    }

    const headerOffset = 100 // Account for sticky headers

    if (scrollContainer) {
      // Scroll within container
      const containerRect = scrollContainer.getBoundingClientRect()
      const elementRect = element.getBoundingClientRect()

      // Position relative to container
      const elementTopInContainer = elementRect.top - containerRect.top
      const elementBottomInContainer = elementRect.bottom - containerRect.top
      const containerHeight = containerRect.height
      const visibleHeight = containerHeight - headerOffset

      // If element is taller than visible area, scroll to show its top
      if (elementRect.height > visibleHeight) {
        const targetScrollTop = scrollContainer.scrollTop + elementTopInContainer - headerOffset
        scrollContainer.scrollTo({
          top: Math.max(0, targetScrollTop),
          behavior: 'instant'
        })
        return
      }

      // Check if already visible with margin
      const isVisible = elementTopInContainer >= headerOffset &&
                       elementBottomInContainer <= containerHeight - 50

      if (!isVisible) {
        // Calculate target scroll to center the element
        const elementMiddle = elementTopInContainer + elementRect.height / 2
        const containerMiddle = containerHeight / 2
        const scrollAdjustment = elementMiddle - containerMiddle
        const newScrollTop = scrollContainer.scrollTop + scrollAdjustment

        // Don't scroll past the top
        scrollContainer.scrollTo({
          top: Math.max(0, newScrollTop),
          behavior: 'instant'
        })
      }
    } else {
      // Window scrolling (mobile, single column mode)
      const rect = element.getBoundingClientRect()
      const viewportHeight = window.innerHeight
      const visibleHeight = viewportHeight - headerOffset

      // If element is taller than visible area, scroll to show its top
      if (rect.height > visibleHeight) {
        const targetScrollTop = window.scrollY + rect.top - headerOffset
        window.scrollTo({
          top: Math.max(0, targetScrollTop),
          behavior: 'instant'
        })
        return
      }

      // Check if already visible
      const isVisible = rect.top >= headerOffset && rect.bottom <= viewportHeight - 50

      if (!isVisible) {
        // Calculate target scroll to center the element
        const elementMiddle = rect.top + rect.height / 2
        const viewportMiddle = viewportHeight / 2
        const scrollAdjustment = elementMiddle - viewportMiddle
        const newScrollTop = window.scrollY + scrollAdjustment

        window.scrollTo({
          top: Math.max(0, newScrollTop),
          behavior: 'instant'
        })
      }
    }
  }, [])

  // ============================================================================
  // Legacy Item Management
  // ============================================================================

  const setSelectedIndex = useCallback((column: TNavigationColumn, index: number) => {
    selectedIndexRef.current = { ...selectedIndexRef.current, [column]: index }
    setSelectedIndexState((prev) => ({
      ...prev,
      [column]: index
    }))
  }, [])

  const resetPrimarySelection = useCallback(() => {
    setSelectedIndex(1, 0)
    setActiveColumn(1)
  }, [setSelectedIndex, setActiveColumn])

  const offsetSelection = useCallback(
    (column: TNavigationColumn, offset: number) => {
      setSelectedIndexState((prev) => {
        const newState = { ...prev, [column]: Math.max(0, prev[column] + offset) }
        selectedIndexRef.current = newState
        return newState
      })
    },
    []
  )

  const clearColumn = useCallback((column: TNavigationColumn) => {
    itemsRef.current[column].clear()
    setSelectedIndexState((prev) => ({
      ...prev,
      [column]: 0
    }))
  }, [])

  const registerItem = useCallback(
    (column: TNavigationColumn, index: number, ref: RefObject<HTMLElement>, meta?: TItemMeta) => {
      itemsRef.current[column].set(index, { ref, meta })
    },
    []
  )

  const unregisterItem = useCallback((column: TNavigationColumn, index: number) => {
    itemsRef.current[column].delete(index)
  }, [])

  const getItemCount = useCallback((column: TNavigationColumn) => {
    return itemsRef.current[column].size
  }, [])

  const registerLoadMore = useCallback((column: TNavigationColumn, callback: () => void) => {
    loadMoreRef.current[column] = callback
  }, [])

  const unregisterLoadMore = useCallback((column: TNavigationColumn) => {
    loadMoreRef.current[column] = null
  }, [])

  const isItemSelected = useCallback(
    (column: TNavigationColumn, index: number) => {
      return isEnabled && activeColumn === column && selectedIndex[column] === index
    },
    [isEnabled, activeColumn, selectedIndex]
  )

  // ============================================================================
  // Column Navigation
  // ============================================================================

  const getAvailableColumns = useCallback((): TNavigationColumn[] => {
    if (isSmallScreen) {
      // Mobile: sidebar is in a drawer, only one column visible at a time
      if (sidebarDrawerOpen) return [0]
      if (secondaryStackLength > 0) return [2]
      return [1]
    }
    if (enableSingleColumnLayout) {
      // Single column desktop: sidebar is always visible alongside main content
      if (secondaryStackLength > 0) return [0, 2]
      return [0, 1]
    }
    // Two column desktop
    if (secondaryStackLength > 0) return [0, 1, 2]
    return [0, 1]
  }, [isSmallScreen, enableSingleColumnLayout, sidebarDrawerOpen, secondaryStackLength])

  const moveColumn = useCallback(
    (direction: 1 | -1) => {
      const available = getAvailableColumns()
      const currentColumn = activeColumnRef.current
      const currentIdx = available.indexOf(currentColumn)
      if (currentIdx === -1) {
        setActiveColumn(available[0])
        return
      }
      const newIdx = Math.max(0, Math.min(available.length - 1, currentIdx + direction))
      const newColumn = available[newIdx]
      if (newColumn === currentColumn) return // Already at edge

      setActiveColumn(newColumn)

      // Scroll to currently selected item in the new column
      const items = itemsRef.current[newColumn]
      const currentSelectedIdx = selectedIndexRef.current[newColumn]
      const item = items.get(currentSelectedIdx)
      if (item?.ref.current) {
        scrollToCenter(item.ref.current)
      } else if (items.size > 0) {
        // If no item at current index, select the first one
        const firstIndex = Array.from(items.keys()).sort((a, b) => a - b)[0]
        if (firstIndex !== undefined) {
          setSelectedIndex(newColumn, firstIndex)
          const firstItem = items.get(firstIndex)
          if (firstItem?.ref.current) {
            scrollToCenter(firstItem.ref.current)
          }
        }
      }
    },
    [getAvailableColumns, scrollToCenter, setSelectedIndex, setActiveColumn]
  )

  // ============================================================================
  // Item Navigation with Centered Scrolling
  // ============================================================================

  const moveItem = useCallback(
    (direction: 1 | -1) => {
      const currentColumn = activeColumnRef.current
      const items = itemsRef.current[currentColumn]
      if (items.size === 0) return

      const indices = Array.from(items.keys()).sort((a, b) => a - b)
      if (indices.length === 0) return

      const currentSelected = selectedIndexRef.current[currentColumn]
      let currentIdx = indices.indexOf(currentSelected)

      if (currentIdx === -1) {
        // Find nearest valid index
        let nearestIdx = 0
        let minDistance = Infinity
        for (let i = 0; i < indices.length; i++) {
          const distance = Math.abs(indices[i] - currentSelected)
          if (distance < minDistance) {
            minDistance = distance
            nearestIdx = i
          }
        }
        currentIdx = nearestIdx
      }

      // Calculate new index with wrap-around
      let newIdx = currentIdx + direction

      // Wrap around behavior
      if (newIdx < 0) {
        // At top, going up -> wrap to bottom
        newIdx = indices.length - 1
      } else if (newIdx >= indices.length) {
        // At bottom, going down -> trigger load more and wrap to top
        const loadMore = loadMoreRef.current[currentColumn]
        if (loadMore) {
          loadMore()
        }
        newIdx = 0
      }

      const newItemIndex = indices[newIdx]
      if (newItemIndex === undefined) return

      setSelectedIndex(currentColumn, newItemIndex)

      // Scroll to center
      const item = items.get(newItemIndex)
      if (item?.ref.current) {
        scrollToCenter(item.ref.current)
      }
    },
    [setSelectedIndex, scrollToCenter]
  )

  const jumpToTop = useCallback(() => {
      // For primary feed, use column 1; for secondary, use column 2
      // Fall back to current column if it has items
      let targetColumn = activeColumnRef.current

      // Check if current column has items, if not try column 1 (primary)
      if (itemsRef.current[targetColumn].size === 0) {
        if (itemsRef.current[1].size > 0) {
          targetColumn = 1
        } else if (itemsRef.current[2].size > 0) {
          targetColumn = 2
        }
      }

      const items = itemsRef.current[targetColumn]
      if (items.size === 0) return

      const indices = Array.from(items.keys()).sort((a, b) => a - b)
      if (indices.length === 0) return

      const newItemIndex = indices[0]
      if (newItemIndex === undefined) return

      // Set active column and selection immediately
      setActiveColumn(targetColumn)
      setSelectedIndex(targetColumn, newItemIndex)

      // Scroll to top of the page/container
      window.scrollTo({ top: 0, behavior: 'smooth' })

      // Also scroll any parent containers to top
      const item = items.get(newItemIndex)
      if (item?.ref.current) {
        let scrollContainer: HTMLElement | null = item.ref.current.parentElement
        while (scrollContainer && scrollContainer !== document.body) {
          const style = window.getComputedStyle(scrollContainer)
          if ((style.overflowY === 'auto' || style.overflowY === 'scroll') &&
              scrollContainer.scrollHeight > scrollContainer.clientHeight) {
            scrollContainer.scrollTo({ top: 0, behavior: 'smooth' })
            break
          }
          scrollContainer = scrollContainer.parentElement
        }
      }
    },
    [setSelectedIndex, setActiveColumn]
  )

  // ============================================================================
  // Action Mode
  // ============================================================================

  const enterActionMode = useCallback((noteEvent: Event) => {
    setActionMode({
      active: true,
      selectedAction: 'reply',
      noteEvent
    })
  }, [])

  const exitActionMode = useCallback(() => {
    setActionMode({
      active: false,
      selectedAction: null,
      noteEvent: null
    })
  }, [])

  const cycleAction = useCallback(
    (direction: 1 | -1 = 1) => {
      setActionMode((prev) => {
        if (!prev.active) {
          const currentColumn = activeColumnRef.current
          const currentSelected = selectedIndexRef.current[currentColumn]
          const item = itemsRef.current[currentColumn].get(currentSelected)
          if (item?.meta?.type === 'note' && item.meta.event) {
            return {
              active: true,
              selectedAction: 'reply',
              noteEvent: item.meta.event
            }
          }
          return prev
        }

        const currentIdx = prev.selectedAction ? ACTIONS.indexOf(prev.selectedAction) : 0
        const newIdx = (currentIdx + direction + ACTIONS.length) % ACTIONS.length
        return {
          ...prev,
          selectedAction: ACTIONS[newIdx]
        }
      })
    },
    []
  )

  // ============================================================================
  // Intent Handlers
  // ============================================================================

  const handleEnter = useCallback(() => {
    const currentColumn = activeColumnRef.current
    const currentSelected = selectedIndexRef.current[currentColumn]

    if (actionMode.active) {
      const item = itemsRef.current[currentColumn].get(currentSelected)
      if (item?.ref.current && actionMode.selectedAction) {
        const stuffStats = item.ref.current.querySelector('[data-stuff-stats]')
        const actionButton = stuffStats?.querySelector(
          `[data-action="${actionMode.selectedAction}"]`
        ) as HTMLButtonElement | null
        actionButton?.click()
        exitActionMode()
      }
      return
    }

    const item = itemsRef.current[currentColumn].get(currentSelected)
    if (!item) return

    if (currentColumn === 0 && item.meta?.type === 'sidebar') {
      setSelectedIndex(1, 0)
      setActiveColumn(1)
    }

    if (item.meta?.onActivate) {
      item.meta.onActivate()
    } else if (item.ref.current) {
      item.ref.current.click()
    }
  }, [actionMode, exitActionMode, setSelectedIndex, setActiveColumn])

  const handleEscape = useCallback(() => {
    // Track Escape press for triple-Escape detection
    const now = Date.now()
    escapeTimestampsRef.current.push(now)
    // Keep only presses within the time window
    escapeTimestampsRef.current = escapeTimestampsRef.current.filter(
      (t) => now - t < TRIPLE_ESCAPE_WINDOW
    )
    // Check for triple-Escape to disable keyboard mode
    if (escapeTimestampsRef.current.length >= 3 && isEnabled) {
      setIsEnabled(false)
      escapeTimestampsRef.current = []
      return
    }

    if (actionMode.active) {
      exitActionMode()
      return
    }

    if (openAccordionItem) {
      setOpenAccordionItem(null)
      return
    }

    if ((isSmallScreen || enableSingleColumnLayout) && secondaryStackLength > 0) {
      onBack?.()
      return
    }

    const currentColumn = activeColumnRef.current
    if (currentColumn === 2 && secondaryStackLength > 0) {
      onCloseSecondary?.()
      setActiveColumn(1)
      return
    }

    if (currentColumn !== 0) {
      setActiveColumn(0)
      setSelectedIndex(0, 0)
    }
  }, [
    actionMode.active,
    exitActionMode,
    openAccordionItem,
    isSmallScreen,
    enableSingleColumnLayout,
    secondaryStackLength,
    onBack,
    onCloseSecondary,
    setSelectedIndex,
    setActiveColumn,
    isEnabled,
    TRIPLE_ESCAPE_WINDOW
  ])

  // Handle back action - move left through columns or close secondary panel
  const handleBack = useCallback(() => {
    const currentColumn = activeColumnRef.current

    // If focused on secondary column (2), close it and move to primary
    if (currentColumn === 2) {
      if (secondaryStackLength > 0) {
        if (isSmallScreen || enableSingleColumnLayout) {
          // On mobile/single column, use onBack to pop the stack
          onBack?.()
        } else {
          // On desktop with columns, close secondary and focus primary
          onCloseSecondary?.()
          setActiveColumn(1)
        }
      } else {
        // No secondary stack, just move to primary
        setActiveColumn(1)
      }
      return
    }

    // If focused on primary column (1), move to sidebar
    if (currentColumn === 1) {
      setActiveColumn(0)
      return
    }

    // If focused on sidebar (0), do nothing (already at leftmost)
  }, [secondaryStackLength, isSmallScreen, enableSingleColumnLayout, onBack, onCloseSecondary, setActiveColumn])

  // Default intent handler (fallback when no region handles)
  const handleDefaultIntent = useCallback(
    (intent: NavigationIntent) => {
      switch (intent) {
        case 'up':
          moveItem(-1)
          break
        case 'down':
          moveItem(1)
          break
        case 'left':
          moveColumn(-1)
          break
        case 'right':
          moveColumn(1)
          break
        case 'pageUp':
          jumpToTop()
          break
        case 'activate':
          handleEnter()
          break
        case 'back':
          handleBack()
          break
        case 'cancel':
          handleEscape()
          break
        case 'nextAction':
          cycleAction(1)
          break
        case 'prevAction':
          cycleAction(-1)
          break
      }
    },
    [moveItem, moveColumn, jumpToTop, handleEnter, handleBack, handleEscape, cycleAction]
  )

  // Keep the ref updated with the latest handleDefaultIntent
  useEffect(() => {
    handleDefaultIntentRef.current = handleDefaultIntent
  }, [handleDefaultIntent])

  // ============================================================================
  // Keyboard Event Handler
  // ============================================================================

  // Helper to trigger an action on the currently selected note
  const triggerNoteAction = useCallback((action: TActionType) => {
    const currentColumn = activeColumnRef.current
    const currentSelected = selectedIndexRef.current[currentColumn]
    const item = itemsRef.current[currentColumn].get(currentSelected)
    if (item?.meta?.type === 'note' && item.ref.current) {
      const stuffStats = item.ref.current.querySelector('[data-stuff-stats]')
      const actionButton = stuffStats?.querySelector(`[data-action="${action}"]`) as HTMLButtonElement | null
      actionButton?.click()
    }
  }, [])

  // Main keyboard handler - translates keys to intents
  // Also handles enabling keyboard nav on first navigation key press
  useEffect(() => {
    if (isTouchDevice()) return

    const handleKeyDown = (e: KeyboardEvent) => {
      if (isInputFocused()) return
      if (modalManager.hasOpenModal?.()) return

      // Map keys to intents
      let intent: NavigationIntent | null = null
      const isNavKey = ['ArrowUp', 'ArrowDown', 'j', 'k', 'Tab'].includes(e.key)

      switch (e.key) {
        case 'ArrowUp':
        case 'k': // Vim-style
          intent = 'up'
          break
        case 'ArrowDown':
        case 'j': // Vim-style
          intent = 'down'
          break
        case 'ArrowLeft':
        case 'h': // Vim-style
          intent = 'back'
          break
        case 'ArrowRight':
        case 'l': // Vim-style
          intent = 'right'
          break
        case 'Enter':
          intent = 'activate'
          break
        case 'PageUp':
          intent = 'pageUp'
          break
        case 'Escape':
          intent = 'cancel'
          break
        case 'Backspace':
          intent = 'back'
          break
        case 'Tab':
          // Tab switches between columns
          e.preventDefault()
          intent = e.shiftKey ? 'left' : 'right'
          break
        // Direct note actions
        case 'r':
          if (isEnabled) {
            e.preventDefault()
            triggerNoteAction('reply')
            return
          }
          break
        case 'R':
          if (isEnabled) {
            e.preventDefault()
            triggerNoteAction('react')
            return
          }
          break
        case 'p':
          if (isEnabled) {
            e.preventDefault()
            triggerNoteAction('repost')
            return
          }
          break
        case 'q':
          if (isEnabled) {
            e.preventDefault()
            triggerNoteAction('quote')
            return
          }
          break
        case 'z':
          if (isEnabled) {
            e.preventDefault()
            triggerNoteAction('zap')
            return
          }
          break
        case 'K':
          // Shift+K toggles keyboard mode
          if (e.shiftKey) {
            e.preventDefault()
            toggleKeyboardMode()
            return
          }
          break
      }

      // Enable keyboard nav on first navigation key press
      if (!isEnabled && isNavKey) {
        setIsEnabled(true)

        // Initialize selection to first item in active column
        const available = getAvailableColumns()
        const currentColumn = activeColumnRef.current
        const column = available.includes(currentColumn) ? currentColumn : available[0]
        const items = itemsRef.current[column]
        if (items.size > 0) {
          const firstIndex = Array.from(items.keys()).sort((a, b) => a - b)[0]
          if (firstIndex !== undefined) {
            setSelectedIndex(column, firstIndex)
            const item = items.get(firstIndex)
            if (item?.ref.current) {
              scrollToCenter(item.ref.current)
            }
          }
        }
      }

      if (intent && isEnabled) {
        e.preventDefault()
        emitIntent(intent)
      } else if (intent && isNavKey) {
        // First keypress enables and processes the intent
        e.preventDefault()
        emitIntent(intent)
      }
    }

    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [isEnabled, emitIntent, getAvailableColumns, setSelectedIndex, scrollToCenter, triggerNoteAction, toggleKeyboardMode])

  // ============================================================================
  // Layout Effects
  // ============================================================================

  useEffect(() => {
    const available = getAvailableColumns()
    if (!available.includes(activeColumn)) {
      // When current column becomes unavailable, find the best fallback:
      // - If coming from column 2 (secondary), prefer column 1 (primary) over column 0 (sidebar)
      // - Otherwise use the first available column
      if (activeColumn === 2 && available.includes(1)) {
        setActiveColumn(1)
      } else {
        setActiveColumn(available[0])
      }
    }
  }, [getAvailableColumns, activeColumn, setActiveColumn])

  // Track secondary panel changes to switch focus
  const prevSecondaryStackLength = useRef(secondaryStackLength)
  useEffect(() => {
    if (secondaryStackLength > prevSecondaryStackLength.current && isEnabled) {
      // Secondary opened - switch to column 2 immediately
      // This ensures the user can navigate back with left/Escape even if
      // the secondary panel doesn't have keyboard-navigable items
      setActiveColumn(2)
      setSelectedIndex(2, 0)

      // If there are items in column 2, scroll to the first one
      const items = itemsRef.current[2]
      if (items.size > 0) {
        const indices = Array.from(items.keys()).sort((a, b) => a - b)
        const firstIndex = indices[0]
        if (firstIndex !== undefined) {
          setSelectedIndex(2, firstIndex)
          const item = items.get(firstIndex)
          if (item?.ref.current) {
            scrollToCenter(item.ref.current)
          }
        }
      }
    } else if (secondaryStackLength < prevSecondaryStackLength.current && isEnabled) {
      // Secondary closed - return to primary only if we were in the secondary column
      // Don't move focus if user is already in sidebar or primary
      if (activeColumnRef.current === 2) {
        setActiveColumn(1)
      }
    }
    prevSecondaryStackLength.current = secondaryStackLength
  }, [secondaryStackLength, isEnabled, setSelectedIndex, scrollToCenter, setActiveColumn])

  // ============================================================================
  // Context Value
  // ============================================================================

  const value = useMemo(
    () => ({
      // Intent system
      emitIntent,
      registerRegion,
      unregisterRegion,

      // Legacy
      activeColumn,
      setActiveColumn,
      selectedIndex,
      setSelectedIndex,
      resetPrimarySelection,
      offsetSelection,
      clearColumn,
      registerItem,
      unregisterItem,
      getItemCount,
      registerLoadMore,
      unregisterLoadMore,
      actionMode,
      enterActionMode,
      exitActionMode,
      cycleAction,
      isItemSelected,
      openAccordionItem,
      setOpenAccordionItem,
      isEnabled,
      toggleKeyboardMode,
      scrollToCenter
    }),
    [
      emitIntent,
      registerRegion,
      unregisterRegion,
      activeColumn,
      selectedIndex,
      setSelectedIndex,
      resetPrimarySelection,
      offsetSelection,
      clearColumn,
      registerItem,
      unregisterItem,
      getItemCount,
      registerLoadMore,
      unregisterLoadMore,
      actionMode,
      enterActionMode,
      exitActionMode,
      cycleAction,
      isItemSelected,
      openAccordionItem,
      isEnabled,
      toggleKeyboardMode,
      scrollToCenter
    ]
  )

  return (
    <KeyboardNavigationContext.Provider value={value}>
      {children}
    </KeyboardNavigationContext.Provider>
  )
}
