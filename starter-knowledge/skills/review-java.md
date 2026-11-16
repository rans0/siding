# review-java

Loaded at review when a repo in this track has `pom.xml` or `build.gradle(.kts)`. Run the tools first; review what they cannot see.

Tools: the repo's build with tests (`mvn verify`, `gradle check`), plus Error Prone, SpotBugs, PMD or Checkstyle if configured.

- **Null.** Values from maps, DB and external calls checked; `Optional` not used for fields or parameters. Annotations (`@Nullable`) match behavior.
- **Exceptions.** No empty catch blocks; checked exceptions not wrapped into `RuntimeException` without context. Resources in try-with-resources.
- **Equality.** `equals` and `hashCode` overridden together; `==` not used on boxed numbers or strings.
- **Concurrency.** Shared mutable state guarded or immutable. No blocking calls inside `synchronized` on hot paths. Thread pools bounded and shut down. `@Transactional` not on private or self-invoked methods (Spring proxies skip them).
- **Collections.** Defensive copies on public getters of mutable collections. Streams not reused; no side effects in `map`.
- **Security.** No string-built SQL, JPQL or LDAP queries. Deserialization of untrusted data avoided. Bean validation on request DTOs.

Sources: [Oracle Secure Coding Guidelines for Java SE](https://www.oracle.com/java/technologies/javase/seccodeguide.html), [Google Java Style Guide](https://google.github.io/styleguide/javaguide.html), [Error Prone bug patterns](https://errorprone.info/bugpatterns), *Effective Java* (Bloch).
