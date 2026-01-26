import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Badge, Text, TextField, Spinner, Select } from '@radix-ui/themes'
import { MagnifyingGlassIcon } from '@radix-ui/react-icons'
import { fetchConsignmentsWithOGATasks, type Consignment } from '../api'

function getStatusColor(status: string): 'gray' | 'blue' | 'orange' | 'green' | 'red' {
  switch (status) {
    case 'PENDING':
      return 'blue'
    case 'IN_PROGRESS':
      return 'orange'
    case 'COMPLETED':
      return 'green'
    case 'REJECTED':
      return 'red'
    default:
      return 'gray'
  }
}

function formatStatus(status: string): string {
  return status.replace('_', ' ').replace(/\b\w/g, (c) => c.toUpperCase())
}

function formatDate(dateString: string): string {
  return new Date(dateString).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })
}

export function ConsignmentListScreen() {
  const navigate = useNavigate()
  const [consignments, setConsignments] = useState<Consignment[]>([])
  const [loading, setLoading] = useState(true)
  const [searchQuery, setSearchQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<string>('all')

  useEffect(() => {
    async function fetchData() {
      try {
        const data = await fetchConsignmentsWithOGATasks()
        setConsignments(data)
      } catch (error) {
        console.error('Failed to fetch consignments:', error)
      } finally {
        setLoading(false)
      }
    }

    fetchData()

    // Poll for new consignments
    const interval = setInterval(fetchData, 10000)
    return () => clearInterval(interval)
  }, [])

  const filteredConsignments = consignments.filter((c) => {
    const matchesSearch =
      searchQuery === '' ||
      c.id.toLowerCase().includes(searchQuery.toLowerCase()) ||
      c.traderId.toLowerCase().includes(searchQuery.toLowerCase())

    const matchesStatus = statusFilter === 'all' || c.state === statusFilter

    return matchesSearch && matchesStatus
  })

  if (loading) {
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
    <div className="animate-fade-in">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-semibold text-gray-900">Pending Reviews</h1>
          <p className="text-gray-500 text-sm mt-1">Manage and review trader consignments</p>
        </div>
        <div className="flex items-center gap-4">
          <Text size="2" color="gray">
            {filteredConsignments.length} consignments pending
          </Text>
        </div>
      </div>

      <div className="bg-white rounded-xl shadow-sm border border-gray-200 overflow-hidden">
        <div className="p-4 border-b border-gray-200 bg-gray-50/50">
          <div className="flex flex-col md:flex-row gap-4">
            <div className="flex-1">
              <TextField.Root
                size="2"
                placeholder="Search by ID or Trader..."
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
                  <Select.Item value="IN_PROGRESS">In Progress</Select.Item>
                  <Select.Item value="PENDING">Pending</Select.Item>
                  <Select.Item value="COMPLETED">Completed</Select.Item>
                </Select.Content>
              </Select.Root>
            </div>
          </div>
        </div>

        {filteredConsignments.length === 0 ? (
          <div className="p-12 text-center">
            <div className="bg-gray-50 w-16 h-16 rounded-full flex items-center justify-center mx-auto mb-4">
              <ArchiveIcon className="w-8 h-8 text-gray-300" />
            </div>
            <Text size="3" color="gray" weight="medium">
              No consignments pending review at the moment.
            </Text>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className="bg-gray-50/50 border-b border-gray-200">
                  <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                    Consignment ID
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                    Trader ID
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                    Trade Flow
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                    Status
                  </th>
                  <th className="px-6 py-3 text-left text-xs font-semibold text-gray-500 uppercase tracking-wider">
                    Submitted
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-gray-200 bg-white">
                {filteredConsignments.map((consignment) => (
                  <tr
                    key={consignment.id}
                    onClick={() => navigate(`/consignments/${consignment.id}`)}
                    className="hover:bg-blue-50/30 cursor-pointer transition-colors group"
                  >
                    <td className="px-6 py-4 whitespace-nowrap">
                      <Text size="2" weight="bold" className="text-primary-600 group-hover:text-primary-700">
                        {consignment.id.substring(0, 8)}...
                      </Text>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <Text size="2" color="gray">
                        {consignment.traderId}
                      </Text>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <Badge
                        size="1"
                        color={consignment.tradeFlow === 'IMPORT' ? 'blue' : 'green'}
                        variant="soft"
                        highContrast
                      >
                        {consignment.tradeFlow}
                      </Badge>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <Badge size="1" color={getStatusColor(consignment.state)} variant="surface">
                        {formatStatus(consignment.state)}
                      </Badge>
                    </td>
                    <td className="px-6 py-4 whitespace-nowrap">
                      <Text size="2" color="gray">
                        {formatDate(consignment.createdAt)}
                      </Text>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
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
