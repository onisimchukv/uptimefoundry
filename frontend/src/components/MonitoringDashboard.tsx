'use client';

import React, { useState, useEffect } from 'react';
import {
  LineChart,
  Line,
  AreaChart,
  Area,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  Legend,
  ResponsiveContainer,
} from 'recharts';
import { useQuery } from '@tanstack/react-query';
import { api } from '../lib/api';
import type { Metric, LogEntry } from '../types';

export const MonitoringDashboard: React.FC = () => {
  // Fetch monitoring data using React Query
  const { data: monitoringData, isLoading, error } = useQuery({
    queryKey: ['monitoring'],
    queryFn: async () => {
      const data = await api.getMonitoring();
      return data;
    },
    staleTime: 5000,
    refetchInterval: 5000,
  });
  const [selectedMetric, setSelectedMetric] = useState<string>('cpu_usage');
  const [logFilter, setLogFilter] = useState<string>('all');
  const [timeRange, setTimeRange] = useState<string>('5m');

  // Mock data for demonstration
  const mockMetrics = [
    {
      name: 'cpu_usage',
      labels: { '0': '12', '1': '25', '2': '18', '3': '32', '4': '28', '5': '22', '6': '35', '7': '42', '8': '38', '9': '30' }
    },
    {
      name: 'memory_usage',
      labels: { '0': '45', '1': '48', '2': '52', '3': '55', '4': '50', '5': '47', '6': '53', '7': '58', '8': '56', '9': '54' }
    },
    {
      name: 'network_in',
      labels: { '0': '120', '1': '135', '2': '118', '3': '142', '4': '128', '5': '133', '6': '145', '7': '150', '8': '140', '9': '138' }
    },
    {
      name: 'network_out',
      labels: { '0': '85', '1': '92', '2': '78', '3': '105', '4': '88', '5': '95', '6': '110', '7': '115', '8': '100', '9': '98' }
    },
    {
      name: 'disk_io',
      labels: { '0': '45', '1': '52', '2': '48', '3': '65', '4': '55', '5': '58', '6': '70', '7': '75', '8': '68', '9': '62' }
    },
    {
      name: 'connections',
      labels: { '0': '12', '1': '15', '2': '18', '3': '22', '4': '20', '5': '25', '6': '28', '7': '30', '8': '26', '9': '24' }
    }
  ];

  const mockLogs: LogEntry[] = [
    { id: '1', timestamp: new Date().toISOString(), level: 'info', service: 'api-gateway', message: 'Request processed successfully' },
    { id: '2', timestamp: new Date(Date.now() - 60000).toISOString(), level: 'debug', service: 'auth-service', message: 'Token validated' },
    { id: '3', timestamp: new Date(Date.now() - 120000).toISOString(), level: 'warn', service: 'database', message: 'Slow query detected' },
    { id: '4', timestamp: new Date(Date.now() - 180000).toISOString(), level: 'error', service: 'cache-service', message: 'Connection timeout' },
    { id: '5', timestamp: new Date(Date.now() - 240000).toISOString(), level: 'info', service: 'api-gateway', message: 'Health check passed' },
    { id: '6', timestamp: new Date(Date.now() - 300000).toISOString(), level: 'info', service: 'worker', message: 'Job completed' },
    { id: '7', timestamp: new Date(Date.now() - 360000).toISOString(), level: 'debug', service: 'api-gateway', message: 'Cache hit' },
    { id: '8', timestamp: new Date(Date.now() - 420000).toISOString(), level: 'warn', service: 'auth-service', message: 'Rate limit approaching' },
    { id: '9', timestamp: new Date(Date.now() - 480000).toISOString(), level: 'info', service: 'database', message: 'Connection pool healthy' },
    { id: '10', timestamp: new Date(Date.now() - 540000).toISOString(), level: 'error', service: 'worker', message: 'Task failed' },
  ];

  const filteredLogs = logFilter === 'all'
    ? mockLogs
    : mockLogs.filter(log => log.level === logFilter);

  const getMetricData = () => {
    if (isLoading) return [];
    
    if (mockMetrics.length > 0) {
      const metric = mockMetrics.find(m => m.name === selectedMetric) || mockMetrics[0];
      const values = metric.labels ? Object.entries(metric.labels) : [];
      return values.map(([label, value], index) => ({
        name: index,
        [selectedMetric]: parseFloat(value)
      }));
    }
    return [];
  };
  
  const getLogLevels = () => {
    return ['all', 'debug', 'info', 'warn', 'error'];
  };

  const getLogColors = () => {
    return {
      all: '#666',
      debug: '#90caf9',
      info: '#4caf50',
      warn: '#ff9800',
      error: '#f44336',
    };
  };

  return (
    <div className="card">
      <div className="card-header">
        <h3 className="card-title">Real-time Monitoring</h3>
        <div style={{ display: 'flex', gap: 8 }}>
          <select
            value={timeRange}
            onChange={(e) => console.log('Time range:', e.target.value)}
            style={{ padding: '6px 12px', borderRadius: 4, border: '1px solid #ddd' }}
          >
            <option value="5m">Last 5 minutes</option>
            <option value="15m">Last 15 minutes</option>
            <option value="1h">Last 1 hour</option>
            <option value="4h">Last 4 hours</option>
            <option value="24h">Last 24 hours</option>
          </select>
        </div>
      </div>

      {/* Metrics Section */}
      <div style={{ marginBottom: 24 }}>
        <h4 style={{ margin: '0 0 12px 0', fontSize: '0.95rem', color: '#666' }}>
          Metrics
        </h4>
        
        <div style={{ marginBottom: 16, display: 'flex', gap: 8, flexWrap: 'wrap' }}>
          {['cpu_usage', 'memory_usage', 'network_in', 'network_out', 'disk_io', 'connections'].map((metric) => (
            <button
              key={metric}
              onClick={() => setSelectedMetric(metric)}
              style={{
                padding: '6px 12px',
                borderRadius: 4,
                border: 'none',
                background: selectedMetric === metric ? '#2d5a87' : '#e0e0e0',
                color: selectedMetric === metric ? '#fff' : '#333',
                fontSize: '0.8rem',
                cursor: 'pointer',
              }}
            >
              {metric.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase())}
            </button>
          ))}
        </div>

        <div style={{ height: 250 }}>
          <ResponsiveContainer>
            <AreaChart data={getMetricData()}>
              <defs>
                <linearGradient id="colorCpu" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#2d5a87" stopOpacity={0.8} />
                  <stop offset="95%" stopColor="#2d5a87" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#e0e0e0" />
              <XAxis dataKey="name" tick={{ fontSize: 10 }} />
              <YAxis tick={{ fontSize: 10 }} />
              <Tooltip
                contentStyle={{
                  backgroundColor: '#fff',
                  border: '1px solid #ddd',
                  borderRadius: 4,
                }}
              />
              <Legend />
              <Area
                type="monotone"
                dataKey={selectedMetric}
                stroke="#2d5a87"
                fillOpacity={1}
                fill="url(#colorCpu)"
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Logs Section */}
      <div>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
          <h4 style={{ margin: 0, fontSize: '0.95rem', color: '#666' }}>
            Recent Logs
          </h4>
          <select
            value={logFilter}
            onChange={(e) => setLogFilter(e.target.value)}
            style={{ padding: '6px 12px', borderRadius: 4, border: '1px solid #ddd', fontSize: '0.8rem' }}
          >
            {getLogLevels().map((level) => (
              <option key={level} value={level}>
                {level === 'all' ? 'All Levels' : level.charAt(0).toUpperCase() + level.slice(1)}
              </option>
            ))}
          </select>
        </div>

        <div style={{ height: 250, overflowY: 'auto' }}>
          {filteredLogs.length === 0 ? (
            <div style={{ textAlign: 'center', padding: 24, color: '#999' }}>
              No logs available
            </div>
          ) : (
            <table style={{ width: '100%', borderCollapse: 'collapse' }}>
              <thead style={{ position: 'sticky', top: 0, background: '#fff', zIndex: 1 }}>
                <tr style={{ borderBottom: '2px solid #e0e0e0' }}>
                  <th style={{ textAlign: 'left', padding: 8, fontSize: '0.8rem', color: '#666' }}>
                    Time
                  </th>
                  <th style={{ textAlign: 'left', padding: 8, fontSize: '0.8rem', color: '#666' }}>
                    Level
                  </th>
                  <th style={{ textAlign: 'left', padding: 8, fontSize: '0.8rem', color: '#666' }}>
                    Service
                  </th>
                  <th style={{ textAlign: 'left', padding: 8, fontSize: '0.8rem', color: '#666' }}>
                    Message
                  </th>
                </tr>
              </thead>
              <tbody>
                {filteredLogs.map((log) => (
                  <tr key={log.id} style={{ borderBottom: '1px solid #f0f0f0' }}>
                    <td style={{ padding: 8, fontSize: '0.85rem', color: '#666' }}>
                      {new Date(log.timestamp).toLocaleTimeString()}
                    </td>
                    <td style={{ padding: 8 }}>
                      <span
                        style={{
                          padding: '2px 8px',
                          borderRadius: 4,
                          fontSize: '0.75rem',
                          fontWeight: 600,
                          textTransform: 'uppercase',
                          background: getLogColors()[log.level] || '#ccc',
                        }}
                      >
                        {log.level}
                      </span>
                    </td>
                    <td style={{ padding: 8, fontSize: '0.85rem' }}>{log.service}</td>
                    <td style={{ padding: 8, fontSize: '0.85rem', color: '#333' }}>
                      {log.message}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  );
};
