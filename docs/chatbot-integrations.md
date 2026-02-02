# Chatbot Integrations

Chronos provides native bot integrations for Slack and Microsoft Teams, allowing teams to manage scheduled jobs directly from their chat applications.

## Features

- **Job Management**: List, run, pause, and resume jobs
- **Status Monitoring**: Check job status and cluster health
- **Execution History**: View recent job executions
- **Interactive Actions**: Quick action buttons for common operations
- **Real-time Notifications**: Get notified about job failures and completions

## Slack Integration

### Setup

1. **Create a Slack App**
   - Go to https://api.slack.com/apps
   - Click "Create New App" → "From scratch"
   - Name it "Chronos Bot" and select your workspace

2. **Configure OAuth & Permissions**
   Add these Bot Token Scopes:
   - `chat:write` - Send messages
   - `commands` - Add slash commands
   - `users:read` - Read user information

3. **Create Slash Command**
   - Go to "Slash Commands" → "Create New Command"
   - Command: `/chronos`
   - Request URL: `https://your-domain.com/webhooks/slack/command`
   - Description: "Manage Chronos scheduled jobs"

4. **Enable Interactivity**
   - Go to "Interactivity & Shortcuts"
   - Enable Interactivity
   - Request URL: `https://your-domain.com/webhooks/slack/interaction`

5. **Configure Chronos**
   ```yaml
   chatbot:
     slack:
       enabled: true
       signing_secret: "your-signing-secret"
       bot_token: "xoxb-your-bot-token"
       allowed_channels:
         - C0123456789  # Optional: restrict to specific channels
   ```

### Commands

| Command | Description | Example |
|---------|-------------|---------|
| `/chronos help` | Show available commands | `/chronos help` |
| `/chronos list` | List all jobs | `/chronos list` |
| `/chronos status <job>` | Get job status and metrics | `/chronos status daily-backup` |
| `/chronos run <job>` | Manually trigger a job | `/chronos run daily-backup` |
| `/chronos pause <job>` | Pause a scheduled job | `/chronos pause daily-backup` |
| `/chronos resume <job>` | Resume a paused job | `/chronos resume daily-backup` |
| `/chronos history <job>` | View recent executions | `/chronos history daily-backup` |
| `/chronos health` | Check cluster health | `/chronos health` |

### Interactive Features

The bot provides interactive buttons for common actions:

```
📊 daily-backup
━━━━━━━━━━━━━━━━━━━━━
Status:      ✅ active
Health:      ✅ Healthy  
Success Rate: 98.5%
Schedule:    0 0 * * *

[Run Now] [View History] [Pause]
```

### Code Integration

```go
import (
    "github.com/chronos/chronos/internal/api"
    "github.com/chronos/chronos/internal/chatbot"
)

// Create Slack bot
slackBot := chatbot.NewSlackBot(chatbot.SlackConfig{
    SigningSecret:   os.Getenv("SLACK_SIGNING_SECRET"),
    BotToken:        os.Getenv("SLACK_BOT_TOKEN"),
    AllowedChannels: []string{"C0123456789"},
}, jobManager)

// Mount routes
api.MountChatbotRoutes(router, slackBot.HandleSlashCommand, nil)
```

## Microsoft Teams Integration

### Setup

1. **Register Azure AD Application**
   - Go to Azure Portal → Azure Active Directory → App registrations
   - Create new registration
   - Note the Application (client) ID

2. **Create Bot Channel Registration**
   - Go to Azure Portal → Create a resource → Bot Channels Registration
   - Configure the messaging endpoint: `https://your-domain.com/webhooks/teams/messages`

3. **Configure Chronos**
   ```yaml
   chatbot:
     teams:
       enabled: true
       app_id: "your-app-id"
       app_password: "your-app-password"
       tenant_id: "your-tenant-id"  # Optional for single-tenant
   ```

### Commands

Teams uses natural language commands (mention the bot first):

| Command | Description | Example |
|---------|-------------|---------|
| `help` | Show available commands | `@Chronos help` |
| `list` | List all jobs | `@Chronos list` |
| `status <job>` | Get job status | `@Chronos status daily-backup` |
| `run <job>` | Trigger a job | `@Chronos run daily-backup` |
| `pause <job>` | Pause a job | `@Chronos pause daily-backup` |
| `resume <job>` | Resume a job | `@Chronos resume daily-backup` |
| `history <job>` | View executions | `@Chronos history daily-backup` |
| `health` | Cluster health | `@Chronos health` |

### Adaptive Cards

Teams responses use Adaptive Cards for rich formatting:

```json
{
  "type": "AdaptiveCard",
  "version": "1.4",
  "body": [
    {"type": "TextBlock", "text": "📊 daily-backup", "size": "large", "weight": "bolder"},
    {"type": "FactSet", "facts": [
      {"title": "Status", "value": "✅ active"},
      {"title": "Success Rate", "value": "98.5%"},
      {"title": "Schedule", "value": "0 0 * * *"}
    ]}
  ],
  "actions": [
    {"type": "Action.Submit", "title": "Run Now", "data": {"action": "runJob", "jobId": "daily-backup"}}
  ]
}
```

### Code Integration

```go
import (
    "github.com/chronos/chronos/internal/api"
    "github.com/chronos/chronos/internal/chatbot"
)

// Create Teams bot
teamsBot := chatbot.NewTeamsBot(chatbot.TeamsConfig{
    AppID:       os.Getenv("TEAMS_APP_ID"),
    AppPassword: os.Getenv("TEAMS_APP_PASSWORD"),
    TenantID:    os.Getenv("TEAMS_TENANT_ID"),
}, jobManager)

// Mount routes
api.MountChatbotRoutes(router, nil, teamsBot.HandleActivity)
```

## Custom Commands

You can register custom commands for both platforms:

```go
// Slack custom command
slackBot.RegisterCommand("deploy", func(ctx context.Context, cmd *chatbot.SlashCommand) (*chatbot.SlackResponse, error) {
    // Custom logic
    return &chatbot.SlackResponse{
        ResponseType: "in_channel",
        Text: "Deployment triggered!",
    }, nil
})

// Teams custom command
teamsBot.RegisterCommand("deploy", func(ctx context.Context, activity *chatbot.Activity) (*chatbot.TeamsResponse, error) {
    return &chatbot.TeamsResponse{
        Type: "message",
        Text: "Deployment triggered!",
    }, nil
})
```

## Security

### Request Verification

Both bots verify incoming requests:

- **Slack**: HMAC-SHA256 signature verification using signing secret
- **Teams**: Azure AD JWT token validation

### Rate Limiting

Built-in rate limiting prevents abuse:
- 100 requests per minute per user
- Configurable limits per organization

### Channel Restrictions

Optionally restrict bot usage to specific channels:

```yaml
chatbot:
  slack:
    allowed_channels:
      - C0123456789  # #ops
      - C9876543210  # #engineering
```

## Notifications

Configure the bot to send proactive notifications:

```go
// Send job failure notification
slackBot.SendNotification(ctx, channelID, &chatbot.Notification{
    Event:   "job_failed",
    JobID:   "daily-backup",
    JobName: "Daily Backup",
    Error:   "Connection timeout",
})
```

See [Notification Hub](./notifications.md) for more notification options.
