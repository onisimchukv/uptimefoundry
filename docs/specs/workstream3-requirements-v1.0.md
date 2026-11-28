# Work Stream 3: Operator Web Console & Topology UI
## Requirements Specification v1.0

**Created:** November 28, 2026  
**Project:** UptimeFoundry  
**Milestone:** M10b  
**Status:** Initial Specification  
**Dependencies:** Platform M1-M7, Agent A3, M9

---

## 1. Executive Summary

Work Stream 3 delivers the **Operator Web Console & Topology UI**, a modern web-based interface for operators to monitor, investigate, and manage UptimeFoundry deployments. The console provides real-time visualization of infrastructure topology, incident management, AI-assisted investigation results, and controlled remediation capabilities.

---

## 2. Scope

### 2.1 In Scope

- **Topology Visualization**: Interactive graph-based display of infrastructure relationships
- **Real-time Monitoring**: Live dashboard with metrics, logs, and system health
- **Incident Management**: Incident lifecycle management and investigation workflows
- **AI Investigation Interface**: Human-in-the-loop AI investigation results and approval
- **Remediation Console**: Controlled remediation proposal and approval workflow
- **Configuration Management**: Agent and control plane configuration management
- **User Management**: Authentication, authorization, and user roles
- **Audit Trail**: Cryptographic audit logging for all operator actions

### 2.2 Out of Scope (Future Work)

- Mobile applications
- Third-party integration dashboards
- Advanced reporting and analytics (beyond real-time)
- Multi-language support (beyond English initially)

---

## 3. Functional Requirements Summary

### 3.1 Topology Visualization
- Interactive graph display of infrastructure (hosts, containers, pods, services, networks)
- Drill-down navigation from high-level to detailed views
- Real-time topology updates via WebSocket/SSE
- Node filtering and search capabilities
- Topology snapshot and comparison

### 3.2 Real-time Monitoring
- Dashboard with customizable widget layout
- Metrics visualization (CPU, memory, network, latency)
- Log streaming and filtering
- System health indicators and alerts
- Historical data access and trending

### 3.3 Incident Management
- Incident creation and assignment
- Investigation workflow tracking
- AI investigation result review and approval
- Incident resolution and closure
- Incident history and reporting

### 3.4 AI Investigation Interface
- Display AI-generated investigation results
- Human-in-the-loop approval workflow
- Evidence and context presentation
- Investigation history and learning feedback
- AI capability configuration

### 3.5 Remediation Console
- Remediation proposal display
- Risk assessment and impact analysis
- Approval workflow (single/multi-operator)
- Remediation execution and monitoring
- Rollback capabilities

### 3.6 Configuration Management
- Agent configuration view and edit
- Control plane configuration management
- Configuration validation and testing
- Configuration history and rollback
- Configuration sync status

### 3.7 User Management
- User authentication and session management
- Role-based access control (RBAC)
- User profile management
- Permission configuration
- Session audit and monitoring

### 3.8 Audit Trail
- Cryptographic audit logging
- Immutable audit record storage
- Audit log search and filtering
- Compliance reporting
- Audit log export

---

## 4. Non-Functional Requirements

### 4.1 Performance
- Topology updates: < 2 seconds latency
- Dashboard load: < 3 seconds initial render
- Search response: < 500ms for 10k nodes
- WebSocket/SSE connection stability: 99.9% uptime

### 4.2 Security
- TLS 1.3 for all communications
- OAuth2/OpenID Connect authentication
- Role-based access control
- Audit logging for all actions
- Data encryption at rest

### 4.3 Reliability
- 99.9% availability
- Automatic failover and recovery
- Graceful degradation on backend failures
- Connection reconnection with state recovery

### 4.4 Scalability
- Support 10k+ nodes in topology
- 100+ concurrent users
- Horizontal scaling capability
- Database sharding support

---

## 5. API Requirements

### 5.1 REST API Endpoints

- `/api/v1/topology` - Get topology snapshot
- `/api/v1/topology/realtime` - WebSocket/SSE stream
- `/api/v1/dashboard` - Get dashboard configuration
- `/api/v1/incidents` - Incident CRUD operations
- `/api/v1/incidents/{id}/ai-investigation` - AI investigation results
- `/api/v1/remediation` - Remediation proposal and approval
- `/api/v1/config` - Configuration management
- `/api/v1/users` - User management
- `/api/v1/audit` - Audit log access

### 5.2 WebSocket/SSE Events

- `topology.update` - Topology changes
- `metrics.stream` - Real-time metrics
- `incident.created` - New incident
- `incident.updated` - Incident status changes
- `incident.resolved` - Incident resolution
- `ai.investigation.complete` - AI investigation results
- `remediation.proposal` - Remediation proposals
- `config.changed` - Configuration changes

---

## 6. Integration Requirements

### 6.1 Control Plane
- REST API at `/api/v1`
- WebSocket/SSE for real-time updates
- MCP interface compatibility
- Authentication integration

### 6.2 Agent Integration
- Topology data from Agent A3
- Real-time WAL streaming
- Heartbeat and capability reports
- Event stream processing

### 6.3 External Systems
- Authentication provider (OAuth2/OpenID)
- Alerting systems (integrations TBD)
- Log aggregation systems
- Database for audit storage

---

## 7. Technology Stack Recommendations

### 7.1 Frontend
- React 18+ with TypeScript
- WebSockets for real-time updates
- D3.js or React Flow for topology visualization
- Material-UI or Tailwind CSS for UI components

### 7.2 Backend
- Node.js or Go for API server
- WebSocket/SSE implementation
- PostgreSQL for persistent data
- Redis for caching and pub/sub

### 7.3 Infrastructure
- Docker containers
- Kubernetes for orchestration
- Nginx for reverse proxy
- CI/CD pipeline

---

## 8. Implementation Phases

### Phase 1: Core Infrastructure
- API server setup
- Authentication system
- Database schema
- WebSocket/SSE infrastructure

### Phase 2: Topology Visualization
- Topology data integration
- Graph visualization components
- Real-time update handling
- Drill-down navigation

### Phase 3: Monitoring Dashboard
- Metrics collection and display
- Log streaming
- Dashboard widgets
- Alert integration

### Phase 4: Incident Management
- Incident CRUD operations
- Workflow engine
- AI investigation interface
- Investigation approval flow

### Phase 5: Remediation Console
- Remediation proposal display
- Risk assessment
- Approval workflow
- Execution and monitoring

### Phase 6: Configuration Management
- Config view and edit
- Validation and testing
- History and rollback
- Sync status display

### Phase 7: User Management
- User CRUD operations
- RBAC implementation
- Profile management
- Session management

### Phase 8: Audit Trail
- Audit logging implementation
- Immutable storage
- Search and filtering
- Compliance reporting

### Phase 9: Testing & Documentation
- Unit and integration tests
- E2E testing
- User documentation
- API documentation

---

## 9. Success Criteria

- Functional requirements 90%+ implemented
- Performance targets met
- Security requirements validated
- User acceptance testing passed
- Documentation complete
- Production deployment successful

---

## 10. Open Questions

1. Specific authentication provider to integrate?
2. Topology visualization library preference?
3. Database choice for audit storage?
4. Alerting system integration requirements?
5. Target user count and topology scale?

---

**End of Document**
