# kloudsPanel Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| 0.x (pre-release) | :white_check_mark: |

## Reporting a Vulnerability

Please **do not** open public GitHub issues for security vulnerabilities.

Email: security@example.com (replace with your actual security contact)

Include:
- Description of the vulnerability
- Steps to reproduce
- Potential impact
- Any suggested remediation

We will acknowledge receipt within 48 hours and provide an initial assessment within 7 days.

## Security Design

- Docker socket access is restricted to the node agent process only
- All secrets are envelope-encrypted; plaintext never persists to disk
- User accounts require admin approval before activation
- MCP server uses OAuth 2.1 with PKCE and narrow workspace-scoped permissions
- Terminal access requires short-lived grants with idle/absolute timeouts
