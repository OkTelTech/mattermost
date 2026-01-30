# Mattermost Bot Service

Internal chat bot service for attendance tracking and budget approval workflows.

## Features

- **Attendance Bot**: Check-in, check-out, break time, leave requests
- **Budget Bot**: 7-step budget approval workflow

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        INTERNET                                 │
│                           │                                     │
│                     User Browser                                │
└───────────────────────────┼─────────────────────────────────────┘
                            │
                            ▼
┌─────────────────────────────────────────────────────────────────┐
│                   KUBERNETES CLUSTER                            │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │              MATTERMOST SERVER                           │   │
│  │              (LoadBalancer - Public)                     │   │
│  │                                                          │   │
│  │   - Authentication (login, session)                      │   │
│  │   - Authorization (channel membership)                   │   │
│  │   - Forward requests to Bot Service                      │   │
│  └──────────────────────────┬───────────────────────────────┘   │
│                             │ Internal HTTP                     │
│                             ▼                                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                BOT SERVICE (Go)                          │   │
│  │                (ClusterIP - Internal Only)               │   │
│  │                                                          │   │
│  │   - Not exposed to internet                              │   │
│  │   - Trust user info from Mattermost                      │   │
│  │   - Business logic only                                  │   │
│  └──────────────────────────┬───────────────────────────────┘   │
│                             │                                   │
│                             ▼                                   │
│  ┌─────────────────────────────────────────────────────────┐   │
│  │                     MONGODB                              │   │
│  │                   (ClusterIP)                            │   │
│  └─────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

## Channel Naming Convention

Each team has 2 channels per feature:

```
attendance-{team}              All team members (check-in/out, notifications)
attendance-{team}-approval     Leaders only (approve/reject buttons)

budget-{team}                  All team members (request status, notifications)
budget-{team}-approval         Leaders only (approve/reject buttons)
```

### Examples

```
#attendance-dev                 All developers
#attendance-dev-approval        Dev team leaders only

#attendance-sales               All sales team
#attendance-sales-approval      Sales managers only

#budget-marketing               All marketing team
#budget-marketing-approval      Marketing managers only
```

### Why 2 Channels?

```
Problem with single channel:
  #attendance-dev (everyone)
  └── Everyone sees [Approve] button → Anyone can approve!

Solution with 2 channels:
  #attendance-dev (everyone)
  └── Check-in/out notifications, leave request submissions

  #attendance-dev-approval (leaders only)
  └── Approval requests with [Approve] [Reject] buttons
  └── Only leaders see and can click
```

**Approver management = Channel membership management**
- Add approver → Invite to `-approval` channel
- Remove approver → Remove from `-approval` channel
- No extra config needed!

## Security Model

### Why no complex token verification?

1. **Bot Service is internal only** (ClusterIP) - not exposed to internet
2. **Only Mattermost can call** Bot Service
3. **Mattermost already authenticated user** before forwarding request
4. **User info in request is trustworthy** because it comes from Mattermost

### Security Flow

```
User clicks button in Mattermost
        │
        ▼
Mattermost CHECKS:
  ✓ User logged in (has session)?
  ✓ User in channel?
  ✓ Session valid?
        │
        ▼
Mattermost FORWARDS request to Bot Service (internal)
  Body: { user_id, user_name, channel_id, context }
        │
        ▼
Bot Service TRUSTS user info, only checks business logic:
  • Is user approving their own request?
  • Is request still pending?
```

## Project Structure

```
mattermost/bot/                  # Source code (this repo)
├── README.md
├── cmd/
│   └── server/
│       └── main.go              # Entry point
├── internal/
│   ├── config/
│   │   └── config.go            # Configuration
│   ├── handler/
│   │   ├── attendance.go        # Attendance handlers
│   │   ├── budget.go            # Budget handlers
│   │   └── middleware.go        # Middleware
│   ├── model/
│   │   ├── attendance.go        # Attendance models
│   │   ├── leave.go             # Leave request models
│   │   └── budget.go            # Budget request models
│   ├── store/
│   │   ├── mongodb.go           # MongoDB connection
│   │   ├── attendance.go        # Attendance repository
│   │   └── budget.go            # Budget repository
│   ├── mattermost/
│   │   └── client.go            # Mattermost API client
│   └── service/
│       ├── attendance.go        # Attendance business logic
│       └── budget.go            # Budget business logic
├── Dockerfile
├── go.mod
└── go.sum

infra/mattermost/k8s/bot/       # Deployment configs (infra repo)
├── deployment.yaml
├── service.yaml
└── configmap.yaml
```

## Bot 1: Attendance

### Slash Command

User types `/attendance` in `#attendance-{team}` channel:

