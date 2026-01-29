---
sidebar_position: 2
title: Installation
description: All installation methods for Chronos - binary, Docker, Kubernetes, and package managers
---

# Installation

Chronos can be installed in multiple ways depending on your environment and requirements.

## System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| CPU | 1 core | 2+ cores |
| Memory | 512 MB | 2 GB |
| Disk | 1 GB | 10 GB SSD |
| Go | 1.22+ | Latest |

## Binary Installation

### Linux (amd64)

```bash
# Download the latest release
curl -L https://github.com/chronos/chronos/releases/latest/download/chronos-linux-amd64.tar.gz | tar xz

# Move to PATH
sudo mv chronos /usr/local/bin/
sudo mv chronosctl /usr/local/bin/

# Verify installation
chronos --version
chronosctl --version
```

### Linux (arm64)

```bash
curl -L https://github.com/chronos/chronos/releases/latest/download/chronos-linux-arm64.tar.gz | tar xz
sudo mv chronos chronosctl /usr/local/bin/
```

### macOS

```bash
# Intel Mac
curl -L https://github.com/chronos/chronos/releases/latest/download/chronos-darwin-amd64.tar.gz | tar xz

# Apple Silicon
curl -L https://github.com/chronos/chronos/releases/latest/download/chronos-darwin-arm64.tar.gz | tar xz

sudo mv chronos chronosctl /usr/local/bin/
```

### Windows

```powershell
# Download from GitHub releases
Invoke-WebRequest -Uri "https://github.com/chronos/chronos/releases/latest/download/chronos-windows-amd64.zip" -OutFile chronos.zip
Expand-Archive chronos.zip -DestinationPath C:\chronos
$env:PATH += ";C:\chronos"
```

## Docker

### Quick Start

```bash
docker run -d \
  --name chronos \
  -p 8080:8080 \
  -v chronos-data:/var/lib/chronos \
  chronos/chronos:latest
```

### With Custom Configuration

```bash
docker run -d \
  --name chronos \
  -p 8080:8080 \
  -v $(pwd)/chronos.yaml:/etc/chronos/chronos.yaml \
  -v chronos-data:/var/lib/chronos \
  chronos/chronos:latest \
  --config /etc/chronos/chronos.yaml
```

### Docker Compose

```yaml title="docker-compose.yml"
version: '3.8'
services:
  chronos:
    image: chronos/chronos:latest
    ports:
      - "8080:8080"
    volumes:
      - chronos-data:/var/lib/chronos
      - ./chronos.yaml:/etc/chronos/chronos.yaml
    command: ["--config", "/etc/chronos/chronos.yaml"]
    restart: unless-stopped

volumes:
  chronos-data:
```

## Kubernetes (Helm)

### Add the Helm Repository

```bash
helm repo add chronos https://chronos.github.io/charts
helm repo update
```

### Install

```bash
# Basic installation
helm install chronos chronos/chronos

# With custom values
helm install chronos chronos/chronos -f values.yaml

# In a specific namespace
helm install chronos chronos/chronos -n scheduling --create-namespace
```

### Example Values

```yaml title="values.yaml"
replicaCount: 3

resources:
  requests:
    cpu: 500m
    memory: 512Mi
  limits:
    cpu: 2000m
    memory: 2Gi

persistence:
  enabled: true
  size: 10Gi
  storageClass: standard

service:
  type: LoadBalancer

config:
  cluster:
    raft:
      heartbeat_timeout: 500ms
      election_timeout: 1s
```

## Package Managers

### Homebrew (macOS/Linux)

```bash
brew install chronos/tap/chronos
```

### APT (Debian/Ubuntu)

```bash
# Add repository
curl -fsSL https://packages.chronos.dev/gpg.key | sudo gpg --dearmor -o /usr/share/keyrings/chronos.gpg
echo "deb [signed-by=/usr/share/keyrings/chronos.gpg] https://packages.chronos.dev/apt stable main" | sudo tee /etc/apt/sources.list.d/chronos.list

# Install
sudo apt update
sudo apt install chronos
```

### YUM/DNF (RHEL/CentOS/Fedora)

```bash
# Add repository
sudo tee /etc/yum.repos.d/chronos.repo << EOF
[chronos]
name=Chronos Repository
baseurl=https://packages.chronos.dev/yum
enabled=1
gpgcheck=1
gpgkey=https://packages.chronos.dev/gpg.key
EOF

# Install
sudo dnf install chronos
```

## Build from Source

```bash
# Clone the repository
git clone https://github.com/chronos/chronos.git
cd chronos

# Build
make build

# Install
sudo make install
```

## Verify Installation

After installation, verify Chronos is working:

```bash
# Check version
chronos --version

# Start with default configuration
chronos --config chronos.yaml

# In another terminal, check health
curl http://localhost:8080/health
```

## Next Steps

- [Create your first job](/docs/getting-started/first-job)
- [Configure Chronos](/docs/reference/configuration)
- [Set up high availability](/docs/guides/high-availability)
