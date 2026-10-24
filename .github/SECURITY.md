# Security policy

## Reporting a vulnerability

Please **do not** open a public issue. Report privately through GitHub: **Security → Report a vulnerability** on this repository ([direct link](https://github.com/kubedojolearn/kubedojo/security/advisories/new)).

Include what you found, how to reproduce it, and the impact you expect. We aim to acknowledge reports within 3 working days and to agree on a disclosure timeline with you.

## Scope

In scope: the KubeDojo web app, API, WebSocket endpoint, command parser and simulator, content pipeline, and this repository's CI configuration.

Especially interesting: anything that executes learner input, reads or changes another learner's labs or progress, forges identity between the web app and the API, awards XP without doing the work, or injects script into pages.

Out of scope: findings that require a compromised device or browser, denial of service by volume, and reports from automated scanners without a demonstrated impact.

## Supported versions

Until the first release, only the `main` branch is supported.

## Design

The threat model is public: [docs/security/threat-model.md](../docs/security/threat-model.md).
