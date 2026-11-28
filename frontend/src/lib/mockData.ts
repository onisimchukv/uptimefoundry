import type { Topology, Metric, Metrics, LogEntry, Logs, Incident, Investigation, RemediationAction, Node, Edge, NodeType, NodeStatus, EdgeType } from '../types';

// Generate mock topology data
export function generateMockTopology(): Topology {
  const now = new Date().toISOString();
  
  const nodes: Node[] = [
    {
      id: 'cluster-prod-1',
      type: 'cluster',
      name: 'Production Cluster 1',
      status: 'healthy',
      metadata: { nodes: 12, region: 'us-east-1' },
      createdAt: now,
      updatedAt: now,
    },
    {
      id: 'host-1',
      type: 'host',
      name: 'worker-1',
      status: 'healthy',
      metadata: { cpu: 45, memory: 62 },
      createdAt: now,
      updatedAt: now,
    },
    {
      id: 'host-2',
      type: 'host',
      name: 'worker-2',
      status: 'healthy',
      metadata: { cpu: 38, memory: 55 },
      createdAt: now,
      updatedAt: now,
    },
    {
      id: 'service-api',
      type: 'service',
      name: 'API Gateway',
      status: 'healthy',
      metadata: { replicas: 3, version: 'v2.1.0' },
      createdAt: now,
      updatedAt: now,
    },
    {
      id: 'service-db',
      type: 'service',
      name: 'Database',
      status: 'healthy',
      metadata: { replicas: 1, version: '14.5' },
      createdAt: now,
      updatedAt: now,
    },
    {
      id: 'container-api-1',
      type: 'container',
      name: 'api-gateway-1',
      status: 'healthy',
      metadata: { pod: 'api-1' },
      createdAt: now,
      updatedAt: now,
    },
    {
      id: 'container-api-2',
      type: 'container',
      name: 'api-gateway-2',
      status: 'healthy',
      metadata: { pod: 'api-2' },
      createdAt: now,
      updatedAt: now,
    },
    {
      id: 'network-internal',
      type: 'network',
      name: 'Internal Network',
      status: 'healthy',
      metadata: { subnet: '10.0.0.0/16' },
      createdAt: now,
      updatedAt: now,
    },
  ];

  const edges: Edge[] = [
    { id: 'e1', source: 'cluster-prod-1', target: 'host-1', type: 'owned_by' },
    { id: 'e2', source: 'cluster-prod-1', target: 'host-2', type: 'owned_by' },
    { id: 'e3', source: 'host-1', target: 'service-api', type: 'communicates_with' },
    { id: 'e4', source: 'host-1', target: 'service-db', type: 'communicates_with' },
    { id: 'e5', source: 'host-2', target: 'service-api', type: 'communicates_with' },
    { id: 'e6', source: 'service-api', target: 'container-api-1', type: 'owned_by' },
    { id: 'e7', source: 'service-api', target: 'container-api-2', type: 'owned_by' },
    { id: 'e8', source: 'container-api-1', target: 'network-internal', type: 'routes_to' },
    { id: 'e9', source: 'container-api-2', target: 'network-internal', type: 'routes_to' },
  ];

  return {
    nodes,
    edges,
    metadata: {
      clusterId: 'cluster-prod-1',
      lastUpdated: now,
      version: '1.0.0',
    },
  };
}

// Generate mock metrics data
export function generateMockMetrics(): Metrics {
  return {
    cpu: {
      usage: 45.2,
      load: 2.5,
      cores: 8,
      history: [42, 45, 48, 43, 46, 44, 47, 49, 45, 45],
    },
    memory: {
      usage: 62.5,
      available: 8192,
      total: 16384,
      history: [60, 61, 63, 62, 64, 65, 63, 62, 61, 62],
    },
    network: {
      inbound: 125.5,
      outbound: 89.3,
      packets: 15000,
      history: [120, 125, 130, 128, 135, 132, 138, 135, 130, 125],
    },
    disk: {
      usage: 58.2,
      read: 125.5,
      write: 89.3,
      history: [55, 56, 57, 58, 57, 58, 59, 58, 57, 58],
    },
  };
}

// Generate mock logs data
export function generateMockLogs(): Logs {
  const now = new Date();
  return {
    entries: [
      {
        timestamp: new Date(now.getTime() - 1000).toISOString(),
        level: 'info',
        service: 'api-gateway',
        message: 'Request processed successfully',
      },
      {
        timestamp: new Date(now.getTime() - 2000).toISOString(),
        level: 'info',
        service: 'database',
        message: 'Query executed in 15ms',
      },
      {
        timestamp: new Date(now.getTime() - 3000).toISOString(),
        level: 'warn',
        service: 'api-gateway',
        message: 'High latency detected: 250ms',
      },
      {
        timestamp: new Date(now.getTime() - 4000).toISOString(),
        level: 'info',
        service: 'scheduler',
        message: 'Health check passed',
      },
      {
        timestamp: new Date(now.getTime() - 5000).toISOString(),
        level: 'error',
        service: 'cache',
        message: 'Connection timeout to Redis',
      },
    ],
  };
}

