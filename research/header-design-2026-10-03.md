# Project header and wordmark

Research and approved implementation, 3 October 2026. The compact header is
implemented with the final requested Archivo 900 wordmark.

## References

- [Herdr](https://herdr.dev/): compact brand at the left, restrained utility
  controls at the right, and heavy typography that carries the identity. A live
  browser inspection confirmed Archivo at weight 900 for its homepage headline.
  Use the requested Archivo for Shellyficator's wordmark at weight 900, with
  tight spacing; keep Plex for application content and supporting text.
- [Carbon's application header](https://www.carbondesignsystem.com/building-blocks/core/components/ui-shell-header/guidelines):
  product identity belongs at the left, global utilities at the right. Keep
  target areas generous and retain the existing skip-to-content link. Its
  hamburger pattern serves larger navigation structures; Shellyficator's two
  utilities can remain visible without adding a menu.
- [GitHub Primer's PageHeader](https://primer.style/product/components/page-header/guidelines/):
  separate identity, auxiliary information, and actions. Adapt their placement
  to narrow viewports. Apply that hierarchy to Shellyficator's global header;
  the device title and local section navigation remain separate.

## Approved direction

An instrument-panel masthead using the existing dark teal and lime theme:

- Original circuit-inspired S mark in a lime square; deliberate geometry replaces
  the platform-dependent lightning glyph.
- Archivo 900 wordmark with a compact, solid lime 9000 stamp. The stamp's backing
  has cut corners and a small tilt; its upright Plex 600 numerals use 16px desktop / 14px phone type with
  generous spacing for readability. Preserve
  the Shellyficator name and the approved tagline, “Keeping your Shellys in line.
  Allegedly.”
- Keep the tagline on sign-in/setup screens. Remove it from the app header so
  the brand and utilities share one compact row. Move version metadata to the
  quiet utility area; hide it below 768px.
- Data admin and Sign out retain readable desktop labels, with small supporting
  icons and a divider. Below 640px, show 40px icon buttons with complete
  accessible names. No new destinations or implied live-status badges.
- Match the content's 1440px maximum and 20px/40px gutters. Desktop header is
  65px high including the divider, reduced from the initial 89px proposal.
  Phones use a 61px single row, reduced from 133px, with the full wordmark and
  9000 badge visible at 320px.

## Implementation and verification

The dashboard beneath it is rendered from current templates using four sample
devices; it is visual context, not a reading of the live inventory. Local draft
files are under ignored `.superdesign/` and `test-results/header-design/`.

The shared `brand` and `header` templates in `internal/web/templates/layout.html`
use styles from `ui/styles.css`. Archivo 900 is locally served from the pinned
`@fontsource/archivo` 5.3.0 package; the existing vendor script and Makefile copy
the Latin WOFF2 file and its OFL license. The head preloads the font. The compact
brand fits auth cards at a 320px viewport. Native home/admin links and the POST
logout form retain their behavior and CSRF protection.

Dashboard and device detail were browser-checked at 1440, 1024, 768, 640, 639,
390, and 320px: no horizontal overflow or brand/control collisions, with 65px
desktop and 61px phone headers. Login and setup fit at 320px and retain the
tagline. The running development app serves the Archivo file successfully and
renders the wordmark at weight 900.

Keyboard focus is visible on both utilities. Data admin opens a new tab; the
home link and mouse/touch sign-out work. Touch checks at 768, 390, and 320px
confirmed 40px control heights and 40px square phone controls. These interaction
checks used a separate temporary database, without changing live accounts or
inventory. No browser errors were reported.

Validation passed: `make test` (Go tests and vet), `make build`, template
formatting, and `git diff --check`.

The stamp refinement uses larger upright Plex 600 numerals over a separately tilted,
solid lime backing with cut corners. Desktop and phone layouts were rechecked
down to 320px, including the narrow auth card, with no collisions or overflow.
Normal and high pixel density previews were inspected; the CSS build, app build,
and whitespace checks passed.
