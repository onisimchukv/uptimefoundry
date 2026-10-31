<!-- SPDX-License-Identifier: MIT -->
# Security Policy

UptimeFoundry is under active development and does not yet publish a production support commitment. Security fixes will be prioritized based on severity and affected releases.

## Reporting a vulnerability

Please do not report security vulnerabilities through public issues or discussions. Use GitHub's **Report a vulnerability** flow in the repository Security tab when private vulnerability reporting is available. If that option is not available, email **onisimchukvitalii@gmail.com** with the subject `[UptimeFoundry Security] <brief description>`.

Include the affected commit or version, component, environment, reproduction steps, impact, and any proposed mitigation. Do not include live credentials or sensitive customer data in the report; use a minimal proof of concept.

We will acknowledge reports as soon as practical, coordinate a fix and disclosure with the reporter, and share status privately. Do not assume a fixed response-time SLA until one is published here.

## Supported versions

There are no tagged stable releases yet. Until the first supported release is published, the current default branch is the only development line and is provided without a security maintenance guarantee.

## Deployment guidance

The development Compose stack and example configurations are not production deployment templates. Plain HTTP is intended only for local development. Deployments reachable beyond the local machine must use TLS with certificate verification; agent connections must use the authenticated mTLS design described in the current agent specification. Review privileges, secrets handling, network exposure, and capability requirements for your environment before deploying.
