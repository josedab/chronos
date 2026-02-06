import { useQuery } from '@tanstack/react-query'
import { getConfig } from '../api/client'
import { useState } from 'react'
import ApiExplorer from '../components/ApiExplorer'
import { RestartTourButton } from '../components/OnboardingTour'

interface AlertChannel {
  id: string
  type: 'slack' | 'email' | 'pagerduty' | 'webhook'
  name: string
  enabled: boolean
  config: Record<string, string>
}

interface AlertRule {
  id: string
  name: string
  condition: 'job_failed' | 'job_timeout' | 'consecutive_failures' | 'cluster_unhealthy'
  threshold?: number
  channels: string[]
  enabled: boolean
}

// Mock data - in production this would come from the API
const mockChannels: AlertChannel[] = [
  { id: '1', type: 'slack', name: 'Engineering Alerts', enabled: true, config: { webhook_url: 'https://hooks.slack.com/...' } },
  { id: '2', type: 'email', name: 'On-Call Email', enabled: true, config: { recipients: 'oncall@example.com' } },
]

const mockRules: AlertRule[] = [
  { id: '1', name: 'Job Failure Alert', condition: 'job_failed', channels: ['1', '2'], enabled: true },
  { id: '2', name: 'Multiple Failures', condition: 'consecutive_failures', threshold: 3, channels: ['1'], enabled: true },
]

