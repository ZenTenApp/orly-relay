import { describe, it, expect } from 'vitest'
import { MediaAttachment } from './MediaAttachment'

describe('MediaAttachment', () => {
  describe('fromUrl', () => {
    it('detects image from URL extension', () => {
      const attachment = MediaAttachment.fromUrl('https://example.com/photo.jpg')

      expect(attachment.type).toBe('image')
      expect(attachment.url).toBe('https://example.com/photo.jpg')
      expect(attachment.status).toBe('completed')
      expect(attachment.isImage).toBe(true)
    })

    it('detects video from URL extension', () => {
      const attachment = MediaAttachment.fromUrl('https://example.com/video.mp4')

      expect(attachment.type).toBe('video')
      expect(attachment.isVideo).toBe(true)
    })

    it('detects audio from URL extension', () => {
      const attachment = MediaAttachment.fromUrl('https://example.com/audio.mp3')

      expect(attachment.type).toBe('audio')
      expect(attachment.isAudio).toBe(true)
    })

    it('defaults to file for unknown extensions', () => {
      const attachment = MediaAttachment.fromUrl('https://example.com/document.pdf')

      expect(attachment.type).toBe('file')
    })

    it('uses mime type over URL extension', () => {
      const attachment = MediaAttachment.fromUrl(
        'https://example.com/media',
        'video/mp4'
      )

      expect(attachment.type).toBe('video')
    })

    it('handles URLs with query parameters', () => {
      const attachment = MediaAttachment.fromUrl(
        'https://example.com/photo.png?size=large'
      )

      expect(attachment.type).toBe('image')
    })

    it('detects various image formats', () => {
      const formats = ['jpg', 'jpeg', 'png', 'gif', 'webp', 'heic', 'avif', 'svg']

      for (const format of formats) {
        const attachment = MediaAttachment.fromUrl(`https://example.com/img.${format}`)
        expect(attachment.type).toBe('image')
      }
    })

    it('detects various video formats', () => {
      const formats = ['mp4', 'webm', 'mov', 'avi', 'mkv']

      for (const format of formats) {
        const attachment = MediaAttachment.fromUrl(`https://example.com/vid.${format}`)
        expect(attachment.type).toBe('video')
      }
    })

    it('detects various audio formats', () => {
      const formats = ['mp3', 'wav', 'ogg', 'flac', 'm4a']

      for (const format of formats) {
        const attachment = MediaAttachment.fromUrl(`https://example.com/aud.${format}`)
        expect(attachment.type).toBe('audio')
      }
    })
  })

  describe('fromMetadata', () => {
    it('creates attachment with full metadata', () => {
      const metadata = {
        url: 'https://example.com/photo.jpg',
        mimeType: 'image/jpeg',
        width: 1920,
        height: 1080,
        size: 102400,
        blurhash: 'LEHV6nWB2yk8pyo0adR*.7kCMdnj',
        sha256: 'd'.repeat(64),
        alt: 'A beautiful sunset'
      }

      const attachment = MediaAttachment.fromMetadata(metadata)

      expect(attachment.url).toBe(metadata.url)
      expect(attachment.mimeType).toBe('image/jpeg')
      expect(attachment.metadata?.width).toBe(1920)
      expect(attachment.metadata?.height).toBe(1080)
      expect(attachment.alt).toBe('A beautiful sunset')
    })
  })

  describe('pending', () => {
    it('creates pending attachment', () => {
      const attachment = MediaAttachment.pending('photo.jpg', 'image')

      expect(attachment.url).toBe('')
      expect(attachment.type).toBe('image')
      expect(attachment.status).toBe('pending')
      expect(attachment.isUploaded).toBe(false)
    })
  })

  describe('toImetaTag', () => {
    it('generates imeta tag for images', () => {
      const attachment = MediaAttachment.fromMetadata({
        url: 'https://example.com/photo.jpg',
        mimeType: 'image/jpeg',
        width: 800,
        height: 600,
        blurhash: 'LGF5?xYk^6#M@-5c,1J5@[or[Q6.'
      })

      const tag = attachment.toImetaTag()

      expect(tag).not.toBeNull()
      expect(tag![0]).toBe('imeta')
      expect(tag).toContain('url https://example.com/photo.jpg')
      expect(tag).toContain('m image/jpeg')
      expect(tag).toContain('dim 800x600')
      expect(tag).toContain('blurhash LGF5?xYk^6#M@-5c,1J5@[or[Q6.')
    })

    it('returns null for non-images', () => {
      const attachment = MediaAttachment.fromUrl('https://example.com/video.mp4')

      const tag = attachment.toImetaTag()

      expect(tag).toBeNull()
    })

    it('includes alt text in tag', () => {
      const attachment = MediaAttachment.fromUrl('https://example.com/photo.jpg').withAlt(
        'Description'
      )

      const tag = attachment.toImetaTag()

      expect(tag).toContain('alt Description')
    })
  })

  describe('immutable modifications', () => {
    it('withAlt returns new instance', () => {
      const original = MediaAttachment.fromUrl('https://example.com/photo.jpg')
      const modified = original.withAlt('New alt text')

      expect(original.alt).toBeNull()
      expect(modified.alt).toBe('New alt text')
    })

    it('withStatus returns new instance', () => {
      const original = MediaAttachment.pending('file.jpg', 'image')
      const modified = original.withStatus('uploading')

      expect(original.status).toBe('pending')
      expect(modified.status).toBe('uploading')
    })

    it('withUrl returns new instance with completed status', () => {
      const original = MediaAttachment.pending('file.jpg', 'image')
      const modified = original.withUrl('https://example.com/uploaded.jpg')

      expect(original.url).toBe('')
      expect(original.status).toBe('pending')
      expect(modified.url).toBe('https://example.com/uploaded.jpg')
      expect(modified.status).toBe('completed')
      expect(modified.isUploaded).toBe(true)
    })
  })

  describe('equals', () => {
    it('returns true for same URL', () => {
      const a = MediaAttachment.fromUrl('https://example.com/photo.jpg')
      const b = MediaAttachment.fromUrl('https://example.com/photo.jpg')

      expect(a.equals(b)).toBe(true)
    })

    it('returns false for different URLs', () => {
      const a = MediaAttachment.fromUrl('https://example.com/photo1.jpg')
      const b = MediaAttachment.fromUrl('https://example.com/photo2.jpg')

      expect(a.equals(b)).toBe(false)
    })
  })
})
