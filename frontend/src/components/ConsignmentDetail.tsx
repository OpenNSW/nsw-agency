import { useState } from 'react';
import { Button, Card } from '@lsf/ui';
import type { ConsignmentDetail, ApproveRequest } from '../api';
import { approveTask } from '../api';
import './ConsignmentDetail.css';

interface JsonSchema {
  properties?: Record<string, {
    type?: string;
    title?: string;
  }>;
}

interface ConsignmentDetailProps {
  consignment: ConsignmentDetail;
  onApproved: () => void;
}

export function ConsignmentDetail({ consignment, onApproved }: ConsignmentDetailProps) {
  const [formData, setFormData] = useState<Record<string, unknown>>({});
  const [reviewerName, setReviewerName] = useState('');
  const [decision, setDecision] = useState<'APPROVED' | 'REJECTED'>('APPROVED');
  const [comments, setComments] = useState('');
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  // Get the first OGA task for approval
  const ogaTask = consignment.ogaTasks?.[0];

  const handleFormChange = (field: string, value: unknown) => {
    setFormData((prev) => ({
      ...prev,
      [field]: value,
    }));
  };

  const handleApprove = async () => {
    if (!reviewerName.trim()) {
      setError('Reviewer name is required');
      return;
    }

    if (!ogaTask) {
      setError('No OGA task found for this consignment');
      return;
    }

    setIsSubmitting(true);
    setError(null);
    setSuccess(false);

    try {
      const requestBody: ApproveRequest = {
        decision,
        comments: comments.trim() || undefined,
        reviewerName: reviewerName.trim(),
        formData: formData,
        consignmentId: consignment.id,
      };
      await approveTask(ogaTask.id, requestBody);
      setSuccess(true);

      // Mock SMS/Email notification
      console.log(`[MOCK] Sending notification to trader for consignment ${consignment.id}: Consignment has been ${decision.toLowerCase()}`);

      // Call callback to refresh consignment list
      setTimeout(() => {
        onApproved();
      }, 1500);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to approve consignment');
    } finally {
      setIsSubmitting(false);
    }
  };

  const ogaForm = consignment.ogaForm;

  return (
    <Card className="consignment-detail">
      <h2>Consignment Review: {consignment.tradeFlow}</h2>

      {error && (
        <div className="error">
          {error}
        </div>
      )}

      {success && (
        <div className="success">
          Consignment has been {decision.toLowerCase()} successfully! Notification sent to trader.
        </div>
      )}

      {/* Consignment Info */}
      <div className="section consignment-info">
        <h3>Consignment Information</h3>
        <div className="info-grid">
          <div className="info-item">
            <label>Consignment ID:</label>
            <span>{consignment.id}</span>
          </div>
          <div className="info-item">
            <label>Trader ID:</label>
            <span>{consignment.traderId}</span>
          </div>
          <div className="info-item">
            <label>Trade Flow:</label>
            <span>{consignment.tradeFlow}</span>
          </div>
          <div className="info-item">
            <label>Status:</label>
            <span>{consignment.state}</span>
          </div>
        </div>
      </div>

      {/* Trader Submission Section (Read-only) */}
      <div className="section trader-form-section">
        <h3>Trader Submission</h3>
        {consignment.traderForm && Object.keys(consignment.traderForm).length > 0 ? (
          <div className="form-data-display">
            {Object.entries(consignment.traderForm).map(([key, value]) => (
              <div key={key} className="form-field">
                <label>{key}</label>
                <input
                  type="text"
                  value={String(value)}
                  disabled
                  readOnly
                />
              </div>
            ))}
          </div>
        ) : (
          <p className="no-data">No trader form submission available yet.</p>
        )}
      </div>

      {/* OGA Review Form Section (Dynamic Rendering from Schema) */}
      <div className="section oga-form-section">
        <h3>OGA Review Form</h3>
        {ogaForm ? (
          <div className="oga-form-fields">
            <div className="form-field">
              <label htmlFor="reviewerName">Reviewer Name *</label>
              <input
                id="reviewerName"
                type="text"
                value={reviewerName}
                onChange={(e) => setReviewerName(e.target.value)}
                placeholder="Enter your name"
                disabled={isSubmitting || success}
              />
            </div>

            {/* Render dynamic fields from JSON Schema */}
            {(() => {
              const schema = ogaForm.schema as unknown as JsonSchema;
              const properties = schema?.properties || {};

              return Object.entries(properties).map(([key, fieldSchema]) => {
                const fieldType = fieldSchema.type || 'string';
                const fieldTitle = fieldSchema.title || key;

                return (
                  <div key={key} className="form-field">
                    <label htmlFor={key}>{fieldTitle}</label>
                    {fieldType === 'string' && (
                      <input
                        id={key}
                        type="text"
                        value={(formData[key] as string) || ''}
                        onChange={(e) => handleFormChange(key, e.target.value)}
                        disabled={isSubmitting || success}
                      />
                    )}
                    {fieldType === 'boolean' && (
                      <input
                        id={key}
                        type="checkbox"
                        checked={(formData[key] as boolean) || false}
                        onChange={(e) => handleFormChange(key, e.target.checked)}
                        disabled={isSubmitting || success}
                      />
                    )}
                  </div>
                );
              });
            })()}

            <div className="form-field">
              <label htmlFor="decision">Final Decision *</label>
              <select
                id="decision"
                value={decision}
                onChange={(e) => setDecision(e.target.value as 'APPROVED' | 'REJECTED')}
                disabled={isSubmitting || success}
              >
                <option value="APPROVED">Approve</option>
                <option value="REJECTED">Reject</option>
              </select>
            </div>

            <div className="form-field">
              <label htmlFor="comments">Additional Comments</label>
              <textarea
                id="comments"
                value={comments}
                onChange={(e) => setComments(e.target.value)}
                placeholder="Enter any additional comments..."
                disabled={isSubmitting || success}
              />
            </div>
          </div>
        ) : (
          <p className="no-data">OGA form schema not available for this consignment.</p>
        )}
      </div>

      {/* Action Button */}
      <div className="approve-section">
        <Button
          className="approve-button"
          onClick={handleApprove}
          disabled={isSubmitting || success || !reviewerName.trim() || !ogaTask}
        >
          {isSubmitting ? 'Processing...' : success ? 'Consignment Processed' : 'Submit Review'}
        </Button>
      </div>
    </Card>
  );
}
