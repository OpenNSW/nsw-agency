import { useState, useEffect } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { Button, Badge, Spinner, Text, Card, Flex, Box, Callout, Tabs } from '@radix-ui/themes'
import { ArrowLeftIcon, CheckCircledIcon, ExclamationTriangleIcon, InfoCircledIcon, ChatBubbleIcon } from '@radix-ui/react-icons'
import { fetchApplicationDetail, submitReview, submitFeedback, type OGAApplication } from '../api'
import { JsonForms } from '@jsonforms/react';
import { radixRenderers } from '@opennsw/jsonforms-renderers';
import type { JsonSchema, UISchemaElement } from '@jsonforms/core';
import { useApi } from '../services/useApi'

export function WorkflowDetailScreen() {
  const navigate = useNavigate()
  const apiClient = useApi()

  const [searchParams] = useSearchParams()
  const taskId = searchParams.get('taskId')

  const [application, setApplication] = useState<OGAApplication | null>(null)
  const [loading, setLoading] = useState(true)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)

  const [ogaFormConfig, setOgaFormConfig] = useState<{ schema: JsonSchema; uiSchema: UISchemaElement } | null>(null)
  const [ogaFormData, setOgaFormData] = useState<Record<string, unknown>>({})
  const [formErrors, setFormErrors] = useState<unknown[]>([])

  const [activeTab, setActiveTab] = useState('review')
  const [showFeedbackInput, setShowFeedbackInput] = useState(false)
  const [feedbackText, setFeedbackText] = useState('')
  const [isSendingFeedback, setIsSendingFeedback] = useState(false)

  const handleSendFeedback = async () => {
    if (!taskId || !feedbackText.trim()) return
    setIsSendingFeedback(true)
    setError(null)
    try {
      await submitFeedback(apiClient, taskId, { feedback: feedbackText.trim() })
      setSuccess(true)
      setTimeout(() => navigate('/workflows'), 2000)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to send feedback')
    } finally {
      setIsSendingFeedback(false)
    }
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!taskId || !application) {
      setError('Application data not available')
      return
    }
    if (formErrors.length > 0) {
      setError('Please fix validation errors before submitting.');
      return;
    }
    setIsSubmitting(true)
    setError(null)
    try {
      await submitReview(apiClient, taskId, ogaFormData)
      setSuccess(true)
      setTimeout(() => navigate('/workflows'), 2000)
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
        const data = await fetchApplicationDetail(apiClient, taskId)
        setApplication(data)
        if (data.ogaForm) {
          setOgaFormConfig({ schema: data.ogaForm.schema, uiSchema: data.ogaForm.uiSchema })
        } else {
          setOgaFormConfig(null)
        }
        setOgaFormData(data.ogaActionData || {})
      } catch (err) {
        setError('Failed to load application details')
        console.error(err)
      } finally {
        setLoading(false)
      }
    }
    void fetchData()
  }, [apiClient, taskId])

  if (loading) {
    return (
      <Flex align="center" justify="center" py="9">
        <Spinner size="3" />
        <Text size="3" color="gray" ml="3">Loading application details...</Text>
      </Flex>
    )
  }

  if (error && !application) {
    return (
      <Box p="6">
        <Callout.Root color="red">
          <Callout.Icon><ExclamationTriangleIcon /></Callout.Icon>
          <Callout.Text>{error}</Callout.Text>
        </Callout.Root>
        <Button variant="soft" mt="4" onClick={() => { void navigate('/workflows') }}>
          <ArrowLeftIcon /> Back to List
        </Button>
      </Box>
    )
  }

  if (!application) {
    return (
      <Box p="6">
        <Callout.Root color="red">
          <Callout.Icon><ExclamationTriangleIcon /></Callout.Icon>
          <Callout.Text>Application not found</Callout.Text>
        </Callout.Root>
        <Button variant="soft" mt="4" onClick={() => { void navigate('/workflows') }}>
          <ArrowLeftIcon /> Back to List
        </Button>
      </Box>
    )
  }

  const isActionable = application.status === 'PENDING'
  const feedbackCount = application.feedbackHistory?.length ?? 0

  const statusColor = application.status === 'APPROVED' ? 'green'
    : application.status === 'REJECTED' ? 'red'
      : application.status === 'FEEDBACK_REQUESTED' ? 'amber'
        : 'blue'

  return (
    <div className="animate-fade-in max-w-5xl mx-auto">
      <Flex justify="between" align="center" mb="6">
        <Button variant="ghost" color="gray" onClick={() => { void navigate('/workflows') }}>
          <ArrowLeftIcon /> Back to Workflows
        </Button>
        <Badge size="2" color={statusColor} highContrast>
          {application.status}
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
        {/* Left Column: Info */}
        <div className="lg:col-span-1 space-y-6">
          <Card size="2">
            <Text size="2" weight="bold" color="gray" mb="3" as="div" className="uppercase tracking-wider">
              Application Details
            </Text>
            <div className="space-y-4 mt-4">
              <Box>
                <Text size="1" color="gray" as="div" mb="1">Workflow ID</Text>
                <Text size="2" weight="medium" className="break-all font-mono">{application.workflowId}</Text>
              </Box>
              <Box>
                <Text size="1" color="gray" as="div" mb="1">Status</Text>
                <Badge size="2" color={statusColor}>{application.status}</Badge>
              </Box>
              <Box>
                <Text size="1" color="gray" as="div" mb="1">Submitted On</Text>
                <Text size="2" weight="medium">
                  {(() => {
                    const date = new Date(application.createdAt)
                    const datePart = date.toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' })
                    const timePart = date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: true })
                    return `${datePart} at ${timePart}`
                  })()}
                </Text>
              </Box>
              {application.reviewedAt && (
                <Box>
                  <Text size="1" color="gray" as="div" mb="1">Reviewed On</Text>
                  <Text size="2" weight="medium">
                    {(() => {
                      const date = new Date(application.reviewedAt)
                      const datePart = date.toLocaleDateString('en-US', { month: 'long', day: 'numeric', year: 'numeric' })
                      const timePart = date.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: true })
                      return `${datePart} at ${timePart}`
                    })()}
                  </Text>
                </Box>
              )}
            </div>
          </Card>

          {application.reviewerNotes && application.status !== 'PENDING' && (
            <Card size="2">
              <Text size="2" weight="bold" color="gray" mb="3" as="div" className="uppercase tracking-wider">
                Reviewer Notes
              </Text>
              <Text size="2" className="whitespace-pre-wrap">{application.reviewerNotes}</Text>
            </Card>
          )}
        </div>

        {/* Right Column: Tabbed content */}
        <div className="lg:col-span-2">
          <Card size="3">
            {(application.status === 'APPROVED' || application.status === 'REJECTED') && (
              <Callout.Root color={application.status === 'APPROVED' ? 'green' : 'red'} mb="4">
                <Callout.Icon>
                  {application.status === 'APPROVED' ? <CheckCircledIcon /> : <ExclamationTriangleIcon />}
                </Callout.Icon>
                <Callout.Text>
                  This application has been {application.status.toLowerCase()}.
                </Callout.Text>
              </Callout.Root>
            )}

            {/* Submitted Information — always visible, above the tab bar */}
            <div className="bg-gray-50 rounded-lg p-5 border border-gray-200 mb-5">
              <Text size="2" weight="bold" color="gray" mb="4" as="div"
                className="uppercase tracking-wider flex items-center gap-2">
                <InfoCircledIcon />
                Submitted Information
              </Text>
              {(() => {
                if (application.dataForm) {
                  return (
                    <Box className="bg-white p-4 rounded border border-gray-100">
                      <JsonForms
                        schema={application.dataForm.schema}
                        uischema={application.dataForm.uiSchema}
                        data={application.data}
                        renderers={radixRenderers}
                        readonly={true}
                      />
                    </Box>
                  );
                }

                if (application.data && Object.keys(application.data).length > 0) {
                  return (
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      {Object.entries(application.data).map(([key, value]) => (
                        <Box key={key} className="bg-white p-3 rounded border border-gray-100">
                          <Text size="1" color="gray" as="div" className="capitalize mb-1">
                            {key.replace(/([A-Z])/g, ' $1').replace(/_/g, ' ')}
                          </Text>
                          <Text size="2" weight="medium">
                            {typeof value === 'object' && value !== null ? JSON.stringify(value) : String(value)}
                          </Text>
                        </Box>
                      ))}
                    </div>
                  );
                }

                return (
                  <Text size="2" color="gray" className="italic text-center py-2">
                    No submission data available
                  </Text>
                );
              })()}
            </div>

            <Tabs.Root value={activeTab} onValueChange={setActiveTab}>
              <Tabs.List>
                <Tabs.Trigger value="review">
                  <Flex align="center" gap="2">
                    <InfoCircledIcon />
                    Review
                  </Flex>
                </Tabs.Trigger>
                <Tabs.Trigger value="comments">
                  <Flex align="center" gap="2">
                    <ChatBubbleIcon />
                    Comments
                    {feedbackCount > 0 && (
                      <Badge color="amber" size="1" variant="solid" radius="full">
                        {feedbackCount}
                      </Badge>
                    )}
                  </Flex>
                </Tabs.Trigger>
              </Tabs.List>

              <Box pt="4">
                {/* Review Tab */}
                <Tabs.Content value="review">
                  {ogaFormConfig && isActionable ? (
                      <form onSubmit={(event) => {
                  void handleSubmit(event)
                }} noValidate>
                        <JsonForms
                          schema={ogaFormConfig.schema}
                          uischema={ogaFormConfig.uiSchema}
                          data={ogaFormData}
                          renderers={radixRenderers}
                          onChange={({ data, errors }: { data: Record<string, unknown>; errors?: unknown[] }) => {
                            setOgaFormData(data);
                            setFormErrors(errors || []);
                          }}
                        />
                        <Flex justify="end" gap="3" mt="6">
                          <Button
                            variant="soft"
                            color="gray"
                            onClick={() => { void navigate('/workflows') }}
                            disabled={isSubmitting || isSendingFeedback}
                            type="button"
                          >
                            Cancel
                          </Button>
                          {application.status !== 'FEEDBACK_REQUESTED' && (
                            <Button
                              variant="soft"
                              color="amber"
                              type="button"
                              disabled={isSubmitting || isSendingFeedback}
                              onClick={() => {
                                setShowFeedbackInput(true)
                                setActiveTab('comments')
                              }}
                            >
                              Request Changes
                            </Button>
                          )}
                          <Button type="submit" disabled={isSubmitting || isSendingFeedback}>
                            {isSubmitting ? <Spinner size="1" /> : null}
                            Submit Review
                          </Button>
                        </Flex>
                      </form>
                    ) : ogaFormConfig ? (
                      <JsonForms
                        schema={ogaFormConfig.schema}
                        uischema={ogaFormConfig.uiSchema}
                        data={ogaFormData}
                        renderers={radixRenderers}
                        readonly
                        onChange={({ data, errors }: { data: Record<string, unknown>; errors?: unknown[] }) => {
                          setOgaFormData(data);
                          setFormErrors(errors || []);
                        }}
                      />
                  ) : null}
                </Tabs.Content>

                {/* Comments Tab */}
                <Tabs.Content value="comments">
                  <div className="space-y-4">
                    {feedbackCount > 0 ? (
                      <div className="rounded-lg border border-amber-200 overflow-hidden">
                        <div className="bg-amber-50 px-4 py-2 border-b border-amber-200">
                          <Text size="1" weight="bold" className="uppercase tracking-wider text-amber-700">
                            Feedback History
                          </Text>
                        </div>
                        <div className="divide-y divide-amber-100">
                          {application.feedbackHistory!.map((entry) => (
                            <div key={entry.round} className="bg-white px-4 py-3">
                              <Flex justify="between" mb="1">
                                <Text size="1" weight="bold" color="amber">Round {entry.round}</Text>
                                <Text size="1" color="gray">{new Date(entry.timestamp).toLocaleString()}</Text>
                              </Flex>
                              <Text size="2" className="whitespace-pre-wrap">
                                {entry.content.feedback as string}
                              </Text>
                            </div>
                          ))}
                        </div>
                      </div>
                    ) : !showFeedbackInput ? (
                      <Text size="2" color="gray" className="italic text-center py-8">
                        No comments yet.
                      </Text>
                    ) : null}

                    {showFeedbackInput && (
                      <div className="rounded-lg border border-amber-200 bg-amber-50 p-4">
                        <Text size="2" weight="bold" color="amber" as="div" mb="2">
                          Request Changes
                        </Text>
                        <textarea
                          className="w-full rounded border border-amber-300 bg-white p-2 text-sm resize-none focus:outline-none focus:ring-2 focus:ring-amber-400"
                          rows={4}
                          placeholder="Describe what the trader needs to correct..."
                          value={feedbackText}
                          onChange={(e) => setFeedbackText(e.target.value)}
                        />
                        <Flex gap="2" mt="2" justify="end">
                          <Button
                            variant="soft"
                            color="gray"
                            size="2"
                            type="button"
                            onClick={() => { setShowFeedbackInput(false); setFeedbackText('') }}
                          >
                            Cancel
                          </Button>
                          <Button
                            color="amber"
                            size="2"
                            type="button"
                            disabled={isSendingFeedback || !feedbackText.trim()}
                            onClick={() => { void handleSendFeedback() }}
                          >
                            {isSendingFeedback ? <Spinner size="1" /> : null}
                            Send Feedback
                          </Button>
                        </Flex>
                      </div>
                    )}

                    {application.status === 'PENDING' && !showFeedbackInput && (
                      <Flex justify="end">
                        <Button
                          variant="soft"
                          color="amber"
                          type="button"
                          onClick={() => setShowFeedbackInput(true)}
                        >
                          Request Changes
                        </Button>
                      </Flex>
                    )}

                    {application.status === 'FEEDBACK_REQUESTED' && (
                      <Callout.Root color="amber" variant="surface">
                        <Callout.Icon><ChatBubbleIcon /></Callout.Icon>
                        <Callout.Text>
                          Feedback has been sent. Awaiting trader resubmission before further changes can be requested.
                        </Callout.Text>
                      </Callout.Root>
                    )}
                  </div>
                </Tabs.Content>
              </Box>
            </Tabs.Root>
          </Card>
        </div>
      </div>
    </div>
  )
}