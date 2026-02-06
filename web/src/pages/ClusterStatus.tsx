import { useQuery } from '@tanstack/react-query'
import { getClusterDetails } from '../api/client'
import { format } from 'date-fns'

export default function ClusterStatus() {
  const { data, isLoading, error, refetch } = useQuery({
    queryKey: ['cluster-details'],
    queryFn: getClusterDetails,
    refetchInterval: 10000, // Refresh every 10 seconds
  })

  if (isLoading) {
    return <div className="text-center py-12">Loading cluster status...</div>
  }

  if (error) {
    return (
      <div className="text-center py-12">
        <p className="text-red-600 mb-4">Error loading cluster status</p>
        <button
          onClick={() => refetch()}
          className="text-indigo-600 hover:text-indigo-800"
        >
          Retry
        </button>
      </div>
    )
  }

  const getNodeStatusColor = (state: string) => {
    switch (state.toLowerCase()) {
      case 'leader':
        return 'bg-green-100 text-green-800 border-green-200'
      case 'follower':
        return 'bg-blue-100 text-blue-800 border-blue-200'
      case 'candidate':
        return 'bg-yellow-100 text-yellow-800 border-yellow-200'
      default:
        return 'bg-gray-100 text-gray-800 border-gray-200'
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex justify-between items-center">
        <h2 className="text-2xl font-bold text-gray-900">Cluster Status</h2>
        <button
          onClick={() => refetch()}
          className="inline-flex items-center px-3 py-2 border border-gray-300 text-sm font-medium rounded-md text-gray-700 bg-white hover:bg-gray-50"
        >
          ↻ Refresh
        </button>
      </div>

      {/* Cluster Overview */}
      <div className="bg-white shadow rounded-lg overflow-hidden">
        <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
          <h3 className="text-lg font-medium text-gray-900">Cluster Overview</h3>
        </div>
        <div className="px-4 py-5 sm:p-6">
          <dl className="grid grid-cols-1 gap-5 sm:grid-cols-3">
            <div>
              <dt className="text-sm font-medium text-gray-500">Cluster State</dt>
              <dd className="mt-1 text-lg font-semibold text-gray-900">
                <span className={`inline-flex px-2 py-1 text-sm font-medium rounded ${
                  data?.state === 'healthy' 
                    ? 'bg-green-100 text-green-800' 
                    : 'bg-yellow-100 text-yellow-800'
                }`}>
                  {data?.state || 'Unknown'}
                </span>
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Current Leader</dt>
              <dd className="mt-1 text-lg font-semibold text-gray-900">
                {data?.leader || 'Unknown'}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Raft Term</dt>
              <dd className="mt-1 text-lg font-semibold text-gray-900">
                {data?.term || 0}
              </dd>
            </div>
          </dl>
        </div>
      </div>

      {/* Cluster Nodes */}
      <div className="bg-white shadow rounded-lg overflow-hidden">
        <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
          <h3 className="text-lg font-medium text-gray-900">Nodes ({data?.nodes?.length || 0})</h3>
        </div>
        <div className="divide-y divide-gray-200">
          {!data?.nodes || data.nodes.length === 0 ? (
            <div className="px-4 py-8 text-center text-gray-500">
              No nodes available
            </div>
          ) : (
            data.nodes.map((node) => (
              <div key={node.id} className="px-4 py-4 sm:px-6">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-4">
                    <span className={`inline-flex px-3 py-1 text-sm font-medium rounded-full border ${getNodeStatusColor(node.state)}`}>
                      {node.state}
                    </span>
                    <div>
                      <h4 className="text-sm font-medium text-gray-900">{node.id}</h4>
                      <p className="text-sm text-gray-500">{node.address}</p>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-sm text-gray-500">
                      Commit Index: {node.commitIndex}
                    </div>
                    <div className="text-sm text-gray-500">
                      Applied Index: {node.appliedIndex}
                    </div>
                  </div>
                </div>
                {node.lastContact && (
                  <div className="mt-2 text-xs text-gray-400">
                    Last contact: {format(new Date(node.lastContact), 'HH:mm:ss')}
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      </div>

      {/* Raft Stats */}
      <div className="bg-white shadow rounded-lg overflow-hidden">
        <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
          <h3 className="text-lg font-medium text-gray-900">Raft Statistics</h3>
        </div>
        <div className="px-4 py-5 sm:p-6">
          <dl className="grid grid-cols-2 gap-5 sm:grid-cols-4">
            <div>
              <dt className="text-sm font-medium text-gray-500">Commit Index</dt>
              <dd className="mt-1 text-xl font-semibold text-gray-900">
                {data?.commitIndex || 0}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Applied Index</dt>
              <dd className="mt-1 text-xl font-semibold text-gray-900">
                {data?.appliedIndex || 0}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Last Log Index</dt>
              <dd className="mt-1 text-xl font-semibold text-gray-900">
                {data?.lastLogIndex || 0}
              </dd>
            </div>
            <div>
              <dt className="text-sm font-medium text-gray-500">Snapshot Index</dt>
              <dd className="mt-1 text-xl font-semibold text-gray-900">
                {data?.snapshotIndex || 0}
              </dd>
            </div>
          </dl>
        </div>
      </div>

      {/* Storage Info */}
      {data?.storage && (
        <div className="bg-white shadow rounded-lg overflow-hidden">
          <div className="px-4 py-5 sm:px-6 border-b border-gray-200">
            <h3 className="text-lg font-medium text-gray-900">Storage (BadgerDB)</h3>
          </div>
          <div className="px-4 py-5 sm:p-6">
            <dl className="grid grid-cols-2 gap-5 sm:grid-cols-4">
              <div>
                <dt className="text-sm font-medium text-gray-500">Total Keys</dt>
                <dd className="mt-1 text-xl font-semibold text-gray-900">
                  {data.storage.totalKeys?.toLocaleString() || 0}
                </dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500">LSM Size</dt>
                <dd className="mt-1 text-xl font-semibold text-gray-900">
                  {formatBytes(data.storage.lsmSize || 0)}
                </dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500">VLog Size</dt>
                <dd className="mt-1 text-xl font-semibold text-gray-900">
                  {formatBytes(data.storage.vlogSize || 0)}
                </dd>
              </div>
              <div>
                <dt className="text-sm font-medium text-gray-500">Total Size</dt>
                <dd className="mt-1 text-xl font-semibold text-gray-900">
                  {formatBytes((data.storage.lsmSize || 0) + (data.storage.vlogSize || 0))}
                </dd>
              </div>
            </dl>
          </div>
        </div>
      )}
    </div>
  )
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`
}
