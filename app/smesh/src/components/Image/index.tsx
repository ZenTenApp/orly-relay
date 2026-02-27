import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import blossomService from '@/services/blossom.service'
import { TImetaInfo } from '@/types'
import { decode } from 'blurhash'
import { ImageOff } from 'lucide-react'
import { HTMLAttributes, useEffect, useMemo, useRef, useState } from 'react'
import { thumbHashToDataURL } from 'thumbhash'

export default function Image({
  image: { url, blurHash, thumbHash, pubkey, dim, variant, sha256 },
  alt,
  className = '',
  classNames = {},
  hideIfError = false,
  errorPlaceholder = <ImageOff />,
  originalUrl,
  ...props
}: HTMLAttributes<HTMLDivElement> & {
  classNames?: {
    wrapper?: string
    errorPlaceholder?: string
    skeleton?: string
  }
  image: TImetaInfo
  alt?: string
  hideIfError?: boolean
  errorPlaceholder?: React.ReactNode
  originalUrl?: string
}) {
  const [isLoading, setIsLoading] = useState(true)
  const [displaySkeleton, setDisplaySkeleton] = useState(true)
  const [hasError, setHasError] = useState(false)
  const [imageUrl, setImageUrl] = useState<string>()
  const [naturalDim, setNaturalDim] = useState<{ width: number; height: number } | null>(null)
  const [showTooltip, setShowTooltip] = useState(false)
  const timeoutRef = useRef<NodeJS.Timeout | null>(null)

  useEffect(() => {
    setIsLoading(true)
    setHasError(false)
    setDisplaySkeleton(true)

    if (pubkey) {
      // BlossomService now actively validates URLs and races mirrors.
      // The promise will resolve with the best available URL.
      blossomService.getValidUrl(url, pubkey).then((validUrl) => {
        setImageUrl(validUrl)
        if (timeoutRef.current) {
          clearTimeout(timeoutRef.current)
          timeoutRef.current = null
        }
      })
      // Fallback timeout in case something goes wrong with the service
      timeoutRef.current = setTimeout(() => {
        if (!imageUrl) {
          setImageUrl(url)
        }
      }, 3000)
    } else {
      setImageUrl(url)
    }

    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current)
        timeoutRef.current = null
      }
    }
  }, [url])

  if (hideIfError && hasError) return null

  const handleError = async () => {
    const nextUrl = await blossomService.tryNextUrl(url)
    if (nextUrl) {
      setImageUrl(nextUrl)
    } else {
      setIsLoading(false)
      setHasError(true)
    }
  }

  const handleLoad = (e: React.SyntheticEvent<HTMLImageElement>) => {
    const img = e.currentTarget
    setNaturalDim({ width: img.naturalWidth, height: img.naturalHeight })
    setIsLoading(false)
    setHasError(false)
    setTimeout(() => setDisplaySkeleton(false), 600)
    blossomService.markAsSuccess(url, imageUrl || url)
  }

  return (
    <div
      className={cn('relative overflow-hidden rounded-xl group/imgdebug', classNames.wrapper)}
      onMouseEnter={() => setShowTooltip(true)}
      onMouseLeave={() => setShowTooltip(false)}
      {...props}
    >
      {/* Debug tooltip overlay */}
      {showTooltip && !isLoading && !hasError && (
        <div className="absolute top-2 left-2 z-50 max-w-[90%] pointer-events-none">
          <div className="bg-black/85 text-white text-xs rounded-lg px-3 py-2 shadow-lg backdrop-blur-sm space-y-1 font-mono">
            {variant && (
              <div className="flex items-center gap-1.5">
                <span className="inline-block bg-blue-500 text-white text-[10px] font-bold px-1.5 py-0.5 rounded uppercase tracking-wide">
                  variant
                </span>
                <span className="text-blue-300">{variant}</span>
              </div>
            )}
            <div className="truncate">
              <span className="text-gray-400">url: </span>
              <span className="text-green-300">{imageUrl || url}</span>
            </div>
            {naturalDim && (
              <div>
                <span className="text-gray-400">rendered: </span>
                <span className="text-yellow-300">{naturalDim.width}x{naturalDim.height}</span>
              </div>
            )}
            {dim && (
              <div>
                <span className="text-gray-400">declared: </span>
                <span className="text-yellow-300">{dim.width}x{dim.height}</span>
              </div>
            )}
            {variant && originalUrl && (
              <div className="truncate">
                <span className="text-gray-400">original: </span>
                <span className="text-orange-300">{originalUrl}</span>
              </div>
            )}
            {sha256 && (
              <div className="truncate">
                <span className="text-gray-400">sha256: </span>
                <span className="text-purple-300">{sha256}</span>
              </div>
            )}
          </div>
        </div>
      )}
      {/* Spacer: transparent image to maintain dimensions when image is loading */}
      {isLoading && dim?.width && dim?.height && (
        <img
          src={`data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='${dim.width}' height='${dim.height}'%3E%3C/svg%3E`}
          className={cn(
            'object-cover transition-opacity pointer-events-none w-full h-full',
            className
          )}
          alt=""
        />
      )}
      {displaySkeleton && (
        <div className="absolute inset-0 z-10">
          {thumbHash ? (
            <ThumbHashPlaceholder
              thumbHash={thumbHash}
              className={cn(
                'w-full h-full transition-opacity',
                isLoading ? 'opacity-100' : 'opacity-0'
              )}
            />
          ) : blurHash ? (
            <BlurHashCanvas
              blurHash={blurHash}
              className={cn(
                'w-full h-full transition-opacity',
                isLoading ? 'opacity-100' : 'opacity-0'
              )}
            />
          ) : (
            <Skeleton
              className={cn(
                'w-full h-full transition-opacity',
                isLoading ? 'opacity-100' : 'opacity-0',
                classNames.skeleton
              )}
            />
          )}
        </div>
      )}
      {!hasError && (
        <img
          src={imageUrl}
          alt={alt}
          decoding="async"
          draggable={false}
          {...props}
          onLoad={handleLoad}
          onError={handleError}
          className={cn(
            'object-cover transition-opacity pointer-events-none w-full h-full',
            isLoading ? 'opacity-0 absolute inset-0' : '',
            className
          )}
        />
      )}
      {hasError &&
        (typeof errorPlaceholder === 'string' ? (
          <img
            src={errorPlaceholder}
            alt={alt}
            decoding="async"
            loading="lazy"
            className={cn('object-cover w-full h-full transition-opacity', className)}
          />
        ) : (
          <div
            className={cn(
              'object-cover flex flex-col items-center justify-center w-full h-full bg-muted',
              className,
              classNames.errorPlaceholder
            )}
          >
            {errorPlaceholder}
          </div>
        ))}
    </div>
  )
}

