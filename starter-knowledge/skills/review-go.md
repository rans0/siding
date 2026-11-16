# review-go

Loaded at review when a repo in this track has `go.mod`. Run the tools first; review what they cannot see.

Tools: `go vet ./...`, `go test -race ./...`, `staticcheck ./...` or `golangci-lint run` if the repo configures it, `govulncheck ./...`.

- **Errors.** Every error checked or explicitly discarded with a reason. Wrapped with `%w` when callers need `errors.Is/As`; not wrapped twice with the same context. No `panic` for expected failures.
- **Goroutines.** Each one has a way to stop (context, closed channel, `WaitGroup`). No goroutine blocked forever on an unread channel. Loop variables captured correctly on Go < 1.22.
- **Context.** Passed as the first parameter, never stored in a struct. Network and DB calls take it. Cancellation honored in long loops.
- **Concurrency.** Shared maps and slices guarded; `sync.Mutex` not copied by value. `-race` green on the changed packages.
- **Resources.** `defer Close()` after the error check, not inside a loop. HTTP response bodies closed. Timeouts on `http.Client` and servers.
- **Nil traps.** A typed nil returned as `error` interface. Nil map writes. Slices sharing a backing array after `append`.
- **API shape.** Accept interfaces, return structs. No interface with one implementation unless the repo does it. Exported names documented.

Sources: [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), [Google Go Style](https://google.github.io/styleguide/go/), [Uber Go Style Guide](https://github.com/uber-go/guide).
