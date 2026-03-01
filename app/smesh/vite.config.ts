import react from '@vitejs/plugin-react'
import { execSync } from 'child_process'
import path from 'path'
import { defineConfig } from 'vite'
import { VitePWA } from 'vite-plugin-pwa'
import packageJson from './package.json'

const getGitHash = () => {
  try {
    return JSON.stringify(execSync('git rev-parse --short HEAD').toString().trim())
  } catch (error) {
    console.warn('Failed to retrieve commit hash:', error)
    return '"unknown"'
  }
}

const getAppVersion = () => {
  try {
    return JSON.stringify(packageJson.version)
  } catch (error) {
    console.warn('Failed to retrieve app version:', error)
    return '"unknown"'
  }
}

// https://vite.dev/config/
export default defineConfig({
  server: {
    allowedHosts: ['smesh.mleku.dev']
  },
  define: {
    'import.meta.env.GIT_COMMIT': getGitHash(),
    'import.meta.env.APP_VERSION': getAppVersion()
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src')
    }
  },
  plugins: [
    react(),
    VitePWA({
      registerType: 'autoUpdate',
      workbox: {
        globPatterns: ['**/*.{js,css,html,png,jpg,svg}'],
        globDirectory: 'dist/',
        maximumFileSizeToCacheInBytes: 5 * 1024 * 1024,
        cleanupOutdatedCaches: true,
        skipWaiting: true,
        clientsClaim: true
      },
      devOptions: {
        enabled: true
      },
      manifest: {
        name: 'Smesh',
        short_name: 'Smesh',
        icons: [
          {
            src: '/pwa-512x512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'any'
          },
          {
            src: '/pwa-192x192.png',
            sizes: '192x192',
            type: 'image/png',
            purpose: 'any'
          },
          {
            src: '/pwa-512x512.png',
            sizes: '512x512',
            type: 'image/png',
            purpose: 'maskable'
          },
          {
            src: '/pwa-192x192.png',
            sizes: '192x192',
            type: 'image/png',
            purpose: 'maskable'
          },
          {
            src: '/pwa-monochrome.svg',
            sizes: '512x512',
            type: 'image/svg+xml',
            purpose: 'monochrome'
          }
        ],
        start_url: '/',
        display: 'standalone',
        background_color: '#171717',
        theme_color: '#171717',
        description: packageJson.description
      }
    })
  ]
})
