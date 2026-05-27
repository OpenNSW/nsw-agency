import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Badge, Text, TextField, Spinner, IconButton } from '@radix-ui/themes'
import { MagnifyingGlassIcon, ChevronLeftIcon, ChevronRightIcon, ArchiveIcon } from '@radix-ui/react-icons'
import { type ConsignmentSummary } from '../api'
import { useApi } from '../services/useApi'

const PAGE_SIZE = 20

export function ConsignmentListScreen() {
  const navigate = useNavigate()
  const { fetchConsignments } = useApi()
  const [consignments, setConsignments] = useState<ConsignmentSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState('')
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)

  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  useEffect(() => {
    async function fetchData(isSilent = false) {
      try {
        if (!isSilent) setLoading(true)
        const result = await fetchConsignments({
          page,
          pageSize: PAGE_SIZE,
          q: searchQuery,
        })
        setConsignments(result.items || [])
        setTotal(result.total || 0)
        // Reset to page 1 if current page is out of bounds
        const maxPages = Math.max(1, Math.ceil((result.total || 0) / PAGE_SIZE))
        if (page > maxPages) {
          setPage(1)
        }
      } catch (error) {
        console.error('Failed to fetch consignments:', error)
      } finally {
        if (!isSilent) setLoading(false)
      }
    }
    void fetchData()
    // Poll for new consignments every 15 seconds
    const interval = setInterval(() => void fetchData(true), 15000)
    return () => clearInterval(interval)
  }, [fetchConsignments, page, searchQuery])

  // Format date: Jan 27, 2026
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
          Loading consignments...
        </Text>
      </div>
    )
  }

  return (
    <div className="animate-fade-in max-w-6xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">Consignments</h1>
          <p className="text-gray-500 text-sm mt-1">Manage and review trader application groups</p>
        </div>
        <div className="flex items-center gap-4">
          <Badge color="blue" variant="soft" size="2">
            {total} Total Consignments
          </Badge>
        </div>
      </div>

      <div className="space-y-4">
        {/* Search */}
        <div className="flex flex-col md:flex-row gap-4 mb-6">
          <div className="flex-1">
            <TextField.Root
              size="2"
              placeholder="Search by Consignment ID..."
              value={searchQuery}
              onChange={(e) => {
                setSearchQuery(e.target.value)
                setPage(1)
              }}
            >
              <TextField.Slot>
                <MagnifyingGlassIcon height="16" width="16" />
              </TextField.Slot>
            </TextField.Root>
          </div>
        </div>

        <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
          {consignments.length === 0 ? (
            <div className="p-12 text-center">
              <div className="bg-white w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-4 shadow-sm border border-gray-100">
                <ArchiveIcon className="w-8 h-8 text-gray-300" />
              </div>
              <Text size="3" color="gray" weight="medium">
                No active consignments found.
              </Text>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full">
                <thead>
                  <tr className="bg-gray-50/50 border-b border-gray-200 text-left">
                    <th className="px-6 py-4 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Consignment ID
                    </th>
                    <th className="px-6 py-4 text-xs font-semibold text-gray-500 uppercase tracking-wider text-center">
                      Tasks
                    </th>
                    <th className="px-6 py-4 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Latest Status
                    </th>
                    <th className="px-6 py-4 text-xs font-semibold text-gray-500 uppercase tracking-wider">
                      Last Activity
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-200 bg-white">
                  {consignments.map((consignment) => (
                    <tr
                      key={consignment.consignmentId}
                      onClick={() => {
                        void navigate(`/consignments/${consignment.consignmentId}/tasks`)
                      }}
                      className="hover:bg-blue-50/30 cursor-pointer transition-colors group text-sm"
                    >
                      <td className="px-6 py-4 break-all font-mono text-blue-600 font-medium hover:underline">
                        {consignment.consignmentId}
                      </td>

                      <td className="px-6 py-4 whitespace-nowrap text-center">{consignment.taskCount}</td>

                      <td className="px-6 py-4 whitespace-nowrap">
                        <Badge
                          size="1"
                          color={
                            consignment.status === 'APPROVED'
                              ? 'green'
                              : consignment.status === 'REJECTED'
                                ? 'red'
                                : consignment.status === 'FEEDBACK_REQUESTED'
                                  ? 'amber'
                                  : 'blue'
                          }
                          variant="surface"
                        >
                          {consignment.status}
                        </Badge>
                      </td>
                      <td className="px-6 py-4 whitespace-nowrap text-gray-600">
                        {formatDateForTable(consignment.updatedAt)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between pt-2">
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
    </div>
  )
}
