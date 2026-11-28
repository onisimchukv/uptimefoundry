# Work Stream 3: UI/UX Specification
## Operator Web Console & Topology UI

**Created:** November 28, 2026  
**Project:** UptimeFoundry  
**Milestone:** M10b  
**Version:** 1.0

---

## 1. Design Principles

### 1.1 Primary Principles

- **Clarity Over Complexity**: Present complex infrastructure data in simple, actionable formats
- **Real-Time Awareness**: Operators should always know the current state of their infrastructure
- **Contextual Actions**: Show relevant actions based on current view and context
- **Progressive Disclosure**: Show essential information first, details on demand
- **Human-in-the-Loop**: AI suggestions require human validation for critical actions

### 1.2 Secondary Principles

- **Consistency**: Use consistent patterns, terminology, and behavior
- **Accessibility**: WCAG 2.1 AA compliance
- **Performance**: Fast, responsive interface with minimal latency perception
- **Mobile-First Design**: Responsive layouts that work on various screen sizes

---

## 2. Application Layout

### 2.1 Main Layout Structure

```
┌─────────────────────────────────────────────────────────────┐
│  Header (Fixed)                                             │
│  - Logo, User Profile, Notifications, Theme Toggle          │
├──────────────┬──────────────────────────────────────────────┤
│              │                                              │
│  Sidebar     │  Main Content Area                          │
│  (Fixed)     │  - Dashboard Views                          │
│              │  - Topology Visualization                    │
│  Navigation  │  - Incident Management                       │
│  - Dashboard │  - Configuration Management                  │
│  - Topology  │                                              │
│  - Incidents │  - Audit Trail                              │
│  - Config    │                                              │
│  - Users     │                                              │
│              │                                              │
└──────────────┴──────────────────────────────────────────────┘
```

### 2.2 Header Component

**Height:** 60px (fixed)

**Elements:**
- **Left:** UptimeFoundry logo
- **Center:** Search bar (global search)
- **Right:**
  - Notification bell icon (with badge)
  - User profile dropdown
  - Theme toggle (light/dark)
  - Settings gear icon

### 2.3 Sidebar Navigation

**Width:** 280px (collapsible to 80px)

**Sections:**
1. **Dashboard** - Home view with widgets
2. **Topology** - Infrastructure visualization
3. **Incidents** - Incident management
4. **Configuration** - Agent and control plane config
5. **Audit** - Audit log viewer
6. **Users** - User management

**Collapsed State:**
- Only icons visible
- Tooltips on hover
- Keyboard shortcuts (Tab key)

---

## 3. Dashboard View

### 3.1 Dashboard Layout

**Grid System:** 12-column responsive grid

**Default Widgets:**
1. **System Health** (3x2 grid)
   - Overall infrastructure health score
   - Uptime percentage
   - Active incidents count
   - Recent alerts

2. **Topology Overview** (6x3 grid)
   - Mini topology graph
   - Node counts by type
   - Health distribution

3. **Metrics Summary** (6x2 grid)
   - CPU usage average
   - Memory usage average
   - Network throughput
   - Request latency

4. **Recent Incidents** (6x3 grid)
   - List of recent incidents
   - Severity badges
   - Quick actions

5. **AI Insights** (6x2 grid)
   - AI recommendations
   - Investigation summaries
   - Capability status

### 3.2 Widget Behavior

- **Draggable**: Widgets can be rearranged
- **Resizable**: Width/height adjustable
- **Collapsible**: Click to collapse/expand
- **Refresh**: Manual and auto-refresh options
- **Fullscreen**: Expand to full view

---

## 4. Topology Visualization

### 4.1 Topology Graph Components

#### Node Types

**Host Node:**
- Icon: Server icon
- Color: Green (healthy), Yellow (degraded), Red (unhealthy)
- Labels: hostname, region, availability zone
- Click: Expand to show containers/pods
- Double-click: Open host details panel

**Container/Pod Node:**
- Icon: Container icon
- Color: Same as host health
- Labels: name, image, status
- Children: Services, databases
- Click: Open container details

