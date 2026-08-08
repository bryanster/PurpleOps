import { CheckIcon, CopyIcon, DownloadIcon } from 'lucide-react'
import { type ReactNode, useId, useState } from 'react'

import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'

import type { RecoveryCodes } from './queries'

/**
 * The one time recovery codes exist outside the server (M1-007).
 *
 * There is no endpoint that reads them back — only their hashes are stored — so
 * this component is the last chance anybody has to keep them. That shapes every
 * decision in it:
 *
 * - The codes are text on the page, selectable, in a monospace face, grouped as
 *   the server grouped them. Nothing is behind a "reveal" control.
 * - Copy and download are both offered, because one of them is always the wrong
 *   one: a copied block is lost to the next copy, and a downloaded file is
 *   awkward on a shared machine.
 * - Continuing takes a deliberate act — the checkbox — rather than a button
 *   that is easy to hit on the way past. It is not a legal gesture; it is the
 *   half-second in which somebody notices they have not saved them.
 *
 * The checkbox is not a security control and does not pretend to be: the codes
 * are minted whether or not it is ticked. What it buys is that the screen
 * cannot be dismissed by reflex.
 */
export function RecoveryCodesPanel({
  codes,
  onContinue,
  continueLabel = 'Continue',
  heading = 'Save your recovery codes',
}: {
  codes: RecoveryCodes
  onContinue: () => void
  continueLabel?: string
  heading?: string
}): ReactNode {
  const [saved, setSaved] = useState(false)
  const [copied, setCopied] = useState(false)
  const checkboxId = useId()

  const asText = codes.codes.join('\n')

  async function copy(): Promise<void> {
    try {
      await navigator.clipboard.writeText(asText)
      setCopied(true)
    } catch {
      // A clipboard write can be refused — an insecure origin, a browser
      // permission — and silently doing nothing would be worse than saying so.
      // The codes are on the page either way, which is why this is not fatal.
      setCopied(false)
    }
  }

  function download(): void {
    const blob = new Blob([`${asText}\n`], { type: 'text/plain' })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = 'blacklight-recovery-codes.txt'
    anchor.click()
    URL.revokeObjectURL(url)
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-1">
        <h2 className="font-semibold">{heading}</h2>
        <p className="text-muted-foreground text-sm">
          Each code signs you in once if you lose your authenticator. This is the only time they are
          shown — nobody, including an administrator, can read them back.
        </p>
      </div>

      <ul
        aria-label="Recovery codes"
        className="bg-muted/50 grid grid-cols-1 gap-1 rounded-md border p-4 font-mono text-sm select-all sm:grid-cols-2"
      >
        {codes.codes.map((code) => (
          <li key={code}>{code}</li>
        ))}
      </ul>

      <div className="flex flex-wrap gap-2">
        <Button
          type="button"
          variant="outline"
          size="sm"
          onClick={() => {
            void copy()
          }}
        >
          {copied ? <CheckIcon aria-hidden="true" /> : <CopyIcon aria-hidden="true" />}
          {copied ? 'Copied' : 'Copy'}
        </Button>
        <Button type="button" variant="outline" size="sm" onClick={download}>
          <DownloadIcon aria-hidden="true" />
          Download
        </Button>
      </div>

      <div className="flex items-start gap-2">
        <Checkbox
          id={checkboxId}
          checked={saved}
          onCheckedChange={(checked) => {
            setSaved(checked === true)
          }}
        />
        <Label htmlFor={checkboxId} className="text-sm leading-snug font-normal">
          I have saved these codes somewhere I can reach without this device.
        </Label>
      </div>

      <Button type="button" onClick={onContinue} disabled={!saved}>
        {continueLabel}
      </Button>
    </div>
  )
}
