import './i18n'
import './index.css'
import './polyfill'
import './services/lightning.service'

import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App.tsx'
import { ErrorBoundary } from './components/ErrorBoundary.tsx'

const setVh = () => {
  // Prefer visualViewport for accurate height when mobile keyboard is open
  const height = window.visualViewport?.height ?? window.innerHeight
  document.documentElement.style.setProperty('--vh', `${height}px`)
}
if (window.visualViewport) {
  window.visualViewport.addEventListener('resize', setVh)
} else {
  window.addEventListener('resize', setVh)
}
window.addEventListener('orientationchange', setVh)
setVh()

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </StrictMode>
)
