# Shellyficator mascot

A cheeky relay robot with screw-terminal ears, a wink, and a crooked smile.
The full character is for larger artwork; its compact face is the app mark.
Keep Archivo 900, the readable upright 9000 stamp, and the existing tagline.

[Review the design canvas](https://superdesign.dev/teams/71e18a6e-73d2-4436-96ce-71ed93fcaf25/projects/47ebe5fd-6487-44ac-b16d-70ab77c57906?node=draft-variant-841c7c5a-389f-4373-b32a-7122f0f5fd8f).

## Assets

| Use | Files |
| --- | --- |
| Full character, transparent master | [PNG](shellyficator-mascot.png) |
| Compact face, transparent master | [PNG](shellyficator-mascot-head.png) |
| Wordmark for dark backgrounds | [SVG](shellyficator-logo.svg), [PNG](shellyficator-logo.png) |
| Wordmark for light backgrounds | [SVG](shellyficator-logo-light.svg), [PNG](shellyficator-logo-light.png) |
| Design presentation | [PNG](mascot-presentation.png) |
| App, favicon, home-screen icon | [128px](../../internal/web/static/logo-mascot.png), [32px](../../internal/web/static/logo-mascot-32.png), [180px](../../internal/web/static/logo-mascot-180.png) |

The SVG wordmarks contain an embedded raster face and outlined lettering; they
do not depend on installed fonts. The mascot itself is raster artwork. PNG
wordmarks have transparent backgrounds. Use the source masters for large
artwork and the small face for header and favicon sizes.

Keep clear space around the character, preserve its proportions, and avoid
adding another colored box behind its silhouette. Lime, midnight ink, warm
cream, and the app's dark teal form the palette. The tagline belongs on
authentication screens and larger brand artwork; the app header stays compact.

Artwork was created with the built-in image generator. The exact generation
and edit prompts are in [generation.md](generation.md). Archivo and Plex
letterforms use the existing locally vendored fonts and their
[Archivo](../../internal/web/static/archivo.LICENSE) and
[Plex](../../internal/web/static/fonts.LICENSE) licenses.

Verified in the running dashboard, device detail, and sign-in screens down to
320px: the mark loads, the wordmark fits, and header height stays 65px on desktop
and 61px on phones. PNG icon endpoints return successfully. The app build,
web tests, template formatting, and whitespace checks pass.
