import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Badge, Text, TextField, Spinner, Select } from '@radix-ui/themes'
import { MagnifyingGlassIcon } from '@radix-ui/react-icons'
import { fetchPendingApplications, type OGAApplication } from '../api'



export function WorkflowListScreen() {
  const navigate = useNavigate()
  const [applications, setApplications] = useState<OGAApplication[]>([])
  const [loading, setLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('PENDING')

  useEffect(() => {
    async function fetchData() {
      try {
        const filterStatus = statusFilter === 'all' ? undefined : statusFilter
        const apps = await fetchPendingApplications(filterStatus)
        setApplications(apps)
      } catch (error) {
        console.error('Failed to fetch applications:', error)
      } finally {
        setLoading(false)
      }
    }

    void fetchData()

    // Poll for new applications every 10 seconds
    const interval = setInterval(fetchData, 10000)
    return () => clearInterval(interval)
  }, [statusFilter])

  const filteredApplications = applications.filter((app) => {
    return searchQuery === '' ||
      app.workflowId.toLowerCase().includes(searchQuery.toLowerCase())
  })

  // Format date: Jan 27, 2026
  const formatDateForTable = (dateString?: string) => {
    if (!dateString) return '-'
    // Format: Jan 27, 2026
    return new Date(dateString).toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric'
    })
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Spinner size="3" />
        <Text size="3" color="gray" className="ml-3">
          Loading applications...
        </Text>
      </div>
    )
  }

  return (
    <div className="animate-fade-in max-w-6xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">Workflows</h1>
          <p className="text-gray-500 text-sm mt-1">Manage and review trader workflows</p>
        </div>
        <div className="flex items-center gap-4">
          <Badge color="blue" variant="soft" size="2">
            {filteredApplications.length} Pending Review
          </Badge>
        </div>
      </div>

      <div className="space-y-4">
        {/* Search & Filter */}
        <div className="flex flex-col md:flex-row gap-4 mb-6">
          <div className="flex-1">
            <TextField.Root
              size="2"
              placeholder="Search by Workflow ID..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            >
              <TextField.Slot>
                <MagnifyingGlassIcon height="16" width="16" />
              </TextField.Slot>
            </TextField.Root>
          </div>
          <div className="flex gap-3">
            <Select.Root value={statusFilter} onValueChange={setStatusFilter}>
              <Select.Trigger placeholder="Status" />
              <Select.Content>
                <Select.Item value="all">All Statuses</Select.Item>
                <Select.Item value="PENDING">Pending</Select.Item>
                <Select.Item value="APPROVED">Approved</Select.Item>
                <Select.Item value="REJECTED">Rejected</Select.Item>
              </Select.Content>
            </Select.Root>
          </div>
        </div>

        <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
          {filteredApplications.length === 0 ? (
            <div className="p-12 text-center">
              <div className="bg-white w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-4 shadow-sm border border-gray-100">
                <ArchiveIcon className="w-8 h-8 text-gray-300" />
              </div>
              <Text size="3" color="gray" weight="medium">
                No workflows pending review at the moment.
              </Text>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="bg-gray-50/50 border-b border-gray-200 text-left">
                    <th className="px-6 py-4 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Workflow ID
                    </th>

                    <th className="px-6 py-4 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Status
                    </th>
                    <th className="px-6 py-4 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Submitted
                    </th>
                    <th className="px-6 py-4 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Last Updated
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200 bg-white">
                  {filteredApplications.map((app) => (
                    <tr
                      key={app.taskId}
                      onClick={() => { void navigate(`/workflows/${app.workflowId}?taskId=${app.taskId}`) }}
                      className="hover:bg-blue-50/30 cursor-pointer transition-colors group text-sm"
                    >
                      <td className="px-6 py-4 break-all font-mono text-blue-600 font-medium hover:underline">
                        {app.workflowId}
                      </td>

                      <td className="px-6 py-4 whitespace-nowrap">
                        <Badge
                          size="1"
                          color={
                            app.status === 'APPROVED' ? 'green' :
                              app.status === 'REJECTED' ? 'red' :
                                'blue'
                          }
                          variant="surface"
                        >
                          {app.status}
                        </Badge>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-gray-600">
                        {formatDateForTable(app.createdAt)}
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-gray-600">
                        {formatDateForTable(app.updatedAt)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}

function ArchiveIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      xmlns="http://www.w3.org/2000/svg"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 8h14M5 8a2 2 0 110-4h14a2 2 0 110 4M5 8v10a2 2 0 002 2h10a2 2 0 002-2V8m-9 4h4" />
    </svg>
  )
}