// Generate mock incidents data
export function generateMockIncidents(): Incident[] {
  const now = new Date();
  
  return [
    {
      id: 'inc-001',
      title: 'High CPU Usage on worker-1',
      description: 'CPU usage exceeded 90% threshold on worker-1 for more than 5 minutes',
      severity: 'high',
      status: 'investigating',
      createdAt: new Date(now.getTime() - 3600000).toISOString(),
      updatedAt: new Date(now.getTime() - 1800000).toISOString(),
      nodeIds: ['host-1'],
      metadata: { cpu_usage: 92.5, duration: '2 hours' },
      impact: 'Service latency increased by 200ms',
      actions: [],
    },
    {
      id: 'inc-002',
      title: 'Service timeout errors',
      description: 'Multiple timeout errors reported from API Gateway service',
      severity: 'medium',
      status: 'open',
      createdAt: new Date(now.getTime() - 7200000).toISOString(),
      updatedAt: new Date(now.getTime() - 7200000).toISOString(),
      nodeIds: ['service-api', 'container-api-1'],
      metadata: { timeout_count: 15, service: 'api-gateway' },
      impact: 'API response times exceeding SLA',
      actions: [],
    },
    {
      id: 'inc-003',
      title: 'Database connection pool exhausted',
      description: 'Database connection pool reached maximum capacity',
      severity: 'critical',
      status: 'resolving',
      createdAt: new Date(now.getTime() - 1800000).toISOString(),
      updatedAt: new Date(now.getTime() - 900000).toISOString(),
      nodeIds: ['service-db'],
      metadata: { pool_size: 100, active: 100, waiting: 25 },
      impact: 'Database queries failing, affecting all dependent services',
      actions: [],
    },
  ];
}

// Generate mock investigations data
export function generateMockInvestigations(): Investigation[] {
  return [
    {
      id: 'inv-001',
      incidentId: 'inc-003',
      status: 'complete',
      title: 'Database Connection Pool Exhaustion',
      summary: 'Connection pool reached maximum capacity causing application timeouts',
      findings: [
        {
          id: 'f-001',
          title: 'Connection Pool Exhaustion',
          description: 'The database connection pool has been exhausted due to high traffic',
          evidence: [
            'Connection pool logs show 100% utilization',
            'Queue depth increased to 25 waiting connections',
          ],
          severity: 'high',
          category: 'resource_exhaustion',
        },
      ],
      recommendations: [
        {
          id: 'r-001',
          title: 'Increase Connection Pool Size',
          description: 'Increase the database connection pool size to handle higher traffic loads',
          action: 'Increase database connection pool size from 100 to 200',
          impact: 'high',
          risk: 10,
          implementation: 'Update database configuration to increase pool size',
        },
      ],
      rootCause: 'Connection pool size is too small for current traffic load',
      recommendedAction: 'Increase database connection pool size from 100 to 200',
      confidence: 0.92,
      model: 'sentry-ai-v2',
      evidence: [
        {
          type: 'log',
          timestamp: new Date().toISOString(),
          description: 'Connection pool logs show 100% utilization',
        },
        {
          type: 'metric',
          timestamp: new Date().toISOString(),
          description: 'Queue depth increased to 25 waiting connections',
        },
        {
          type: 'metric',
          timestamp: new Date().toISOString(),
          description: 'Application response times increased by 300%',
        },
      ],
      fullAnalysis: `
## Root Cause Analysis

The database connection pool has reached its maximum capacity of 100 connections, causing new connections to queue up and resulting in application timeouts.

### Contributing Factors:
1. Sudden traffic spike during peak hours
2. Connection leaks in legacy code paths
3. Insufficient pool size for current load

### Impact:
- API response times increased by 300%
- Multiple timeout errors reported
- User experience significantly degraded

### Resolution:
Increased connection pool size from 100 to 200 and added connection timeout handling.
      `,
      timeline: [
        { timestamp: '10:00', action: 'Traffic spike detected', details: 'Sudden increase in incoming requests' },
        { timestamp: '10:05', action: 'Connection pool reached 90% capacity', details: 'Pool utilization approaching critical threshold' },
        { timestamp: '10:10', action: 'Pool exhausted, connections queuing', details: 'All available connections in use' },
        { timestamp: '10:15', action: 'Incident detected and escalated', details: 'Alert triggered due to connection queue depth' },
        { timestamp: '10:30', action: 'Pool size increased to 200', details: 'Connection pool resized to handle increased load' },
        { timestamp: '10:45', action: 'Connections cleared, normal operation resumed', details: 'Queue depth normalized, service healthy' },
      ],
      createdAt: new Date(Date.now() - 1800000).toISOString(),
      expiresAt: new Date(Date.now() + 86400000).toISOString(),
      completedAt: new Date(Date.now() - 900000).toISOString(),
      error: '',
    },
  ];
}

// Generate mock remediation action
export function generateMockRemediation(): RemediationAction {
  const now = new Date();
  
  return {
    id: `rem-${Date.now()}`,
    investigationId: 'inv-001',
    incidentId: 'inc-003',
    status: 'ready',
    actionType: 'Scale Database Connection Pool',
    description: 'Increase database connection pool size from 100 to 200 and add connection timeout handling',
    severity: 'medium',
    targets: [
      { id: 'service-db', name: 'Database Service', type: 'service' },
      { id: 'host-1-db', name: 'DB Host 1', type: 'host' },
    ],
    estimatedDuration: '5-10 minutes',
    progress: {
      completed: 0,
      total: 2,
      details: [],
    },
    logs: [],
    createdAt: now.toISOString(),
    completedAt: '',
    error: '',
  };
}

// Export combined mock data
export const mockData = {
  topology: generateMockTopology(),
  metrics: generateMockMetrics(),
  logs: generateMockLogs(),
  incidents: generateMockIncidents(),
  investigations: generateMockInvestigations(),
  remediation: generateMockRemediation(),
};
