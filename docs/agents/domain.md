# Domain Docs

Before exploring code:

- Read `CONTEXT.md` at the repository root if it exists.
- Read relevant ADRs under `docs/adr/` if they exist.
- Use the vocabulary defined by `CONTEXT.md`.
- Surface conflicts with existing ADRs instead of silently overriding them.

## File structure

This repository uses the single-context layout:

```
/
├── CONTEXT.md
├── docs/adr/
└── src/
```

The `/domain-modeling` skill creates `CONTEXT.md` and ADRs lazily when domain terms or architectural decisions need to be recorded.
