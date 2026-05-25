# Security Policy

Nikki is a self-hosted diary for one person or a small trusted household. It is not yet a hardened multi-user SaaS and should not be deployed as a public multi-tenant service.

Operators are responsible for host security, HTTPS configuration, access control, backup protection, and restore testing.

## Supported Versions

Until a release versioning policy exists, security support is limited to the current `main` branch / current release only.

## Reporting a Vulnerability

If GitHub private vulnerability reporting is enabled for this repository, please use it.

If private vulnerability reporting is not enabled, open a public issue that contains only a non-sensitive request for contact. Do not include exploit details in the public issue.

Do not paste any of the following into public issues, pull requests, logs, screenshots, or shared chat:

- diary contents
- secrets or environment files
- session cookies
- bootstrap tokens
- passwords
- database dumps
- backup artifacts
- uploaded private images

## Scope Notes

Security reports are especially useful for authentication, session handling, upload validation, path traversal, backup exposure, production configuration, and unsafe public deployment assumptions.

Feature requests for public SaaS, multi-tenant hosting, broad sharing, or mobile-first photo workflows are outside Nikki's current product scope.
