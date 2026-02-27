import { ExternalLink } from 'lucide-react'
import { memo } from 'react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import remarkBreaks from 'remark-breaks'

const MarkdownText = memo(function MarkdownText({ text }: { text: string }) {
  return (
    <Markdown
      remarkPlugins={[remarkGfm, remarkBreaks]}
      components={{
        a: ({ href, children, ...props }) => (
          <a
            {...props}
            href={href}
            target="_blank"
            rel="noreferrer noopener"
            className="break-words inline-flex items-baseline gap-1 underline text-foreground"
          >
            {children} <ExternalLink className="size-3" />
          </a>
        ),
        p: ({ children, ...props }) => <span {...props} className="break-words">{children}</span>,
        code: ({ className, children, ...props }) => {
          const isBlock = className?.startsWith('language-')
          if (isBlock) {
            return (
              <code {...props} className={`${className ?? ''} break-words whitespace-pre-wrap`}>
                {children}
              </code>
            )
          }
          return (
            <code {...props} className="bg-muted px-1 py-0.5 rounded text-sm break-words">
              {children}
            </code>
          )
        },
        pre: (props) => (
          <pre
            {...props}
            className="bg-muted rounded-md p-3 overflow-x-auto my-2 text-sm whitespace-pre-wrap"
          />
        ),
        img: (props) => (
          <img
            {...props}
            className="max-w-full max-h-[50vh] object-contain rounded-md my-2"
            loading="lazy"
          />
        )
      }}
    >
      {text}
    </Markdown>
  )
})

export default MarkdownText
