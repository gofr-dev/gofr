# Discord Release Notifications

This document describes the automated Discord notification system for GoFr releases.

## Overview

GoFr automatically posts release announcements to Discord when a new version is published on GitHub. The system uses a custom Go tool that handles message splitting, Markdown preservation, and retry logic.

## Features

- **UTF-8 Safe**: Correctly handles emojis and multi-byte Unicode characters
- **Markdown Preservation**: Intelligently splits messages while preserving code block formatting
- **Custom Format**: Posts with `# [🌟 GoFr {version} Released! 🌟](link)` heading
- **@everyone Mention**: Notifies all Discord members
- **Retry Logic**: Automatically retries failed posts (3 attempts)
- **Discord Limit Handling**: Splits messages >2000 characters across multiple posts

## Setup

### 1. Create Discord Webhook

1. Go to your Discord server → Server Settings → Integrations → Webhooks
2. Click "New Webhook"
3. Configure the webhook:
   - Name: `GoFr Releases`
   - Channel: Select your releases channel
4. Copy the webhook URL

### 2. Add GitHub Secret

1. Go to GitHub repository → Settings → Secrets and variables → Actions
2. Click "New repository secret"
3. Add:
   - **Name**: `DISCORD_WEBHOOK`
   - **Value**: Your webhook URL from step 1

## How It Works

The workflow is triggered automatically when a release is published:

1. GitHub Actions workflow starts (`.github/workflows/discord-release-notification.yml`)
2. Go tool reads release data from environment variables
3. Constructs formatted message with header and release body
4. Splits message if needed (respecting Markdown and 2000 char limit)
5. Posts to Discord with retry logic

## Message Format

```
@everyone

# [🌟 GoFr v1.26.0 Released! 🌟](https://github.com/gofr-dev/gofr/releases/tag/v1.26.0)

# **Release v1.50.2**

## 🚀 Enhancements
...
```

## Testing

To test the workflow:

1. Create a test release on GitHub
2. Check your Discord channel for the notification
3. Verify formatting and links are correct

## Future Enhancements

- LinkedIn integration for professional visibility
- X (Twitter) integration for thread-based announcements
- Support for additional platforms (Slack, Telegram)

## Technical Details

- **Implementation**: `.github/workflows/scripts/discord_notifier.go`
- **Tests**: `.github/workflows/scripts/discord_notifier_test.go`
- **Test Coverage**: 55.2%
- **Language**: Go (for reliability and Unicode support)
