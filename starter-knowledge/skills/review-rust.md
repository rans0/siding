# review-rust

Loaded at review when a repo in this track has `Cargo.toml`. Run the tools first; review what they cannot see.

Tools: `cargo clippy --all-targets -- -D warnings`, `cargo test`, `cargo audit` or `cargo deny check` if configured.

- **Panics.** No `unwrap`, `expect`, indexing or arithmetic that can panic on input a caller controls. `expect` messages state the invariant.
- **Errors.** Library code returns typed errors (the repo's `thiserror` style); binaries may use `anyhow`. Context added at boundaries, not at every `?`.
- **unsafe.** Each `unsafe` block has a `// SAFETY:` comment naming the invariant, and the invariant holds. Prefer a safe API.
- **Async.** No blocking calls or long CPU work on the async runtime; use `spawn_blocking`. No lock guard held across `.await`. Cancellation-safe at `select!` points.
- **Ownership.** No `clone()` added only to satisfy the borrow checker on a hot path. `Arc<Mutex<_>>` scope minimal.
- **API.** Public types implement the common traits the guidelines expect (`Debug`, `Clone`, `Send`/`Sync` where sound). Breaking changes to public items noted.

Sources: [Rust API Guidelines](https://rust-lang.github.io/api-guidelines/), [Clippy lints](https://rust-lang.github.io/rust-clippy/), [The Rustonomicon](https://doc.rust-lang.org/nomicon/), [RustSec](https://rustsec.org/).