```
┌─────────────────────────────────────────┐
│ Attendance - Select action:             │
│ [Check In] [Break] [Check Out]          │
│ [Leave] [Emergency] [Sick]              │
└─────────────────────────────────────────┘
         (Only user sees this - ephemeral)
```

### Check-in Flow

```
User in #attendance-dev types /attendance
         │
         ▼
User clicks [Check In]
         │
         ▼
Bot Service:
  1. Check if already checked-in today
  2. Save to MongoDB
  3. Return confirmation
         │
         ▼
#attendance-dev shows (everyone sees):
"✓ @nguyenvana checked in at 08:30"
```

### Leave Request Flow

```
User in #attendance-dev clicks [Leave]
         │
         ▼
Dialog form opens:
┌─────────────────────────────────────┐
│        Leave Request Form           │
│  Type:     [Annual Leave ▼]         │
│  From:     [2026-01-30]             │
│  To:       [2026-01-31]             │
│  Reason:   [________________]       │
│                                     │
│              [Cancel] [Submit]      │
└─────────────────────────────────────┘
         │
         ▼ Submit
         │
         ▼
#attendance-dev shows (everyone sees):
┌─────────────────────────────────────┐
│ LEAVE REQUEST #LR-2026012901        │
│ User: @nguyenvana                   │
│ Type: Annual Leave                  │
│ Date: 30/01 → 31/01 (2 days)        │
│ Reason: Family matters              │
│ Status: PENDING                     │
└─────────────────────────────────────┘
         │
         ▼
#attendance-dev-approval shows (leaders only):
┌─────────────────────────────────────┐
│ LEAVE REQUEST #LR-2026012901        │
│ User: @nguyenvana                   │
│ Type: Annual Leave                  │
│ Date: 30/01 → 31/01 (2 days)        │
│ Reason: Family matters              │
│ Status: PENDING                     │
│                                     │
│      [Approve]  [Reject]            │
└─────────────────────────────────────┘
```

### Approval Flow

```
Leader in #attendance-dev-approval clicks [Approve]
         │
         ▼
Bot Service:
  1. Get leave request from MongoDB
  2. Check: approver != requester
  3. Update MongoDB: status = approved
  4. Update message in #attendance-dev-approval (remove buttons)
  5. Post notification to #attendance-dev
  6. Send DM to requester
         │
         ▼
#attendance-dev-approval updates:
┌─────────────────────────────────────┐
│ LEAVE REQUEST #LR-2026012901        │
│ ...                                 │
│ Status: APPROVED                    │
│ Approved by: @teamlead at 09:15     │
└─────────────────────────────────────┘
         │
         ▼
#attendance-dev shows:
"@nguyenvana's leave request #LR-2026012901 was APPROVED by @teamlead"
         │
         ▼
DM to @nguyenvana:
"Your leave request #LR-2026012901 was APPROVED by @teamlead"
```

## Bot 2: Budget

### 7-Step Workflow

| Step | Actor | Action |
|------|-------|--------|
| 1 | Sale | Create request (campaign, partner, amount, purpose) |
| 2 | Partner | Submit content (post content, link, page) |
| 3 | TLQC | Confirm (ad account ID, correct post/page) |
| 4 | Partner | Payment info (recipient, bank account, amount) |
| 5 | Team Lead | Approve + voice confirmation |
| 6 | TL Bank | Add bank note |
| 7 | Finance | Transfer & upload bill |

### Budget Request Flow

```
Sale in #budget-marketing types /budget
         │
         ▼
Fill step 1 form → Submit
         │
         ▼
#budget-marketing shows:
┌─────────────────────────────────────┐
│ BUDGET REQUEST #BR-2026012901       │
│ Campaign: Tet 2026                  │
│ Partner: ABC Company                │
│ Amount: 10,000,000 VND              │
│ Status: Step 1/7 - Waiting Partner  │
└─────────────────────────────────────┘
         │
         ▼
#budget-marketing-approval shows (with action buttons):
┌─────────────────────────────────────┐
│ BUDGET REQUEST #BR-2026012901       │
│ ...                                 │
│ Status: Step 1/7 - Waiting Partner  │
│                                     │
│      [Fill Step 2]                  │
└─────────────────────────────────────┘
         │
         ▼
... continues through all 7 steps ...
         │
         ▼
Final notification to all stakeholders
```

## Data Models

### AttendanceRecord

```go
type AttendanceRecord struct {
    ID         primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    UserID     string             `bson:"user_id" json:"user_id"`
    Username   string             `bson:"username" json:"username"`
    ChannelID  string             `bson:"channel_id" json:"channel_id"`
    Date       string             `bson:"date" json:"date"` // YYYY-MM-DD
    CheckIn    *time.Time         `bson:"check_in,omitempty" json:"check_in"`
    BreakStart *time.Time         `bson:"break_start,omitempty" json:"break_start"`
    BreakEnd   *time.Time         `bson:"break_end,omitempty" json:"break_end"`
    CheckOut   *time.Time         `bson:"check_out,omitempty" json:"check_out"`
    Status     string             `bson:"status" json:"status"` // working, break, completed
    CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
    UpdatedAt  time.Time          `bson:"updated_at" json:"updated_at"`
}
```