const blurHashWidth = 32
const blurHashHeight = 32
function BlurHashCanvas({ blurHash, className = '' }: { blurHash: string; className?: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  const pixels = useMemo(() => {
    if (!blurHash) return null
    try {
      return decode(blurHash, blurHashWidth, blurHashHeight)
    } catch (error) {
      console.warn('Failed to decode blurhash:', error)
      return null
    }
  }, [blurHash])

  useEffect(() => {
    if (!pixels || !canvasRef.current) return

    const canvas = canvasRef.current
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    const imageData = ctx.createImageData(blurHashWidth, blurHashHeight)
    imageData.data.set(pixels)
    ctx.putImageData(imageData, 0, 0)
  }, [pixels])

  if (!blurHash) return null

  return (
    <canvas
      ref={canvasRef}
      width={blurHashWidth}
      height={blurHashHeight}
      className={cn('w-full h-full object-cover rounded-xl', className)}
      style={{
        imageRendering: 'auto',
        filter: 'blur(0.5px)'
      }}
    />
  )
}

function ThumbHashPlaceholder({
  thumbHash,
  className = ''
}: {
  thumbHash: Uint8Array
  className?: string
}) {
  const dataUrl = useMemo(() => {
    if (!thumbHash) return null
    try {
      return thumbHashToDataURL(thumbHash)
    } catch (error) {
      console.warn('failed to decode thumbhash:', error)
      return null
    }
  }, [thumbHash])

  if (!dataUrl) return null

  return (
    <div
      className={cn('w-full h-full object-cover rounded-lg', className)}
      style={{
        backgroundImage: `url(${dataUrl})`,
        backgroundSize: 'cover',
        backgroundPosition: 'center',
        filter: 'blur(1px)'
      }}
    />
  )
}
