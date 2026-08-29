# Spatial grammar prototype

Prototype only. It compares three original low-resolution pixel-art layouts for codex-village:

- `?variant=A` — HQ radial village
- `?variant=B` — explicit execution tree
- `?variant=C` — district work town

Run it from the repository root with:

```bash
python3 -m http.server 4173 --directory prototype/spatial-grammar
```

Then open <http://localhost:4173/?variant=A> and use the bottom switcher or the left/right arrow keys.