### LeaveRequest

```go
type LeaveRequest struct {
    ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    RequestID        string             `bson:"request_id" json:"request_id"` // LR-YYYYMMDDNN
    UserID           string             `bson:"user_id" json:"user_id"`
    Username         string             `bson:"username" json:"username"`
    ChannelID        string             `bson:"channel_id" json:"channel_id"`
    ApprovalChannelID string            `bson:"approval_channel_id" json:"approval_channel_id"`
    PostID           string             `bson:"post_id" json:"post_id"`
    ApprovalPostID   string             `bson:"approval_post_id" json:"approval_post_id"`
    Type             string             `bson:"type" json:"type"` // leave, emergency, sick
    StartDate        string             `bson:"start_date" json:"start_date"`
    EndDate          string             `bson:"end_date" json:"end_date"`
    Days             int                `bson:"days" json:"days"`
    Reason           string             `bson:"reason" json:"reason"`
    Status           string             `bson:"status" json:"status"` // pending, approved, rejected
    ApproverID       string             `bson:"approver_id,omitempty" json:"approver_id"`
    ApproverUsername string             `bson:"approver_username,omitempty" json:"approver_username"`
    ApprovedAt       *time.Time         `bson:"approved_at,omitempty" json:"approved_at"`
    RejectReason     string             `bson:"reject_reason,omitempty" json:"reject_reason"`
    CreatedAt        time.Time          `bson:"created_at" json:"created_at"`
    UpdatedAt        time.Time          `bson:"updated_at" json:"updated_at"`
}
```

### BudgetRequest

```go
type BudgetRequest struct {
    ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    RequestID   string             `bson:"request_id" json:"request_id"` // BR-YYYYMMDDNN
    ChannelID   string             `bson:"channel_id" json:"channel_id"`
    ApprovalChannelID string        `bson:"approval_channel_id" json:"approval_channel_id"`
    PostID      string             `bson:"post_id" json:"post_id"`
    ApprovalPostID string          `bson:"approval_post_id" json:"approval_post_id"`
    CurrentStep int                `bson:"current_step" json:"current_step"`
    Status      string             `bson:"status" json:"status"` // step1...step7, completed, rejected

    // Step 1: Sale Info
    SaleUserID string  `bson:"sale_user_id" json:"sale_user_id"`
    Campaign   string  `bson:"campaign" json:"campaign"`
    Partner    string  `bson:"partner" json:"partner"`
    Amount     float64 `bson:"amount" json:"amount"`
    Purpose    string  `bson:"purpose" json:"purpose"`
    Deadline   string  `bson:"deadline" json:"deadline"`

    // Step 2: Partner Content
    PostContent string `bson:"post_content,omitempty" json:"post_content"`
    PostLink    string `bson:"post_link,omitempty" json:"post_link"`
    PageLink    string `bson:"page_link,omitempty" json:"page_link"`

    // Step 3: TLQC Confirmation
    AdAccountID   string `bson:"ad_account_id,omitempty" json:"ad_account_id"`
    TLQCUserID    string `bson:"tlqc_user_id,omitempty" json:"tlqc_user_id"`
    TLQCConfirmed bool   `bson:"tlqc_confirmed" json:"tlqc_confirmed"`

    // Step 4: Payment Info
    RecipientName string  `bson:"recipient_name,omitempty" json:"recipient_name"`
    BankAccount   string  `bson:"bank_account,omitempty" json:"bank_account"`
    BankName      string  `bson:"bank_name,omitempty" json:"bank_name"`
    PaymentAmount float64 `bson:"payment_amount,omitempty" json:"payment_amount"`

    // Step 5: Team Lead Approval
    TeamLeadID  string `bson:"team_lead_id,omitempty" json:"team_lead_id"`
    VoiceFileID string `bson:"voice_file_id,omitempty" json:"voice_file_id"`
    Approved    bool   `bson:"approved" json:"approved"`

    // Step 6: Bank Note
    BankNote string `bson:"bank_note,omitempty" json:"bank_note"`

    // Step 7: Finance
    BillFileID      string     `bson:"bill_file_id,omitempty" json:"bill_file_id"`
    TransactionCode string     `bson:"transaction_code,omitempty" json:"transaction_code"`
    CompletedAt     *time.Time `bson:"completed_at,omitempty" json:"completed_at"`

    CreatedAt time.Time `bson:"created_at" json:"created_at"`
    UpdatedAt time.Time `bson:"updated_at" json:"updated_at"`
}
```

