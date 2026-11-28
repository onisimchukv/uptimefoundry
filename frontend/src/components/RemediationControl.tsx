'use client';

import React, { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import type { RemediationAction } from '../types';
import { useRemediation } from '../hooks/useRemediation';
import { api } from '../lib/api';

interface RemediationControlProps {
  remediationId?: string;
  remediationActionId?: string;
}

export const RemediationControl: React.FC<RemediationControlProps> = ({
  remediationId,
  remediationActionId,
}) => {
  const queryClient = useQueryClient();
  
  const [selectedTarget, setSelectedTarget] = useState<string>('');
  const [executionMode, setExecutionMode] = useState<'single' | 'all'>('single');
  
  // Use React Query hook for remediation
  const { remediation, isLoading: remediationLoading, execute, executing, cancel } = useRemediation(
    remediationId || remediationActionId || ''
  );

  // Update selected target when remediation changes
  useEffect(() => {
    if (remediation && remediation.status === 'ready' && remediation.targets.length > 0) {
      setSelectedTarget(remediation.targets[0].id);
    }
  }, [remediation]);

  if (!remediation) {
    return (
      <div className="card">
        <div className="card-header">
          <h3 className="card-title">Remediation Control</h3>
        </div>
        <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
          <div style={{ fontSize: '3rem', marginBottom: 16 }}>🔧</div>
          <p>No remediation actions available</p>
        </div>
      </div>
    );
  }

  const getStatusBadge = (status: string) => {
    const badges: Record<string, { label: string; color: string }> = {
      'pending': { label: 'Pending', color: '#ff9800' },
      'ready': { label: 'Ready', color: '#2196f3' },
      'executing': { label: 'Executing', color: '#2196f3' },
      'completed': { label: 'Completed', color: '#4caf50' },
      'failed': { label: 'Failed', color: '#f44336' },
      'cancelled': { label: 'Cancelled', color: '#9e9e9e' },
    };
    return badges[status] || { label: status, color: '#666' };
  };

  const getSeverityBadge = (severity: string) => {
    const badges: Record<string, { label: string; color: string }> = {
      'low': { label: 'Low', color: '#2196f3' },
      'medium': { label: 'Medium', color: '#ff9800' },
      'high': { label: 'High', color: '#f44336' },
      'critical': { label: 'Critical', color: '#d32f2f' },
    };
    return badges[severity] || { label: severity, color: '#666' };
  };

  const handleExecute = async () => {
    if (!selectedTarget && executionMode === 'single') {
      alert('Please select a target to execute remediation on');
      return;
    }

    try {
      await execute({
        remediationId: remediation.id || '',
        targetId: executionMode === 'single' ? selectedTarget : undefined,
        mode: executionMode
      });
      // Refresh the remediation data after execution
      queryClient.invalidateQueries({ queryKey: [remediation.id] });
    } catch (error) {
      console.error('Failed to execute remediation:', error);
    }
  };

  const handleCancel = async () => {
    try {
      await cancel(remediation.id || '');
      // Refresh the remediation data after cancellation
      queryClient.invalidateQueries({ queryKey: [remediation.id] });
    } catch (error) {
      console.error('Failed to cancel remediation:', error);
    }
  };

  return (
    <div className="card">
      <div className="card-header">
        <h3 className="card-title">Remediation Control</h3>
        <span
          style={{
            padding: '4px 8px',
            borderRadius: 4,
            fontSize: '0.8rem',
            fontWeight: 600,
            background: getStatusBadge(remediation.status).color,
            color: '#fff',
          }}
        >
          {getStatusBadge(remediation.status).label}
        </span>
      </div>

      {remediationLoading && remediation.status !== 'executing' ? (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <div className="spinner"></div>
          <p style={{ marginTop: 16 }}>Preparing remediation action...</p>
        </div>
      ) : remediation.status === 'failed' ? (
        <div style={{ padding: 24, background: '#ffebee', borderRadius: 6 }}>
          <p style={{ margin: '0 0 8px 0', color: '#c62828' }}>
            Remediation failed: {remediation.error}
          </p>
          <button
            onClick={handleCancel}
            style={{
              padding: '8px 16px',
              borderRadius: 4,
              border: 'none',
              background: '#f44336',
              color: '#fff',
              cursor: 'pointer',
            }}
          >
            Retry
          </button>
        </div>
      ) : (
        <div>
          {/* Action Details */}
          <div style={{ marginBottom: 24 }}>
            <h4 style={{ margin: '0 0 12px 0', fontSize: '0.95rem', color: '#666' }}>Action Details</h4>
            <div style={{ padding: 16, background: '#f5f5f5', borderRadius: 6 }}>
              <p style={{ margin: '4px 0' }}><strong>Action:</strong> {remediation.actionType}</p>
              <p style={{ margin: '4px 0' }}><strong>Description:</strong> {remediation.description}</p>
              <p style={{ margin: '4px 0' }}><strong>Severity:</strong></p>
              <span
                style={{
                  padding: '4px 8px',
                  borderRadius: 4,
                  fontSize: '0.8rem',
                  fontWeight: 600,
                  background: getSeverityBadge(remediation.severity).color,
                  color: '#fff',
                  display: 'inline-block',
                  marginTop: 4,
                }}
              >
                {getSeverityBadge(remediation.severity).label}
              </span>
              <p style={{ margin: '8px 0 0 0', fontSize: '0.85rem', color: '#999' }}>
                {remediation.estimatedDuration}
              </p>
            </div>
          </div>

          {/* Targets */}
          {remediation.targets && remediation.targets.length > 0 && (
            <div style={{ marginBottom: 24 }}>
              <h4 style={{ margin: '0 0 12px 0', fontSize: '0.95rem', color: '#666' }}>Targets</h4>
              <div style={{ display: 'grid', gap: 8 }}>
                {remediation.targets.map((target: { id: string; name: string; type: string }, index: number) => (
                  <div
                    key={index}
                    style={{
                      padding: 12,
                      border: '1px solid #e0e0e0',
                      borderRadius: 4,
                      background: selectedTarget === target.id ? '#e3f2fd' : '#fff',
                      cursor: 'pointer',
                    }}
                    onClick={() => setSelectedTarget(target.id)}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                      <span style={{ fontWeight: 600 }}>{target.name}</span>
                      <span style={{ fontSize: '0.8rem', color: '#999' }}>{target.type}</span>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Execution Mode */}
          {remediation.targets && remediation.targets.length > 1 && (
            <div style={{ marginBottom: 24 }}>
              <h4 style={{ margin: '0 0 12px 0', fontSize: '0.95rem', color: '#666' }}>Execution Mode</h4>
              <div style={{ display: 'flex', gap: 8 }}>
                <button
                  onClick={() => setExecutionMode('single')}
                  style={{
                    flex: 1,
                    padding: '8px 12px',
                    borderRadius: 4,
                    border: 'none',
                    background: executionMode === 'single' ? '#2d5a87' : '#e0e0e0',
                    color: executionMode === 'single' ? '#fff' : '#333',
                    cursor: 'pointer',
                    fontWeight: 600,
                  }}
                >
                  Single Target
                </button>
                <button
                  onClick={() => setExecutionMode('all')}
                  style={{
                    flex: 1,
                    padding: '8px 12px',
                    borderRadius: 4,
                    border: 'none',
                    background: executionMode === 'all' ? '#2d5a87' : '#e0e0e0',
                    color: executionMode === 'all' ? '#fff' : '#333',
                    cursor: 'pointer',
                    fontWeight: 600,
                  }}
                >
                  All Targets
                </button>
              </div>
            </div>
          )}

          {/* Execution Controls */}
          {remediation.status === 'ready' && (
            <div style={{ display: 'flex', gap: 8, marginTop: 24, paddingTop: 24, borderTop: '1px solid #e0e0e0' }}>
              <button
                onClick={handleExecute}
                disabled={executing}
                style={{
                  flex: 1,
                  padding: '14px 16px',
                  borderRadius: 4,
                  border: 'none',
                  background: executing ? '#bdbdbd' : '#4caf50',
                  color: '#fff',
                  cursor: executing ? 'not-allowed' : 'pointer',
                  fontWeight: 600,
                  fontSize: '1.1rem',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: 8,
                }}
              >
                {executing ? (
                  <>
                    <div className="spinner" style={{ width: 20, height: 20 }}></div>
                    Executing...
                  </>
                ) : (
                  <>
                    ▶️ Execute Remediation
                  </>
                )}
              </button>
              <button
                onClick={handleCancel}
                style={{
                  padding: '14px 16px',
                  borderRadius: 4,
                  border: 'none',
                  background: '#f44336',
                  color: '#fff',
                  cursor: 'pointer',
                  fontWeight: 600,
                }}
              >
                Cancel
              </button>
            </div>
          )}

          {/* Progress */}
          {remediation.status === 'executing' && remediation.progress && (
            <div style={{ marginBottom: 24 }}>
              <h4 style={{ margin: '0 0 8px 0', fontSize: '0.95rem', color: '#666' }}>
                Execution Progress
              </h4>
              <div style={{ marginBottom: 8 }}>
                <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.85rem', color: '#999' }}>
                  <span>{remediation.progress.completed} / {remediation.progress.total} targets</span>
                  <span>{Math.round((remediation.progress.completed / remediation.progress.total) * 100)}%</span>
                </div>
                <div style={{ height: 8, background: '#e0e0e0', borderRadius: 4, overflow: 'hidden' }}>
                  <div
                    style={{
                      width: `${(remediation.progress.completed / remediation.progress.total) * 100}%`,
                      height: '100%',
                      background: '#4caf50',
                      transition: 'width 0.3s',
                    }}
                  />
                </div>
              </div>
              {remediation.progress.details && (
                <div style={{ fontSize: '0.85rem', color: '#666' }}>
                  {remediation.progress.details.map((detail: string, index: number) => (
                    <div key={index} style={{ marginBottom: 4 }}>
                      {detail}
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* Execution Log */}
          {remediation.logs && remediation.logs.length > 0 && (
            <div>
              <h4 style={{ margin: '0 0 8px 0', fontSize: '0.95rem', color: '#666' }}>Execution Log</h4>
              <div style={{ height: 200, overflowY: 'auto', border: '1px solid #e0e0e0', borderRadius: 4, padding: 8 }}>
                {remediation.logs.map((log: { timestamp: string; level: string; message: string }, index: number) => (
                  <div
                    key={index}
                    style={{
                      padding: 4,
                      marginBottom: 4,
                      fontSize: '0.85rem',
                      borderLeft: `3px solid ${log.level === 'error' ? '#f44336' : log.level === 'warn' ? '#ff9800' : '#4caf50'}`,
                      background: log.level === 'error' ? '#ffebee' : log.level === 'warn' ? '#fff3e0' : '#e8f5e9',
                    }}
                  >
                    <span style={{ color: '#999', marginRight: 8 }}>{new Date(log.timestamp).toLocaleTimeString()}</span>
                    <span style={{ fontWeight: 600, marginRight: 8 }}>{log.level.toUpperCase()}</span>
                    <span>{log.message}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

export default RemediationControl;
