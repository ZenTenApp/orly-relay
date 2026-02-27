import { RECOMMENDED_BLOSSOM_SERVERS } from '@/constants'
import { generateImageVariants, isSupportedImage, getExtensionFromMimeType } from '@/lib/image-scaler'
import {
  createResponsiveImageEvent,
  UploadedVariant
} from '@/lib/responsive-image-event'
import { simplifyUrl } from '@/lib/url'
import { TDraftEvent, TMediaUploadServiceConfig } from '@/types'
import { BlossomClient } from 'blossom-client-sdk'
import { VerifiedEvent } from 'nostr-tools'
import { z } from 'zod'
import client from './client.service'
import storage from './local-storage.service'

type UploadOptions = {
  onProgress?: (progressPercent: number) => void
  signal?: AbortSignal
}

type ResponsiveUploadOptions = UploadOptions & {
  /** Description/caption for the image */
  description?: string
  /** Alt text for accessibility */
  alt?: string
}

export const UPLOAD_ABORTED_ERROR_MSG = 'Upload aborted'

class MediaUploadService {
  static instance: MediaUploadService

  private serviceConfig: TMediaUploadServiceConfig = storage.getMediaUploadServiceConfig()
  private nip96ServiceUploadUrlMap = new Map<string, string | undefined>()
  private imetaTagMap = new Map<string, string[]>()

  constructor() {
    if (!MediaUploadService.instance) {
      MediaUploadService.instance = this
    }
    return MediaUploadService.instance
  }

  setServiceConfig(config: TMediaUploadServiceConfig) {
    this.serviceConfig = config
  }

  async upload(file: File, options?: UploadOptions) {
    let result: { url: string; tags: string[][] }
    if (this.serviceConfig.type === 'nip96') {
      result = await this.uploadByNip96(this.serviceConfig.service, file, options)
    } else {
      result = await this.uploadByBlossom(file, options)
    }

    if (result.tags.length > 0) {
      this.imetaTagMap.set(result.url, ['imeta', ...result.tags.map(([n, v]) => `${n} ${v}`)])
    }
    return result
  }

