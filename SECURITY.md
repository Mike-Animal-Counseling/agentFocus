# Security policy

The current default branch is supported. Install the locked dependencies and use a supported, patched runtime. Examples and showcase assets do not constitute an audited production service.

Report vulnerabilities privately using [GitHub private vulnerability reporting](https://github.com/Mike-Animal-Counseling/agentFocus/security/advisories/new). Include the affected commit, impact, and a minimal reproduction with synthetic data. Never publish passwords, access tokens, customer data, or a working exploit against a deployed instance in an issue.

Do not test systems you do not own. Rotate exposed credentials at the issuing provider and in every deployment; removing a file or rewriting Git history does not revoke a credential.

See [production deployment requirements](docs/PRODUCTION_SECURITY.md) for the supported deployment boundary and required operator configuration.
