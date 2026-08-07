import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import { BrowserRouter } from 'react-router'

import { QueryProvider } from '@/api/query-provider'
import { ErrorBoundary } from '@/app/error/error-boundary'
import { AppRoutes } from '@/app/routes/app-routes'
import { applyTheme, readStoredPreference, resolveTheme } from '@/app/theme/theme'
import { TooltipProvider } from '@/components/ui/tooltip'
import { ThemeProvider } from '@/app/theme/theme-provider'

import './index.css'

// Before the first render, not inside a component: this is the earliest a
// script may run under `script-src 'self'` (see index.html). Between first
// paint and this line the CSS media query in index.css is what decides, so only
// a user whose stored preference contradicts their OS can see a change here.
applyTheme(resolveTheme(readStoredPreference()))

const container = document.getElementById('root')
if (!container) {
  throw new Error('index.html is missing #root')
}

createRoot(container).render(
  <StrictMode>
    <TooltipProvider>
      {/* Outside the router and the shell, so that a failure in either of those
          still renders something rather than a blank document. */}
      <ErrorBoundary>
        <ThemeProvider>
          {/* Above the router and the shell: the query cache outlives every
              navigation, and a screen unmounting mid-request does not take it
              with it. */}
          <QueryProvider>
            <BrowserRouter>
              <AppRoutes />
            </BrowserRouter>
          </QueryProvider>
        </ThemeProvider>
      </ErrorBoundary>
    </TooltipProvider>
  </StrictMode>,
)
