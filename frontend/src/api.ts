// API service for Agency Portal
import type { ThunderIDContextProps } from '@thunderid/react'
import type { JsonSchema, UISchemaElement } from '@jsonforms/core'
import { API_BASE_URL } from './constants'

/**
 * The authenticated HTTP request function provided by useThunderID().http.
 * Domain functions accept this as their transport so they stay pure and testable,
 * with no direct dependency on the React context.
 */
export type HttpRequest = ThunderIDContextProps['http']['request']

// ── Response / domain interfaces ──────────────────────────────────────────────

export interface ReviewResponse {
  success: boolean
  message?: string
  error?: string
}

export interface FeedbackEntry {
  content: Record<string, unknown>
  timestamp: string
  round: number
}

export interface FormDefinition {
  schema: JsonSchema
  uiSchema: UISchemaElement
}

export interface AgencyApplication {
  taskId: string
  consignmentId: string
  serviceUrl: string
  data: Record<string, unknown>
  agencyActionData?: Record<string, unknown>

  // Task metadata from config
  title?: string
  description?: string
  icon?: string
  category?: string

  // Form definitions
  dataForm?: FormDefinition
  agencyForm?: FormDefinition

  status: string
  feedbackHistory?: FeedbackEntry[]
  reviewerNotes?: string
  reviewedAt?: string
  createdAt: string
  updatedAt: string
}

export interface ConsignmentSummary {
  consignmentId: string
  updatedAt: string
  status: string
  taskCount: number
}

export interface PaginatedResponse<T> {
  items: T[]
  total: number
  page: number
  pageSize: number
}

// ── Domain functions ──────────────────────────────────────────────────────────
// Each function accepts `request` (ThunderID's http.request) as its transport.
// Token injection is handled automatically via `attachToken: true`.

export async function fetchConsignments(
  request: HttpRequest,
  params?: { q?: string; page?: number; pageSize?: number },
  signal?: AbortSignal,
): Promise<PaginatedResponse<ConsignmentSummary>> {
  const res = await request({
    url: `${API_BASE_URL}/api/v1/consignments`,
    method: 'GET',
    params: Object.fromEntries(
      Object.entries({ q: params?.q, page: params?.page, pageSize: params?.pageSize }).filter(
        ([, v]) => v !== undefined,
      ),
    ),
    attachToken: true,
    signal,
  })
  return res.data as PaginatedResponse<ConsignmentSummary>
}

export async function fetchApplications(
  request: HttpRequest,
  params?: { status?: string; consignmentId?: string; q?: string; page?: number; pageSize?: number },
  signal?: AbortSignal,
): Promise<PaginatedResponse<AgencyApplication>> {
  const res = await request({
    url: `${API_BASE_URL}/api/v1/applications`,
    method: 'GET',
    params: Object.fromEntries(
      Object.entries({
        status: params?.status,
        consignmentId: params?.consignmentId,
        q: params?.q,
        page: params?.page,
        pageSize: params?.pageSize,
      }).filter(([, v]) => v !== undefined),
    ),
    attachToken: true,
    signal,
  })
  return res.data as PaginatedResponse<AgencyApplication>
}

export async function fetchApplicationDetail(
  request: HttpRequest,
  taskId: string,
  signal?: AbortSignal,
): Promise<AgencyApplication> {
  const res = await request({
    url: `${API_BASE_URL}/api/v1/applications/${taskId}`,
    method: 'GET',
    attachToken: true,
    signal,
  })
  return res.data as AgencyApplication
}

export async function submitReview(
  request: HttpRequest,
  taskId: string,
  formValues: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<ReviewResponse> {
  const res = await request({
    url: `${API_BASE_URL}/api/v1/applications/${taskId}/review`,
    method: 'POST',
    data: formValues,
    attachToken: true,
    signal,
  })
  return res.data as ReviewResponse
}

export async function submitFeedback(
  request: HttpRequest,
  taskId: string,
  content: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<ReviewResponse> {
  const res = await request({
    url: `${API_BASE_URL}/api/v1/applications/${taskId}/feedback`,
    method: 'POST',
    data: content,
    attachToken: true,
    signal,
  })
  return res.data as ReviewResponse
}
