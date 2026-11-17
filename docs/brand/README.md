# Shellyficator mascot

A cheeky relay robot with screw-terminal ears, a wink, and a crooked smile.
The full character is for larger artwork; its compact face is the app mark.
Use Archivo 900 for the wordmark, Plex for application text, and readable upright
9000 numerals. The tagline is “Keeping your Shellys in line. Allegedly.”

## Assets

| Use | Files |
| --- | --- |
| Full character, transparent master | [PNG](shellyficator-mascot.png) |
| Full character, embedded login artwork | [PNG](../../internal/web/static/mascot-full.png) |
| Compact face, transparent master | [PNG](shellyficator-mascot-head.png) |
| Wordmark for dark backgrounds | [SVG](shellyficator-logo.svg), [PNG](shellyficator-logo.png) |
| Wordmark for light backgrounds | [SVG](shellyficator-logo-light.svg), [PNG](shellyficator-logo-light.png) |
| Design presentation | [PNG](mascot-presentation.png) |
| App, favicon, home-screen icon | [128px](../../internal/web/static/logo-mascot.png), [32px](../../internal/web/static/logo-mascot-32.png), [180px](../../internal/web/static/logo-mascot-180.png) |
| Winking head, looping transparent GIF | [192px GIF](shellyficator-mascot-head-wink.gif) |
| Full character, wink, lean and little hop | [512px GIF](shellyficator-mascot-wink.gif) |
| App animation frames | [Head WebP](../../internal/web/static/logo-mascot-frames.webp), [full WebP](../../internal/web/static/mascot-full-frames.webp) |
| Header wink layers | [Fixed head](../../internal/web/static/logo-mascot-base.png), [open eye](../../internal/web/static/logo-mascot-eye-open.png), [closed eye](../../internal/web/static/logo-mascot-eye-closed.png) |

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

## Motion

The app mark rests in its wink for 2.3 seconds, opens its eye for 900 milliseconds,
then settles back into the wink, on a 3.5-second loop. It uses a fixed head bitmap
and separate eyelids. The head, pupil and eyebrow stay in place while the eyelid
closes smoothly. The login mascot leans into its wink, then makes a small pleased
hop, with a twelve-second rest between performances.
Hover the mascot, or hover/focus the header's home link, to replay the motion.
The login mascot leaves its next performance until after you finish typing.

The login uses a four-frame WebP sheet with a controllable browser timeline;
the header uses PNG layers, with no movement of its head image. The GIFs are
standalone exports for sharing. The approved silhouette, smile, limbs and
palette stay intact.
The open-eye edits and their built-in image-generator prompts are recorded in
[generation.md](generation.md#animation-frame-prompts).

Motion stops when the mascot is offscreen or the document is hidden. Reduced
motion, unavailable JavaScript, unsupported animation APIs, or failed frame
loading leave the original static PNG visible. Favicons stay static.
The animation assets load only when motion is allowed and the mascot is visible.

To rebuild both sheets and GIFs from the checked-in masters and eye edits:

```sh
npm run build:mascot
```

This optional asset-authoring step requires ImageMagick 7 at `/usr/bin/magick`;
set `IMAGEMAGICK_BINARY` to another absolute path when needed. Ordinary
app builds use the checked-in assets and do not need ImageMagick or an image API.
