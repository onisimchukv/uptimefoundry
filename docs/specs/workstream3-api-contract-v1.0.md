# Work Stream 3: API Contract Specification
## Control Plane & Web Console Integration

**Created:** November 28, 2026  
**Project:** UptimeFoundry  
**Milestone:** M10b  
**Version:** 1.0

---

## 1. Overview

This document defines the API contract between the UptimeFoundry control plane and the Operator Web Console. The control plane exposes REST APIs and WebSocket/SSE streams for real-time topology, monitoring, incident management, and control operations.

---

## 2. Base URLs

- **REST API:** `https://control-plane.example.com/api/v1`
- **WebSocket:** `wss://control-plane.example.com/ws`
- **SSE:** `https://control-plane.example.com/events`

---

## 3. Authentication

### 3.1 Token-Based Authentication

All REST API requests must include an authentication token:

```
Authorization: Bearer <access_token>
```

### 3.2 Token Types

- **Access Token:** Short-lived (1 hour), used for API requests
- **Refresh Token:** Long-lived (7 days), used to obtain new access tokens
- **Session Token:** WebSocket/SSE session identifier

---

## 4. REST API Endpoints

### 4.1 Topology Endpoints

#### Get Topology Snapshot

```
GET /api/v1/topology
Authorization: Bearer <access_token>

Query Parameters:
- depth: integer (0-10, default: 1)
- filter: object - Filter criteria
- format: string ("json", "graph")

Response (JSON):
{
  "version": 1,
  "timestamp": "2026-11-28T10:00:00Z",
  "nodes": [
    {
      "id": "host-001",
      "type": "host",
      "name": "worker-001",
      "labels": ["production", "us-east-1a"],
      "status": "healthy",
      "metrics": {"cpu": 45, "memory": 62},
      "children": ["pod-001", "pod-002"]
    }
  ],
  "edges": [
    {
      "source": "pod-001",
      "target": "service-001",
      "type": "belongs_to"
    }
  ]
}
```

#### Get Topology Real-time Stream

```
GET /api/v1/topology/realtime
Authorization: Bearer <access_token>
Content-Type: application/json

Request Body:
{
  "stream_type": "websocket" | "sse",
  "events": ["topology.update", "metrics.stream"]
}

Response: Establishes WebSocket or SSE connection
```

---

### 4.2 Incident Endpoints

#### List Incidents

```
GET /api/v1/incidents
Authorization: Bearer <access_token>

Query Parameters:
- status: string (open, investigating, resolved, closed)
- severity: string (critical, high, medium, low)
- from: datetime
- to: datetime

Response:
{
  "incidents": [
    {
      "id": "inc-001",
      "title": "Database connection timeout",
      "severity": "critical",
      "status": "investigating",
      "created_at": "2026-11-28T09:00:00Z",
      "updated_at": "2026-11-28T10:00:00Z",
      "assigned_to": "operator-001",
      "ai_investigation": {
        "status": "complete",
        "result": "Connection pool exhausted due to connection leak",
        "confidence": 0.92
      }
    }
  ]
}
```

#### Get Incident Details

```
GET /api/v1/incidents/{id}
Authorization: Bearer <access_token>

Response:
{
  "id": "inc-001",
  "title": "Database connection timeout",
  "description": "Database connections timing out",
  "severity": "critical",
  "status": "investigating",
  "created_at": "2026-11-28T09:00:00Z",
  "updated_at": "2026-11-28T10:00:00Z",
  "assigned_to": "operator-001",
  "ai_investigation": {
    "status": "complete",
    "result": "Connection pool exhausted due to connection leak",
    "confidence": 0.92,
    "evidence": [...],
    "recommendations": [...]
  },
  "timeline": [...]
}
```

#### Update Incident

```
PATCH /api/v1/incidents/{id}
Authorization: Bearer <access_token>
Content-Type: application/json

Request Body:
{
  "status": "investigating",
  "assigned_to": "operator-002",
  "notes": "Initial investigation started"
}

Response:
{
  "id": "inc-001",
  "updated_at": "2026-11-28T10:05:00Z",
  "audit_log": {
    "action": "incident.updated",
    "user": "operator-002",
    "timestamp": "2026-11-28T10:05:00Z"
  }
}
```

---

### 4.3 AI Investigation Endpoints

#### Trigger AI Investigation

```
POST /api/v1/incidents/{id}/ai-investigate
Authorization: Bearer <access_token>
Content-Type: application/json

Request Body:
{
  "depth": "deep" | "shallow",
  "include_recommendations": true
}

Response:
{
  "status": "queued",
  "request_id": "req-001",
  "estimated_completion": "2026-11-28T10:02:00Z"
}
```

#### Get AI Investigation Results

```
GET /api/v1/incidents/{id}/ai-investigation
Authorization: Bearer <access_token>

Response:
{
  "status": "complete",
  "result": "Connection pool exhausted due to connection leak",
  "confidence": 0.92,
  "evidence": [
    {
      "type": "log",
      "source": "database",
      "content": "Connection timeout after 30s",
      "timestamp": "2026-11-28T09:05:00Z"
    }
  ],
  "recommendations": [
    {
      "action": "Increase connection pool size",
      "impact": "high",
      "risk": "low",
      "command": "kubectl patch deployment db --patch '{\"spec\":{\"replicas\":3}}'"
    }
  ],
  "approval_required": true
}
```

