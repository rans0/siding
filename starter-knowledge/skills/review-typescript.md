# review-typescript

Loaded at review when a repo in this track has `tsconfig.json`. Run the tools first; review what they cannot see.

Tools: `tsc --noEmit`, the repo's ESLint config (`typescript-eslint` type-checked rules if present).

- **Escape hatches.** No new `any`, `as` casts, non-null `!` or `@ts-ignore` without a comment saying why it is safe. Prefer `unknown` and narrowing.
- **Boundaries.** Data from network, storage or `JSON.parse` is untyped until validated. A type annotation on a fetch result is not validation.
- **Unions.** Discriminated unions handled exhaustively (`never` check or the repo's pattern). New enum or union members handled at every `switch`.
- **Nullability.** Optional fields checked before use; `?.` not hiding a value that must exist. `strictNullChecks` assumptions hold.
- **Promises.** No floating promises; `Promise<void>` not passed where a sync callback is expected.
- **Types match runtime.** Types not widened to make an error go away. Generics with one call site are a smell.

Sources: [TypeScript Handbook](https://www.typescriptlang.org/docs/handbook/intro.html), [typescript-eslint strict-type-checked](https://typescript-eslint.io/users/configs#strict-type-checked), [Google TypeScript Style Guide](https://google.github.io/styleguide/tsguide.html).
