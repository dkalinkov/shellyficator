# Shellyficator mascot

Use Archivo 900 for the wordmark and Plex for UI text. Keep the mascot's
proportions and clear space. The tagline is “Keeping your Shellys in line. Allegedly.”

## Assets

| Use | Files |
| --- | --- |
| Full character, transparent master and login artwork | [PNG](../../internal/web/static/mascot-full.png) |
| Compact face, transparent master | [PNG](shellyficator-mascot-head.png) |
| Wordmark for dark backgrounds | [SVG](shellyficator-logo.svg), [PNG](shellyficator-logo.png) |
| Wordmark for light backgrounds | [SVG](shellyficator-logo-light.svg), [PNG](shellyficator-logo-light.png) |
| Design presentation | [PNG](mascot-presentation.png) |
| App, favicon, home-screen icon | [128px](../../internal/web/static/logo-mascot.png), [32px](../../internal/web/static/logo-mascot-32.png), [180px](../../internal/web/static/logo-mascot-180.png) |
| Winking head, looping transparent GIF | [192px GIF](shellyficator-mascot-head-wink.gif) |
| Full character, wink, lean and little hop | [512px GIF](shellyficator-mascot-wink.gif) |
| Animation sheets | [Head WebP export](animation/logo-mascot-frames.webp), [login WebP](../../internal/web/static/mascot-full-frames.webp) |
| Header wink layers | [Fixed head](../../internal/web/static/logo-mascot-base.png), [open eye](../../internal/web/static/logo-mascot-eye-open.png), [closed eye](../../internal/web/static/logo-mascot-eye-closed.png) |

The SVG wordmarks use outlined lettering and an embedded raster face.
See [generation prompts](generation.md) and the
[Archivo](../../internal/web/static/archivo.LICENSE) and
[Plex](../../internal/web/static/fonts.LICENSE) licenses.

## Motion

The header winks using separate eye layers. The login uses a WebP sprite and
pauses while the user types. Both use static artwork when reduced motion is
enabled or animation is unavailable.

Rebuild the sheets and GIFs with `npm run build:mascot`. It requires ImageMagick 7
at `/usr/bin/magick` or the absolute path in `IMAGEMAGICK_BINARY`.
