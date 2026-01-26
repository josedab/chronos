# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.1.x   | :white_check_mark: |

## Reporting a Vulnerability

We take security vulnerabilities seriously. If you discover a security issue, please report it responsibly.

### How to Report

**Please DO NOT open public GitHub issues for security vulnerabilities.**

Instead, please report security vulnerabilities by emailing:

**security@chronos.dev**

Include the following information in your report:

- Description of the vulnerability
- Steps to reproduce the issue
- Potential impact
- Any suggested fixes (if applicable)

### What to Expect

- **Acknowledgment:** We will acknowledge receipt of your report within 48 hours.
- **Assessment:** We will investigate and assess the vulnerability within 7 days.
- **Resolution:** We aim to release a fix within 30 days for critical vulnerabilities.
- **Disclosure:** We will coordinate with you on public disclosure timing.

### Security Best Practices

When deploying Chronos, we recommend:

1. **Use HTTPS** for all webhook endpoints
2. **Enable authentication** in production deployments
3. **Use strong API keys** and rotate them regularly
4. **Run in a private network** or behind a reverse proxy
5. **Keep Chronos updated** to the latest version
6. **Review job configurations** to ensure webhooks point to trusted endpoints

### Security Features

Chronos includes several security features:

- API key authentication with bcrypt hashing
- Configurable RBAC (Role-Based Access Control)
- Webhook signing for request verification
- TLS support for gRPC and HTTP endpoints
- Audit logging for security-relevant events
- Rate limiting to prevent abuse

## Security Advisories

Security advisories will be published on:

- [GitHub Security Advisories](https://github.com/chronos/chronos/security/advisories)
- Release notes in CHANGELOG.md

Thank you for helping keep Chronos secure!
