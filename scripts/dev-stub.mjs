// Dev-only launcher: plain Vite dev server for the frontend + a middleware
// that answers the Wails runtime backend calls (`POST /wails/runtime`) with
// the deterministic demo data from vectile-stub.mjs. Lets you open the UI in
// a normal browser without the real Go backend / model.
//
//   node scripts/dev-stub.mjs
//
// Reuses the same stub data as the screenshot pipeline so the UI behaves
// identically (search results, collections, settings config, etc.).

import { createServer } from "../frontend/node_modules/vite/dist/node/index.js";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { stub } from "./vectile-stub.mjs";

const frontendRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "../frontend");

const stubPlugin = {
  name: "vectile-stub-middleware",
  enforce: "pre",
  configureServer(server) {
    server.middlewares.use(async (req, res, next) => {
      if (req.method === "POST" && req.url?.startsWith("/wails/runtime")) {
        let raw = "";
        for await (const chunk of req) raw += chunk;
        const fakeRequest = {
          url: () => req.url ?? "",
          postDataJSON: () => {
            try {
              return JSON.parse(raw || "{}");
            } catch {
              return null;
            }
          },
        };
        const out = await stub(fakeRequest);
        if (out) {
          res.setHeader("Content-Type", out.contentType ?? "application/json");
          res.end(typeof out.body === "string" ? out.body : JSON.stringify(out.body));
          return;
        }
      }
      next();
    });
  },
};

// The stub middleware only answers request/response calls; it can't push the
// indexing events the real backend emits. This injected client script makes
// the Index page honest in the browser: it watches the Wails runtime POSTs
// for the index / cancel method IDs and drives a fake run — indexing:file
// ticks ending in indexing:complete / indexing:all-done, or
// indexing:cancelled when Cancel is hit. Screenshots don't run this server,
// and the simulation only starts on a real click, so it never fires during
// captures.
const indexSimPlugin = {
  name: "vectile-index-sim",
  transformIndexHtml() {
    return [
      {
        tag: "script",
        attrs: { type: "module" },
        children: `(() => {
  const INDEX_COL = 180963702;
  const INDEX_ALL = 2589092493;
  const CANCEL = 4016222948;

  let sim = null;

  const boot = () => {
    if (!(window._wails && window._wails.dispatchWailsEvent)) {
      setTimeout(boot, 20);
      return;
    }
    const emit = (name, data) => window._wails.dispatchWailsEvent({ name, data });

    const stop = () => { if (sim) { clearTimeout(sim.timer); sim = null; } };

    const start = (all, collection) => {
      stop();
      sim = {
        // Same order as the real backend's configuredCollections: obsidian,
        // calibre, then repos, then projects.
        all,
        names: all ? ["obsidian", "calibre", "vectile", "field-notes"] : [collection || "notes"],
        colIdx: 0,
        collection: all ? "obsidian" : collection || "notes",
        total: 60,
        indexed: 0,
      };
      tick();
    };

    const tick = () => {
      if (!sim) return;
      if (sim.indexed >= sim.total) {
        // The first collection stands in for a PDF-heavy library, so the
        // "pages had no readable text" offer has something to report.
        const noText = sim.colIdx === 0 ? 12 : 0;
        emit("indexing:complete", {
          collection: sim.collection,
          indexed: sim.indexed,
          skipped: 0,
          errors: 0,
          messages: [],
          pdfNoTextPages: noText,
        });
        sim.colIdx++;
        sim.noText = (sim.noText || 0) + noText;
        if (sim.all && sim.colIdx < sim.names.length) {
          sim.collection = sim.names[sim.colIdx];
          sim.indexed = 0;
          sim.timer = setTimeout(tick, 80);
          return;
        }
        if (sim.all) emit("indexing:all-done", { pdfNoTextPages: sim.noText || 0 });
        sim = null;
        return;
      }
      sim.indexed++;
      emit("indexing:file", {
        collection: sim.collection,
        file: sim.collection + "/doc-" + sim.indexed + ".md",
        indexed: sim.indexed,
        total: sim.total,
      });
      sim.timer = setTimeout(tick, 30);
    };

    const cancel = () => {
      if (!sim) return;
      const { all, collection, indexed } = sim;
      stop();
      emit("indexing:cancelled", { collection, indexed, skipped: 0, errors: 0 });
      if (all) emit("indexing:all-done", null);
    };

    // The runtime calls fetch(url, ...) with a URL object, so coerce to a
    // string before matching.
    const origFetch = window.fetch.bind(window);
    window.fetch = async (url, opts) => {
      try {
        if (
          String(url).indexOf("/wails/runtime") !== -1 &&
          opts &&
          opts.method === "POST"
        ) {
          const body = JSON.parse(opts.body || "{}");
          const args = body && body.args;
          const mid = args && args.methodID;
          if (mid === INDEX_COL) start(false, args.args && args.args[0]);
          else if (mid === INDEX_ALL) start(true);
          else if (mid === CANCEL) cancel();
        }
      } catch (_) {}
      return origFetch(url, opts);
    };
  };

  boot();
})();`,
      },
    ];
  },
};