**Service Node:**
- Icon: Network icon
- Color: Blue
- Labels: service name, port, protocol
- Children: Endpoints, consumers
- Click: Open service details

**Database Node:**
- Icon: Database icon
- Color: Purple
- Labels: database name, type, version
- Children: Tables, connections
- Click: Open database details

**Network Node:**
- Icon: Network icon
- Color: Orange
- Labels: network name, vpc, subnet
- Click: Open network details

#### Edge Types

**belongs_to:** Container belongs to host
**provides:** Service provides functionality
**consumes:** Service consumes another
**connected:** Network connectivity
**depends_on:** Dependency relationship

### 4.2 Graph Interactions

**Pan:** Click and drag background
**Zoom:** Mouse wheel or +/- buttons
**Select:** Click node
**Filter:** Filter by node type, health, label
**Search:** Search bar filters nodes
**Collapse/Expand:** Toggle node children
**Fullscreen:** Expand to full screen

### 4.3 Topology Panel

**Location:** Right side (400px width)

**Content:**
- Selected node details
- Metrics and health
- Related incidents
- Available actions
- Remediation proposals

**Tabs:**
1. **Overview** - Basic information
2. **Metrics** - Real-time metrics
3. **Incidents** - Associated incidents
4. **Config** - Configuration
5. **Actions** - Available actions

---

## 5. Incident Management

### 5.1 Incident List View

**Table Columns:**
- ID (clickable)
- Title
- Severity (badge)
- Status (badge)
- Created At
- Assigned To
- Actions (dropdown)

**Filters:**
- Status dropdown
- Severity dropdown
- Date range picker
- Search bar

**Actions:**
- View details
- Assign
- Update status
- Close

### 5.2 Incident Detail View

**Layout:** Two-column (60% / 40%)

**Left Column - Incident Information:**
- Title
- Description
- Severity badge
- Status badges
- Timeline
- Related incidents

**Right Column - Actions:**
- AI Investigate button
- Remediation proposals
- Notes textarea
- Status update dropdown
- Assignee dropdown

### 5.3 AI Investigation Interface

**Layout:** Three-column (40% / 20% / 40%)

**Left Column - Investigation Request:**
- Depth selector (shallow/deep)
- Include recommendations checkbox
- Trigger button
- Progress indicator

**Center Column - Results:**
- Status indicator
- AI result (confidence score)
- Evidence list
- Timeline

**Right Column - Actions:**
- Approve button (disabled if not complete)
- Reject button
- Add notes
- Export report

### 5.4 Severity Color Coding

