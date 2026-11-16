# review-node

Loaded at review when a repo in this track has `package.json`. Run the tools first; review what they cannot see.

Tools: the repo's test and lint scripts, `npm audit --omit=dev` (or the repo's package manager equivalent).

- **Async.** Every promise awaited, returned or deliberately detached with its error handled. No `async` callback passed to `forEach`. No unhandled rejection paths.
- **Event loop.** No sync I/O (`readFileSync`, `execSync`), heavy JSON, crypto or regex on large input in a request path. Regex from user input checked for catastrophic backtracking.
- **Errors.** Thrown as `Error` objects, not strings. Operational errors (bad input, timeout) handled; programmer errors not swallowed. Express-style handlers pass errors to `next`.
- **Input.** Request body, query and headers validated at the boundary with the repo's schema library. No object spread of request bodies into DB models.
- **Resources.** Streams handle `error`; DB clients and pools reused, not created per request. Timeouts on outbound HTTP.
- **Config.** Secrets from env, never logged. `NODE_ENV`-dependent behavior intentional.
- **Dependencies.** New packages justified; a few lines of code beat a dependency. Lockfile updated with the change.

Sources: [Node.js: Don't block the event loop](https://nodejs.org/en/learn/asynchronous-work/dont-block-the-event-loop), [Node.js security best practices](https://nodejs.org/en/learn/getting-started/security-best-practices), [Node.js Best Practices](https://github.com/goldbergyoni/nodebestpractices).