## API Endpoints

### Attendance Bot

| Endpoint | Method | Trigger | Description |
|----------|--------|---------|-------------|
| `/api/attendance` | POST | Slash command | Show attendance menu |
| `/api/attendance/checkin` | POST | Button | Record check-in |
| `/api/attendance/break-start` | POST | Button | Record break start |
| `/api/attendance/break-end` | POST | Button | Record break end |
| `/api/attendance/checkout` | POST | Button | Record check-out |
| `/api/attendance/leave-form` | POST | Button | Open leave form |
| `/api/attendance/leave` | POST | Dialog | Process leave request |
| `/api/attendance/approve` | POST | Button | Approve leave |
| `/api/attendance/reject` | POST | Button | Reject leave |

### Budget Bot

| Endpoint | Method | Trigger | Description |
|----------|--------|---------|-------------|
| `/api/budget` | POST | Slash command | Create budget request (step 1 form) |
| `/api/budget/step1` | POST | Dialog | Process step 1 |
| `/api/budget/step2` | POST | Button/Dialog | Partner fill step 2 |
| `/api/budget/step3` | POST | Button | TLQC confirm step 3 |
| `/api/budget/step4` | POST | Button/Dialog | Partner fill step 4 |
| `/api/budget/step5` | POST | Button | Team Lead approve step 5 |
| `/api/budget/step6` | POST | Button/Dialog | TL Bank note step 6 |
| `/api/budget/step7` | POST | Button/Dialog | Finance complete step 7 |

### Utility

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/health` | GET | Health check |
| `/ready` | GET | Readiness check |

## Environment Variables

```env
# Server
PORT=3000
ENV=production

# MongoDB
MONGODB_URI=mongodb://mongodb:27017
MONGODB_DATABASE=mattermost_bots

# Mattermost
MATTERMOST_URL=http://mattermost:8065
BOT_TOKEN=<bot access token>
```

## Mattermost Setup

### 1. Create Bot Account

```
System Console → Integrations → Bot Accounts → Add Bot

Username: internal-bot
Display Name: Internal Bot

→ Copy Access Token (BOT_TOKEN)
```

### 2. Create Slash Commands

```
Main Menu → Integrations → Slash Commands → Add

Command 1:
- Trigger: attendance
- URL: http://bot-service:3000/api/attendance
- Method: POST

Command 2:
- Trigger: budget
- URL: http://bot-service:3000/api/budget
- Method: POST
```

### 3. Create Channels

For each team, create channel pairs:

```
Team: dev
├── #attendance-dev           (all developers)
├── #attendance-dev-approval  (dev leaders only)
├── #budget-dev               (all developers)
└── #budget-dev-approval      (dev leaders only)

Team: sales
├── #attendance-sales
├── #attendance-sales-approval
├── #budget-sales
└── #budget-sales-approval

Team: marketing
├── #attendance-marketing
├── #attendance-marketing-approval
├── #budget-marketing
└── #budget-marketing-approval
```

## Channel Resolution Logic

Bot Service resolves approval channel automatically:

```go
const ApprovalSuffix = "-approval"

func GetApprovalChannel(channelName string) string {
    return channelName + ApprovalSuffix
}

// attendance-dev → attendance-dev-approval
// budget-sales → budget-sales-approval
```

## Kubernetes Deployment

K8s manifests are in the **infra repo** at `infra/mattermost/k8s/bot/`.

Key points:
- **Service type: ClusterIP** - internal only, not exposed to internet
- Health check: `GET /health`
- Readiness check: `GET /ready`

## Development

### Prerequisites

- Go 1.21+
- MongoDB 6.0+
- Mattermost Server

### Run Locally

```bash
# Set environment variables
export MONGODB_URI=mongodb://localhost:27017
export MATTERMOST_URL=http://localhost:8065
export BOT_TOKEN=your_bot_token

# Run
go run cmd/server/main.go
```

### Build Docker Image

```bash
docker build -t bot-service:latest .
```

## Testing

### Attendance Bot

1. In `#attendance-dev`: Type `/attendance` → see menu (only you see)
2. Click [Check In] → see confirmation in channel (everyone sees)
3. Type `/attendance` → Click [Check In] again → see error "already checked in"
4. Click [Leave] → fill form → submit
5. See request in `#attendance-dev` (no buttons)
6. Leaders see request in `#attendance-dev-approval` (with buttons)
7. Leader clicks [Approve] → status updates, DM sent to requester

### Budget Bot

1. In `#budget-marketing`: Type `/budget` → fill step 1 → submit
2. See request status in `#budget-marketing`
3. Leaders see action buttons in `#budget-marketing-approval`
4. Complete all 7 steps
5. All stakeholders receive completion notification
