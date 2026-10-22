// Generates docs/vectile-banner.svg — the README hero banner.
//
// The banner is drawn in the app's "field notebook" world: paper ground,
// graph-paper dots, hairline borders, one leaf-green accent, a serif-italic
// tagline, and mono data lines. The mascot comes from the app's own sprite
// sheet (`frontend/public/mascot/idle.png`): frame 1 is cropped to its drawn
// area and downscaled with nearest-neighbour so the pixel grid survives, then
// inlined so the banner is self-contained for GitHub. The app's original logo
// sits at the top, also inlined.
//
// Run:  node scripts/make-banner.mjs

import { createRequire } from "node:module";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

// playwright is installed alongside the screenshot-skill scripts, not in this
// repo — resolve it from there (dev-only asset generator).
const require = createRequire(
  "C:/Users/deuce/.copilot/skills/screenshot-skill/scripts/capture.mjs",
);
const { chromium } = require("playwright");

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const sheetPath = path.join(root, "frontend", "public", "mascot", "idle.png");
const logoPath = path.join(root, "frontend", "public", "vectile-logo.png");
const outPath = path.join(root, "docs", "vectile-banner.svg");
const LOGO_MARK = 48; // original logo resized down for the top-left mark

// Mascot placement. The sheet is one 960x960 strip of 10 frames; frame 1's
// drawn area is cropped and divided by SCALE (nearest-neighbour) so one art
// pixel lands on a clean multiple of the screen pixel.
const MASCOT_FRAME = { x: 144, y: 336, width: 664, height: 560 };
const MASCOT_SCALE = 5;
const MASCOT_W = Math.round(MASCOT_FRAME.width / MASCOT_SCALE); // 133
const MASCOT_H = Math.round(MASCOT_FRAME.height / MASCOT_SCALE); // 112

const sheetPng = await readFile(sheetPath);
const sheetB64 = sheetPng.toString("base64");

// Resize the original logo for the top-left mark via a canvas.
const logoPng = await readFile(logoPath);
const logoB64 = logoPng.toString("base64");

const browser = await chromium.launch();
const page = await browser.newPage();
await page.setContent(`<canvas id="c" width="${LOGO_MARK}" height="${LOGO_MARK}"></canvas>`);
const logoDataUri = await page.evaluate(
  ({ b64, size }) => {
    return new Promise((resolve, reject) => {
      const img = new Image();
      img.onload = () => {
        const c = document.getElementById("c");
        const ctx = c.getContext("2d");
        ctx.imageSmoothingEnabled = true;
        ctx.imageSmoothingQuality = "high";
        ctx.drawImage(img, 0, 0, size, size);
        resolve(c.toDataURL("image/png"));
      };
      img.onerror = reject;
      img.src = "data:image/png;base64," + b64;
    });
  },
  { b64: logoB64, size: LOGO_MARK },
);

// Crop frame 1 to the drawn area and downscale it without smoothing.
const mascotDataUri = await page.evaluate(
  ({ b64, frame, w, h }) => {
    return new Promise((resolve, reject) => {
      const img = new Image();
      img.onload = () => {
        const c = document.createElement("canvas");
        c.width = w;
        c.height = h;
        const ctx = c.getContext("2d");
        ctx.imageSmoothingEnabled = false;
        ctx.drawImage(img, frame.x, frame.y, frame.width, frame.height, 0, 0, w, h);
        resolve(c.toDataURL("image/png"));
      };
      img.onerror = reject;
      img.src = "data:image/png;base64," + b64;
    });
  },
  { b64: sheetB64, frame: MASCOT_FRAME, w: MASCOT_W, h: MASCOT_H },
);
await browser.close();

const sans = "'Instrument Sans', -apple-system, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif";
const serif = "'Instrument Serif', Georgia, 'Times New Roman', serif";
const mono = "'IBM Plex Mono', ui-monospace, 'SF Mono', 'Cascadia Mono', Consolas, monospace";

const svg = `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="1440" height="500" viewBox="0 0 1440 500" role="img" aria-label="vectile — your private library">
  <defs>
    <pattern id="dots" width="20" height="20" patternUnits="userSpaceOnUse">
      <circle cx="1.25" cy="1.25" r="1.25" fill="#1b2226" opacity="0.06"/>
    </pattern>
  </defs>

  <!-- paper ground + graph-paper dots -->
  <rect width="1440" height="500" fill="#fbfcff"/>
  <rect width="1440" height="500" fill="url(#dots)"/>

  <!-- hairline frame -->
  <rect x="0.5" y="0.5" width="1439" height="499" fill="none" stroke="#e4e8ec"/>

  <!-- app's original logo at the top of the banner -->
  <image x="82" y="48" width="${LOGO_MARK}" height="${LOGO_MARK}" preserveAspectRatio="xMidYMid meet"
         href="${logoDataUri}" xlink:href="${logoDataUri}"/>

  <!-- mascot on a white card, from the app's own sprite sheet -->
  <rect x="80" y="126" width="160" height="160" rx="20" fill="#ffffff" stroke="#d1d8de"/>
  <image x="94" y="150" width="132" height="111" preserveAspectRatio="xMidYMid meet"
         style="image-rendering:pixelated"
         href="${mascotDataUri}" xlink:href="${mascotDataUri}"/>

  <!-- wordmark -->
  <text x="272" y="214" font-family="${sans}" font-size="52" font-weight="700" letter-spacing="-1.2" fill="#1b2226">vectile</text>

  <!-- serif-italic tagline + leaf underscore -->
  <text x="276" y="270" font-family="${serif}" font-style="italic" font-size="23" fill="#56616b">your private library</text>
  <rect x="280" y="286" width="46" height="3" rx="1.5" fill="#1f8a50"/>

  <!-- vertical hairline between the lockup and the data block -->
  <line x1="1010" y1="118" x2="1010" y2="382" stroke="#e4e8ec"/>

  <!-- right data block: left accent rule + prominent all-local -->
  <rect x="1076" y="126" width="4" height="232" rx="2" fill="#1f8a50"/>
  <text x="1108" y="156" font-family="${sans}" font-size="19" font-weight="600" fill="#1f8a50">all local</text>

  <text x="1108" y="200" font-family="${mono}" font-size="14" fill="#1b2226">hybrid search — vector + full-text</text>
  <text x="1108" y="228" font-family="${mono}" font-size="14" fill="#1b2226">bge-m3 embeddings, in-process</text>
  <text x="1108" y="256" font-family="${mono}" font-size="14" fill="#1b2226">nothing leaves this machine</text>

  <line x1="1108" y1="282" x2="1396" y2="282" stroke="#e4e8ec"/>
  <text x="1108" y="324" font-family="${mono}" font-size="12" fill="#66707a">jump to search · ⌘K</text>
</svg>
`;

await mkdir(path.dirname(outPath), { recursive: true });
await writeFile(outPath, svg, "utf-8");
console.log(`wrote ${outPath} (${(svg.length / 1024).toFixed(1)} KB)`);
