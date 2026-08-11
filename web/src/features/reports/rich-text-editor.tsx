import { useEditor, EditorContent } from '@tiptap/react'
import StarterKit from '@tiptap/starter-kit'
import LinkExtension from '@tiptap/extension-link'
import {
  Bold,
  Italic,
  Code,
  List,
  ListOrdered,
  Quote,
  Heading1,
  Heading2,
  Heading3,
  Link,
  Undo,
  Redo,
} from 'lucide-react'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { useCallback } from 'react'

/**
 * RichTextEditor is a TipTap-based editor limited to the M6-005 server
 * allowlist: headings 1-3, bold, italic, code, blockquote, bullet/ordered
 * lists, and links (http/https/mailto only). No image, video, or table
 * extensions are loaded.
 *
 * The editor produces HTML suitable for storage as a block parameter. The
 * server sanitizes on write and again at render (defense in depth).
 */

export interface RichTextEditorProps {
  /** Initial HTML content. */
  content?: string
  /** Called on every change with the current HTML. */
  onChange?: (html: string) => void
  /** Placeholder text shown when empty. */
  placeholder?: string
  /** When true, the editor is read-only. */
  readOnly?: boolean
  /** Additional class names for the outer container. */
  className?: string
}

export function RichTextEditor({
  content = '',
  onChange,
  placeholder = 'Start writing\u2026',
  readOnly = false,
  className,
}: RichTextEditorProps) {
  const editor = useEditor({
    extensions: [
      StarterKit.configure({
        // Only enable what the server allowlist permits.
        heading: { levels: [1, 2, 3] },
        codeBlock: false,
        horizontalRule: false,
        strike: false,
      }),
      LinkExtension.configure({
        openOnClick: false,
        autolink: false,
        protocols: ['http', 'https', 'mailto'],
      }),
    ],
    content,
    editable: !readOnly,
    editorProps: {
      attributes: {
        class: cn(
          'prose prose-sm dark:prose-invert max-w-none min-h-[8rem] rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
          readOnly && 'cursor-default',
        ),
      },
    },
    onUpdate: ({ editor: ed }) => {
      onChange?.(ed.getHTML())
    },
  })

  const setLink = useCallback(() => {
    // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
    /* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-argument, @typescript-eslint/prefer-nullish-coalescing, @typescript-eslint/no-unnecessary-condition */
    if (!editor) return
    const previousUrl = editor.getAttributes('link').href
    const url = window.prompt('URL', previousUrl || 'https://')
    if (url === null) return
    if (url === '') {
      editor.chain().focus().extendMarkRange('link').unsetLink().run()
      return
    }
    try {
      const parsed = new URL(url)
      if (!['http:', 'https:', 'mailto:'].includes(parsed.protocol)) {
        window.alert('Only http, https, and mailto links are allowed.')
        return
      }
    } catch {
      window.alert('Please enter a valid URL.')
      return
    }
    editor.chain().focus().extendMarkRange('link').setLink({ href: url }).run()
    /* eslint-enable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/no-unsafe-argument, @typescript-eslint/prefer-nullish-coalescing, @typescript-eslint/no-unnecessary-condition */
  }, [editor])

  // eslint-disable-next-line @typescript-eslint/no-unnecessary-condition
  if (!editor) return null

  return (
    <div className={cn('flex flex-col gap-1', className)}>
      {!readOnly && <Toolbar editor={editor} onSetLink={setLink} />}
      <EditorContent editor={editor} placeholder={placeholder} />
    </div>
  )
}

// ---------------------------------------------------------------------------
// Toolbar
// ---------------------------------------------------------------------------

interface ToolbarProps {
  editor: NonNullable<ReturnType<typeof useEditor>>
  onSetLink: () => void
}
function Toolbar({ editor, onSetLink }: ToolbarProps) {
  return (
    <div className="bg-muted/40 flex flex-wrap items-center gap-0.5 rounded-md border p-1">
      <ToolButton
        title="Undo"
        disabled={!editor.can().undo()}
        onClick={() => editor.chain().focus().undo().run()}
      >
        <Undo className="h-4 w-4" />
      </ToolButton>
      <ToolButton
        title="Redo"
        disabled={!editor.can().redo()}
        onClick={() => editor.chain().focus().redo().run()}
      >
        <Redo className="h-4 w-4" />
      </ToolButton>

      <Sep />

      <ToolButton
        title="Heading 1"
        active={editor.isActive('heading', { level: 1 })}
        onClick={() => editor.chain().focus().toggleHeading({ level: 1 }).run()}
      >
        <Heading1 className="h-4 w-4" />
      </ToolButton>
      <ToolButton
        title="Heading 2"
        active={editor.isActive('heading', { level: 2 })}
        onClick={() => editor.chain().focus().toggleHeading({ level: 2 }).run()}
      >
        <Heading2 className="h-4 w-4" />
      </ToolButton>
      <ToolButton
        title="Heading 3"
        active={editor.isActive('heading', { level: 3 })}
        onClick={() => editor.chain().focus().toggleHeading({ level: 3 }).run()}
      >
        <Heading3 className="h-4 w-4" />
      </ToolButton>

      <Sep />

      <ToolButton
        title="Bold"
        active={editor.isActive('bold')}
        onClick={() => editor.chain().focus().toggleBold().run()}
      >
        <Bold className="h-4 w-4" />
      </ToolButton>
      <ToolButton
        title="Italic"
        active={editor.isActive('italic')}
        onClick={() => editor.chain().focus().toggleItalic().run()}
      >
        <Italic className="h-4 w-4" />
      </ToolButton>
      <ToolButton
        title="Inline code"
        active={editor.isActive('code')}
        onClick={() => editor.chain().focus().toggleCode().run()}
      >
        <Code className="h-4 w-4" />
      </ToolButton>

      <Sep />

      <ToolButton
        title="Bullet list"
        active={editor.isActive('bulletList')}
        onClick={() => editor.chain().focus().toggleBulletList().run()}
      >
        <List className="h-4 w-4" />
      </ToolButton>
      <ToolButton
        title="Numbered list"
        active={editor.isActive('orderedList')}
        onClick={() => editor.chain().focus().toggleOrderedList().run()}
      >
        <ListOrdered className="h-4 w-4" />
      </ToolButton>
      <ToolButton
        title="Blockquote"
        active={editor.isActive('blockquote')}
        onClick={() => editor.chain().focus().toggleBlockquote().run()}
      >
        <Quote className="h-4 w-4" />
      </ToolButton>

      <Sep />

      <ToolButton title="Add link" active={editor.isActive('link')} onClick={onSetLink}>
        <Link className="h-4 w-4" />
      </ToolButton>
    </div>
  )
}

function ToolButton({
  title,
  active,
  disabled,
  onClick,
  children,
}: {
  title: string
  active?: boolean
  disabled?: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <Button
      variant="ghost"
      size="icon"
      title={title}
      disabled={disabled}
      data-state={active ? 'on' : 'off'}
      className="text-muted-foreground hover:bg-muted hover:text-foreground data-[state=on]:bg-accent data-[state=on]:text-accent-foreground flex h-8 w-8 items-center justify-center rounded-md p-0"
      onClick={onClick}
      type="button"
    >
      {children}
    </Button>
  )
}

function Sep() {
  return <div className="bg-border mx-0.5 h-5 w-px" />
}
