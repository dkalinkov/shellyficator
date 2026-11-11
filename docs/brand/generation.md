# Mascot generation

Built-in image generator; transparent output. The user selected a playful
machine mascot after rejecting the circuit-S monogram. The character and compact
head are one identity. Lettering was composed separately with the approved fonts.

## Character prompt

```text
Use case: logo-brand / original mascot illustration.
Create ONE beautifully designed original mascot mark for Shellyficator 9000, a playful local app for managing smart electrical relay devices. Brand personality: a small competent machine that keeps your devices in line, allegedly; charming, slightly mischievous, never corporate.
Subject: a squat little relay-controller robot, front-facing with a slight confident lean. Its broad rounded lime-green head/body is the shape of a compact electrical relay module. The head occupies most of the silhouette, with a very large dark navy faceplate, two oversized warm-cream expressive eyes (one open, one cheerfully squinting), and a small crooked smile. Give it two chunky terminal/contact nubs on top as an unmistakable machine detail, and very short simple mechanical arms tucked alongside its body. A tiny side indicator and one small diagonal panel seam can add character. The robot should feel like a cheeky workshop helper, not an AI chatbot. Distinctive silhouette, memorable proportions, emotionally appealing expression.
Art direction: premium flat 2D mascot-logo illustration, bold clean contours, carefully drawn curves, restrained geometric details, chunky uniform deep-navy outlines and simple graphic shadow shapes. Crisp screen-print / die-cut sticker quality. Main colors electric lime #bdff00, midnight ink #000611, warm cream #ffd6a7. Three-color artwork, no gradients, no metallic 3D rendering, no glossy light effects. Limbs and features should be large and simple enough to read in an app header. Much more character than a generic letter monogram.
Composition: ONE isolated compact character centered on a genuinely transparent square canvas, fully visible, filling roughly 85 percent of the image, modest clear margins around the silhouette. No colored tile, no enclosing badge, no ground plane, no background, no scenery, no drop shadow outside the character.
No text, no lettering, no numbers, no typography, no watermark, no signature. The Shellyficator name and 9000 will be composed separately using the approved fonts. Avoid the letter S, lightning-bolt monograms, house/wifi icons, generic round chatbot faces, long antennae, intricate circuitry, realistic screws, props, or tiny decorative details.
```

## Compact-head edit prompt

Edit target: `shellyficator-mascot.png`; transparent output.

```text
Create the compact app-icon / logo-mark derivative of THIS exact Shellyficator robot mascot. Preserve its identity: the lime electrical relay head, two screw-terminal ears, navy faceplate, large warm-cream open left eye, cheerful winking right eye, crooked smirk and confident workshop-helper personality. Change ONLY the framing and detail density to make a small icon: show just the head with its two terminal ears, remove torso, arms and legs entirely. Keep the same front three-quarter orientation and unmistakable face. Simplify the outlines and face into bold crisp flat logo geometry with large open shapes; eliminate gradient texture, light streaks and small panel details. Use only electric lime #bdff00, midnight ink #000611 and warm cream #ffd6a7 with a single simple lime side shadow if needed. Smooth consistent chunky contours. The head should fill 90% of a square transparent canvas with small even clear margins, centered, with both terminal ears fully visible. No enclosing badge or background tile. No extra objects. Absolutely no lettering, no numbers, no wordmark, no watermark. Genuine transparent background. This is the same character adapted to remain recognizable at 28px and 16px, not a new character or a different design direction.
```

## Animation frame prompts

Created with the built-in image generator, transparent output. The original
masters remain unchanged. Only the cream eye and eyebrow layers from these
edits are used by [the asset builder](../../scripts/animate-mascot.mjs); it
recomposes them over the original artwork, makes intermediate squint poses,
and exports the sheets and looping GIFs. Eye placement is adjusted slightly
upward to keep a clear gap above the original smile.

### Head

Edit target: `shellyficator-mascot-head.png`.
Saved output: [animation/eye-open-source.png](animation/eye-open-source.png).

```text
Use case: precise-object-edit.
Asset type: alternate animation frame for an existing mascot logo.
Edit target: the attached Shellyficator head PNG. It is an existing approved character, not a new design.
Change ONLY the eye on the viewer's RIGHT: open this currently winking eye into a friendly warm-cream oval eye with a midnight-navy oval pupil and a small cream catchlight, matching the existing open eye on the viewer's LEFT. The new right eye should be a little smaller than the left eye because of the slight perspective. Keep its eyebrow, and keep enough separation between the eye and the crooked smile.
Preserve the exact character, original silhouette, head proportions, viewpoint, position, screw-terminal ears, lime shell, midnight faceplate, the LEFT eye, eyebrows, crooked smile, contours, colors, shading, framing and transparent background. Keep the character aligned to the input so this can alternate with the original for a wink animation. Do not add limbs, text, props, effects, or background. Same square canvas as the input. The only intended difference is the open right eye.
Output a single image, genuine transparent background.
```

### Full character

Edit target: `shellyficator-mascot.png`.
Saved output: [animation/full-eye-open-source.png](animation/full-eye-open-source.png).

```text
Use case: precise-object-edit.
Asset type: alternate animation frame for the existing full-body Shellyficator mascot.
Edit target: the attached transparent full-body mascot PNG.
Change ONLY the eye on the viewer's RIGHT and its eyebrow: open this currently winking eye into a friendly warm-cream oval eye with a midnight-navy oval pupil and a small cream catchlight, matching the existing open LEFT eye. The right eye should be smaller than the left eye due to the slight perspective. Move its eyebrow slightly upward if needed. Leave a clean navy gap between eye and crooked smile.
Preserve EVERYTHING else exactly: the complete original silhouette, position, proportions, lime relay shell, screw-terminal ears, original left eye and its eyebrow, crooked smirk, arms, hands, legs, boots, outlines, colors, shading, viewpoint, square framing, transparent background. No movement or different pose yet: this frame must align with the existing winking character for an animation.
Single image on a genuine transparent background. No words, new objects or effects.
```
