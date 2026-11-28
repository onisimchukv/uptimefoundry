// API client for UptimeFoundry control plane

import axios, {
  AxiosInstance,
  AxiosRequestConfig,
  AxiosResponse,
  InternalAxiosRequestConfig,
} from 'axios';
import { v4 as uuidv4 } from 'uuid';
import type {
  ApiResponse,
  TopologyResponse,
  MetricsResponse,
  LogsResponse,
  IncidentsResponse,
  InvestigationResponse,
  InvestigationsResponse,
  ActionsResponse,
  ActionApprovalRequest,
  InvestigationRequest,
  LogsRequest,
  MetricsRequest,
  IncidentsRequest,
  TopologyRequest,
  IncidentCreateRequest,
  IncidentUpdateRequest,
  ActionsRequest,
  RemediationAction,
  Incident,
  MonitoringResponse,
  LogEntry,
  DashboardResponse,
} from '../types/api';

// Configuration
const API_BASE_URL = process.env.NEXT_PUBLIC_API_BASE_URL || 'http://localhost:8080/api/v1';

// Create axios instance
const client: AxiosInstance = axios.create({
  baseURL: API_BASE_URL,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// Request interceptor for auth token
let accessToken: string | null = null;

export function setAuthToken(token: string): void {
  accessToken = token;
  client.defaults.headers.common['Authorization'] = `Bearer ${token}`;
}

export function clearAuthToken(): void {
  accessToken = null;
  delete client.defaults.headers.common['Authorization'];
}

// Request interceptor to add request ID
client.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    config.headers['X-Request-ID'] = uuidv4();
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// Response interceptor for error handling
client.interceptors.response.use(
  (response: AxiosResponse) => response.data,
  (error) => {
    const response = error.response;
    if (response?.status === 401) {
      // Token expired, clear and redirect to login
      clearAuthToken();
      window.location.href = '/login';
    }
    return Promise.reject({
      success: false,
      error: {
        code: response?.status.toString() || 'NETWORK_ERROR',
        message: response?.data?.message || error.message || 'Request failed',
        details: response?.data?.details,
        requestId: response?.headers?.['x-request-id'],
      },
      timestamp: new Date().toISOString(),
    });
  }
);

// ============ Topology API ============

export async function getTopology(
  params: TopologyRequest
): Promise<ApiResponse<TopologyResponse>> {
  const response = await client.get('/topology', { params });
  return { success: true, data: response.data as TopologyResponse, timestamp: new Date().toISOString() };
}

export async function getTopologyStream(
  clusterId: string,
  onMessage: (message: TopologyResponse) => void
): Promise<() => void> {
  // WebSocket connection for topology streaming
  // Implementation in WebSocket client
  return () => {};
}

// ============ Metrics API ============

export async function getMetrics(
  params: MetricsRequest
): Promise<ApiResponse<MetricsResponse>> {
  const response = await client.get('/metrics', { params });
  return { success: true, data: response.data as MetricsResponse, timestamp: new Date().toISOString() };
}

// ============ Logs API ============

export async function getLogs(
  params: LogsRequest
): Promise<ApiResponse<LogsResponse>> {
  const response = await client.get('/logs', { params });
  return { success: true, data: response.data as LogsResponse, timestamp: new Date().toISOString() };
}

export async function followLogs(
  params: LogsRequest,
  onLog: (log: LogEntry) => void
): Promise<() => void> {
  // WebSocket connection for log streaming
  return () => {};
}

// ============ Incidents API ============

export async function getIncidents(
  params: IncidentsRequest
): Promise<ApiResponse<IncidentsResponse>> {
  const response = await client.get('/incidents', { params });
  return { success: true, data: response.data as IncidentsResponse, timestamp: new Date().toISOString() };
}

export async function createIncident(
  data: IncidentCreateRequest
): Promise<ApiResponse<Incident>> {
  const response = await client.post('/incidents', data);
  return { success: true, data: response.data, timestamp: new Date().toISOString() };
}

export async function updateIncident(
  incidentId: string,
  data: IncidentUpdateRequest
): Promise<ApiResponse<Incident>> {
  const response = await client.patch(`/incidents/${incidentId}`, data);
  return { success: true, data: response.data, timestamp: new Date().toISOString() };
}

export async function deleteIncident(
  incidentId: string
): Promise<ApiResponse<null>> {
  const response = await client.delete(`/incidents/${incidentId}`);
  return { success: true, data: null, timestamp: new Date().toISOString() };
}

// ============ AI Investigation API ============

export async function triggerInvestigation(
  params: InvestigationRequest
): Promise<ApiResponse<InvestigationResponse>> {
  const response = await client.post('/investigations', params);
  return { success: true, data: response.data as InvestigationResponse, timestamp: new Date().toISOString() };
}

export async function getInvestigation(
  investigationId: string
): Promise<ApiResponse<InvestigationResponse>> {
  const response = await client.get(`/investigations/${investigationId}`);
  return { success: true, data: response.data as InvestigationResponse, timestamp: new Date().toISOString() };
}

export async function getInvestigations(
  params?: {
    status?: string;
    limit?: number;
    offset?: number;
  }
): Promise<ApiResponse<InvestigationsResponse>> {
  const queryParams = new URLSearchParams();
  if (params?.status) queryParams.append('status', params.status);
  if (params?.limit) queryParams.append('limit', params.limit.toString());
  if (params?.offset) queryParams.append('offset', params.offset.toString());

  const response = await client.get(`/investigations${queryParams.toString() ? '?' + queryParams.toString() : ''}`);
  return { success: true, data: response.data as InvestigationsResponse, timestamp: new Date().toISOString() };
}

// ============ Actions API ============

export async function initiateAction(
  params: ActionsRequest
): Promise<ApiResponse<ActionsResponse>> {
  const response = await client.post('/actions', params);
  return { success: true, data: response.data as ActionsResponse, timestamp: new Date().toISOString() };
}

export async function approveAction(
  actionId: string,
  approval: ActionApprovalRequest
): Promise<ApiResponse<ActionsResponse>> {
  const response = await client.post(`/actions/${actionId}/approve`, approval);
  return { success: true, data: response.data as ActionsResponse, timestamp: new Date().toISOString() };
}

export async function rejectAction(
  actionId: string,
  reason: string
): Promise<ApiResponse<ActionsResponse>> {
  const response = await client.post(`/actions/${actionId}/reject`, { reason });
  return { success: true, data: response.data as ActionsResponse, timestamp: new Date().toISOString() };
}

// ============ Remediation API ============

export async function getRemediation(
  remediationId: string
): Promise<ApiResponse<RemediationAction>> {
  const response = await client.get(`/remediation/${remediationId}`);
  return { success: true, data: response.data, timestamp: new Date().toISOString() };
}

export async function executeRemediation(
  remediationId: string,
  targetId?: string,
  mode: 'single' | 'all' = 'all'
): Promise<ApiResponse<RemediationAction>> {
  const response = await client.post(`/remediation/${remediationId}/execute`, {
    targetId,
    mode,
  });
  return { success: true, data: response.data, timestamp: new Date().toISOString() };
}

export async function cancelRemediation(
  remediationId: string
): Promise<ApiResponse<RemediationAction>> {
  const response = await client.post(`/remediation/${remediationId}/cancel`);
  return { success: true, data: response.data, timestamp: new Date().toISOString() };
}

export async function updateRemediationProgress(
  remediationId: string,
  progress: Partial<{
    completed: number;
    total: number;
    details: string[];
  }>
): Promise<ApiResponse<RemediationAction>> {
  const response = await client.patch(`/remediation/${remediationId}/progress`, progress);
  return { success: true, data: response.data, timestamp: new Date().toISOString() };
}

export async function addRemediationLog(
  remediationId: string,
  log: {
    level: 'info' | 'warn' | 'error';
    message: string;
  }
): Promise<ApiResponse<RemediationAction>> {
  const response = await client.post(`/remediation/${remediationId}/logs`, log);
  return { success: true, data: response.data, timestamp: new Date().toISOString() };
}

// ============ Monitoring API ============

export async function getMonitoring(): Promise<ApiResponse<MonitoringResponse>> {
  // Fetch metrics and logs for monitoring
  const metricsResponse = await getMetrics({
    service: '*',
    metrics: ['cpu_usage', 'memory_usage', 'disk_usage', 'network_in', 'network_out'],
    resolution: '1m',
    startTime: new Date(Date.now() - 3600000).toISOString(), // 1 hour ago
    endTime: new Date().toISOString(),
  });
  const logsResponse = await getLogs({
    service: '*',
    level: 'info',
    limit: 100,
    startTime: new Date(Date.now() - 3600000).toISOString(), // 1 hour ago
    endTime: new Date().toISOString(),
  });
  
  return {
    success: true,
    data: {
      metrics: (metricsResponse.data as MetricsResponse).series,
      logs: (logsResponse.data as LogsResponse).entries,
    },
    timestamp: new Date().toISOString(),
  };
}

// ============ Dashboard API ============

export async function getDashboard(): Promise<ApiResponse<DashboardResponse>> {
  // Fetch both topology and incidents for the dashboard
  const topologyResponse = await getTopology({ format: 'tree', depth: 2 });
  const incidentsResponse = await getIncidents({ limit: 5 });
  
  return {
    success: true,
    data: {
      topology: topologyResponse.data || ({} as TopologyResponse),
      incidents: {
        count: incidentsResponse.data?.total || 0,
        recent: incidentsResponse.data?.incidents?.slice(0, 5) || [],
      },
    },
    timestamp: new Date().toISOString(),
  };
}

// Create and export API object for namespace-style imports
export const api = {
  setAuthToken,
  clearAuthToken,
  getTopology,
  getTopologyStream,
  getMetrics,
  getLogs,
  followLogs,
  getMonitoring,
  getIncidents,
  createIncident,
  updateIncident,
  deleteIncident,
  triggerInvestigation,
  getInvestigation,
  getInvestigations,
  initiateAction,
  approveAction,
  rejectAction,
  getRemediation,
  executeRemediation,
  cancelRemediation,
  updateRemediationProgress,
  addRemediationLog,
  getDashboard,
};

// Export types
export type {
  ApiResponse,
  TopologyResponse,
  MetricsResponse,
  LogsResponse,
  IncidentsResponse,
  InvestigationResponse,
  ActionsResponse,
  ActionApprovalRequest,
  InvestigationRequest,
  LogsRequest,
  MetricsRequest,
  IncidentsRequest,
  TopologyRequest,
  IncidentCreateRequest,
  IncidentUpdateRequest,
  ActionsRequest,
} from '../types/api';
