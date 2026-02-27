import {
  TItemMeta,
  TNavigationColumn,
  useKeyboardNavigation
} from '@/providers/KeyboardNavigationProvider'
import { useEffect, useRef } from 'react'

export function useKeyboardNavigable(
  column: TNavigationColumn,
  index: number,
  options?: {
    meta?: TItemMeta
  }
) {
  const ref = useRef<HTMLDivElement>(null)
  const { registerItem, unregisterItem, isItemSelected } = useKeyboardNavigation()

  useEffect(() => {
    registerItem(column, index, ref as React.RefObject<HTMLElement>, options?.meta)
    return () => unregisterItem(column, index)
  }, [column, index, registerItem, unregisterItem, options?.meta])

  const isSelected = isItemSelected(column, index)

  return {
    ref,
    isSelected,
    navProps: {
      'data-nav-column': column,
      'data-nav-index': index,
      'data-nav-selected': isSelected || undefined
    }
  }
}
