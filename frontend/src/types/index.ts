// Core type definitions for UptimeFoundry Web Console

export interface Node {
  id: string;
  type: NodeType;
  name: string;
  status: NodeStatus;
  metadata: Record<string, unknown>;
  position?: { x: number; y: number };
  children?: Node[];
  createdAt: string;
  updatedAt: string;
}

export type NodeType = 
  | 'host'
  | 'service'
  | 'swarm'
  | 'cluster'
  | 'container'
  | 'network'
  | 'storage'
  | 'external'
  | 'database'
  | 'cache'
  | 'queue'
  | 'load_balancer'
  | 'unknown';

export type NodeStatus = 
  | 'healthy'
  | 'degraded'
  | 'unhealthy'
  | 'unknown'
  | 'maintenance'
  | 'offline';

export interface Edge {
  id: string;
  source: string;
  target: string;
  type: EdgeType;
  metadata?: Record<string, unknown>;
}

export type EdgeType = 
  | 'depends_on'
  | 'communicates_with'
  | 'owned_by'
  | 'routes_to'
  | 'stores_data';

export interface Topology {
  nodes: Node[];
  edges: Edge[];
  metadata: {
    clusterId: string;
    lastUpdated: string;
    version: string;
  };
}

export interface Metric {
  name: string;
  value: number;
  unit: string;
  timestamp: string;
  labels: Record<string, string>;
}

export interface Metrics {
  cpu: {
    usage: number;
    load: number;
    cores: number;
    history: number[];
  };
  memory: {
    usage: number;
    available: number;
    total: number;
    history: number[];
  };
  network: {
    inbound: number;
    outbound: number;
    packets: number;
    history: number[];
  };
  disk: {
    usage: number;
    read: number;
    write: number;
    history: number[];
  };
}

export interface LogEntry {
  id: string;
  timestamp: string;
  level: LogLevel;
  message: string;
  service: string;
  container?: string;
  host?: string;
  metadata?: Record<string, unknown>;
}

export interface Logs {
  entries: Array<{
    timestamp: string;
    level: LogLevel;
    service: string;
    message: string;
  }>;
}

export type LogLevel = 'debug' | 'info' | 'warn' | 'error';

export interface Incident {
  id: string;
  title: string;
  description: string;
  severity: IncidentSeverity;
  status: IncidentStatus;
  assignedTo?: string;
  createdAt: string;
  updatedAt: string;
  resolvedAt?: string;
  rootCause?: string;
  impact: string;
  affectedService?: string;
  duration?: string;
  detectedAt?: string;
  reportedAt?: string;
  actions: IncidentAction[];
  nodeIds?: string[];
  metadata?: Record<string, unknown>;
}

export type IncidentSeverity = 'critical' | 'high' | 'medium' | 'low';
export type IncidentStatus = 'open' | 'investigating' | 'identified' | 'monitoring' | 'resolving' | 'resolved';

export interface IncidentAction {
  id: string;
  title: string;
  description: string;
  type: ActionCategory;
  status: ActionStatus;
  createdAt: string;
  createdBy: string;
  approvedBy?: string;
  completedAt?: string;
}

export type ActionCategory = 'restart' | 'scale' | 'rollback' | 'investigate' | 'deploy';
export type ActionStatus = 'pending' | 'approved' | 'rejected' | 'executing' | 'completed' | 'failed';

export interface EvidenceItem {
  type: string;
  timestamp: string;
  description: string;
  metadata?: Record<string, unknown>;
}

export interface InvestigationResult {
  id: string;
  incidentId: string;
  title: string;
  summary: string;
  findings: InvestigationFinding[];
  recommendations: InvestigationRecommendation[];
  rootCause?: string;
  recommendedAction?: string;
  confidence: number;
  model: string;
  createdAt: string;
  expiresAt: string;
  status: InvestigationStatus;
  evidence?: EvidenceItem[];
  fullAnalysis?: string;
  timeline?: InvestigationTimelineEvent[];
  completedAt?: string;
  error?: string;
  approved?: boolean;
  rejected?: boolean;
  requestingMoreInfo?: boolean;
}

export interface InvestigationTimelineEvent {
  timestamp: string;
  action: string;
  details: string;
}

export type InvestigationStatus = 'pending' | 'analyzing' | 'complete' | 'expired';

// Type alias for Investigation (matches InvestigationResult structure)
export type Investigation = InvestigationResult;

export interface InvestigationFinding {
  id: string;
  title: string;
  description: string;
  evidence: string[];
  severity: FindingSeverity;
  category: FindingCategory;
}

export type FindingSeverity = 'critical' | 'high' | 'medium' | 'low';
export type FindingCategory = 
  | 'resource_exhaustion'
  | 'configuration_error'
  | 'network_issue'
  | 'dependency_failure'
  | 'security_vulnerability'
  | 'performance_degradation'
  | 'unknown';

export interface InvestigationRecommendation {
  id: string;
  title: string;
  description: string;
  action: string;
  impact: string;
  risk: number; // 0-100
  implementation: string;
}

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

export type RemediationCategory = 
  | 'restart_service'
  | 'scale_up'
  | 'scale_down'
  | 'rollback'
  | 'deploy_fix'
  | 'network_restart'
  | 'config_update';

export interface User {
  id: string;
  email: string;
  name: string;
  role: UserRole;
  permissions: string[];
}

export type UserRole = 'admin' | 'operator' | 'viewer';

export interface DashboardConfig {
  id: string;
  name: string;
  layout: DashboardLayout[];
  filters: {
    clusterId?: string;
    timeRange?: string;
    severity?: IncidentSeverity[];
  };
}

export type DashboardLayout = {
  id: string;
  type: 'metric' | 'log' | 'incident' | 'topology' | 'custom';
  title: string;
  x: number;
  y: number;
  width: number;
  height: number;
  config: Record<string, unknown>;
};