---

### 4.4 Remediation Endpoints

#### Get Remediation Proposals

```
GET /api/v1/remediation
Authorization: Bearer <access_token>
Content-Type: application/json

Request Body:
{
  "incident_id": "inc-001",
  "severity": "high"
}

Response:
{
  "proposals": [
    {
      "id": "rem-001",
      "incident_id": "inc-001",
      "title": "Increase connection pool",
      "description": "Scale up database connections",
      "impact": "high",
      "risk": "low",
      "estimated_duration": "5 minutes",
      "commands": [
        {
          "description": "Patch deployment",
          "command": "kubectl patch deployment db ..."
        }
      ],
      "rollback_commands": [...],
      "approval_required": true,
      "approvers": ["admin-001", "senior-001"]
    }
  ]
}
```

#### Approve Remediation

```
POST /api/v1/remediation/{id}/approve
Authorization: Bearer <access_token>
Content-Type: application/json

Request Body:
{
  "approved_by": "operator-001",
  "notes": "Approved for production"
}

Response:
{
  "status": "approved",
  "approved_by": "operator-001",
  "approved_at": "2026-11-28T10:10:00Z",
  "execution_started": true
}
```

#### Execute Remediation

```
POST /api/v1/remediation/{id}/execute
Authorization: Bearer <access_token>

Response:
{
  "status": "running",
  "progress": 25,
  "logs": ["Step 1/3: Starting database connection pool increase..."],
  "estimated_completion": "2026-11-28T10:15:00Z"
}
```

---

### 4.5 Configuration Endpoints

#### Get Configuration

```
GET /api/v1/config/{agent_id}
Authorization: Bearer <access_token>

Response:
{
  "agent_id": "agent-001",
  "config": {
    "version": "1.2.0",
    "settings": {...},
    "capabilities": ["topology", "monitoring", "remediation"]
  },
  "last_sync": "2026-11-28T10:00:00Z"
}
```

#### Update Configuration

```
PUT /api/v1/config/{agent_id}
Authorization: Bearer <access_token>
Content-Type: application/json

Request Body:
{
  "settings": {
    "max_connections": 1000,
    "heartbeat_interval": 30
  }
}

Response:
{
  "status": "updated",
  "validation": "passed",
  "sync_status": "pending"
}
```

---

### 4.6 Audit Endpoints

#### Get Audit Logs

```
GET /api/v1/audit
Authorization: Bearer <access_token>

Query Parameters:
- action: string
- user: string
- from: datetime
- to: datetime
- limit: integer

Response:
{
  "audit_logs": [
    {
      "id": "audit-001",
      "action": "incident.updated",
      "user": "operator-001",
      "timestamp": "2026-11-28T10:05:00Z",
      "resource": "inc-001",
      "details": {
        "old_status": "open",
        "new_status": "investigating"
      }
    }
  ]
}
```

---

## 5. WebSocket/SSE Events

### 5.1 Event Types

#### Topology Events

```
Event: topology.update
Payload:
{
  "type": "topology.update",
  "timestamp": "2026-11-28T10:00:00Z",
  "changes": {
    "added": ["host-002"],
    "removed": ["host-003"],
    "updated": [{"id": "host-001", "changes": {"status": "degraded"}}]
  }
}
```

#### Metrics Events

```
Event: metrics.stream
Payload:
{
  "type": "metrics.stream",
  "timestamp": "2026-11-28T10:00:00Z",
  "metrics": {
    "host-001": {
      "cpu": 45,
      "memory": 62,
      "network_in": 1000,
      "network_out": 500
    }
  }
}
```

#### Incident Events

```
Event: incident.created
Payload:
{
  "type": "incident.created",
  "incident": {
    "id": "inc-001",
    "title": "Database connection timeout",
    "severity": "critical"
  }
}

Event: incident.updated
Payload:
{
  "type": "incident.updated",
  "incident": {
    "id": "inc-001",
    "status": "investigating"
  }
}

Event: incident.resolved
Payload:
{
  "type": "incident.resolved",
  "incident": {
    "id": "inc-001",
    "resolved_by": "operator-001",
    "resolution": "Connection pool increased"
  }
}
```

#### AI Investigation Events

```
Event: ai.investigation.complete
Payload:
{
  "type": "ai.investigation.complete",
  "incident_id": "inc-001",
  "result": "Connection pool exhausted",
  "confidence": 0.92
}
```

#### Remediation Events

```
Event: remediation.proposal
Payload:
{
  "type": "remediation.proposal",
  "proposal": {
    "id": "rem-001",
    "title": "Increase connection pool"
  }
}

Event: remediation.started
Payload:
{
  "type": "remediation.started",
  "proposal_id": "rem-001",
  "progress": 0
}

Event: remediation.completed
Payload:
{
  "type": "remediation.completed",
  "proposal_id": "rem-001",
  "success": true
}
```

---

## 6. Error Responses

### 6.1 Standard Error Format

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Request validation failed",
    "details": {
      "field": "incident_id",
      "reason": "not a valid UUID"
    }
  }
}
```

### 6.2 HTTP Status Codes

- 200: Success
- 201: Created
- 204: No Content
- 400: Bad Request
- 401: Unauthorized
- 403: Forbidden
- 404: Not Found
- 429: Too Many Requests
- 500: Internal Server Error

---

**End of Document**