  private async uploadByBlossom(file: File, options?: UploadOptions) {
    const pubkey = client.pubkey
    const signer = async (draft: TDraftEvent) => {
      if (!client.signer) {
        throw new Error('You need to be logged in to upload media')
      }
      return client.signer.signEvent(draft)
    }
    if (!pubkey) {
      throw new Error('You need to be logged in to upload media')
    }

    if (options?.signal?.aborted) {
      throw new Error(UPLOAD_ABORTED_ERROR_MSG)
    }

    options?.onProgress?.(0)

    // Pseudo-progress: advance gradually until main upload completes
    let pseudoProgress = 1
    let pseudoTimer: number | undefined
    const startPseudoProgress = () => {
      if (pseudoTimer !== undefined) return
      pseudoTimer = window.setInterval(() => {
        // Cap pseudo progress to 90% until we get real completion
        pseudoProgress = Math.min(pseudoProgress + 3, 90)
        options?.onProgress?.(pseudoProgress)
        if (pseudoProgress >= 90) {
          stopPseudoProgress()
        }
      }, 300)
    }
    const stopPseudoProgress = () => {
      if (pseudoTimer !== undefined) {
        clearInterval(pseudoTimer)
        pseudoTimer = undefined
      }
    }
    startPseudoProgress()

    let servers = await client.fetchBlossomServerList(pubkey)
    // Add recommended servers as fallback
    const uniqueServers = new Set(servers)
    RECOMMENDED_BLOSSOM_SERVERS.forEach((s) => uniqueServers.add(s))
    servers = Array.from(uniqueServers)

    if (servers.length === 0) {
      throw new Error('No Blossom services available')
    }
    const [mainServer, ...mirrorServers] = servers

    const auth = await BlossomClient.createUploadAuth(signer, file, {
      message: 'Uploading media file'
    })

    // Try each server until one succeeds
    let blob: { url: string; sha256?: string; size?: number; type?: string; nip94?: string[][] } | undefined
    let lastError: Error | undefined
    const allServers = [mainServer, ...mirrorServers]

    // Upload with timeout using XMLHttpRequest (works better on mobile)
    const uploadWithXHR = (server: string): Promise<typeof blob> => {
      return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest()
        const uploadUrl = server.replace(/\/$/, '') + '/upload'

        xhr.open('PUT', uploadUrl)
        xhr.setRequestHeader('Authorization', 'Nostr ' + btoa(JSON.stringify(auth)))
        xhr.setRequestHeader('Content-Type', file.type || 'application/octet-stream')
        xhr.responseType = 'json'
        xhr.timeout = 15000 // 15 second timeout per server

        const handleAbort = () => {
          xhr.abort()
          reject(new Error(UPLOAD_ABORTED_ERROR_MSG))
        }
        if (options?.signal) {
          if (options.signal.aborted) return handleAbort()
          options.signal.addEventListener('abort', handleAbort, { once: true })
        }

        xhr.ontimeout = () => reject(new Error('Upload timed out'))
        xhr.onerror = () => reject(new Error('Network error'))
        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) {
            const data = xhr.response
            if (data?.url) {
              resolve({
                url: data.url,
                sha256: data.sha256,
                size: data.size,
                type: data.type,
                nip94: data.nip94
              })
            } else {
              reject(new Error('No URL in response'))
            }
          } else {
            reject(new Error(`Upload failed: ${xhr.status} ${xhr.statusText}`))
          }
        }
        xhr.send(file)
      })
    }

    for (const server of allServers) {
      try {
        // Try XHR first (better mobile support)
        blob = await uploadWithXHR(server)
        break
      } catch (xhrErr) {
        console.error(`Blossom XHR upload failed for ${server}:`, xhrErr)
        // Fallback to SDK with timeout
        try {
          const sdkPromise = BlossomClient.uploadBlob(server, file, { auth })
          const timeoutPromise = new Promise<never>((_, reject) =>
            setTimeout(() => reject(new Error('SDK upload timed out')), 15000)
          )
          const sdkBlob = await Promise.race([sdkPromise, timeoutPromise])
          blob = {
            url: sdkBlob.url,
            sha256: sdkBlob.sha256,
            size: sdkBlob.size,
            type: sdkBlob.type,
            nip94: (sdkBlob as any).nip94
          }
          break
        } catch (err) {
          console.error(`Blossom SDK upload failed for ${server}:`, err)
          lastError = err instanceof Error ? err : new Error(String(err))
        }
      }
    }

    if (!blob) {
      throw lastError ?? new Error('All Blossom servers failed')
    }

    // Main upload finished
    stopPseudoProgress()
    options?.onProgress?.(80)

    // Mirror to other servers (best effort) - only if we have sha256
    if (blob.sha256) {
      const successServer = blob.url ? new URL(blob.url).origin : mainServer
      const otherServers = allServers.filter((s) => s !== successServer)
      if (otherServers.length > 0) {
        const blobDescriptor = {
          url: blob.url,
          sha256: blob.sha256,
          size: blob.size ?? 0,
          type: blob.type ?? file.type,
          uploaded: Date.now()
        }
        await Promise.allSettled(
          otherServers.map((server) => BlossomClient.mirrorBlob(server, blobDescriptor, { auth }))
        )
      }
    }

    let tags: string[][] = []
    const parseResult = z.array(z.array(z.string())).safeParse((blob as any).nip94 ?? [])
    if (parseResult.success) {
      tags = parseResult.data
    }

    options?.onProgress?.(100)
    return { url: blob.url, tags }
  }

  private async uploadByNip96(service: string, file: File, options?: UploadOptions) {
    if (options?.signal?.aborted) {
      throw new Error(UPLOAD_ABORTED_ERROR_MSG)
    }
    let uploadUrl = this.nip96ServiceUploadUrlMap.get(service)
    if (!uploadUrl) {
      const response = await fetch(`${service}/.well-known/nostr/nip96.json`)
      if (!response.ok) {
        throw new Error(
          `${simplifyUrl(service)} does not work, please try another service in your settings`
        )
      }
      const data = await response.json()
      uploadUrl = data?.api_url
      if (!uploadUrl) {
        throw new Error(
          `${simplifyUrl(service)} does not work, please try another service in your settings`
        )
      }
      this.nip96ServiceUploadUrlMap.set(service, uploadUrl)
    }

    if (options?.signal?.aborted) {
      throw new Error(UPLOAD_ABORTED_ERROR_MSG)
    }
    const formData = new FormData()
    formData.append('file', file)

    const auth = await client.signHttpAuth(uploadUrl, 'POST', 'Uploading media file')

    // Use XMLHttpRequest for upload progress support
    const result = await new Promise<{ url: string; tags: string[][] }>((resolve, reject) => {
      const xhr = new XMLHttpRequest()
      xhr.open('POST', uploadUrl as string)
      xhr.responseType = 'json'
      xhr.setRequestHeader('Authorization', auth)

      const handleAbort = () => {
        try {
          xhr.abort()
        } catch {
          // ignore
        }
        reject(new Error(UPLOAD_ABORTED_ERROR_MSG))
      }
      if (options?.signal) {
        if (options.signal.aborted) {
          return handleAbort()
        }
        options.signal.addEventListener('abort', handleAbort, { once: true })
      }

      xhr.upload.onprogress = (event) => {
        if (event.lengthComputable) {
          const percent = Math.round((event.loaded / event.total) * 100)
          options?.onProgress?.(percent)
        }
      }
      xhr.onerror = () => reject(new Error('Network error'))
      xhr.onload = () => {
        if (xhr.status >= 200 && xhr.status < 300) {
          const data = xhr.response
          try {
            const tags = z.array(z.array(z.string())).parse(data?.nip94_event?.tags ?? [])
            const url = tags.find(([tagName]: string[]) => tagName === 'url')?.[1]
            if (url) {
              resolve({ url, tags })
            } else {
              reject(new Error('No url found'))
            }
          } catch (e) {
            reject(e as Error)
          }
        } else {
          reject(new Error(xhr.status.toString() + ' ' + xhr.statusText))
        }
      }
      xhr.send(formData)
    })

    return result
  }

  getImetaTagByUrl(url: string) {
    return this.imetaTagMap.get(url)
  }

  /**
   * Delete a blob from Blossom servers
   * First tries with server tag (for replay protection), then falls back to without
   * @param sha256 - The SHA256 hash of the blob to delete
   * @param serverUrls - Optional list of servers to delete from (defaults to user's Blossom server list)
   */
  async deleteBlob(sha256: string, serverUrls?: string[]): Promise<{ deleted: string[]; failed: string[] }> {
    const pubkey = client.pubkey
    const signer = async (draft: TDraftEvent) => {
      if (!client.signer) {
        throw new Error('You need to be logged in to delete media')
      }
      return client.signer.signEvent(draft)
    }
    if (!pubkey) {
      throw new Error('You need to be logged in to delete media')
    }

    // Get servers to delete from
    let servers = serverUrls
    if (!servers || servers.length === 0) {
      servers = await client.fetchBlossomServerList(pubkey)
    }
    if (servers.length === 0) {
      throw new Error('No Blossom servers configured')
    }

    const deleted: string[] = []
    const failed: string[] = []

    for (const server of servers) {
      const normalizedServer = server.replace(/\/$/, '')
      const deleteUrl = `${normalizedServer}/${sha256}`

      try {
        // First try WITH server tag (replay protection)
        const authWithServer = await BlossomClient.createDeleteAuth(signer, sha256, {
          servers: [normalizedServer],
          message: 'Deleting media file'
        })

        const response = await fetch(deleteUrl, {
          method: 'DELETE',
          headers: {
            'Authorization': 'Nostr ' + btoa(JSON.stringify(authWithServer))
          }
        })

        if (response.ok) {
          deleted.push(server)
          continue
        }

        // If server tag was required but missing, we'd get 401
        // If server tag was rejected as unknown, we might get 400 or 401
        // Try again WITHOUT server tag for backwards compatibility
        if (response.status === 400 || response.status === 401) {
          const authWithoutServer = await BlossomClient.createDeleteAuth(signer, sha256, {
            message: 'Deleting media file'
          })

          const retryResponse = await fetch(deleteUrl, {
            method: 'DELETE',
            headers: {
              'Authorization': 'Nostr ' + btoa(JSON.stringify(authWithoutServer))
            }
          })

          if (retryResponse.ok) {
            deleted.push(server)
            continue
          }
        }

        console.error(`Failed to delete blob from ${server}: ${response.status} ${response.statusText}`)
        failed.push(server)
      } catch (err) {
        console.error(`Error deleting blob from ${server}:`, err)
        failed.push(server)
      }
    }

    return { deleted, failed }
  }

  /**
   * Upload an image with automatic responsive variant generation
   *
   * Generates multiple resolution variants of the image (thumb, mobile, mobile-lg,
   * desktop, desktop-lg, original), uploads each to Blossom, and publishes a
   * NIP-94 File Metadata event (kind 1063) binding all variants together.
   *
   * @param file - The image file to upload
   * @param options - Upload options including description and alt text
   * @returns The published binding event and array of uploaded variants
   */
  async uploadResponsiveImage(
    file: File,
    options?: ResponsiveUploadOptions
  ): Promise<{ event: VerifiedEvent; variants: UploadedVariant[] }> {
    if (!isSupportedImage(file)) {
      throw new Error('Unsupported image format. Supported: JPEG, PNG, WebP, GIF')
    }

    const pubkey = client.pubkey
    const signer = client.signer
    if (!pubkey || !signer) {
      throw new Error('You need to be logged in to upload media')
    }

    if (options?.signal?.aborted) {
      throw new Error(UPLOAD_ABORTED_ERROR_MSG)
    }

    options?.onProgress?.(0)

    // Phase 1: Generate variants (0-30%)
    const scaledImages = await generateImageVariants(file, {
      onProgress: (percent) => {
        // Scale to 0-30%
        options?.onProgress?.(Math.round(percent * 0.3))
      }
    })

    if (options?.signal?.aborted) {
      throw new Error(UPLOAD_ABORTED_ERROR_MSG)
    }

    // Get Blossom servers
    let servers = await client.fetchBlossomServerList(pubkey)
    const uniqueServers = new Set(servers)
    RECOMMENDED_BLOSSOM_SERVERS.forEach((s) => uniqueServers.add(s))
    servers = Array.from(uniqueServers)

    if (servers.length === 0) {
      throw new Error('No Blossom services available')
    }

    // Phase 2: Upload each variant (30-90%)
    const uploadedVariants: UploadedVariant[] = []
    const progressPerVariant = 60 / scaledImages.length

    for (let i = 0; i < scaledImages.length; i++) {
      if (options?.signal?.aborted) {
        throw new Error(UPLOAD_ABORTED_ERROR_MSG)
      }

      const scaled = scaledImages[i]
      const extension = getExtensionFromMimeType(scaled.mimeType)
      const variantFile = new File([scaled.blob], `image-${scaled.variant}.${extension}`, {
        type: scaled.mimeType
      })

      // Upload this variant
      const result = await this.uploadSingleBlob(variantFile, servers, options?.signal)

      if (!result.sha256) {
        throw new Error(`Upload failed for ${scaled.variant} variant: no hash returned`)
      }

      uploadedVariants.push({
        variant: scaled.variant,
        url: result.url,
        sha256: result.sha256,
        width: scaled.width,
        height: scaled.height,
        mimeType: scaled.mimeType,
        size: scaled.blob.size
      })

      const progress = 30 + Math.round((i + 1) * progressPerVariant)
      options?.onProgress?.(progress)
    }

    if (options?.signal?.aborted) {
      throw new Error(UPLOAD_ABORTED_ERROR_MSG)
    }

    // Phase 3: Create and publish binding event (90-100%)
    options?.onProgress?.(90)

    const draftEvent = createResponsiveImageEvent(uploadedVariants, {
      description: options?.description,
      alt: options?.alt
    })

    const signedEvent = await signer.signEvent(draftEvent)

    // Publish to user's write relays
    let userRelays = client.currentRelays.length > 0 ? client.currentRelays : []

    // If no current relays, fetch user's relay list
    if (userRelays.length === 0) {
      try {
        const relayList = await client.fetchRelayList(pubkey)
        userRelays = relayList.write.slice(0, 10)
      } catch (e) {
        console.warn('[MediaUploadService] Failed to fetch relay list:', e)
      }
    }

    if (userRelays.length > 0) {
      await client.publishEvent(userRelays, signedEvent)
    } else {
      console.warn('[MediaUploadService] No relays available to publish binding event - event not published')
    }

    options?.onProgress?.(100)

    return { event: signedEvent, variants: uploadedVariants }
  }

  /**
   * Upload a single blob to Blossom servers (internal helper)
   */
  private async uploadSingleBlob(
    file: File,
    servers: string[],
    signal?: AbortSignal
  ): Promise<{ url: string; sha256?: string; size?: number; type?: string }> {
    const signer = async (draft: TDraftEvent) => {
      if (!client.signer) {
        throw new Error('You need to be logged in to upload media')
      }
      return client.signer.signEvent(draft)
    }

    const auth = await BlossomClient.createUploadAuth(signer, file, {
      message: 'Uploading media file'
    })

    let blob: { url: string; sha256?: string; size?: number; type?: string } | undefined
    let lastError: Error | undefined

    // Upload with timeout using XMLHttpRequest
    const uploadWithXHR = (server: string): Promise<typeof blob> => {
      return new Promise((resolve, reject) => {
        const xhr = new XMLHttpRequest()
        const uploadUrl = server.replace(/\/$/, '') + '/upload'

        xhr.open('PUT', uploadUrl)
        xhr.setRequestHeader('Authorization', 'Nostr ' + btoa(JSON.stringify(auth)))
        xhr.setRequestHeader('Content-Type', file.type || 'application/octet-stream')
        xhr.responseType = 'json'
        xhr.timeout = 30000 // 30 second timeout for variants

        const handleAbort = () => {
          xhr.abort()
          reject(new Error(UPLOAD_ABORTED_ERROR_MSG))
        }
        if (signal) {
          if (signal.aborted) return handleAbort()
          signal.addEventListener('abort', handleAbort, { once: true })
        }

        xhr.ontimeout = () => reject(new Error('Upload timed out'))
        xhr.onerror = () => reject(new Error('Network error'))
        xhr.onload = () => {
          if (xhr.status >= 200 && xhr.status < 300) {
            const data = xhr.response
            if (data?.url) {
              resolve({
                url: data.url,
                sha256: data.sha256,
                size: data.size,
                type: data.type
              })
            } else {
              reject(new Error('No URL in response'))
            }
          } else {
            reject(new Error(`Upload failed: ${xhr.status} ${xhr.statusText}`))
          }
        }
        xhr.send(file)
      })
    }

    for (const server of servers) {
      try {
        blob = await uploadWithXHR(server)
        break
      } catch (err) {
        console.error(`Blossom upload failed for ${server}:`, err)
        lastError = err instanceof Error ? err : new Error(String(err))
      }
    }

    if (!blob) {
      throw lastError ?? new Error('All Blossom servers failed')
    }

    return blob
  }
}

const instance = new MediaUploadService()
export default instance
