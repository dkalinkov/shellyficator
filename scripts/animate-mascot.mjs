// Rebuild the animation assets from the approved artwork and generated eye edits.
// Requires ImageMagick 7 (`magick`). No image-generation API is used at build time.
import { execFileSync } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';

process.chdir(fileURLToPath(new URL('..', import.meta.url)));
const work = mkdtempSync(join(tmpdir(), 'shellyficator-mascot-'));
const magick = (...args) => execFileSync('magick', args.map(String));
const geometry = ([x, y, w, h]) => `${w}x${h}+${x}+${y}`;
const lerp = (a, b, t) => Math.round(a + (b - a) * t);

const variants = [
  {
    name: 'head', master: 'docs/brand/shellyficator-mascot-head.png',
    source: 'docs/brand/animation/eye-open-source.png', size: 192,
    erase: [670, 635, 280, 214],
    eye: [696, 624, 205, 257], brow: [703, 548, 208, 64],
    closedEye: [683, 759, 258, 88], closedBrow: [693, 656, 231, 77],
    shift: [0, -24], sprite: 'logo-mascot-frames.webp', gif: 'shellyficator-mascot-head-wink.gif',
  },
  {
    name: 'full', master: 'docs/brand/shellyficator-mascot.png',
    source: 'docs/brand/animation/full-eye-open-source.png', size: 640,
    erase: [675, 444, 220, 169],
    eye: [677, 422, 172, 207], brow: [694, 361, 158, 55],
    closedEye: [682, 537, 207, 73], closedBrow: [694, 456, 179, 61],
    shift: [6, -16], sprite: 'mascot-full-frames.webp', gif: 'shellyficator-mascot-wink.gif',
  },
];

try {
  for (const variant of variants) {
    const file = name => join(work, `${variant.name}-${name}.png`);
    const [ex, ey, ew, eh] = variant.erase;
    // Lift only cream features from the generated edit. Everything outside this
    // eye and eyebrow, including the original smile, stays from the master.
    for (const part of ['eye', 'brow']) {
      magick(variant.source,
        '-crop', geometry(variant[part]), '+repage', '-channel', 'A',
        '-fx', 'clamp((r-0.025)/0.94)', '+channel', file(part));
    }
    if (variant.name === 'head') {
      magick(variant.master, '-crop', geometry(variant.closedEye), '+repage',
        '-channel', 'A', '-fx', 'clamp((r-0.025)/0.94)', '+channel', file('closedEye'));
    }
    magick(variant.master, '-crop', geometry(variant.erase), '+repage',
      '-alpha', 'off', '-colorspace', 'gray', '-threshold', '40%',
      '-morphology', 'Dilate', 'Disk:3', '-blur', '0x2', file('mask'));
    magick('-size', `${ew}x${eh}`, 'xc:#000713', file('mask'),
      '-alpha', 'off', '-compose', 'CopyOpacity', '-composite', file('patch'));
    magick(variant.master, file('patch'), '-geometry', `+${ex}+${ey}`,
      '-compose', 'Over', '-composite', file('clean'));

    if (variant.name === 'head') {
      // The header never moves a whole sprite. Its head is one fixed bitmap,
      // with independent eye layers clipped smoothly in the browser.
      const [bx, by] = variant.brow;
      magick(file('clean'), file('brow'), '-geometry',
        `+${bx + variant.shift[0]}+${by + variant.shift[1]}`,
        '-compose', 'Over', '-composite', '-resize', '192x192',
        'internal/web/static/logo-mascot-base.png');
      magick(file('eye'), 'internal/web/static/logo-mascot-eye-open.png');
      const [, , eyeWidth, eyeHeight] = variant.eye;
      const closedHeight = Math.round(variant.closedEye[3] * eyeWidth / variant.closedEye[2]);
      magick(file('closedEye'), '-resize', `${eyeWidth}x${closedHeight}!`,
        '-background', 'none', '-gravity', 'center', '-extent', `${eyeWidth}x${eyeHeight}`,
        'internal/web/static/logo-mascot-eye-closed.png');
    }

    const frames = [];
    for (let frame = 0; frame < 4; frame++) {
      const output = file(`frame-${frame}`);
      if (frame === 3 && variant.name === 'full') {
        magick(variant.master, '-resize', `${variant.size}x${variant.size}`, output);
      } else {
        const t = [0, 0.4, 0.78, 1][frame];
        const args = [file('clean')];
        for (const [part, closed] of [['eye', 'closedEye'], ['brow', 'closedBrow']]) {
          const start = variant[part].map((value, index) => value + (variant.shift[index] || 0));
          const target = variant[closed];
          let [x, y, width, height] = start.map((value, index) => lerp(value, target[index], t));
          let source = part;
          if (variant.name === 'head') {
            // At header sizes, travelling brows and eyes read as a shaking
            // face. Keep the brow fixed and close the eye around one centre.
            [x, y, width, height] = start;
            if (part === 'eye') {
              height = lerp(start[3], Math.round(target[3] * width / target[2]), t);
              y = Math.round(start[1] + (start[3] - height) / 2);
              if (frame === 3) source = 'closedEye';
            }
          }
          magick(file(source), '-resize', `${width}x${height}!`, file(`${part}-scaled`));
          args.push(file(`${part}-scaled`), '-geometry', `+${x}+${y}`, '-compose', 'Over', '-composite');
        }
        magick(...args, '-resize', `${variant.size}x${variant.size}`, output);
      }
      frames.push(output);
    }
    const compression = variant.name === 'head' ? ['-define', 'webp:lossless=true'] : [];
    const quality = variant.name === 'head' ? 100 : 94;
    magick(...frames, '+append', '-quality', quality, '-define', 'webp:method=6', ...compression,
      `internal/web/static/${variant.sprite}`);

    // GIF exports are self-contained; the app uses the same frames with a
    // controllable timeline, preserving full alpha and reduced-motion support.
    const timeline = variant.name === 'head'
      ? [[3, 230], [2, 7], [1, 8], [0, 90], [1, 8], [2, 7]]
      : [[0, 150, 0, 0], [0, 9, -2, 1], [0, 9, -4, 2], [0, 9, -3, 2],
        [1, 5, -2, 1], [2, 4, -1, 0], [3, 18, 0, 0], [2, 5, 1, -2],
        [1, 6, 1, -4], [0, 9, 1, -5], [0, 9, 0, -3], [0, 9, 0, -1], [0, 200, 0, 0]];
    const gifArgs = [];
    timeline.forEach(([frame, delay, angle = 0, lift = 0], index) => {
      const pose = file(`pose-${index}`);
      const size = variant.name === 'full' ? 512 : 192;
      magick(frames[frame], '-resize', `${size}x${size}`, '-background', 'none',
        '-rotate', angle, '-gravity', 'center', '-extent', `${size}x${size}`,
        '-page', `+0${lift < 0 ? lift : `+${lift}`}`, pose);
      gifArgs.push('-delay', delay, '-dispose', 'Background', pose);
    });
    magick(...gifArgs, '-coalesce', '-alpha', 'set', '-channel', 'A', '-threshold', '50%',
      '+channel', '-dither', 'None', '-colors', '128', '-loop', '0', '-layers', 'Optimize',
      `docs/brand/${variant.gif}`);
    console.log(`Built ${variant.sprite} and ${variant.gif}`);
  }
} finally {
  rmSync(work, { recursive: true, force: true });
}
