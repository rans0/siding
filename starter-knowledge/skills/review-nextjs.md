# review-nextjs

Loaded at review when a repo in this track depends on `next`. Also apply review-node and review-typescript. Run the tools first; review what they cannot see.

Tools: `next build` (catches server/client boundary errors), `next lint` or the repo's ESLint config.

- **Server Actions and route handlers are public endpoints.** Each one checks auth and authorization itself and validates its input; a hidden button is not access control.
- **Server/client boundary.** Secrets, DB clients and server-only modules never imported into a `"use client"` file. Props passed to client components carry no more data than the UI shows. `server-only` used where the repo uses it.
- **Caching.** Every `fetch` or cached function has a deliberate cache setting. User-specific data never cached across users. Mutations revalidate (`revalidatePath`, `revalidateTag`) what they change.
- **Data fetching.** No request waterfalls: independent fetches run in parallel. No client-side fetch for data the server already has.
- **Rendering.** `"use client"` pushed to leaves, not the layout. No hydration mismatches (dates, random, `window` during render). `key` stable on lists.
- **Hooks.** Effect dependencies complete; no effect used for derived state. No state set during render.
- **Performance.** `next/image` and `next/font` used where the repo does; large client libraries loaded dynamically.

Sources: [Next.js data security](https://nextjs.org/docs/app/guides/data-security), [Next.js caching](https://nextjs.org/docs/app/guides/caching), [React rules of hooks](https://react.dev/reference/rules), [Vercel react-best-practices](https://github.com/vercel-labs/agent-skills/tree/main/skills/react-best-practices).
