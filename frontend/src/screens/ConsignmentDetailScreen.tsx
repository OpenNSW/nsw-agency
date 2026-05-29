import { useState, useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Button, Badge, Spinner, Text, Card, Flex, Box, Callout } from '@radix-ui/themes'
import {
  ArrowLeftIcon,
  CheckCircledIcon,
  ExclamationTriangleIcon,
  InfoCircledIcon,
  ChatBubbleIcon,
} from '@radix-ui/react-icons'
import { type AgencyApplication } from '../services/types'
import { JsonForms } from '@jsonforms/react'
import { radixRenderers } from '@opennsw/jsonforms-renderers'
import type { JsonSchema, UISchemaElement } from '@jsonforms/core'
import { fetchApplicationDetail, submitReview } from '../services/applications'

interface SchemaOption {
  const: unknown
  title?: string
}

interface SchemaProperty {
  oneOf?: SchemaOption[]
  enum?: string[]
}

export function ConsignmentDetailScreen() {
  const navigate = useNavigate()

  const [searchParams] = useSearchParams()
  const taskId = searchParams.get('taskId')

  const [application, setApplication] = useState<AgencyApplication | null>(null)
  const [loading, setLoading] = useState(true)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const [agencyFormConfig, setAgencyFormConfig] = useState<{ schema: JsonSchema; uiSchema: UISchemaElement } | null>(
    null,
  )
  const [agencyFormData, setAgencyFormData] = useState<Record<string, unknown>>({})
  const [formErrors, setFormErrors] = useState<unknown[]>([])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!taskId || !application) {
      setError('Application data not available')
      return
    }
    if (formErrors.length > 0) {
      setError('Please fix validation errors before submitting.')
      return
    }
    setIsSubmitting(true)
    setError(null)
    try {
      await submitReview(taskId, agencyFormData)
      setSuccess(true)
      setTimeout(() => navigate('/consignments'), 500)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to submit review')
    } finally {
      setIsSubmitting(false)
    }
  }

  useEffect(() => {
    async function fetchData() {
      if (!taskId) {
        setError('No task ID provided')
        setLoading(false)
        return
      }
      try {
        const data = await fetchApplicationDetail(taskId)
        setApplication(data)
        if (data.agencyForm) {
          const schema = structuredClone(data.agencyForm.schema)
          const capitalizeOptions = (prop: SchemaProperty) => {
            if (prop.oneOf) {
              prop.oneOf = prop.oneOf.map((opt) => {
                const titleVal = opt.title || String(opt.const)
                const formattedTitle = titleVal
                  .split(/[_\s]+/)
                  .map((word) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
                  .join(' ')
                return { ...opt, title: formattedTitle }
              })
            } else if (prop.enum) {
              prop.oneOf = prop.enum.map((val: string) => {
                const title = val
                  .split(/[_\s]+/)
                  .map((word) => word.charAt(0).toUpperCase() + word.slice(1).toLowerCase())
                  .join(' ')
                return { const: val, title }
              })
              delete prop.enum
            }
          }
          if (schema.properties) {
            Object.values(schema.properties).forEach((prop) => {
              capitalizeOptions(prop as SchemaProperty)
            })
          }
          setAgencyFormConfig({ schema, uiSchema: data.agencyForm.uiSchema })
        } else {
          setAgencyFormConfig(null)
        }
        setAgencyFormData(data.agencyActionData || {})
      } catch (err) {
        setError('Failed to load application details')
        console.error(err)
      } finally {
        setLoading(false)
      }
    }
    void fetchData()
  }, [taskId])

  if (loading) {
    return (
      <Flex align="center" justify="center" py="9">
        <Spinner size="3" />
        <Text size="3" color="gray" ml="3">
          Loading application details...
        </Text>
      </Flex>
    )
  }

  if (error && !application) {
    return (
      <Box p="6">
        <Callout.Root color="red">
          <Callout.Icon>
            <ExclamationTriangleIcon />
          </Callout.Icon>
          <Callout.Text>{error}</Callout.Text>
        </Callout.Root>
        <Button
          variant="soft"
          mt="4"
          onClick={() => {
            void navigate('/consignments')
          }}
        >
          <ArrowLeftIcon /> Back to List
        </Button>
      </Box>
    )
  }

  if (!application) {
    return (
      <Box p="6">
        <Callout.Root color="red">
          <Callout.Icon>
            <ExclamationTriangleIcon />
          </Callout.Icon>
          <Callout.Text>Application not found</Callout.Text>
        </Callout.Root>
        <Button
          variant="soft"
          mt="4"
          onClick={() => {
            void navigate('/consignments')
          }}
        >
          <ArrowLeftIcon /> Back to List
        </Button>
      </Box>
    )
  }

  const isActionable = application.status === 'PENDING'

  const statusColor =
    application.status === 'APPROVED'
      ? 'green'
      : application.status === 'REJECTED'
        ? 'red'
        : application.status === 'FEEDBACK_REQUESTED'
          ? 'amber'
          : 'blue'

  return (
    <div className="animate-fade-in max-w-6xl mx-auto">
      <Flex justify="between" align="center" mb="6">
        <Button
          variant="ghost"
          color="gray"
          onClick={() => {
            void navigate(`/consignments/${application.consignmentId}/tasks`)
          }}
        >
          <ArrowLeftIcon /> Back to Tasks
        </Button>
        <Badge size="2" color={statusColor} highContrast>
          {application.status}
        </Badge>
      </Flex>

      <Box mb="6">
        <Flex align="center" gap="3" mb="1">
          {application.icon?.startsWith('emoji:') && (
            <span className="text-3xl" role="img" aria-label="task-icon">
              {application.icon.slice('emoji:'.length)}
            </span>
          )}
          <h1 className="text-2xl font-bold text-gray-900">{application.title || 'Task Review'}</h1>
        </Flex>
        {application.description && (
          <Text size="2" color="gray">
            {application.description}
          </Text>
        )}
      </Box>

      {error && (
        <Callout.Root color="red" mb="6">
          <Callout.Icon>
            <ExclamationTriangleIcon />
          </Callout.Icon>
          <Callout.Text>{error}</Callout.Text>
        </Callout.Root>
      )}

      {success && (
        <Callout.Root color="green" mb="6">
          <Callout.Icon>
            <CheckCircledIcon />
          </Callout.Icon>
          <Callout.Text>Review submitted successfully! Redirecting...</Callout.Text>
        </Callout.Root>
      )}

      {(application.status === 'APPROVED' || application.status === 'REJECTED') && (
        <Callout.Root color={application.status === 'APPROVED' ? 'green' : 'red'} mb="6">
          <Callout.Icon>
            {application.status === 'APPROVED' ? <CheckCircledIcon /> : <ExclamationTriangleIcon />}
          </Callout.Icon>
          <Callout.Text>This application has been {application.status.toLowerCase()}.</Callout.Text>
        </Callout.Root>
      )}

      <div className="space-y-6">
        {/* Main Column */}
        <div className="space-y-6">
          {/* Review form is now at the very top of the page */}
          <Box className="bg-white rounded-lg p-5 border border-gray-200">
            <Text
              size="2"
              weight="bold"
              color="gray"
              mb="3"
              as="div"
              className="uppercase tracking-wider flex items-center gap-2"
            >
              <InfoCircledIcon />
              Review
            </Text>
            {agencyFormConfig && isActionable ? (
              <form
                onSubmit={(event) => {
                  void handleSubmit(event)
                }}
                noValidate
              >
                <JsonForms
                  schema={agencyFormConfig.schema}
                  uischema={agencyFormConfig.uiSchema}
                  data={agencyFormData}
                  renderers={radixRenderers}
                  onChange={({ data, errors }: { data: Record<string, unknown>; errors?: unknown[] }) => {
                    setAgencyFormData(data)
                    setFormErrors(errors || [])
                  }}
                />
                <Flex justify="end" gap="3" mt="6">
                  <Button
                    variant="soft"
                    color="gray"
                    onClick={() => {
                      void navigate('/consignments')
                    }}
                    disabled={isSubmitting}
                    type="button"
                  >
                    Cancel
                  </Button>
                  <Button type="submit" disabled={isSubmitting}>
                    {isSubmitting ? <Spinner size="1" /> : null}
                    Submit Review
                  </Button>
                </Flex>
              </form>
            ) : agencyFormConfig ? (
              <JsonForms
                schema={agencyFormConfig.schema}
                uischema={agencyFormConfig.uiSchema}
                data={agencyFormData}
                renderers={radixRenderers}
                readonly
                onChange={({ data, errors }: { data: Record<string, unknown>; errors?: unknown[] }) => {
                  setAgencyFormData(data)
                  setFormErrors(errors || [])
                }}
              />
            ) : null}
          </Box>

          <Card size="2">
            <Text size="2" weight="bold" color="gray" mb="3" as="div" className="uppercase tracking-wider">
              Application Details
            </Text>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mt-4">
              <Box>
                <Text size="1" color="gray" as="div" mb="1">
                  Consignment ID
                </Text>
                <Text size="2" weight="medium" className="break-all font-mono">
                  {application.consignmentId}
                </Text>
              </Box>
              <Box>
                <Text size="1" color="gray" as="div" mb="1">
                  Status
                </Text>
                <Badge size="2" color={statusColor}>
                  {application.status}
                </Badge>
              </Box>
              <Box>
                <Text size="1" color="gray" as="div" mb="1">
                  Submitted On
                </Text>
                <Text size="2" weight="medium">
                  {(() => {
                    const date = new Date(application.createdAt)
                    return `${date.toLocaleDateString(undefined, {
                      month: 'long',
                      day: 'numeric',
                      year: 'numeric',
                    })} at ${date.toLocaleTimeString(undefined, {
                      hour: '2-digit',
                      minute: '2-digit',
                      hour12: true,
                    })}`
                  })()}
                </Text>
              </Box>
            </div>
          </Card>

          <Box className="bg-white rounded-lg p-5 border border-gray-200">
            <Text
              size="2"
              weight="bold"
              color="gray"
              mb="3"
              as="div"
              className="uppercase tracking-wider flex items-center gap-2"
            >
              <InfoCircledIcon />
              Submitted Information
            </Text>
            {(() => {
              if (application.dataForm) {
                return (
                  <JsonForms
                    schema={application.dataForm.schema}
                    uischema={application.dataForm.uiSchema}
                    data={application.data}
                    renderers={radixRenderers}
                    readonly={true}
                  />
                )
              }

              if (application.data && Object.keys(application.data).length > 0) {
                return (
                  <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                    {Object.entries(application.data).map(([key, value]) => (
                      <Box key={key}>
                        <Text size="1" color="gray" as="div" className="capitalize mb-1">
                          {key.replace(/([A-Z])/g, ' $1').replace(/_/g, ' ')}
                        </Text>
                        <Text size="2" weight="medium">
                          {typeof value === 'object' && value !== null ? JSON.stringify(value) : String(value)}
                        </Text>
                      </Box>
                    ))}
                  </div>
                )
              }

              return (
                <Text size="2" color="gray" className="italic">
                  No submission data available
                </Text>
              )
            })()}
          </Box>
        </div>

        {/* Sidebar elements now at the bottom of the main flow */}
        <div className="space-y-6">
          {application.reviewedAt && (
            <Card size="2">
              <Text size="2" weight="bold" color="gray" mb="3" as="div" className="uppercase tracking-wider">
                Review Metadata
              </Text>
              <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mt-3">
                <Box>
                  <Text size="1" color="gray" as="div" mb="1">
                    Reviewed On
                  </Text>
                  <Text size="2" weight="medium">
                    {new Date(application.reviewedAt).toLocaleString()}
                  </Text>
                </Box>
              </div>
            </Card>
          )}

          {application.reviewerNotes && application.status !== 'PENDING' && (
            <Card size="2">
              <Text size="2" weight="bold" color="gray" mb="3" as="div" className="uppercase tracking-wider">
                Reviewer Notes
              </Text>
              <Text size="2" className="whitespace-pre-wrap">
                {application.reviewerNotes}
              </Text>
            </Card>
          )}

          {application.feedbackHistory && application.feedbackHistory.length > 0 && (
            <Box className="bg-white rounded-lg p-5 border border-gray-200">
              <Text
                size="2"
                weight="bold"
                color="gray"
                mb="3"
                as="div"
                className="uppercase tracking-wider flex items-center gap-2"
              >
                <ChatBubbleIcon />
                Feedback History
              </Text>
              <div className="divide-y divide-gray-100">
                {application.feedbackHistory.map((entry) => (
                  <div key={entry.round} className="py-3 first:pt-0 last:pb-0">
                    <Flex justify="between" mb="1">
                      <Text size="1" weight="bold" color="amber">
                        Round {entry.round}
                      </Text>
                      <Text size="1" color="gray">
                        {new Date(entry.timestamp).toLocaleString()}
                      </Text>
                    </Flex>
                    <Text size="2" className="whitespace-pre-wrap">
                      {entry.content.feedback as string}
                    </Text>
                  </div>
                ))}
              </div>
            </Box>
          )}
        </div>
      </div>
    </div>
  )
}
