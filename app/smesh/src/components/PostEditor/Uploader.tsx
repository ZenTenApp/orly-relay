import { isSupportedImage } from '@/lib/image-scaler'
import { UploadedVariant } from '@/lib/responsive-image-event'
import mediaUpload, { UPLOAD_ABORTED_ERROR_MSG } from '@/services/media-upload.service'
import { VerifiedEvent } from 'nostr-tools'
import { useRef } from 'react'
import { toast } from 'sonner'

type UploaderProps = {
  children: React.ReactNode
  onUploadStart?: (file: File, cancel: () => void) => void
  onUploadEnd?: (file: File) => void
  onProgress?: (file: File, progress: number) => void
  className?: string
  accept?: string
} & (
  | {
      /** Standard upload mode - returns URL and NIP-94 tags */
      responsive?: false
      onUploadSuccess: ({ url, tags }: { url: string; tags: string[][] }) => void
    }
  | {
      /** Responsive upload mode - generates multiple variants and publishes kind 1063 event */
      responsive: true
      onUploadSuccess: ({
        event,
        variants,
        primaryUrl
      }: {
        event: VerifiedEvent
        variants: UploadedVariant[]
        primaryUrl: string
      }) => void
      /** Description/caption for the image binding event */
      description?: string
      /** Alt text for accessibility */
      alt?: string
    }
)

export default function Uploader(props: UploaderProps) {
  const {
    children,
    onUploadStart,
    onUploadEnd,
    onProgress,
    className,
    accept = 'image/*'
  } = props
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleFileChange = async (event: React.ChangeEvent<HTMLInputElement>) => {
    if (!event.target.files) return

    const abortControllerMap = new Map<File, AbortController>()

    for (const file of event.target.files) {
      const abortController = new AbortController()
      abortControllerMap.set(file, abortController)
      onUploadStart?.(file, () => abortController.abort())
    }

    for (const file of event.target.files) {
      try {
        const abortController = abortControllerMap.get(file)

        if (props.responsive && isSupportedImage(file)) {
          // Responsive upload mode - generate variants and publish binding event
          const result = await mediaUpload.uploadResponsiveImage(file, {
            onProgress: (p) => onProgress?.(file, p),
            signal: abortController?.signal,
            description: props.description,
            alt: props.alt
          })

          // Find the best URL to insert (prefer desktop-sm or original)
          const primaryVariant =
            result.variants.find((v) => v.variant === 'desktop-sm') ||
            result.variants.find((v) => v.variant === 'original') ||
            result.variants[result.variants.length - 1]

          props.onUploadSuccess({
            event: result.event,
            variants: result.variants,
            primaryUrl: primaryVariant?.url ?? result.variants[0]?.url
          })
        } else {
          // Standard upload mode (or fallback for non-image files in responsive mode)
          const result = await mediaUpload.upload(file, {
            onProgress: (p) => onProgress?.(file, p),
            signal: abortController?.signal
          })
          if (props.responsive) {
            // In responsive mode but file is not a supported image - return as single "variant"
            props.onUploadSuccess({
              event: null as unknown as VerifiedEvent, // No binding event for non-images
              variants: [],
              primaryUrl: result.url
            })
          } else {
            props.onUploadSuccess(result)
          }
        }

        onUploadEnd?.(file)
      } catch (error) {
        console.error('Error uploading file', error)
        const message = (error as Error).message
        if (message !== UPLOAD_ABORTED_ERROR_MSG) {
          toast.error(`Failed to upload file: ${message}`)
        }
        if (fileInputRef.current) {
          fileInputRef.current.value = ''
        }
        onUploadEnd?.(file)
      }
    }
  }

  const handleUploadClick = () => {
    if (fileInputRef.current) {
      fileInputRef.current.value = '' // clear the value so that the same file can be uploaded again
      fileInputRef.current.click()
    }
  }

  return (
    <div className={className}>
      <div onClick={handleUploadClick}>{children}</div>
      <input
        type="file"
        ref={fileInputRef}
        style={{ display: 'none' }}
        onChange={handleFileChange}
        accept={accept}
        multiple
      />
    </div>
  )
}
