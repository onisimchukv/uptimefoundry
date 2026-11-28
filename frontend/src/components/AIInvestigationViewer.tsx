'use client';

import React, { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import type { Investigation } from '../types';

export const AIInvestigationViewer: React.FC = () => {
  // Fetch investigations using React Query
  const { data: investigationsData, isLoading, error } = useQuery({
    queryKey: ['investigations'],
    queryFn: async () => {
      const response = await api.getInvestigations();
      return response.data;
    },
    staleTime: 5000,
    refetchInterval: 5000,
  });
  const [showFullAnalysis, setShowFullAnalysis] = useState(false);
  const [confidenceThreshold, setConfidenceThreshold] = useState(0.7);

  const investigations = investigationsData?.investigations || [];
  const activeInvestigation = investigations.find(inv => inv.status === 'complete' || inv.status === 'analyzing');

  useEffect(() => {
    setShowFullAnalysis(false);
  }, [activeInvestigation]);

  if (error) {
    return (
      <div className="card">
        <div className="error-state">
          <div className="error-state-icon">❌</div>
          <div className="error-state-title">Failed to Load Investigation Results</div>
          <p>{error instanceof Error ? error.message : 'An unknown error occurred'}</p>
        </div>
      </div>
    );
  }

  if (!activeInvestigation) {
    return (
      <div className="card">
        <div className="card-header">
          <h3 className="card-title">AI Investigation Results</h3>
        </div>
        <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
          <div style={{ fontSize: '3rem', marginBottom: 16 }}>🤖</div>
          <p>No investigation results available</p>
        </div>
      </div>
    );
  }

  const getStatusBadge = (status: string) => {
    const badges: Record<string, { label: string; color: string }> = {
      'pending': { label: 'Pending', color: '#ff9800' },
      'analyzing': { label: 'Analyzing', color: '#2196f3' },
      'completed': { label: 'Completed', color: '#4caf50' },
      'failed': { label: 'Failed', color: '#f44336' },
    };
    return badges[status] || { label: status, color: '#666' };
  };

  const getConfidenceColor = (confidence: number) => {
    if (confidence >= 0.9) return '#4caf50';
    if (confidence >= 0.7) return '#ff9800';
    return '#f44336';
  };

  const getConfidenceLabel = (confidence: number) => {
    if (confidence >= 0.9) return 'High Confidence';
    if (confidence >= 0.7) return 'Medium Confidence';
    return 'Low Confidence';
  };

  return (
    <div className="card">
      <div className="card-header">
        <h3 className="card-title">AI Investigation Results</h3>
        <span
          style={{
            padding: '4px 8px',
            borderRadius: 4,
            fontSize: '0.8rem',
            fontWeight: 600,
            background: getStatusBadge(activeInvestigation.status).color,
            color: '#fff',
          }}
        >
          {getStatusBadge(activeInvestigation.status).label}
        </span>
      </div>

      {isLoading ? (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <div className="spinner"></div>
          <p style={{ marginTop: 16 }}>Analyzing incident data...</p>
        </div>
      ) : activeInvestigation?.status === 'expired' ? (
        <div style={{ padding: 24, background: '#ffebee', borderRadius: 6 }}>
          <p style={{ margin: 0, color: '#c62828' }}>
            Investigation failed: {activeInvestigation.error}
          </p>
        </div>
      ) : (
        <div>
          {/* Summary Cards */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(250px, 1fr))', gap: 16, marginBottom: 24 }}>
            <div style={{ padding: 16, background: '#e3f2fd', borderRadius: 6 }}>
              <div style={{ fontSize: '0.85rem', color: '#666', marginBottom: 4 }}>Root Cause</div>
              <div style={{ fontWeight: 600, fontSize: '1.1rem' }}>{activeInvestigation.rootCause}</div>
            </div>

            <div style={{ padding: 16, background: '#fff3e0', borderRadius: 6 }}>
              <div style={{ fontSize: '0.85rem', color: '#666', marginBottom: 4 }}>Confidence</div>
              <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <div style={{ display: 'flex', gap: 2, alignItems: 'center', flex: 1 }}>
                  <div style={{
                    width: 100,
                    height: 8,
                    background: '#e0e0e0',
                    borderRadius: 4,
                    overflow: 'hidden',
                  }}>
                    <div style={{
                      width: `${activeInvestigation.confidence * 100}%`,
                      height: '100%',
                      background: getConfidenceColor(activeInvestigation.confidence),
                    }} />
                  </div>
                </div>
                <span style={{ fontSize: '0.85rem', fontWeight: 600 }}>{(activeInvestigation.confidence * 100).toFixed(0)}%</span>
              </div>
              <div style={{ fontSize: '0.8rem', color: '#666', marginTop: 4 }}>{getConfidenceLabel(activeInvestigation.confidence)}</div>
            </div>

            <div style={{ padding: 16, background: '#e8f5e9', borderRadius: 6 }}>
              <div style={{ fontSize: '0.85rem', color: '#666', marginBottom: 4 }}>Recommended Action</div>
              <div style={{ fontWeight: 600, fontSize: '1.1rem' }}>{activeInvestigation.recommendedAction}</div>
            </div>

            <div style={{ padding: 16, background: '#fce4ec', borderRadius: 6 }}>
              <div style={{ fontSize: '0.85rem', color: '#666', marginBottom: 4 }}>Evidence Count</div>
              <div style={{ fontWeight: 600, fontSize: '1.1rem' }}>{activeInvestigation.evidence?.length || 0} items</div>
            </div>
          </div>

          {/* Evidence Section */}
          <div style={{ marginBottom: 24 }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
              <h4 style={{ margin: 0, fontSize: '0.95rem', color: '#666' }}>Evidence</h4>
              <button
                onClick={() => setShowFullAnalysis(!showFullAnalysis)}
                style={{
                  padding: '6px 12px',
                  borderRadius: 4,
                  border: 'none',
                  background: '#e0e0e0',
                  cursor: 'pointer',
                  fontSize: '0.8rem',
                }}
              >
                {showFullAnalysis ? 'Show Less' : 'Show All'}
              </button>
            </div>

            {showFullAnalysis && (
              <div style={{ display: 'grid', gap: 12 }}>
                {activeInvestigation.evidence?.map((item: any, index: number) => (
                  <div
                    key={index}
                    style={{
                      padding: 12,
                      border: '1px solid #e0e0e0',
                      borderRadius: 4,
                      background: '#fafafa',
                    }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 4 }}>
                      <span style={{ fontWeight: 600, fontSize: '0.9rem' }}>{item.type}</span>
                      <span style={{ fontSize: '0.8rem', color: '#999' }}>
                        {new Date(item.timestamp).toLocaleString()}
                      </span>
                    </div>
                    <p style={{ margin: 0, fontSize: '0.85rem', color: '#333' }}>
                      {item.description}
                    </p>
                    {item.metadata && (
                      <pre style={{ margin: '8px 0 0 0', fontSize: '0.75rem', background: '#f5f5f5', padding: 8, borderRadius: 4, overflow: 'auto' }}>
                        {JSON.stringify(item.metadata, null, 2)}
                      </pre>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Full Analysis Section */}
          {activeInvestigation.fullAnalysis && (
            <div style={{ marginBottom: 24 }}>
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                <h4 style={{ margin: 0, fontSize: '0.95rem', color: '#666' }}>Full Analysis</h4>
                <button
                  onClick={() => setShowFullAnalysis(!showFullAnalysis)}
                  style={{
                    padding: '6px 12px',
                    borderRadius: 4,
                    border: 'none',
                    background: '#e0e0e0',
                    cursor: 'pointer',
                    fontSize: '0.8rem',
                  }}
                >
                  {showFullAnalysis ? 'Collapse' : 'Expand'}
                </button>
              </div>
              {showFullAnalysis && (
                <div style={{ padding: 16, background: '#f5f5f5', borderRadius: 4, whiteSpace: 'pre-wrap' }}>
                  {activeInvestigation.fullAnalysis}
                </div>
              )}
            </div>
          )}

          {/* Timeline */}
          {activeInvestigation.timeline && activeInvestigation.timeline.length > 0 && (
            <div style={{ marginBottom: 24 }}>
              <h4 style={{ margin: '0 0 12px 0', fontSize: '0.95rem', color: '#666' }}>Investigation Timeline</h4>
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {activeInvestigation.timeline.map((event: any, index: number) => (
                  <div key={index} style={{ display: 'flex', gap: 12, padding: 8, background: '#fafafa', borderRadius: 4 }}>
                    <div style={{
                      minWidth: 60,
                      textAlign: 'right',
                      fontSize: '0.85rem',
                      color: '#666',
                    }}>
                      {new Date(event.timestamp).toLocaleTimeString()}
                    </div>
                    <div style={{ flex: 1 }}>
                      <div style={{ fontWeight: 600, fontSize: '0.9rem' }}>{event.action}</div>
                      <div style={{ fontSize: '0.85rem', color: '#999' }}>{event.details}</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Action Buttons */}
          <div style={{ display: 'flex', gap: 8, marginTop: 24, paddingTop: 24, borderTop: '1px solid #e0e0e0' }}>
            <button
              onClick={() => activeInvestigation.approved = true}
              style={{
                flex: 1,
                padding: '12px 16px',
                borderRadius: 4,
                border: 'none',
                background: '#4caf50',
                color: '#fff',
                cursor: 'pointer',
                fontWeight: 600,
                fontSize: '1rem',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 8,
              }}
            >
              ✅ Approve Remediation
            </button>
            <button
              onClick={() => activeInvestigation.rejected = true}
              style={{
                flex: 1,
                padding: '12px 16px',
                borderRadius: 4,
                border: 'none',
                background: '#f44336',
                color: '#fff',
                cursor: 'pointer',
                fontWeight: 600,
                fontSize: '1rem',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 8,
              }}
            >
              ❌ Reject
            </button>
            <button
              onClick={() => {
                activeInvestigation.requestingMoreInfo = true;
              }}
              style={{
                padding: '12px 16px',
                borderRadius: 4,
                border: '1px solid #2196f3',
                background: '#fff',
                color: '#2196f3',
                cursor: 'pointer',
                fontWeight: 600,
              }}
              disabled={activeInvestigation.requestingMoreInfo}
            >
              {activeInvestigation.requestingMoreInfo ? 'Requesting Info...' : '🔍 Request More Info'}
            </button>
          </div>

          {/* Confidence Threshold Warning */}
          {activeInvestigation.confidence < confidenceThreshold && (
            <div style={{
              marginTop: 16,
              padding: 12,
              background: '#fff3e0',
              borderRadius: 4,
              border: '1px solid #ff9800',
            }}>
              <div style={{ display: 'flex', gap: 8, alignItems: 'center' }}>
                <span style={{ fontSize: '1.5rem' }}>⚠️</span>
                <div>
                  <strong style={{ color: '#e65100' }}>Low Confidence Alert</strong>
                  <p style={{ margin: '4px 0 0 0', fontSize: '0.85rem', color: '#e65100' }}>
                    This investigation has a confidence level of {(activeInvestigation.confidence * 100).toFixed(0)}%, which is below your threshold of {(confidenceThreshold * 100).toFixed(0)}%. Consider requesting more information before approving.
                  </p>
                </div>
              </div>
              <div style={{ display: 'flex', gap: 8, marginTop: 8 }}>
                <input
                  type="range"
                  min="0.5"
                  max="0.95"
                  step="0.05"
                  value={confidenceThreshold}
                  onChange={(e) => setConfidenceThreshold(parseFloat(e.target.value))}
                  style={{ flex: 1 }}
                />
                <span style={{ minWidth: 50, textAlign: 'right' }}>{(confidenceThreshold * 100).toFixed(0)}%</span>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default AIInvestigationViewer;