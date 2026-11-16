# review-security

Loaded at review for every track. Look at what the diff exposes, not the whole codebase.

Tools if the repo or machine has them: `semgrep --config auto`, `gitleaks detect`, `osv-scanner`, plus the stack's own (`govulncheck`, `npm audit`, `pip-audit`, `cargo audit`, `composer audit`).

- **Injection.** User input never concatenated into SQL, shell commands, file paths, templates, LDAP or regex. Parameterized APIs only.
- **Access control.** Every new endpoint, action or job checks who the caller is and whether they may touch this record (IDOR). Checks on the server, not the UI.
- **Secrets.** No keys, tokens or passwords in code, tests, fixtures or logs. New config read from env or the repo's secret store.
- **Untrusted URLs and files.** Outbound requests to user-supplied URLs are allow-listed (SSRF). Uploads checked for type and size and stored outside the web root. Path traversal on any file name from input.
- **Output.** User content escaped for its context (HTML, attribute, JS, URL). No `dangerouslySetInnerHTML` / `{!! !!}` / `|safe` on untrusted data.
- **Crypto and sessions.** No homemade crypto, MD5/SHA1 for passwords, or `Math.random` for tokens. Cookies `HttpOnly`, `Secure`, `SameSite`. CSRF protection kept on state-changing routes.
- **Defaults.** Debug mode, permissive CORS, verbose errors and disabled TLS verification off outside development.
- **Money and limits.** Amounts in integer minor units or decimals, never floats. Rate limits on login, OTP and expensive endpoints.
- **Supply chain.** New dependencies are maintained, pinned in the lockfile, and needed.

Sources: [OWASP Cheat Sheet Series](https://cheatsheetseries.owasp.org/), [OWASP ASVS](https://owasp.org/www-project-application-security-verification-standard/), [CWE Top 25](https://cwe.mitre.org/top25/), [Trail of Bits skills](https://github.com/trailofbits/skills).
