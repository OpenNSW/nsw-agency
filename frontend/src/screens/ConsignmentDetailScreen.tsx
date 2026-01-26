import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Button, Badge, Spinner, Text, Card, Flex, Box, TextField, TextArea, Callout } from '@radix-ui/themes'
import { ArrowLeftIcon, CheckCircledIcon, ExclamationTriangleIcon, InfoCircledIcon } from '@radix-ui/react-icons'
import { fetchConsignmentDetail, approveTask, type ConsignmentDetail, type ApproveRequest } from '../api'

interface JsonSchema {
  properties?: Record<string, {
    type?: string;
    title?: string;
    description?: string;
  }>;
}

export function ConsignmentDetailScreen() {
  const { consignmentId } = useParams<{ consignmentId: string }>()
  const navigate = useNavigate()

  const [consignment, setConsignment] = useState<ConsignmentDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [formData, setFormData] = useState<Record<string, unknown>>({})
  const [reviewerName, setReviewerName] = useState('')
  const [decision, setDecision] = useState<'APPROVED' | 'REJECTED'>('APPROVED')
  const [comments, setComments] = useState('')
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  useEffect(() => {
    async function fetchData() {
      if (!consignmentId) return
      try {
        const data = await fetchConsignmentDetail(consignmentId)
        setConsignment(data)
      } catch (err) {
        setError('Failed to load application details')
        console.error(err)
      } finally {
        setLoading(false)
      }
    }
    fetchData()
  }, [consignmentId])

  const handleFormChange = (field: string, value: unknown) => {
    setFormData((prev) => ({ ...prev, [field]: value }))
  }

  const handleApprove = async () => {
    if (!reviewerName.trim()) {
      setError('Reviewer name is required')
      return
    }

    const ogaTask = consignment?.ogaTasks?.[0]
    if (!ogaTask) {
      setError('No pending OGA task found')
      return
    }

    setIsSubmitting(true)
    setError(null)

    try {
      const requestBody: ApproveRequest = {
        decision,
        comments: comments.trim() || undefined,
        reviewerName: reviewerName.trim(),
        formData: formData,
        consignmentId: consignment!.id,
      }
      await approveTask(ogaTask.id, requestBody)
      setSuccess(true)
      setTimeout(() => navigate('/consignments'), 2000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit review')
    } finally {
      setIsSubmitting(false)
    }
  }

  if (loading) {
    return (
      <Flex align="center" justify="center" py="9">
        <Spinner size="3" />
        <Text size="3" color="gray" ml="3">Loading application details...</Text>
      </Flex>
    )
  }

  if (!consignment) {
    return (
      <Box p="6">
        <Callout.Root color="red">
          <Callout.Icon><ExclamationTriangleIcon /></Callout.Icon>
          <Callout.Text>Consignment not found</Callout.Text>
        </Callout.Root>
        <Button variant="soft" mt="4" onClick={() => navigate('/consignments')}>
          <ArrowLeftIcon /> Back to List
        </Button>
      </Box>
    )
  }

  const ogaTask = consignment.ogaTasks?.[0]

  return (
    <div className="animate-fade-in max-w-5xl mx-auto">
      <Flex justify="between" align="center" mb="6">
        <Button variant="ghost" color="gray" onClick={() => navigate('/consignments')}>
          <ArrowLeftIcon /> Back to Consignments
        </Button>
        <Badge size="2" color={consignment.tradeFlow === 'IMPORT' ? 'blue' : 'green'} highContrast>
          {consignment.tradeFlow} APPLICATION
        </Badge>
      </Flex>

      {error && (
        <Callout.Root color="red" mb="6">
          <Callout.Icon><ExclamationTriangleIcon /></Callout.Icon>
          <Callout.Text>{error}</Callout.Text>
        </Callout.Root>
      )}

      {success && (
        <Callout.Root color="green" mb="6">
          <Callout.Icon><CheckCircledIcon /></Callout.Icon>
          <Callout.Text>Review submitted successfully! Redirecting...</Callout.Text>
        </Callout.Root>
      )}

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left Column: Info & Trader Submission */}
        <div className="lg:col-span-1 space-y-6">
          <Card size="2">
            <Text size="2" weight="bold" color="gray" mb="3" as="div" className="uppercase tracking-wider">
              Consignment Details
            </Text>
            <div className="space-y-4 mt-4">
              <Box>
                <Text size="1" color="gray" as="div" mb="1">Consignment ID</Text>
                <Text size="2" weight="medium" className="break-all">{consignment.id}</Text>
              </Box>
              <Box>
                <Text size="1" color="gray" as="div" mb="1">Trader ID</Text>
                <Text size="2" weight="medium">{consignment.traderId}</Text>
              </Box>
              <Box>
                <Text size="1" color="gray" as="div" mb="1">Current State</Text>
                <Badge color="orange">{consignment.state}</Badge>
              </Box>
              <Box>
                <Text size="1" color="gray" as="div" mb="1">Submitted On</Text>
                <Text size="2" weight="medium">{new Date(consignment.createdAt).toLocaleString()}</Text>
              </Box>
            </div>
          </Card>

        </div>

        {/* Right Column: Review Form */}
        <div className="lg:col-span-2">
          <Card size="3">
            <Flex align="center" gap="2" mb="4">
              <InfoCircledIcon className="text-primary-600 w-5 h-5" />
              <Text size="4" weight="bold">Officer Review Form</Text>
            </Flex>

            {!ogaTask ? (
              <Callout.Root color="amber">
                <Callout.Icon><ExclamationTriangleIcon /></Callout.Icon>
                <Callout.Text>This application is not currently pending an OGA review task.</Callout.Text>
              </Callout.Root>
            ) : (
              <div className="space-y-6 mt-6">
                {/* Trader Submission Section */}
                <div className="bg-gray-50 rounded-lg p-5 border border-gray-200">
                  <Text size="2" weight="bold" color="gray" mb="4" as="div" className="uppercase tracking-wider flex items-center gap-2">
                    <InfoCircledIcon />
                    Trader Submission Details
                  </Text>

                  {consignment.traderForm && Object.keys(consignment.traderForm).length > 0 ? (
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      {Object.entries(consignment.traderForm).map(([key, value]) => (
                        <Box key={key} className="bg-white p-3 rounded border border-gray-100">
                          <Text size="1" color="gray" as="div" className="capitalize mb-1">{key.replace(/([A-Z])/g, ' $1')}</Text>
                          <Text size="2" weight="medium">{String(value)}</Text>
                        </Box>
                      ))}
                    </div>
                  ) : (
                    <Text size="2" color="gray" className="italic text-center py-2">
                      No trader submission data available
                    </Text>
                  )}
                </div>

                <div className="border-t border-gray-100 my-4"></div>

                <Box>
                  <Text as="label" size="2" weight="bold" mb="1" className="block">Reviewer Name *</Text>
                  <TextField.Root
                    placeholder="Enter your full name"
                    value={reviewerName}
                    onChange={(e) => setReviewerName(e.target.value)}
                    disabled={isSubmitting || success}
                    size="3"
                  />
                </Box>

                {/* Dynamic Fields from OGA Form Schema */}
                {consignment.ogaForm && (
                  <div className="space-y-6 p-4 bg-blue-50/30 rounded-xl border border-blue-100/50">
                    <Text size="2" weight="bold" color="blue" mb="2" as="div" className="uppercase tracking-wider flex items-center gap-2">
                      <div className="w-1 h-4 bg-blue-500 rounded-full" />
                      Required Review Fields
                    </Text>

                    {(() => {
                      const schema = consignment.ogaForm.schema as unknown as JsonSchema
                      const properties = schema?.properties || {}

                      return Object.entries(properties).map(([key, fieldSchema]) => {
                        const fieldType = fieldSchema.type || 'string'
                        const fieldTitle = fieldSchema.title || key

                        return (
                          <Box key={key}>
                            <Text as="label" size="2" weight="bold" mb="1" className="block">{fieldTitle}</Text>
                            {fieldType === 'string' && (
                              <TextField.Root
                                value={(formData[key] as string) || ''}
                                onChange={(e) => handleFormChange(key, e.target.value)}
                                disabled={isSubmitting || success}
                                size="2"
                              />
                            )}
                            {fieldType === 'boolean' && (
                              <Flex align="center" gap="2">
                                <input
                                  type="checkbox"
                                  className="w-4 h-4 text-blue-600 rounded"
                                  checked={(formData[key] as boolean) || false}
                                  onChange={(e) => handleFormChange(key, e.target.checked)}
                                  disabled={isSubmitting || success}
                                />
                                <Text size="2">Confirmed</Text>
                              </Flex>
                            )}
                            {fieldSchema.description && (
                              <Text size="1" color="gray" mt="1" className="block italic">{fieldSchema.description}</Text>
                            )}
                          </Box>
                        )
                      })
                    })()}
                  </div>
                )}

                <Box>
                  <Text as="label" size="2" weight="bold" mb="1" className="block">Final Decision *</Text>
                  <Flex gap="4" mt="2">
                    <Button
                      size="3"
                      variant={decision === 'APPROVED' ? 'solid' : 'soft'}
                      color="green"
                      className="flex-1 cursor-pointer"
                      onClick={() => setDecision('APPROVED')}
                      disabled={isSubmitting || success}
                    >
                      Approve
                    </Button>
                    <Button
                      size="3"
                      variant={decision === 'REJECTED' ? 'solid' : 'soft'}
                      color="red"
                      className="flex-1 cursor-pointer"
                      onClick={() => setDecision('REJECTED')}
                      disabled={isSubmitting || success}
                    >
                      Reject
                    </Button>
                  </Flex>
                </Box>

                <Box>
                  <Text as="label" size="2" weight="bold" mb="1" className="block">Reviewer Comments</Text>
                  <TextArea
                    placeholder="Provide details about your decision..."
                    value={comments}
                    onChange={(e) => setComments(e.target.value)}
                    disabled={isSubmitting || success}
                    rows={4}
                    size="3"
                  />
                </Box>

                <div className="pt-4 border-t border-gray-100">
                  <Button
                    size="4"
                    className="w-full cursor-pointer"
                    onClick={handleApprove}
                    loading={isSubmitting}
                    disabled={success || !reviewerName.trim()}
                  >
                    {success ? 'Review Submitted' : 'Submit Final Review'}
                  </Button>
                </div>
              </div>
            )}
          </Card>
        </div>
      </div>
    </div>
  )
}
