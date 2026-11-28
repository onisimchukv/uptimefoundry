// API types for UptimeFoundry control plane integration
import type { NodeType, NodeStatus, Node, Edge, Incident, IncidentAction, LogLevel, LogEntry, IncidentStatus, IncidentSeverity, Topology, InvestigationResult, InvestigationFinding, InvestigationRecommendation, Metric } from './index';

export interface ApiResponse<T = unknown> {
  success: boolean;
  data?: T;
  error?: ApiError;
  timestamp: string;
}

export interface ApiError {
  code: string;
  message: string;
  details?: Record<string, unknown>;
  requestId: string;
}

// Topology API
export interface TopologyRequest {
  clusterId?: string;
  depth?: number;
  includeMetadata?: boolean;
  format: 'graph' | 'tree';
}

export interface TopologyResponse {
  nodes: Node[];
  edges: Edge[];
  metadata: {
    clusterId: string;
    lastUpdated: string;
    version: string;
  };
}

// Monitoring API
export interface MetricsRequest {
  service?: string;
  container?: string;
  host?: string;
  metrics: string[];
  startTime: string;
  endTime: string;
  resolution: string; // '1m', '5m', '1h'
}

export interface MetricsResponse {
  series: MetricSeries[];
  labels: string[];
}

export interface MetricSeries {
  metric: string;
  values: [number, number][]; // [timestamp, value]
  labels: Record<string, string>;
}

// Logs API
export interface LogsRequest {
  service?: string;
  container?: string;
  host?: string;
  level?: LogLevel;
  startTime: string;
  endTime: string;
  limit: number;
  follow?: boolean;
}

export interface LogsResponse {
  entries: LogEntry[];
  total: number;
  hasNextPage: boolean;
}

// Incidents API
export interface IncidentsRequest {
  status?: IncidentStatus;
  severity?: IncidentSeverity[];
  assignedTo?: string;
  startTime?: string;
  endTime?: string;
  limit?: number;
  offset?: number;
}

export interface IncidentsResponse {
  incidents: Incident[];
  total: number;
  hasNextPage: boolean;
}

export interface IncidentCreateRequest {
  title: string;
  description: string;
  severity: IncidentSeverity;
  impact: string;
  assignedTo?: string;
}

export interface IncidentUpdateRequest {
  status?: IncidentStatus;
  description?: string;
  rootCause?: string;
  assignedTo?: string;
}

// AI Investigation API
export interface InvestigationRequest {
  incidentId: string;
  context?: InvestigationContext;
  includeRecommendations?: boolean;
}

export interface InvestigationContext {
  logs?: string[];
  metrics?: Record<string, unknown>[];
  topologySnapshot?: Topology;
  recentIncidents?: Incident[];
}

export interface InvestigationResponse {
  result: InvestigationResult;
  processingTimeMs: number;
}

export interface InvestigationsResponse {
  investigations: InvestigationResult[];
  total: number;
  hasNextPage: boolean;
}

// Actions API
export interface ActionsRequest {
  incidentId: string;
  actionId: string;
  parameters?: Record<string, unknown>;
}

export interface ActionsResponse {
  action: IncidentAction;
  status: 'pending' | 'approved' | 'rejected' | 'executing' | 'completed' | 'failed';
}

export interface ActionApprovalRequest {
  actionId: string;
  approval: 'approve' | 'reject';
  reason?: string;
}

// WebSocket Events
export interface WebSocketMessage {
  type: string;
  payload: unknown;
  timestamp: string;
  correlationId?: string;
}

export interface TopologyUpdateEvent extends WebSocketMessage {
  type: 'topology:update';
  payload: TopologyResponse;
}

export interface MetricsUpdateEvent extends WebSocketMessage {
  type: 'metrics:update';
  payload: {
    service: string;
    metrics: MetricSeries[];
  };
}

// Monitoring API
export interface MonitoringResponse {
  metrics: MetricSeries[];
  logs: LogEntry[];
}

export interface IncidentUpdateEvent extends WebSocketMessage {
  type: 'incident:update';
  payload: Incident;
}

export interface InvestigationUpdateEvent extends WebSocketMessage {
  type: 'investigation:update';
  payload: InvestigationResult;
}

export interface ActionStatusEvent extends WebSocketMessage {
  type: 'action:status';
  payload: IncidentAction;
}

// SSE Events
export interface SSEEvent {
  id: string;
  event: string;
  data: string;
  retry?: number;
}

export interface AlertEvent {
  id: string;
  event: 'alert';
  data: {
    type: 'incident' | 'metric_threshold' | 'topology_change';
    payload: unknown;
  };
  retry?: number;
}

export interface InvestigationCompleteEvent {
  id: string;
  event: 'investigation:complete';
  data: InvestigationResult;
}

// Remediation API
export interface RemediationAction {
  id: string;
  investigationId?: string;
  incidentId?: string;
  status: 'pending' | 'ready' | 'executing' | 'completed' | 'failed' | 'cancelled';
  actionType: string;
  description: string;
  severity: 'low' | 'medium' | 'high' | 'critical';
  targets: Array<{
    id: string;
    name: string;
    type: NodeType;
  }>;
  estimatedDuration: string;
  progress?: {
    completed: number;
    total: number;
    details: string[];
  };
  logs?: Array<{
    timestamp: string;
    level: 'info' | 'warn' | 'error';
    message: string;
  }>;
  error?: string;
  createdAt: string;
  completedAt: string;
}

// Dashboard API
export interface DashboardResponse {
  topology: TopologyResponse;
  incidents: {
    count: number;
    recent: Incident[];
  };
}

// Re-export core types from index.ts
export type {
  NodeType,
  NodeStatus,
  Node,
  Edge,
  Incident,
  IncidentAction,
  LogEntry,
} from './index';

// Re-export DashboardResponse (already exported above)
