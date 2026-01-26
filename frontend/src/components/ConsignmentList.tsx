import { Card } from '@lsf/ui';
import './ConsignmentList.css';
import type { Consignment } from '../api';

interface ConsignmentListProps {
  consignments: Consignment[];
  selectedConsignmentId?: string;
  onConsignmentSelect: (consignmentId: string) => void;
}

export function ConsignmentList({ consignments, selectedConsignmentId, onConsignmentSelect }: ConsignmentListProps) {
  if (consignments.length === 0) {
    return (
      <div className="empty-state">
        <h3>No consignments pending review</h3>
        <p>All consignments have been processed or there are no pending consignments.</p>
      </div>
    );
  }

  return (
    <div className="consignment-list">
      {consignments.map((consignment) => (
        <Card
          key={consignment.id}
          className={`consignment-item ${selectedConsignmentId === consignment.id ? 'selected' : ''}`}
          onClick={() => onConsignmentSelect(consignment.id)}
        >
          <h3>
            {consignment.tradeFlow === 'IMPORT' ? 'Import' : 'Export'} Consignment
          </h3>
          <p><strong>Consignment ID:</strong> {consignment.id}</p>
          <p><strong>Trader ID:</strong> {consignment.traderId}</p>
          <p><strong>Status:</strong> {consignment.state}</p>
          <p><strong>Submitted:</strong> {new Date(consignment.createdAt).toLocaleDateString()}</p>
        </Card>
      ))}
    </div>
  );
}