// Drives the model download bar in the browser: watches the DownloadModel
// method ID and emits model:download-progress ticks ending in
// model:download-complete, or a cancel when CancelModelDownload is hit.
const downloadSimPlugin = {
  name: "vectile-download-sim",
  transformIndexHtml() {
    return [
      {
        tag: "script",
        attrs: { type: "module" },
        children: `(() => {
  const DOWNLOAD = 3567360488;
  const CANCEL_DL = 3373660950;

  let sim = null;

  const boot = () => {
    if (!(window._wails && window._wails.dispatchWailsEvent)) {
      setTimeout(boot, 20);
      return;
    }
    const emit = (name, data) => window._wails.dispatchWailsEvent({ name, data });
    const stop = () => { if (sim) { clearInterval(sim.timer); sim = null; } };

    const start = (key) => {
      stop();
      const total = 36700000;
      let downloaded = 0;
      sim = { key, total, timer: null };
      sim.timer = setInterval(() => {
        downloaded = Math.min(total, downloaded + total * 0.06);
        emit("model:download-progress", {
          key, downloaded, total,
          percent: (downloaded / total) * 100,
          speed: 12.4 * 1024 * 1024,
        });
        if (downloaded >= total) {
          stop();
          emit("model:download-complete", { key });
        }
      }, 120);
    };

    const origFetch = window.fetch.bind(window);
    window.fetch = async (url, opts) => {
      try {
        if (String(url).indexOf("/wails/runtime") !== -1 && opts && opts.method === "POST") {
          const body = JSON.parse(opts.body || "{}");
          const args = body && body.args;
          const mid = args && args.methodID;
          if (mid === DOWNLOAD) start(args.args && args.args[0]);
          else if (mid === CANCEL_DL) stop();
        }
      } catch (_) {}
      return origFetch(url, opts);
    };
  };

  boot();
})();`,
      },
    ];
  },
};

// Drives the OCR install bar in the browser: watches the InstallOCR method ID
// and emits ocr:install-progress ticks ending in ocr:install-complete, or
// ocr:install-cancelled when CancelOCRInstall is hit. The stub's own timer
// lands the installed flag at the same moment, so the reload the frontend does
// on completion shows the installed card.
const ocrSimPlugin = {
  name: "vectile-ocr-sim",
  transformIndexHtml() {
    return [
      {
        tag: "script",
        attrs: { type: "module" },
        children: `(() => {
  const INSTALL = 2152838503;
  const CANCEL_INSTALL = 4022925375;

  let sim = null;

  const boot = () => {
    if (!(window._wails && window._wails.dispatchWailsEvent)) {
      setTimeout(boot, 20);
      return;
    }
    const emit = (name, data) => window._wails.dispatchWailsEvent({ name, data });
    const stop = () => { if (sim) { clearInterval(sim.timer); sim = null; } };

    const start = () => {
      stop();
      const total = 17882639;
      let downloaded = 0;
      sim = { total, timer: null };
      sim.timer = setInterval(() => {
        downloaded = Math.min(total, downloaded + total / 16);
        emit("ocr:install-progress", {
          downloaded, total,
          percent: (downloaded / total) * 100,
          speed: 7.4 * 1024 * 1024,
        });
        if (downloaded >= total) {
          stop();
          emit("ocr:install-complete", null);
        }
      }, 120);
    };

    const origFetch = window.fetch.bind(window);
    window.fetch = async (url, opts) => {
      try {
        if (String(url).indexOf("/wails/runtime") !== -1 && opts && opts.method === "POST") {
          const body = JSON.parse(opts.body || "{}");
          const args = body && body.args;
          const mid = args && args.methodID;
          if (mid === INSTALL) start();
          else if (mid === CANCEL_INSTALL) { stop(); emit("ocr:install-cancelled", null); }
        }
      } catch (_) {}
      return origFetch(url, opts);
    };
  };

  boot();
})();`,
      },
    ];
  },
};

process.chdir(frontendRoot);
const server = await createServer({
  plugins: [stubPlugin, indexSimPlugin, downloadSimPlugin, ocrSimPlugin],
  server: { port: 9255, strictPort: false },
});

await server.listen();
const url = server.resolvedUrls?.local?.[0] ?? "http://127.0.0.1:9245";
console.log(`vectile stub dev server ready: ${url}`);

process.on("SIGINT", async () => {
  await server.close();
  process.exit(0);
});
