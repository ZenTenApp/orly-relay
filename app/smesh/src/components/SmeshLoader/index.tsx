import { useEffect, useRef } from 'react'
import { useTheme } from '@/providers/ThemeProvider'

const SVG_NS = 'http://www.w3.org/2000/svg'

interface BranchEdge {
  x1: number
  y1: number
  x2: number
  y2: number
  color: string
  width: number
  delay: number
}

const DARK_COLORS = ['#e07030', '#8833bb', '#00aabb']
const LIGHT_COLORS = ['#2266cc', '#cc2233', '#22aa44']

const BASE_LEN = 110
const DECAY = 0.56
const BASE_WIDTH = 32
const DEPTH = 6
const SPREAD = Math.PI * 0.68
const CX = 400
const CY = 400

const BRANCH_ANGLES = [
  -Math.PI / 2,
  -Math.PI / 2 + (2 * Math.PI) / 3,
  -Math.PI / 2 + (4 * Math.PI) / 3,
]

function buildEdges(colors: string[]): BranchEdge[] {
  const edges: BranchEdge[] = []
  let delay = 50

  function buildBranch(
    px: number,
    py: number,
    angle: number,
    depth: number,
    maxDepth: number,
    branchIdx: number,
    spreadAngle: number
  ) {
    if (depth > maxDepth) return

    const scale = Math.pow(DECAY, depth - 1)
    const len = BASE_LEN * scale
    const width = BASE_WIDTH * scale
    const nx = px + Math.cos(angle) * len
    const ny = py + Math.sin(angle) * len

    const d = delay
    delay += 30

    edges.push({ x1: px, y1: py, x2: nx, y2: ny, color: colors[branchIdx], width, delay: d })

    const childSpread = spreadAngle * 0.82
    const offsets = [-childSpread / 2, childSpread / 2]

    for (const off of offsets) {
      buildBranch(nx, ny, angle + off, depth + 1, maxDepth, branchIdx, childSpread)
    }
  }

  for (let i = 0; i < 3; i++) {
    buildBranch(CX, CY, BRANCH_ANGLES[i], 1, DEPTH, i, SPREAD)
  }

  return edges
}

function renderSVG(container: SVGGElement, isDark: boolean) {
  while (container.firstChild) container.removeChild(container.firstChild)

  const colors = isDark ? DARK_COLORS : LIGHT_COLORS
  const edges = buildEdges(colors)

  for (const e of edges) {
    const line = document.createElementNS(SVG_NS, 'line')
    line.setAttribute('x1', String(e.x1))
    line.setAttribute('y1', String(e.y1))
    line.setAttribute('x2', String(e.x2))
    line.setAttribute('y2', String(e.y2))
    line.setAttribute('stroke', e.color)
    line.setAttribute('stroke-width', String(e.width))
    line.classList.add('smesh-loader-edge')
    line.style.animationDelay = e.delay + 'ms'
    container.appendChild(line)
  }

  // Center hexagon
  const r = 24
  const hexPoints: string[] = []
  for (let i = 0; i < 6; i++) {
    const a = Math.PI / 6 + (i * Math.PI) / 3
    hexPoints.push(`${(CX + r * Math.cos(a)).toFixed(2)},${(CY + r * Math.sin(a)).toFixed(2)}`)
  }
  const hex = document.createElementNS(SVG_NS, 'polygon')
  hex.setAttribute('points', hexPoints.join(' '))
  hex.setAttribute('fill', isDark ? '#e8e4da' : '#1a1a1e')
  hex.setAttribute('stroke', isDark ? '#0a0a0e' : '#f5f5f0')
  hex.setAttribute('stroke-width', '7.5')
  hex.setAttribute('stroke-linejoin', 'round')
  hex.classList.add('smesh-loader-center')
  hex.style.animationDelay = '0ms'
  container.appendChild(hex)
}

export default function SmeshLoader({ className }: { className?: string }) {
  const { theme } = useTheme()
  const gRef = useRef<SVGGElement>(null)
  const isDark = theme !== 'light'

  useEffect(() => {
    if (gRef.current) {
      renderSVG(gRef.current, isDark)
    }
  }, [isDark])

  return (
    <div className={className}>
      <style>{`
        .smesh-loader-edge {
          stroke-linecap: round;
          opacity: 0;
          animation: smeshEdgeFade 0.4s ease forwards;
        }
        .smesh-loader-center {
          opacity: 0;
          animation: smeshNodePop 0.3s ease forwards;
        }
        @keyframes smeshEdgeFade {
          to { opacity: 1; }
        }
        @keyframes smeshNodePop {
          0% { opacity: 0; transform: scale(0); }
          70% { transform: scale(1.2); }
          100% { opacity: 1; transform: scale(1); }
        }
      `}</style>
      <svg viewBox="160.68 160.68 478.65 478.65" className="w-full h-full">
        <g ref={gRef} />
      </svg>
    </div>
  )
}
