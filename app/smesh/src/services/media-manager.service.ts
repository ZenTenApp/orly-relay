import { atom, getDefaultStore } from 'jotai'

export const hasBackgroundAudioAtom = atom(false)
const store = getDefaultStore()

/**
 * MediaManagerService coordinates HTML5 media playback across the app.
 *
 * Since we use privacy-preserving thumbnails instead of embedded players
 * for YouTube and X/Twitter, this service only handles native HTML5 media
 * (audio/video elements).
 */
class MediaManagerService extends EventTarget {
  static instance: MediaManagerService

  private currentMedia: HTMLMediaElement | null = null

  constructor() {
    super()
  }

  public static getInstance(): MediaManagerService {
    if (!MediaManagerService.instance) {
      MediaManagerService.instance = new MediaManagerService()
    }
    return MediaManagerService.instance
  }

  pause(media: HTMLMediaElement | null) {
    if (!media) {
      return
    }
    if (isPipElement(media)) {
      return
    }
    if (this.currentMedia === media) {
      this.currentMedia = null
    }
    media.pause()
  }

  autoPlay(media: HTMLMediaElement) {
    if (
      document.pictureInPictureElement &&
      isMediaPlaying(document.pictureInPictureElement as HTMLMediaElement)
    ) {
      return
    }
    if (
      store.get(hasBackgroundAudioAtom) &&
      this.currentMedia &&
      isMediaPlaying(this.currentMedia)
    ) {
      return
    }
    this.play(media)
  }

  play(media: HTMLMediaElement | null) {
    if (!media) {
      return
    }
    if (document.pictureInPictureElement && document.pictureInPictureElement !== media) {
      ;(document.pictureInPictureElement as HTMLMediaElement).pause()
    }
    if (this.currentMedia && this.currentMedia !== media) {
      this.currentMedia.pause()
    }
    this.currentMedia = media
    if (isMediaPlaying(media)) {
      return
    }

    this.currentMedia.play().catch((error) => {
      console.error('Error playing media:', error)
      this.currentMedia = null
    })
  }

  playAudioBackground(src: string, time: number = 0) {
    this.dispatchEvent(new CustomEvent('playAudioBackground', { detail: { src, time } }))
    store.set(hasBackgroundAudioAtom, true)
  }

  stopAudioBackground() {
    this.dispatchEvent(new Event('stopAudioBackground'))
    store.set(hasBackgroundAudioAtom, false)
  }
}

const instance = MediaManagerService.getInstance()
export default instance

function isMediaPlaying(media: HTMLMediaElement) {
  return media.currentTime > 0 && !media.paused && !media.ended && media.readyState >= 2
}

function isPipElement(media: HTMLMediaElement) {
  if (document.pictureInPictureElement === media) {
    return true
  }
  return (media as any).webkitPresentationMode === 'picture-in-picture'
}
