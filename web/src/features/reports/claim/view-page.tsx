import { Button } from '@/components/ui/button'
import { PageError, PageLoading } from '@/app/shell/page-state'
import { API_BASE_URL } from '@/api/client'
import { LOGIN_PATH } from '@/features/auth/paths'
import { DownloadIcon } from 'lucide-react'
import { type ReactNode } from 'react'
import { Navigate, useParams } from 'react-router'

import { useShareHtml } from '../queries'

/**
 * Shared report HTML view — minimal chrome (M6-014).
 *
 * No SPA nav, no engagement sidebar. Just the report content with a firm
 * logo, title, and download PDF button if allowed.
 */
export function ViewPage(): ReactNode {
  const { token } = useParams<{ token: string }>()
  const html = useShareHtml(token)

  if (!token) return <Navigate to={LOGIN_PATH} replace />

  if (html.isPending) return <PageLoading label="Loading report…" />
  if (html.error) {
    return (
      <PageError
        error={html.error}
        onRetry={() => {
          void html.refetch()
        }}
      />
    )
  }

  const pdfUrl = `${API_BASE_URL}/report-views/${token}/pdf`

  return (
    <div className="flex min-h-screen flex-col">
      {/* Minimal chrome — no SPA nav */}
      <header className="flex items-center justify-between border-b px-6 py-3">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium">Blacklight</span>
        </div>
        <div className="flex items-center gap-2">
          <Button asChild size="sm" variant="outline">
            <a href={pdfUrl}>
              <DownloadIcon className="size-3" />
              Download PDF
            </a>
          </Button>
        </div>
      </header>

      <main className="flex-1">
        {html.data ? (
          <iframe
            className="h-full w-full border-0"
            srcDoc={html.data}
            title="Shared report"
            sandbox="allow-same-origin"
          />
        ) : (
          <div className="flex items-center justify-center p-12">
            <p className="text-sm text-muted-foreground">No content available.</p>
          </div>
        )}
      </main>
    </div>
  )
}