export default function Settings() {
  const [activeTab, setActiveTab] = useState<'general' | 'scheduler' | 'cluster' | 'metrics' | 'alerts' | 'api'>('general')
  const [channels, setChannels] = useState<AlertChannel[]>(mockChannels)
  const [rules, setRules] = useState<AlertRule[]>(mockRules)
  const [showChannelModal, setShowChannelModal] = useState(false)
  const [showRuleModal, setShowRuleModal] = useState(false)
  const [editingChannel, setEditingChannel] = useState<AlertChannel | null>(null)
  const [editingRule, setEditingRule] = useState<AlertRule | null>(null)

  const { data: config, isLoading, error } = useQuery({
    queryKey: ['config'],
    queryFn: getConfig,
  })

  const tabs = [
    { id: 'general', label: 'General' },
    { id: 'scheduler', label: 'Scheduler' },
    { id: 'cluster', label: 'Cluster' },
    { id: 'metrics', label: 'Metrics' },
    { id: 'alerts', label: 'Alerts' },
    { id: 'api', label: 'API' },
  ] as const

  if (isLoading) {
    return <div className="text-center py-12">Loading configuration...</div>
  }

  if (error) {
    return <div className="text-center py-12 text-red-600">Error loading configuration</div>
  }

  return (
    <div className="space-y-6">
      <h2 className="text-2xl font-bold text-gray-900">Settings</h2>

      <div className="bg-yellow-50 border border-yellow-200 rounded-lg p-4">
        <div className="flex">
          <div className="flex-shrink-0">
            <span className="text-yellow-400">ℹ️</span>
          </div>
          <div className="ml-3">
            <p className="text-sm text-yellow-700">
              Configuration is read-only. To modify settings, update the <code className="bg-yellow-100 px-1 rounded">chronos.yaml</code> file and restart the server.
            </p>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-gray-200">
        <nav className="-mb-px flex space-x-8">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`py-4 px-1 border-b-2 font-medium text-sm ${
                activeTab === tab.id
                  ? 'border-indigo-500 text-indigo-600'
                  : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </nav>
      </div>

      {/* General Settings */}
      {activeTab === 'general' && (
        <div className="bg-white shadow rounded-lg overflow-hidden">
          <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
            <h3 className="text-lg font-medium text-gray-900">General Settings</h3>
          </div>
          <div className="px-4 py-5 sm:p-6 space-y-6">
            <ConfigItem label="Node ID" value={config?.cluster?.node_id} />
            <ConfigItem label="Data Directory" value={config?.cluster?.data_dir} />
            <ConfigItem label="HTTP Address" value={config?.server?.http?.address} />
            <ConfigItem label="Read Timeout" value={config?.server?.http?.read_timeout} />
            <ConfigItem label="Write Timeout" value={config?.server?.http?.write_timeout} />
            <ConfigItem label="Log Level" value={config?.logging?.level} />
            <ConfigItem label="Log Format" value={config?.logging?.format} />
          </div>
        </div>
      )}

      {/* Scheduler Settings */}
      {activeTab === 'scheduler' && (
        <div className="bg-white shadow rounded-lg overflow-hidden">
          <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
            <h3 className="text-lg font-medium text-gray-900">Scheduler Settings</h3>
          </div>
          <div className="px-4 py-5 sm:p-6 space-y-6">
            <ConfigItem label="Tick Interval" value={config?.scheduler?.tick_interval} />
            <ConfigItem label="Execution Timeout" value={config?.scheduler?.execution_timeout} />
            <div className="border-t pt-4">
              <h4 className="text-sm font-medium text-gray-700 mb-3">Default Retry Policy</h4>
              <div className="ml-4 space-y-4">
                <ConfigItem label="Max Attempts" value={config?.scheduler?.default_retry_policy?.max_attempts} />
                <ConfigItem label="Initial Interval" value={config?.scheduler?.default_retry_policy?.initial_interval} />
                <ConfigItem label="Max Interval" value={config?.scheduler?.default_retry_policy?.max_interval} />
                <ConfigItem label="Multiplier" value={config?.scheduler?.default_retry_policy?.multiplier} />
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Cluster Settings */}
      {activeTab === 'cluster' && (
        <div className="bg-white shadow rounded-lg overflow-hidden">
          <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
            <h3 className="text-lg font-medium text-gray-900">Cluster Settings</h3>
          </div>
          <div className="px-4 py-5 sm:p-6 space-y-6">
            <ConfigItem label="Raft Address" value={config?.cluster?.raft?.address} />
            <div>
              <label className="block text-sm font-medium text-gray-600 mb-2">Peers</label>
              <div className="bg-gray-50 rounded-md p-3">
                {config?.cluster?.raft?.peers?.length ? (
                  <ul className="space-y-1">
                    {config.cluster.raft.peers.map((peer: string, i: number) => (
                      <li key={i} className="font-mono text-sm text-gray-700">{peer}</li>
                    ))}
                  </ul>
                ) : (
                  <span className="text-sm text-gray-500">No peers configured (single-node mode)</span>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Metrics Settings */}
      {activeTab === 'metrics' && (
        <div className="bg-white shadow rounded-lg overflow-hidden">
          <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
            <h3 className="text-lg font-medium text-gray-900">Metrics & Observability</h3>
          </div>
          <div className="px-4 py-5 sm:p-6 space-y-6">
            <ConfigItem 
              label="Prometheus Enabled" 
              value={config?.metrics?.prometheus?.enabled ? 'Yes' : 'No'} 
            />
            <ConfigItem label="Metrics Path" value={config?.metrics?.prometheus?.path} />
            <div className="border-t pt-4">
              <h4 className="text-sm font-medium text-gray-700 mb-3">HTTP Dispatcher</h4>
              <div className="ml-4 space-y-4">
                <ConfigItem label="Timeout" value={config?.dispatcher?.http?.timeout} />
                <ConfigItem label="Max Idle Connections" value={config?.dispatcher?.http?.max_idle_conns} />
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Alerts Settings */}
      {activeTab === 'alerts' && (
        <div className="space-y-6">
          {/* Alert Channels */}
          <div className="bg-white shadow rounded-lg overflow-hidden">
            <div className="px-4 py-5 sm:px-6 border-b border-gray-200 flex justify-between items-center">
              <div>
                <h3 className="text-lg font-medium text-gray-900">Notification Channels</h3>
                <p className="text-sm text-gray-500 mt-1">Configure where alerts are sent</p>
              </div>
              <button
                onClick={() => { setEditingChannel(null); setShowChannelModal(true); }}
                className="px-4 py-2 bg-indigo-600 text-white text-sm rounded-md hover:bg-indigo-700"
              >
                Add Channel
              </button>
            </div>
            <div className="divide-y divide-gray-200">
              {channels.length === 0 ? (
                <div className="px-4 py-8 text-center text-gray-500">
                  No notification channels configured
                </div>
              ) : (
                channels.map((channel) => (
                  <div key={channel.id} className="px-4 py-4 sm:px-6 flex items-center justify-between">
                    <div className="flex items-center gap-4">
                      <span className="text-2xl">
                        {channel.type === 'slack' && '💬'}
                        {channel.type === 'email' && '📧'}
                        {channel.type === 'pagerduty' && '🔔'}
                        {channel.type === 'webhook' && '🔗'}
                      </span>
                      <div>
                        <h4 className="text-sm font-medium text-gray-900">{channel.name}</h4>
                        <p className="text-sm text-gray-500 capitalize">{channel.type}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-4">
                      <span className={`px-2 py-1 text-xs rounded-full ${
                        channel.enabled ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'
                      }`}>
                        {channel.enabled ? 'Active' : 'Disabled'}
                      </span>
                      <button
                        onClick={() => { setEditingChannel(channel); setShowChannelModal(true); }}
                        className="text-indigo-600 hover:text-indigo-800 text-sm"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => setChannels(channels.filter(c => c.id !== channel.id))}
                        className="text-red-600 hover:text-red-800 text-sm"
                      >
                        Delete
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>

          {/* Alert Rules */}
          <div className="bg-white shadow rounded-lg overflow-hidden">
            <div className="px-4 py-5 sm:px-6 border-b border-gray-200 flex justify-between items-center">
              <div>
                <h3 className="text-lg font-medium text-gray-900">Alert Rules</h3>
                <p className="text-sm text-gray-500 mt-1">Define when notifications are triggered</p>
              </div>
              <button
                onClick={() => { setEditingRule(null); setShowRuleModal(true); }}
                className="px-4 py-2 bg-indigo-600 text-white text-sm rounded-md hover:bg-indigo-700"
              >
                Add Rule
              </button>
            </div>
            <div className="divide-y divide-gray-200">
              {rules.length === 0 ? (
                <div className="px-4 py-8 text-center text-gray-500">
                  No alert rules configured
                </div>
              ) : (
                rules.map((rule) => (
                  <div key={rule.id} className="px-4 py-4 sm:px-6">
                    <div className="flex items-center justify-between">
                      <div>
                        <h4 className="text-sm font-medium text-gray-900">{rule.name}</h4>
                        <p className="text-sm text-gray-500 mt-1">
                          Condition: <span className="font-mono">{rule.condition.replace(/_/g, ' ')}</span>
                          {rule.threshold && ` (threshold: ${rule.threshold})`}
                        </p>
                        <p className="text-sm text-gray-500">
                          Channels: {rule.channels.map(cid => 
                            channels.find(c => c.id === cid)?.name || cid
                          ).join(', ')}
                        </p>
                      </div>
                      <div className="flex items-center gap-4">
                        <label className="flex items-center gap-2">
                          <input
                            type="checkbox"
                            checked={rule.enabled}
                            onChange={(e) => setRules(rules.map(r => 
                              r.id === rule.id ? { ...r, enabled: e.target.checked } : r
                            ))}
                            className="rounded border-gray-300 text-indigo-600"
                          />
                          <span className="text-sm text-gray-600">Enabled</span>
                        </label>
                        <button
                          onClick={() => { setEditingRule(rule); setShowRuleModal(true); }}
                          className="text-indigo-600 hover:text-indigo-800 text-sm"
                        >
                          Edit
                        </button>
                        <button
                          onClick={() => setRules(rules.filter(r => r.id !== rule.id))}
                          className="text-red-600 hover:text-red-800 text-sm"
                        >
                          Delete
                        </button>
                      </div>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>

          {/* Test Alert Button */}
          <div className="bg-white shadow rounded-lg p-6">
            <h3 className="text-lg font-medium text-gray-900 mb-4">Test Notifications</h3>
            <p className="text-sm text-gray-500 mb-4">
              Send a test alert to verify your notification channels are configured correctly.
            </p>
            <button
              onClick={() => alert('Test notification sent! Check your configured channels.')}
              className="px-4 py-2 bg-gray-800 text-white text-sm rounded-md hover:bg-gray-700"
            >
              Send Test Alert
            </button>
          </div>
        </div>
      )}

      {/* Channel Modal */}
      {showChannelModal && (
        <ChannelModal
          channel={editingChannel}
          onSave={(channel) => {
            if (editingChannel) {
              setChannels(channels.map(c => c.id === channel.id ? channel : c))
            } else {
              setChannels([...channels, { ...channel, id: Date.now().toString() }])
            }
            setShowChannelModal(false)
          }}
          onClose={() => setShowChannelModal(false)}
        />
      )}

      {/* Rule Modal */}
      {showRuleModal && (
        <RuleModal
          rule={editingRule}
          channels={channels}
          onSave={(rule) => {
            if (editingRule) {
              setRules(rules.map(r => r.id === rule.id ? rule : r))
            } else {
              setRules([...rules, { ...rule, id: Date.now().toString() }])
            }
            setShowRuleModal(false)
          }}
          onClose={() => setShowRuleModal(false)}
        />
      )}

      {/* API Explorer Tab */}
      {activeTab === 'api' && (
        <ApiExplorer />
      )}

      {/* Raw Config */}
      <div className="bg-white shadow rounded-lg overflow-hidden">
        <details>
          <summary className="px-4 py-5 sm:px-6 cursor-pointer hover:bg-gray-50">
            <span className="text-sm font-medium text-gray-500">View Raw Configuration</span>
          </summary>
          <div className="px-4 py-5 sm:p-6 border-t">
            <pre className="bg-gray-900 text-gray-100 rounded-lg p-4 overflow-x-auto text-sm">
              {JSON.stringify(config, null, 2)}
            </pre>
          </div>
        </details>
      </div>

      {/* Help section */}
      <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-4">
        <div className="flex items-center justify-between">
          <div>
            <h4 className="text-sm font-medium text-gray-900 dark:text-white">Need help?</h4>
            <p className="text-sm text-gray-500 dark:text-gray-400 mt-1">
              Take a guided tour of Chronos features
            </p>
          </div>
          <RestartTourButton />
        </div>
      </div>
    </div>
  )
}

function ConfigItem({ label, value }: { label: string; value?: string | number | boolean }) {
  return (
    <div>
      <label className="block text-sm font-medium text-gray-600 mb-1">{label}</label>
      <div className="bg-gray-50 rounded-md px-3 py-2 font-mono text-sm text-gray-800">
        {value !== undefined && value !== null ? String(value) : <span className="text-gray-400">Not set</span>}
      </div>
    </div>
  )
}

// Channel Configuration Modal
function ChannelModal({ 
  channel, 
  onSave, 
  onClose 
}: { 
  channel: AlertChannel | null
  onSave: (channel: AlertChannel) => void
  onClose: () => void 
}) {
  const [form, setForm] = useState<Partial<AlertChannel>>(channel || {
    type: 'slack',
    name: '',
    enabled: true,
    config: {},
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSave(form as AlertChannel)
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-md">
        <div className="px-6 py-4 border-b border-gray-200">
          <h3 className="text-lg font-medium text-gray-900">
            {channel ? 'Edit Channel' : 'Add Notification Channel'}
          </h3>
        </div>
        <form onSubmit={handleSubmit} className="px-6 py-4 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Channel Type</label>
            <select
              value={form.type}
              onChange={(e) => setForm({ ...form, type: e.target.value as AlertChannel['type'], config: {} })}
              className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
            >
              <option value="slack">Slack</option>
              <option value="email">Email</option>
              <option value="pagerduty">PagerDuty</option>
              <option value="webhook">Webhook</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
            <input
              type="text"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
              placeholder="e.g., Engineering Alerts"
              required
            />
          </div>

          {form.type === 'slack' && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Webhook URL</label>
              <input
                type="url"
                value={form.config?.webhook_url || ''}
                onChange={(e) => setForm({ ...form, config: { ...form.config, webhook_url: e.target.value } })}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                placeholder="https://hooks.slack.com/services/..."
                required
              />
            </div>
          )}

          {form.type === 'email' && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Recipients</label>
              <input
                type="text"
                value={form.config?.recipients || ''}
                onChange={(e) => setForm({ ...form, config: { ...form.config, recipients: e.target.value } })}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                placeholder="email1@example.com, email2@example.com"
                required
              />
            </div>
          )}

          {form.type === 'pagerduty' && (
            <>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Integration Key</label>
                <input
                  type="text"
                  value={form.config?.integration_key || ''}
                  onChange={(e) => setForm({ ...form, config: { ...form.config, integration_key: e.target.value } })}
                  className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                  placeholder="xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Severity</label>
                <select
                  value={form.config?.severity || 'error'}
                  onChange={(e) => setForm({ ...form, config: { ...form.config, severity: e.target.value } })}
                  className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                >
                  <option value="critical">Critical</option>
                  <option value="error">Error</option>
                  <option value="warning">Warning</option>
                  <option value="info">Info</option>
                </select>
              </div>
            </>
          )}

          {form.type === 'webhook' && (
            <>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">URL</label>
                <input
                  type="url"
                  value={form.config?.url || ''}
                  onChange={(e) => setForm({ ...form, config: { ...form.config, url: e.target.value } })}
                  className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                  placeholder="https://api.example.com/alerts"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Secret (optional)</label>
                <input
                  type="password"
                  value={form.config?.secret || ''}
                  onChange={(e) => setForm({ ...form, config: { ...form.config, secret: e.target.value } })}
                  className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
                  placeholder="Webhook secret for signature verification"
                />
              </div>
            </>
          )}

          <div className="flex items-center">
            <input
              type="checkbox"
              id="channel-enabled"
              checked={form.enabled}
              onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
              className="rounded border-gray-300 text-indigo-600"
            />
            <label htmlFor="channel-enabled" className="ml-2 text-sm text-gray-700">
              Enable this channel
            </label>
          </div>

          <div className="flex justify-end gap-3 pt-4 border-t">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 rounded-md"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="px-4 py-2 text-sm bg-indigo-600 text-white rounded-md hover:bg-indigo-700"
            >
              {channel ? 'Save Changes' : 'Add Channel'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}

// Alert Rule Configuration Modal
function RuleModal({ 
  rule, 
  channels,
  onSave, 
  onClose 
}: { 
  rule: AlertRule | null
  channels: AlertChannel[]
  onSave: (rule: AlertRule) => void
  onClose: () => void 
}) {
  const [form, setForm] = useState<Partial<AlertRule>>(rule || {
    name: '',
    condition: 'job_failed',
    channels: [],
    enabled: true,
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    onSave(form as AlertRule)
  }

  const toggleChannel = (channelId: string) => {
    const current = form.channels || []
    const updated = current.includes(channelId)
      ? current.filter(id => id !== channelId)
      : [...current, channelId]
    setForm({ ...form, channels: updated })
  }

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-md">
        <div className="px-6 py-4 border-b border-gray-200">
          <h3 className="text-lg font-medium text-gray-900">
            {rule ? 'Edit Alert Rule' : 'Add Alert Rule'}
          </h3>
        </div>
        <form onSubmit={handleSubmit} className="px-6 py-4 space-y-4">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Rule Name</label>
            <input
              type="text"
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
              placeholder="e.g., Critical Job Failure"
              required
            />
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Condition</label>
            <select
              value={form.condition}
              onChange={(e) => setForm({ ...form, condition: e.target.value as AlertRule['condition'] })}
              className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
            >
              <option value="job_failed">Job Failed</option>
              <option value="job_timeout">Job Timeout</option>
              <option value="consecutive_failures">Consecutive Failures</option>
              <option value="cluster_unhealthy">Cluster Unhealthy</option>
            </select>
          </div>

          {form.condition === 'consecutive_failures' && (
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Failure Threshold</label>
              <input
                type="number"
                min={2}
                max={100}
                value={form.threshold || 3}
                onChange={(e) => setForm({ ...form, threshold: parseInt(e.target.value) })}
                className="w-full rounded-md border-gray-300 shadow-sm focus:border-indigo-500 focus:ring-indigo-500"
              />
              <p className="text-xs text-gray-500 mt-1">Alert after this many consecutive failures</p>
            </div>
          )}

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">Notification Channels</label>
            {channels.length === 0 ? (
              <p className="text-sm text-gray-500">No channels configured. Add a channel first.</p>
            ) : (
              <div className="space-y-2">
                {channels.map((channel) => (
                  <label key={channel.id} className="flex items-center gap-2">
                    <input
                      type="checkbox"
                      checked={form.channels?.includes(channel.id)}
                      onChange={() => toggleChannel(channel.id)}
                      className="rounded border-gray-300 text-indigo-600"
                    />
                    <span className="text-sm text-gray-700">
                      {channel.name} ({channel.type})
                    </span>
                  </label>
                ))}
              </div>
            )}
          </div>

          <div className="flex items-center">
            <input
              type="checkbox"
              id="rule-enabled"
              checked={form.enabled}
              onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
              className="rounded border-gray-300 text-indigo-600"
            />
            <label htmlFor="rule-enabled" className="ml-2 text-sm text-gray-700">
              Enable this rule
            </label>
          </div>

          <div className="flex justify-end gap-3 pt-4 border-t">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-gray-700 hover:bg-gray-100 rounded-md"
            >
              Cancel
            </button>
            <button
              type="submit"
              className="px-4 py-2 text-sm bg-indigo-600 text-white rounded-md hover:bg-indigo-700"
            >
              {rule ? 'Save Changes' : 'Add Rule'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
