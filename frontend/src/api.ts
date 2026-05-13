// API service for OGA Portal
import type { JsonSchema, UISchemaElement } from '@jsonforms/core'
import { API_BASE_URL } from './constants'

export type AccessTokenProvider = () => Promise<string | null | undefined>

export type QueryParams = Record<string, string | number | undefined>

export interface ApiClient {
  get<T>(endpoint: string, params?: QueryParams, signal?: AbortSignal): Promise<T>
  post<T, R>(endpoint: string, body: T, signal?: AbortSignal): Promise<R>
  getAuthHeaders(includeJsonContentType?: boolean): Promise<HeadersInit>
}

function buildQueryString(params: QueryParams): string {
  const entries = Object.entries(params)
    .filter(([, value]) => value !== undefined)
    .sort(([left], [right]) => left.localeCompare(right))

  const searchParams = new URLSearchParams()
  entries.forEach(([key, value]) => {
    searchParams.append(key, String(value))
  })

  return searchParams.toString()
}

export function createApiClient(getAccessToken?: AccessTokenProvider): ApiClient {
  async function resolveAccessToken(): Promise<string | null> {
    if (!getAccessToken) {
      return null
    }

    try {
      return (await getAccessToken()) ?? null
    } catch {
      return null
    }
  }

  async function getAuthHeaders(includeJsonContentType = false): Promise<HeadersInit> {
    const headers: Record<string, string> = {}

    if (includeJsonContentType) {
      headers['Content-Type'] = 'application/json'
    }

    const accessToken = await resolveAccessToken()
    if (accessToken) {
      headers.Authorization = `Bearer ${accessToken}`
    }

    return headers
  }

  return {
    async get<T>(endpoint: string, params: QueryParams = {}, signal?: AbortSignal): Promise<T> {
      const queryString = buildQueryString(params)
      const url = `${API_BASE_URL}${endpoint}${queryString ? `?${queryString}` : ''}`

      const response = await fetch(url, {
        signal,
        headers: await getAuthHeaders(),
      })

      if (!response.ok) {
        throw new Error(`Failed request: ${response.statusText}`)
      }

      return response.json() as Promise<T>
    },

    async post<T, R>(endpoint: string, body: T, signal?: AbortSignal): Promise<R> {
      const url = `${API_BASE_URL}${endpoint}`

      const response = await fetch(url, {
        method: 'POST',
        headers: await getAuthHeaders(true),
        body: JSON.stringify(body),
        signal,
      })

      if (!response.ok) {
        const errorData = (await response.json().catch(() => ({ error: response.statusText }))) as { error?: string }
        throw new Error(errorData.error ?? `Failed request: ${response.statusText}`)
      }

      return response.json() as Promise<R>
    },

    getAuthHeaders,
  }
}

export const defaultApiClient = createApiClient()

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

export interface OGAApplication {
  taskId: string
  workflowId: string
  serviceUrl: string
  data: Record<string, unknown>
  ogaActionData?: Record<string, unknown>

  // Task metadata from config
  title?: string
  description?: string
  icon?: string
  category?: string

  // Form definitions
  dataForm?: FormDefinition
  ogaForm?: FormDefinition

  status: string
  feedbackHistory?: FeedbackEntry[]
  reviewerNotes?: string
  reviewedAt?: string
  createdAt: string
  updatedAt: string
}

export interface WorkflowSummary {
  workflowId: string
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

export async function fetchWorkflows(
  apiClient: ApiClient,
  params?: { q?: string; page?: number; pageSize?: number },
  signal?: AbortSignal,
): Promise<PaginatedResponse<WorkflowSummary>> {
  return apiClient.get<PaginatedResponse<WorkflowSummary>>(
    '/api/oga/workflows',
    {
      q: params?.q,
      page: params?.page,
      pageSize: params?.pageSize,
    },
    signal,
  )
}

export async function fetchApplications(
  apiClient: ApiClient,
  params?: { status?: string; workflowId?: string; q?: string; page?: number; pageSize?: number },
  signal?: AbortSignal,
): Promise<PaginatedResponse<OGAApplication>> {
  return apiClient.get<PaginatedResponse<OGAApplication>>(
    '/api/oga/applications',
    {
      status: params?.status,
      workflowId: params?.workflowId,
      q: params?.q,
      page: params?.page,
      pageSize: params?.pageSize,
    },
    signal,
  )
}

// Fetch application detail by taskId from OGA Service
export async function fetchApplicationDetail(
  apiClient: ApiClient,
  taskId: string,
  signal?: AbortSignal,
): Promise<OGAApplication> {
  return apiClient.get<OGAApplication>(`/api/oga/applications/${taskId}`, {}, signal)
}

// Submit review for a task via OGA Service
export async function submitReview(
  apiClient: ApiClient,
  taskId: string,
  formValues: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<ReviewResponse> {
  return apiClient.post<Record<string, unknown>, ReviewResponse>(
    `/api/oga/applications/${taskId}/review`,
    formValues,
    signal,
  )
}

// Submit feedback (request changes) for a task via OGA Service
export async function submitFeedback(
  apiClient: ApiClient,
  taskId: string,
  content: Record<string, unknown>,
  signal?: AbortSignal,
): Promise<ReviewResponse> {
  return apiClient.post<Record<string, unknown>, ReviewResponse>(
    `/api/oga/applications/${taskId}/feedback`,
    content,
    signal,
  )
}
