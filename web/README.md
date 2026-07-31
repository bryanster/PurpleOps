# web/

The single-page app: React, Vite, TypeScript, Tailwind and shadcn/ui. `web/dist` is embedded into
the server binary via `embed.FS`, so a release is one file.

The scaffold arrives in M0B-008, the generated API client in M0B-009, and the embedding in M0B-010.
Until `web/package.json` exists, the web half of every `make` target is skipped.
