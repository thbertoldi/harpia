# Zitadel branding assets — handoff

This directory holds the **canonical, source-controlled** Harpia branding assets for
the dev cluster's Zitadel instance (M6 §2.7). Everything here is text/SVG so the dev
cluster bootstraps with zero binary tooling. The items below are the **real binary
assets still needed for production polish** — they are intentionally *not* committed
yet (placeholders / SVG stand-ins ship instead).

## What ships now (works, no binaries)

| File | Purpose | Status |
| --- | --- | --- |
| `logo.svg` | Wordmark, light variant for dark surface | Generic-serif stand-in (TODO: Bodoni Moda render) |
| `background.svg` | Obsidian radial gradient panel | Final — SVG gradient, no PNG needed for dev |
| `emails/*.html` + `*.txt` | Branded notification copy (EN + pt-BR) | Final for dev; see "Email wrapper" caveat below |

The CSS theme + asset references live in
`deploy/dev/kind/zitadel-branding-configmap.yaml` (key `theme.css`), and the upload
pipeline lives in `deploy/dev/kind/zitadel-init.yaml`.

## Real assets still needed for production polish

Drop these into the indicated paths, then rebuild the configmap / re-run the init Job.

1. **Bodoni Moda wordmark** — final designer SVG (or layered export), replacing
   `logo.svg`. Keep `viewBox`, cream `#f5f2eb` fill, transparent background. The
   inline `@font-face local('Bodoni Moda')` in `theme.css` will pick up the WOFF2
   below if a browser has it.

2. **Self-hosted WOFF2 fonts** → `deploy/dev/kind/assets/fonts/`
   - `BodoniModa-Regular.woff2` (wordmark / display serif)
   - `DMSans-Regular.woff2` (body)
   - `Manrope-Regular.woff2` (headings / buttons)
   - License files alongside: `BodoniModa-OFL.txt`, `DMSans-OFL.txt`, `Manrope-OFL.txt`
   Source from Google Fonts, subset with `pyftsubset`, Regular weight only (keep each
   < ~40KB). Until present, `theme.css` degrades to system serif/sans via the
   `local()` + generic-fallback chain — no broken rendering.

3. **Background PNG (optional)** → `deploy/dev/kind/assets/background.png`
   1920x1080, `radial-gradient(circle at center, #121318 0%, #0a0b0e 100%)`, ~30KB.
   Only needed if a raster background is preferred over the shipped SVG gradient
   (e.g. for clients that reject SVG backgrounds). Generate with:
   `convert -size 1920x1080 radial-gradient:'#121318'-'#0a0b0e' background.png`

## Email wrapper caveat (important)

Zitadel's **message-text API** (`/management/v1/text/message/{type}/{language}`) only
accepts structured fields — `subject`, `greeting`, `text`, `button_text`, `footer_text`
— not a full custom HTML document. The init Job (Task 32) pushes the **branded copy**
from these templates into those fields per language, which is what actually changes the
emails users receive.

**Locale code:** use `pt` in `email-texts.conf` (e.g. `init|pt|subject|…`). Zitadel does
not accept `pt-BR` on `/management/v1/text/message/{type}/{language}` (`Language is not
supported`). Portuguese email templates should match `PreferredLanguage` `pt` (and
optionally `pt-BR` if the browser sends it).

The full inline-styled `emails/*.html` files are the **design reference + future
notification-template wrapper**. Zitadel's surrounding HTML email shell is customized
separately (console → *Notifications → SMTP/Templates*, or a downstream mail provider
template). When that wrapper is set up for production, paste these HTML bodies in. The
`.txt` files are the plain-text fallbacks.

## Fonts placeholder

`fonts/` currently contains only `.gitkeep` + this note's font rows. The WOFF2 files are
binary and deliberately uncommitted; add them per item 2 above.
