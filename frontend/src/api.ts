// API service for OGA Portal

const API_BASE_URL = (import.meta.env.VITE_OGA_API_BASE_URL as string | undefined) ?? 'http://localhost:8081';

export type Decision = 'APPROVED' | 'REJECTED';

export interface ApproveRequest {
  formData: Record<string, unknown>;
  workflowId: string;
  decision: Decision;
  reviewerName: string;
  comments?: string;
}

export interface ApproveResponse {
  success: boolean;
  message?: string;
  error?: string;
}

export interface OGAApplication {
  taskId: string;
  workflowId: string;
  serviceUrl: string;
  data: Record<string, unknown>;
  status: string;
  reviewerNotes?: string;
  reviewedAt?: string;
  createdAt: string;
  updatedAt: string;
  ogaForm?: {
    schema: Record<string, unknown>;
    uiSchema: Record<string, unknown>;
  };
}


export async function fetchPendingApplications(status?: string, signal?: AbortSignal): Promise<OGAApplication[]> {
  const url = status
    ? `${API_BASE_URL}/api/oga/applications?status=${status}`
    : `${API_BASE_URL}/api/oga/applications`;

  const response = await fetch(url, { signal });
  if (!response.ok) {
    throw new Error(`Failed to fetch pending applications: ${response.statusText}`);
  }

  return response.json() as Promise<OGAApplication[]>;
}

// Fetch application detail by taskId from OGA Service
export async function fetchApplicationDetail(taskId: string, signal?: AbortSignal): Promise<OGAApplication> {
  const response = await fetch(`${API_BASE_URL}/api/oga/applications/${taskId}`, { signal });
  if (!response.ok) {
    throw new Error(`Failed to fetch application: ${response.statusText}`);
  }
  return response.json() as Promise<OGAApplication>;
}

// Submit approval for a task via OGA Service
// OGA Service sends callback to the originating service
export async function approveTask(
  taskId: string,
  _workflowId: string,
  requestBody: ApproveRequest,
  signal?: AbortSignal
): Promise<ApproveResponse> {
  // Build reviewer notes from comments and reviewer name
  const reviewerNotes = [
    `Reviewer: ${requestBody.reviewerName}`,
    requestBody.comments ? `Comments: ${requestBody.comments}` : '',
    requestBody.formData && Object.keys(requestBody.formData).length > 0
      ? `Form Data: ${JSON.stringify(requestBody.formData)}`
      : ''
  ].filter(Boolean).join('\n');

  const response = await fetch(`${API_BASE_URL}/api/oga/applications/${taskId}/review`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({
      decision: requestBody.decision,
      reviewerNotes: reviewerNotes,
    }),
    signal,
  });

  if (!response.ok) {
    const errorData = await response.json().catch(() => ({ error: response.statusText })) as { error?: string };
    throw new Error(errorData.error ?? `Failed to submit review: ${response.statusText}`);
  }

  return response.json() as Promise<ApproveResponse>;
}
