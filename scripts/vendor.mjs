import { copyFileSync, mkdirSync } from 'node:fs';
const target = 'internal/web/static';
mkdirSync(target, { recursive: true });
copyFileSync('node_modules/htmx.org/dist/htmx.min.js', `${target}/htmx.min.js`);
copyFileSync('node_modules/htmx.org/LICENSE', `${target}/htmx.LICENSE`);
for (const weight of [400, 500, 600]) {
  copyFileSync(`node_modules/@fontsource/ibm-plex-sans/files/ibm-plex-sans-latin-${weight}-normal.woff2`, `${target}/plex-${weight}.woff2`);
}
copyFileSync('node_modules/@fontsource/ibm-plex-sans/LICENSE', `${target}/fonts.LICENSE`);
