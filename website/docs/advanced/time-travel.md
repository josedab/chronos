---
sidebar_position: 4
title: Time-Travel Debugging
description: Debug job executions with time-travel replay
---

# Time-Travel Debugging

Step through execution history with VCR-like controls.

## Features

- Step forward/backward through execution
- Set breakpoints on step types
- Compare execution snapshots
- Export sessions for sharing

## API

```bash
# Create debug session
curl -X POST http://localhost:8080/api/v1/debug/sessions \
  -d '{"execution_id": "exec-123"}'
```
