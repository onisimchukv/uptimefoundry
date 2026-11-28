'use client';

import React, { useState, useEffect } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import type { Incident } from '../types';

export const IncidentManagement: React.FC = () => {
  // Fetch incidents using React Query
  const { data: incidentsData, isLoading, error } = useQuery({
    queryKey: ['incidents'],
    queryFn: async () => {
      const data = await api.getIncidents({});
      return data;
    },
    staleTime: 5000,
    refetchInterval: 5000,
  });
  const [filter, setFilter] = useState('all');
  const [selectedIncident, setSelectedIncident] = useState<Incident | null>(null);

  const incidents = incidentsData?.data?.incidents || [];

  const filteredIncidents = filter === 'all'
    ? incidents
    : incidents.filter(inc => inc.status === filter);

  const getStatusBadge = (status: string) => {
    const badges: Record<string, { label: string; color: string }> = {
      'new': { label: 'New', color: '#2196f3' },
      'investigating': { label: 'Investigating', color: '#ff9800' },
      'identified': { label: 'Identified', color: '#ff9800' },
      'monitoring': { label: 'Monitoring', color: '#ff9800' },
      'resolving': { label: 'Resolving', color: '#9c27b0' },
      'resolved': { label: 'Resolved', color: '#4caf50' },
      'false_positive': { label: 'False Positive', color: '#4caf50' },
      'escalated': { label: 'Escalated', color: '#f44336' },
    };
    return badges[status] || { label: status, color: '#666' };
  };

  const getPriorityBadge = (priority: string) => {
    const badges: Record<string, { label: string; color: string }> = {
      'low': { label: 'Low', color: '#2196f3' },
      'medium': { label: 'Medium', color: '#ff9800' },
      'high': { label: 'High', color: '#f44336' },
      'critical': { label: 'Critical', color: '#d32f2f' },
    };
    return badges[priority] || { label: priority, color: '#666' };
  };

  const handleIncidentClick = (incident: Incident) => {
    setSelectedIncident(incident);
  };

  return (
    <div className="card">
      <div className="card-header">
        <h3 className="card-title">Incidents</h3>
        <div style={{ display: 'flex', gap: 8 }}>
          <select
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            style={{ padding: '6px 12px', borderRadius: 4, border: '1px solid #ddd' }}
          >
            <option value="all">All Incidents</option>
            <option value="new">New</option>
            <option value="investigating">Investigating</option>
            <option value="identified">Identified</option>
            <option value="monitoring">Monitoring</option>
            <option value="resolving">Resolving</option>
            <option value="resolved">Resolved</option>
            <option value="escalated">Escalated</option>
          </select>
        </div>
      </div>

      {isLoading ? (
        <div style={{ textAlign: 'center', padding: 40 }}>
          <div className="spinner"></div>
          <p style={{ marginTop: 16 }}>Loading incidents...</p>
        </div>
      ) : filteredIncidents.length === 0 ? (
        <div style={{ textAlign: 'center', padding: 40, color: '#999' }}>
          <div style={{ fontSize: '3rem', marginBottom: 16 }}>📊</div>
          <p>No incidents found</p>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
          {filteredIncidents.map((incident) => (
            <div
              key={incident.id}
              onClick={() => handleIncidentClick(incident)}
              style={{
                padding: 16,
                borderRadius: 6,
                border: '1px solid #e0e0e0',
                background: '#fff',
                cursor: 'pointer',
                transition: 'box-shadow 0.2s',
              }}
              onMouseEnter={(e) => (e.currentTarget.style.boxShadow = '0 2px 8px rgba(0,0,0,0.1)')}
              onMouseLeave={(e) => (e.currentTarget.style.boxShadow = 'none')}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
                <div style={{ flex: 1 }}>
                  <div style={{ display: 'flex', gap: 8, marginBottom: 8 }}>
                    <span
                      style={{
                        padding: '4px 8px',
                        borderRadius: 4,
                        fontSize: '0.75rem',
                        fontWeight: 600,
                        textTransform: 'uppercase',
                        background: getStatusBadge(incident.status).color,
                        color: '#fff',
                      }}
                    >
                      {getStatusBadge(incident.status).label}
                    </span>
                    <span
                      style={{
                        padding: '4px 8px',
                        borderRadius: 4,
                        fontSize: '0.75rem',
                        fontWeight: 600,
                        background: getPriorityBadge(incident.severity).color,
                        color: '#fff',
                      }}
                    >
                      {getPriorityBadge(incident.severity).label}
                    </span>
                  </div>
                  <h4 style={{ margin: 0, fontSize: '1rem', color: '#333' }}>
                    {incident.title}
                  </h4>
                  <p style={{ margin: '8px 0 0 0', fontSize: '0.85rem', color: '#666' }}>
                    {incident.description}
                  </p>
                  <div style={{ display: 'flex', gap: 12, marginTop: 12, fontSize: '0.8rem', color: '#999' }}>
                    <span>📍 {incident.affectedService || 'Unknown'}</span>
                    <span>⏱️ {incident.duration}</span>
                  </div>
                </div>
                <div style={{ fontSize: '1.5rem', opacity: 0.5 }}>
                  ▶️
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Incident Detail Panel */}
      {selectedIncident && (
        <div style={{ marginTop: 24, padding: 20, background: '#f5f5f5', borderRadius: 6 }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
            <h4 style={{ margin: 0 }}>Incident Details</h4>
            <button
              onClick={() => setSelectedIncident(null)}
              style={{
                padding: '6px 12px',
                borderRadius: 4,
                border: 'none',
                background: '#e0e0e0',
                cursor: 'pointer',
              }}
            >
              Close
            </button>
          </div>

          <div style={{ display: 'grid', gap: 16 }}>
            <div>
              <h5 style={{ margin: '0 0 8px 0', fontSize: '0.9rem', color: '#666' }}>Title</h5>
              <p style={{ margin: 0 }}>{selectedIncident.title}</p>
            </div>

            <div>
              <h5 style={{ margin: '0 0 8px 0', fontSize: '0.9rem', color: '#666' }}>Description</h5>
              <p style={{ margin: 0 }}>{selectedIncident.description}</p>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 16 }}>
              <div>
                <h5 style={{ margin: '0 0 8px 0', fontSize: '0.9rem', color: '#666' }}>Status</h5>
                <span
                  style={{
                    padding: '4px 8px',
                    borderRadius: 4,
                    fontSize: '0.8rem',
                    fontWeight: 600,
                    background: getStatusBadge(selectedIncident.status).color,
                    color: '#fff',
                  }}
                >
                  {getStatusBadge(selectedIncident.status).label}
                </span>
              </div>

              <div>
                <h5 style={{ margin: '0 0 8px 0', fontSize: '0.9rem', color: '#666' }}>Priority</h5>
                <span
                  style={{
                    padding: '4px 8px',
                    borderRadius: 4,
                    fontSize: '0.8rem',
                    fontWeight: 600,
                    background: getPriorityBadge(selectedIncident.severity).color,
                    color: '#fff',
                  }}
                >
                  {getPriorityBadge(selectedIncident.severity).label}
                </span>
              </div>

              <div>
                <h5 style={{ margin: '0 0 8px 0', fontSize: '0.9rem', color: '#666' }}>Affected Service</h5>
                <p style={{ margin: 0 }}>{selectedIncident.affectedService}</p>
              </div>

              <div>
                <h5 style={{ margin: '0 0 8px 0', fontSize: '0.9rem', color: '#666' }}>Duration</h5>
                <p style={{ margin: 0 }}>{selectedIncident.duration}</p>
              </div>

              <div>
                <h5 style={{ margin: '0 0 8px 0', fontSize: '0.9rem', color: '#666' }}>Detected At</h5>
                <p style={{ margin: 0 }}>{selectedIncident.detectedAt ? new Date(selectedIncident.detectedAt).toLocaleString() : 'N/A'}</p>
              </div>

              <div>
                <h5 style={{ margin: '0 0 8px 0', fontSize: '0.9rem', color: '#666' }}>Reported At</h5>
                <p style={{ margin: 0 }}>{selectedIncident.reportedAt ? new Date(selectedIncident.reportedAt).toLocaleString() : 'N/A'}</p>
              </div>
            </div>

            {/* Action Buttons */}
            {selectedIncident.status !== 'resolved' && (
              <div style={{ display: 'flex', gap: 8, marginTop: 16 }}>
                <button
                  onClick={() => setSelectedIncident(null)}
                  style={{
                    flex: 1,
                    padding: '10px 16px',
                    borderRadius: 4,
                    border: 'none',
                    background: '#2d5a87',
                    color: '#fff',
                    cursor: 'pointer',
                    fontWeight: 600,
                  }}
                >
                  Manage Incident
                </button>
                <button
                  onClick={() => {
                    // Placeholder for external actions
                    console.log('External actions:', selectedIncident.id);
                  }}
                  style={{
                    padding: '10px 16px',
                    borderRadius: 4,
                    border: '1px solid #ddd',
                    background: '#fff',
                    cursor: 'pointer',
                  }}
                >
                  External Actions
                </button>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
};

export default IncidentManagement;