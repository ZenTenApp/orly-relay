import { Suspense, lazy, useState } from 'react'
import { cn } from '@/lib/utils'

const ExportTab = lazy(() => import('./ExportTab'))
const ImportTab = lazy(() => import('./ImportTab'))
const LogsTab = lazy(() => import('./LogsTab'))
const SprocketTab = lazy(() => import('./SprocketTab'))
const PolicyTab = lazy(() => import('./PolicyTab'))
const CurationTab = lazy(() => import('./CurationTab'))
const ManagedACLTab = lazy(() => import('./ManagedACLTab'))
const BlossomAdminTab = lazy(() => import('./BlossomAdminTab'))
const EventBrowserTab = lazy(() => import('./EventBrowserTab'))
const RecoveryTab = lazy(() => import('./RecoveryTab'))

const TABS = [
  { id: 'events', label: 'Events' },
  { id: 'export', label: 'Export' },
  { id: 'import', label: 'Import' },
  { id: 'policy', label: 'Policy' },
  { id: 'curation', label: 'Curation' },
  { id: 'acl', label: 'Managed ACL' },
  { id: 'sprocket', label: 'Sprockets' },
  { id: 'logs', label: 'Logs' },
  { id: 'blossom', label: 'Blossom' },
  { id: 'recovery', label: 'Recovery' }
] as const

type TabId = (typeof TABS)[number]['id']

function TabContent({ tab }: { tab: TabId }) {
  switch (tab) {
    case 'events':
      return <EventBrowserTab />
    case 'export':
      return <ExportTab />
    case 'import':
      return <ImportTab />
    case 'policy':
      return <PolicyTab />
    case 'curation':
      return <CurationTab />
    case 'acl':
      return <ManagedACLTab />
    case 'sprocket':
      return <SprocketTab />
    case 'logs':
      return <LogsTab />
    case 'blossom':
      return <BlossomAdminTab />
    case 'recovery':
      return <RecoveryTab />
  }
}

export default function RelayAdminPanel() {
  const [activeTab, setActiveTab] = useState<TabId>('events')

  return (
    <div className="w-full">
      <div className="flex items-center gap-1 overflow-x-auto border-b border-border px-2 py-1">
        {TABS.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={cn(
              'whitespace-nowrap rounded-md px-3 py-1.5 text-sm transition-colors',
              activeTab === tab.id
                ? 'bg-primary text-primary-foreground font-medium'
                : 'text-muted-foreground hover:bg-muted hover:text-foreground'
            )}
          >
            {tab.label}
          </button>
        ))}
      </div>
      <Suspense
        fallback={
          <div className="flex items-center justify-center py-12 text-muted-foreground">
            Loading...
          </div>
        }
      >
        <TabContent tab={activeTab} />
      </Suspense>
    </div>
  )
}