- **Critical:** Red (#dc3545)
- **High:** Orange (#fd7e14)
- **Medium:** Yellow (#ffc107)
- **Low:** Blue (#0d6efd)
- **Info:** Gray (#6c757d)

---

## 6. Remediation Console

### 6.1 Proposal Display

**Layout:** Card-based

**Card Sections:**
1. **Header**
   - Title
   - Risk level badge
   - Estimated duration

2. **Description**
   - What will be changed
   - Expected impact

3. **Commands Preview**
   - Code blocks with syntax highlighting
   - Rollback commands
   - Dry-run option

4. **Approval Section**
   - Required approvers list
   - Approval status
   - Approve button

### 6.2 Approval Workflow

**Single Approval:**
- One operator can approve
- Immediate execution after approval

**Multi-Approval:**
- All required approvers must approve
- Status shows "Waiting for X more approvals"
- Timeout handling

### 6.3 Execution View

**Layout:** Two-column

**Left Column - Progress:**
- Progress bar
- Step indicators
- Estimated time remaining

**Right Column - Logs:**
- Real-time execution logs
- Error messages
- Success/failure indicators

**Controls:**
- Pause/Resume
- Cancel (with confirmation)
- Rollback button

---

## 7. Configuration Management

### 7.1 Configuration View

**Layout:** Split view (left: navigation, right: editor)

**Left Panel - Agent List:**
- Agent name
- Status indicator
- Last sync time
- Expand to show settings

**Right Panel - Configuration Editor:**
- Settings tree
- Value editors
- Validation messages
- Save button
- Rollback button

### 7.2 Configuration Validation

**Real-time Validation:**
- Syntax highlighting
- Error underlines
- Warning indicators
- Help tooltips

**Test Mode:**
- Apply changes in test environment
- Validate before production
- Preview impact

### 7.3 Configuration History

**Table:**
- Version
- Changed by
- Changed at
- Diff preview
- Rollback action

---

## 8. User Management

### 8.1 User List

**Table Columns:**
- User
- Role
- Status
- Last Login
- Created At
- Actions

**Actions:**
- Edit
- Deactivate
- Delete
- View audit log

### 8.2 User Editor

**Sections:**
1. **Basic Information**
   - Username
   - Display name
   - Email
   - Phone

2. **Role Assignment**
   - Role selector
   - Permission preview

3. **Session Management**
   - Active sessions list
   - Terminate session button

---

## 9. Audit Trail

### 9.1 Audit Log Viewer

**Table Columns:**
- Timestamp
- Action
- User
- Resource
- Details
- IP Address

**Filters:**
- Action dropdown
- User dropdown
- Date range
- Resource type

**Actions:**
- Export (CSV, JSON)
- View details
- Search within log

### 9.2 Audit Log Details

**Display:**
- Full details
- Request/response (if applicable)
- Before/after values
- IP address and user agent

---

## 10. Design System

### 10.1 Color Palette

**Primary:**
- Main: #2563eb (blue-600)
- Hover: #1d4ed8 (blue-700)
- Focus: #3b82f6 (blue-500)

**Secondary:**
- Gray: #64748b (gray-500)
- Light Gray: #f1f5f9 (gray-100)

**Semantic Colors:**
- Success: #22c55e (green-500)
- Warning: #f59e0b (yellow-500)
- Error: #ef4444 (red-500)
- Info: #3b82f6 (blue-500)

### 10.2 Typography

**Font Family:**
- Primary: Inter (sans-serif)
- Monospace: JetBrains Mono

**Scale:**
- H1: 32px / 720
- H2: 24px / 600
- H3: 20px / 600
- Body: 16px / 400
- Small: 14px / 400
- Caption: 12px / 400

### 10.3 Spacing

**Scale:** 4px base
- xs: 4px
- sm: 8px
- md: 16px
- lg: 24px
- xl: 32px
- 2xl: 48px

### 10.4 Border Radius

- sm: 4px
- md: 8px
- lg: 12px
- xl: 16px
- full: 9999px

### 10.5 Shadows

- sm: 0 1px 2px rgba(0,0,0,0.05)
- md: 0 4px 6px -1px rgba(0,0,0,0.1)
- lg: 0 10px 15px -3px rgba(0,0,0,0.1)
- xl: 0 20px 25px -5px rgba(0,0,0,0.1)

---

## 11. Responsive Breakpoints

- **Mobile:** < 640px
- **Tablet:** 640px - 1024px
- **Desktop:** 1024px - 1280px
- **Large Desktop:** > 1280px

---

## 12. Accessibility Requirements

### 12.1 Keyboard Navigation

- Tab order follows visual layout
- Escape closes dialogs/modals
- Enter activates buttons
- Arrow keys for navigation

### 12.2 Screen Reader Support

- ARIA labels on interactive elements
- Live regions for dynamic content
- Semantic HTML structure
- Alt text for images

### 12.3 Color Contrast

- Minimum 4.5:1 for normal text
- Minimum 3:1 for large text
- Not relying on color alone

---

## 13. Dark Mode

### 13.1 Dark Theme Colors

**Backgrounds:**
- Surface: #1e293b (slate-800)
- Elevated: #334155 (slate-700)
- Border: #475569 (slate-600)

**Text:**
- Primary: #f1f5f9 (gray-100)
- Secondary: #cbd5e1 (gray-300)
- Muted: #94a3b8 (gray-400)

**Accents:**
- Primary: #60a5fa (blue-400)
- Success: #4ade80 (green-400)
- Warning: #fbbf24 (yellow-400)
- Error: #f87171 (red-400)

### 13.2 Auto-Detect

- Detect system preference
- Persist user choice
- Manual toggle available

---

**End of Document**
