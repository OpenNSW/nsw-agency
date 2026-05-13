import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Badge, Text, Spinner, IconButton, Button, Flex } from '@radix-ui/themes'
import { ChevronLeftIcon, ChevronRightIcon, ArrowLeftIcon, ArchiveIcon } from '@radix-ui/react-icons'
import { fetchApplications, type OGAApplication } from '../api'
import { useApi } from '../services/useApi'

const PAGE_SIZE = 20

export function WorkflowTasksScreen() {
  const { workflowId } = useParams<{ workflowId: string }>()
  const navigate = useNavigate()
  const apiClient = useApi()
  const [applications, setApplications] = useState<OGAApplication[]>([])
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  useEffect(() => {
    async function fetchData() {
      if (!workflowId) return
      try {
        setLoading(true)
        const result = await fetchApplications(apiClient, {
          workflowId,
          page,
          pageSize: PAGE_SIZE,
        })
        setApplications(result.items)
        setTotal(result.total)
      } catch (error) {
        console.error('Failed to fetch tasks:', error)
      } finally {
        setLoading(false)
      }
    }

    void fetchData()
  }, [apiClient, workflowId, page])

  const formatDateForTable = (dateString?: string) => {
    if (!dateString) return '-'
    return new Date(dateString).toLocaleDateString('en-US', {
      month: 'short',
      day: 'numeric',
      year: 'numeric',
    })
  }

  if (loading && page === 1) {
    return (
      <div className="flex items-center justify-center py-12">
        <Spinner size="3" />
        <Text size="3" color="gray" className="ml-3">
          Loading tasks...
        </Text>
      </div>
    )
  }

  return (
    <div className="animate-fade-in max-w-6xl mx-auto">
      <div className="mb-6">
        <Button
          variant="ghost"
          onClick={() => navigate('/workflows')}
          className="mb-4 -ml-2 text-gray-600 hover:text-blue-600"
        >
          <ArrowLeftIcon /> Back to Consignments
        </Button>
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold text-gray-900">Consignment Tasks</h1>
            <Text size="2" color="gray" className="font-mono">
              ConsignmentId: {workflowId}
            </Text>
          </div>
          <Badge color="blue" variant="soft" size="2">
            {total} Total Tasks
          </Badge>
        </div>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
        {total === 0 ? (
          <div className="p-12 text-center">
            <div className="bg-white w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-4 shadow-sm border border-gray-100">
              <ArchiveIcon className="w-8 h-8 text-gray-300" />
            </div>
            <Text size="3" color="gray" weight="medium">
              No tasks found for this consignment.
            </Text>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-gray-50/50 border-b border-gray-200 text-left">
                  <th className="px-6 py-4 text-xs font-semibold text-gray-500 uppercase tracking-wider">Task</th>
                  <th className="px-6 py-4 text-xs font-semibold text-gray-500 uppercase tracking-wider">Category</th>
                  <th className="px-6 py-4 text-xs font-semibold text-gray-500 uppercase tracking-wider">Status</th>
                  <th className="px-6 py-4 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                    Last Updated
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 bg-white">
                {applications.map((app) => (
                  <tr
                    key={app.taskId}
                    onClick={() => {
                      void navigate(`/workflows/${app.workflowId}?taskId=${app.taskId}`)
                    }}
                    className="hover:bg-blue-50/30 cursor-pointer transition-colors group text-sm"
                  >
                    <td className="px-6 py-4 whitespace-nowrap">
                      <Flex align="center" gap="2">
                        {app.icon?.startsWith('emoji:') && (
                          <span className="text-xl" role="img" aria-label="task-icon">
                            {app.icon.slice('emoji:'.length)}
                          </span>
                        )}
                        <Text size="2" weight="bold" className="text-gray-900">
                          {app.title || 'Standard Review'}
                        </Text>
                      </Flex>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      {app.category && (
                        <Text size="1" color="gray" className="uppercase tracking-tight">
                          {app.category}
                        </Text>
                      )}
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <Badge
                        size="1"
                        color={
                          app.status === 'APPROVED'
                            ? 'green'
                            : app.status === 'REJECTED'
                              ? 'red'
                              : app.status === 'FEEDBACK_REQUESTED'
                                ? 'amber'
                                : 'blue'
                        }
                        variant="surface"
                      >
                        {app.status}
                      </Badge>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap text-gray-600">{formatDateForTable(app.updatedAt)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between pt-4">
          <Text size="2" color="gray">
            Page {page} of {totalPages} ({total} results)
          </Text>
          <div className="flex items-center gap-2">
            <IconButton size="1" variant="soft" disabled={page <= 1} onClick={() => setPage((p) => p - 1)}>
              <ChevronLeftIcon />
            </IconButton>
            <IconButton size="1" variant="soft" disabled={page >= totalPages} onClick={() => setPage((p) => p + 1)}>
              <ChevronRightIcon />
            </IconButton>
          </div>
        </div>
      )}
    </div>
  )
}
