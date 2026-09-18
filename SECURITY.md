# Security policy

uLab executes commands supplied by the repository being tested. Treat an uLab configuration and its hook scripts as trusted code, not as passive data.

## Supported versions

uLab is currently pre-release. Security fixes are applied to the current `main` line; there is not yet a stable-version support policy.

## Reporting a vulnerability

Please do not publish exploit details in a public issue.

Use GitHub's private vulnerability reporting for this repository when that option is available. If private reporting is not available, open a minimal public issue asking for a private contact channel and omit exploit details, secrets, logs containing credentials, or proof-of-concept payloads.

A useful report includes:

- the affected uLab revision or release;
- operating system and runner type;
- the security impact;
- the smallest safe reproduction you can provide;
- whether the issue requires a malicious project configuration or occurs with trusted configuration.

## Trust boundary

The following behavior is intentional and is not, by itself, a security vulnerability:

- setup, upgrade and verify hooks can execute arbitrary commands with the permissions of the uLab process;
- the process runner executes directly on the host;
- the Docker Compose runner orchestrates project containers but is not presented as a security sandbox.

Security-sensitive bugs include cases where uLab itself crosses a documented boundary unexpectedly, corrupts or exposes evidence outside the configured run context, mishandles command/environment data in a way that expands privileges, or leaves runner resources behind in a way that creates a material security risk.

## Secrets and evidence

Hook output can be persisted in run evidence. Projects should avoid printing credentials, tokens or other secrets. uLab does not currently provide automatic secret redaction, so evidence bundles should be handled with the same care as CI logs.
