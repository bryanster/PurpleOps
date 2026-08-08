import { clsx, type ClassValue } from 'clsx'
import { twMerge } from 'tailwind-merge'

/**
 * Merge class names, letting later Tailwind utilities win over earlier ones.
 *
 * Every shadcn/ui component imports this; it is their convention, not ours, and
 * changing its name or signature means editing every vendored component.
 */
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs))
}
