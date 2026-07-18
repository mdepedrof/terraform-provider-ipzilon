# Security Policy

## Supported Versions

| Version | Supported |
| ------- | --------- |
| latest  | ✅        |

Only the latest released version receives security fixes. Update to the latest version before reporting.

## Reporting a Vulnerability

**Do not open a public issue for security vulnerabilities.**

Report vulnerabilities via GitHub's private security advisory:
👉 [Report a vulnerability](https://github.com/mdepedrof/terraform-provider-ipzilon/security/advisories/new)

Please include:
- A description of the vulnerability and its potential impact
- Steps to reproduce or a proof-of-concept
- Affected versions
- Any suggested mitigations

You will receive a response within **48 hours**. If the vulnerability is confirmed, a fix will be released as soon as possible and credited to the reporter (unless you prefer to remain anonymous).

## Scope

This provider is a Terraform client that communicates with an IPzilon API server. The attack surface is limited to:

- **API credentials** (`token`) — store in environment variables or a secrets manager, never in `.tf` files
- **Provider binary integrity** — verify release checksums (`SHA256SUMS`) and the GPG signature (`SHA256SUMS.sig`) before use
