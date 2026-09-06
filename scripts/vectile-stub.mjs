// Deterministic demo data served for the Wails backend RPC endpoint
// (`POST /wails/runtime`). Screenshot-only: the real backend runs in-process
// (llama.go bge-m3 embeddings + modernc sqlite/vec + FTS5) and the UI ships
// with empty collections until you index something.
//
// The Wails runtime POSTs `{ object, method, args }` to /wails/runtime.
// Backend method calls arrive as object=0, method=0 with
// args = `{ "call-id", methodID, args }`. We answer by methodID.
//
// Usage from capture.mjs: `--route-stub ./scripts/vectile-stub.mjs`

const M = {
  GetStatus: 1831479589,
  GetCPUCount: 899550166,
  GetVersion: 712846061,
  ListCollections: 1339457758,
  ListSources: 553118937,
  ListDocuments: 1318390221,
  Search: 2587852292,
  GetConfig: 2113296768,
  DeleteSource: 2650919656,
  DeleteCollection: 3393632225,
  DeleteDocuments: 2694537237,
  GetIndexingState: 148853163,
  IndexCollection: 180963702,
  IndexAll: 2589092493,
  CancelIndexing: 4016222948,
  ListModels: 4184755701,
  ImportModel: 3637578651,
  SetActiveModel: 859586252,
  DeleteModel: 3374659369,
  UpdateModelSettings: 2688418022,
  ListRecommendedModels: 2741288292,
  DownloadModel: 3567360488,
  CancelModelDownload: 3373660950,
  GetDownloadState: 1292897846,
  GetMCPStatus: 2163182986,
  StartServer: 4062741143,
  StopServer: 2204354075,
};

const MODEL_NAME = "bge-m3";
const MODEL_PATH = "C:\\Users\\you\\AppData\\Roaming\\vectile\\models\\bge-m3-Q4_K_M.gguf";

// ---------------------------------------------------------------------------
// Status + config
// ---------------------------------------------------------------------------

const status = {
  collections: 5,
  sources: 26,
  chunks: 23867,
  dbSize: 53687091, // ~51.2 MB
  modelState: "loaded",
  modelName: MODEL_NAME,
  modelPath: MODEL_PATH,
  modelError: "",
  lastIndexed: "2026-08-24", // most recent last_indexed_at across sources
};

const config = {
  embedding_model: MODEL_NAME,
  active_model: MODEL_PATH,
  embedding_batch_size: 16,
  chunk_size_tokens: 200,
  chunk_overlap_tokens: 30,
  obsidian_vaults: ["C:\\Users\\you\\Documents\\notes"],
  obsidian_exclude_folders: [".obsidian", ".trash"],
  calibre_libraries: ["C:\\Users\\you\\Calibre Library"],
  repositories: { vectile: ["C:\\Users\\you\\code\\vectile"] },
  projects: { "field-notes": ["C:\\Users\\you\\Documents\\field-notes"] },
  disabled_collections: [],
  skip_cloud_placeholders: true,
  git_history_in_months: 6,
  git_commit_subject_blacklist: ["^Merge ", "^fixup! ", "^WIP "],
  search_defaults: { top_k: 12, rrf_k: 60, vector_weight: 1.0, fts_weight: 1.0 },
  gui: {
    auto_reindex: false,
    auto_reindex_interval_minutes: 60,
    start_on_login: false,
    mascot: { show_searching: true, show_indexing: true, show_nothing: true },
  },
  mcp: { enabled: true, port: 31123, allow_write: false },
};

// Live MCP server state returned by GetMCPStatus; StartServer/StopServer
// mutate it so the Settings status plate reacts in the browser.
let mcp = { running: true, port: 31123, url: "http://127.0.0.1:31123/sse" };

// ---------------------------------------------------------------------------
// Models (installed embedding models)
// ---------------------------------------------------------------------------

let models = [
  { id: 1, name: "bge-m3-Q4_K_M", path: MODEL_PATH, dimensions: 1024, contextWindow: 2048, batchSize: 32, threads: 0, isActive: true, created: "2026-08-24" },
  { id: 2, name: "mxbai-embed-large", path: "C:\\Users\\you\\AppData\\Roaming\\vectile\\models\\mxbai-embed-large.gguf", dimensions: 1024, contextWindow: 2048, batchSize: 32, threads: 0, isActive: false, created: "2026-08-24" },
];

const recommendedModels = [
  { key: "bge-small-en-v1.5-q8_0", name: "BGE Small EN v1.5", file: "bge-small-en-v1.5-q8_0.gguf", url: "", sha256: "", dimensions: 384, sizeBytes: 36700000, quantization: "Q8_0", language: "English", recommended: true, description: "Best quality for the size.", hfUrl: "https://huggingface.co/ggml-org/bge-small-en-v1.5-Q8_0-GGUF" },
  { key: "bge-small-en-v1.5-q4_k_m", name: "BGE Small EN v1.5", file: "bge-small-en-v1.5-q4_k_m.gguf", url: "", sha256: "", dimensions: 384, sizeBytes: 24800000, quantization: "Q4_K_M", language: "English", recommended: false, description: "Smallest of the English options.", hfUrl: "https://huggingface.co/CompendiumLabs/bge-small-en-v1.5-gguf" },
  { key: "bge-m3-q4_k_m", name: "BGE-M3", file: "bge-m3-Q4_K_M.gguf", url: "", sha256: "", dimensions: 1024, sizeBytes: 438000000, quantization: "Q4_K_M", language: "Multilingual", recommended: false, description: "Best semantic quality.", hfUrl: "https://huggingface.co/lm-kit/bge-m3-gguf" },
];

let downloadState = { active: false, key: "", status: "", downloaded: 0, total: 0, percent: 0, speed: 0, error: "" };

// ---------------------------------------------------------------------------
// Collections -> sources -> documents
// ---------------------------------------------------------------------------

const collections = [
  { id: 1, name: "calibre", type: "system", description: "your ebook library — chunked for search", sources: 2, chunks: 3810, created: "2025-11-02", enabled: true, lastIndexed: "2025-11-02" },
  { id: 2, name: "email", type: "system", description: "Gmail export — invoices, receipts, long threads", sources: 4, chunks: 2310, created: "2025-09-14", enabled: true, lastIndexed: "2025-09-14" },
  { id: 3, name: "notes", type: "project", description: "Obsidian vault — daily notes, zettels, project journals", sources: 8, chunks: 9812, created: "2025-06-30", enabled: true, lastIndexed: "2026-08-24" },
  { id: 4, name: "rss", type: "system", description: "feeds on Go, databases, and photography", sources: 6, chunks: 3104, created: "2025-10-11", enabled: true, lastIndexed: "2025-10-11" },
  { id: 5, name: "vectile", type: "code", description: "this repo — backend, frontend, and git history", sources: 6, chunks: 4831, created: "2026-08-24", enabled: true, lastIndexed: "2026-08-24" },
];

let sources = [
  // calibre
  { id: 11, collectionId: 1, sourceType: "epub", path: "C:\\Users\\you\\Calibre Library\\Bronson\\The Nudist on the Late Shift", chunks: 310, lastIndexed: "2025-11-02" },
  { id: 12, collectionId: 1, sourceType: "pdf", path: "C:\\Users\\you\\Calibre Library\\Luksa\\Kubernetes in Action", chunks: 820, lastIndexed: "2025-11-02" },
  // email
  { id: 21, collectionId: 2, sourceType: "email", path: "C:\\Users\\you\\mail\\2026\\jan.mbox", chunks: 402, lastIndexed: "2025-09-14" },
  { id: 22, collectionId: 2, sourceType: "email", path: "C:\\Users\\you\\mail\\2026\\sre.mbox", chunks: 318, lastIndexed: "2025-09-14" },
  { id: 23, collectionId: 2, sourceType: "email", path: "C:\\Users\\you\\mail\\2026\\orders.mbox", chunks: 891, lastIndexed: "2025-09-14" },
  { id: 24, collectionId: 2, sourceType: "email", path: "C:\\Users\\you\\mail\\2026\\feb.mbox", chunks: 699, lastIndexed: "2025-09-14" },
  // notes
  { id: 31, collectionId: 3, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\zettelkasten", chunks: 412, lastIndexed: "2026-08-24" },
  { id: 32, collectionId: 3, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\inbox", chunks: 57, lastIndexed: "2026-08-24" },
  { id: 33, collectionId: 3, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\projects\\k8s-upgrade", chunks: 96, lastIndexed: "2026-08-22" },
  { id: 34, collectionId: 3, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\camera", chunks: 88, lastIndexed: "2026-08-20" },
  { id: 35, collectionId: 3, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\cooking", chunks: 71, lastIndexed: "2026-08-18" },
  { id: 36, collectionId: 3, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\reading", chunks: 39, lastIndexed: "2026-08-15" },
  { id: 37, collectionId: 3, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\work", chunks: 26, lastIndexed: "2026-08-12" },
  { id: 38, collectionId: 3, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\travel", chunks: 23, lastIndexed: "2026-08-09" },
  // rss
  { id: 41, collectionId: 4, sourceType: "rss", path: "C:\\Users\\you\\feeds\\systems.xml", chunks: 612, lastIndexed: "2025-10-11" },
  { id: 42, collectionId: 4, sourceType: "rss", path: "C:\\Users\\you\\feeds\\golang.xml", chunks: 388, lastIndexed: "2025-10-11" },
  { id: 43, collectionId: 4, sourceType: "rss", path: "C:\\Users\\you\\feeds\\photo.xml", chunks: 274, lastIndexed: "2025-10-11" },
  { id: 44, collectionId: 4, sourceType: "rss", path: "C:\\Users\\you\\feeds\\news.xml", chunks: 519, lastIndexed: "2025-10-11" },
  { id: 45, collectionId: 4, sourceType: "rss", path: "C:\\Users\\you\\feeds\\papers.xml", chunks: 703, lastIndexed: "2025-10-11" },
  { id: 46, collectionId: 4, sourceType: "rss", path: "C:\\Users\\you\\feeds\\css.xml", chunks: 608, lastIndexed: "2025-10-11" },
  // vectile (code)
  { id: 51, collectionId: 5, sourceType: "code", path: "C:\\Users\\you\\code\\vectile\\backend\\search", chunks: 402, lastIndexed: "2026-08-24" },
  { id: 52, collectionId: 5, sourceType: "code", path: "C:\\Users\\you\\code\\vectile\\backend\\indexer", chunks: 611, lastIndexed: "2026-08-24" },
  { id: 53, collectionId: 5, sourceType: "code", path: "C:\\Users\\you\\code\\vectile\\backend\\chunker", chunks: 318, lastIndexed: "2026-08-24" },
  { id: 54, collectionId: 5, sourceType: "code", path: "C:\\Users\\you\\code\\vectile\\frontend\\src", chunks: 502, lastIndexed: "2026-08-24" },
  { id: 55, collectionId: 5, sourceType: "code", path: "C:\\Users\\you\\code\\vectile\\third_party\\llama-go", chunks: 204, lastIndexed: "2026-08-24" },
  { id: 56, collectionId: 5, sourceType: "commit", path: "C:\\Users\\you\\code\\vectile\\.git", chunks: 2794, lastIndexed: "2026-08-24" },
];

let documents = [
  // calibre — epub
  { id: 1111, sourceId: 11, collectionId: 1, chunkIndex: 0, title: "Prologue — the late shift", content: "The late shift in the Valley started quietly: a handful of engineers in a rented office, shipping while the rest of the industry slept. Nobody set out to make a culture of it. It just turned out that the work got done at night, and the morning was for arguing about what had been built.", metadata: { page: 3 } },
  { id: 1112, sourceId: 11, collectionId: 1, chunkIndex: 1, title: "Chapter 1 — two founders", content: "Both founders came from support desks. That was the whole trick, they said: they knew what the users typed when they were stuck. So the product was built from search logs, not from a vision deck.", metadata: { page: 17 } },
  { id: 1113, sourceId: 11, collectionId: 1, chunkIndex: 2, title: "Chapter 2 — growth without a plan", content: "Growth came from one feature that shipped three weeks before anyone asked for it. The team kept a rule: if a customer mentioned the same pain twice, it was already a spec.", metadata: { page: 41 } },
  // calibre — pdf
  { id: 1211, sourceId: 12, collectionId: 1, chunkIndex: 0, title: "Rolling updates and rollbacks", content: "A rolling update replaces the old ReplicaSet gradually. The Deployment controller keeps the service available the whole time, and you can pause, resume, or roll back from the rollout status.", metadata: { page: 214 } },
  { id: 1212, sourceId: 12, collectionId: 1, chunkIndex: 1, title: "Choosing a strategy", content: "Pick a strategy by blast radius, not by fashion. Rolling suits stateless services. Canary suits services with real traffic you can measure. Blue-green doubles your cost while the switch happens.", metadata: { page: 221 } },
  // email
  { id: 2101, sourceId: 21, collectionId: 2, chunkIndex: 0, title: "RE: Canary rollout tonight", content: "Sending the canary rollout tonight at 21:00 UTC. Dashboard shows healthy so far; if the error rate stays flat for 15 minutes we promote to 100%. Hold any deploys until then.", metadata: { sender: "sre@acme.dev" } },
  { id: 2102, sourceId: 21, collectionId: 2, chunkIndex: 1, title: "Meeting notes — Jan 2026", content: "Decided: this quarter is about killing the toil. Three things on the table — the rollout script, the backup restore drill, and the dashboard that no one reads. The rollout script gets built first.", metadata: { sender: "you@acme.dev" } },
  // notes — zettelkasten
  { id: 3101, sourceId: 31, collectionId: 3, chunkIndex: 0, title: "The exposure triangle, revisited", content: "Shutter, aperture, ISO are a triangle only in the sense that moving one forces the others to move. The real question is always: what do you want frozen, blurred, or clean at this light level?", metadata: { tags: ["photography"] } },
  { id: 3102, sourceId: 31, collectionId: 3, chunkIndex: 1, title: "Notes on quorum and Raft", content: "Raft needs a majority for both election and commit. That is the whole trick — you trade availability during partitions for a guarantee that two leaders never both think they own the term.", metadata: { tags: ["distributed-systems"] } },
  // notes — k8s-upgrade
  { id: 3301, sourceId: 33, collectionId: 3, chunkIndex: 0, title: "Rollout strategies — 2026-06-12", content: "Weighed rolling, blue-green, and canary for the k8s upgrade. Rolling wins on simplicity: one command, no extra infra, and rollout status gives a clean view of progress. Set maxUnavailable to 25% so we keep headroom during the drain.", metadata: { tags: ["kubernetes", "deploy"] } },
  { id: 3302, sourceId: 33, collectionId: 3, chunkIndex: 1, title: "Rolling update checklist", content: "Before you roll: backups done, migration run, feature flag off by default, metrics page open. During the rollout: watch rollout status, error rate, and p99. After: leave the flag on and delete the old ReplicaSet.", metadata: { tags: ["kubernetes"] } },
  { id: 3303, sourceId: 33, collectionId: 3, chunkIndex: 2, title: "Blue-green for the API", content: "If the API breaks again, use blue-green. Point traffic at green, run the old blue beside it, and keep the rollback one DNS flip away. Costs double while both stacks are up — fine for a weekend.", metadata: { tags: ["kubernetes"] } },
  // rss
  { id: 4101, sourceId: 41, collectionId: 4, chunkIndex: 0, title: "Choosing a Kubernetes rollout strategy", content: "Rolling, canary, blue-green — a short field guide. The short version: rolling for stateless, canary when you can measure, blue-green when you need a one-flip rollback.", metadata: { author: "Systems Weekly" } },
  { id: 4102, sourceId: 41, collectionId: 4, chunkIndex: 1, title: "Why quorum matters in etcd", content: "etcd needs a quorum of peers to accept writes. Lose it and the cluster stops electing leaders. That is why the k8s control plane runs three or five members and why split brain is the failure you plan for first.", metadata: { author: "Systems Weekly" } },
];

// ---------------------------------------------------------------------------
// Search results for the demo query ("kubernetes rollout")
// ---------------------------------------------------------------------------

const searchResults = [
  {
    title: "Rollout strategies for the k8s upgrade",
    content: "We weighed rolling, blue-green, and canary for the k8s upgrade. A rolling update won on simplicity: one command, no extra infra, and rollout status gives a clean way to watch progress. Set maxUnavailable to 25% and keep headroom during the node drain.",
    score: 0.92,
    collection: "notes",
    sourceType: "markdown",
    sourcePath: "C:\\Users\\you\\Documents\\notes\\projects\\k8s-upgrade\\rollout-strategies.md",
    metadata: { tags: ["kubernetes", "deploy"] },
  },
  {
    title: "RE: Canary rollout tonight",
    content: "Sending the canary rollout tonight at 21:00 UTC. The Kubernetes dashboard looks healthy so far; if the error rate stays flat for 15 minutes we promote to 100%. Please hold any deploys until then.",
    score: 0.81,
    collection: "email",
    sourceType: "email",
    sourcePath: "C:\\Users\\you\\mail\\2026\\sre.mbox",
    metadata: { sender: "sre@acme.dev" },
  },
  {
    title: "Rolling, canary, blue-green: choosing a Kubernetes rollout strategy",
    content: "A short field guide to Kubernetes rollout patterns: when rolling updates are fine, when you need a canary, and how blue-green changes your blast radius. The short version: measure, then trust the dashboard.",
    score: 0.76,
    collection: "rss",
    sourceType: "rss",
    sourcePath: "C:\\Users\\you\\feeds\\systems.xml",
    metadata: { author: "Systems Weekly" },
  },
  {
    title: "Kubernetes in Action — Rolling updates and rollbacks",
    content: "Rolling updates replace pods gradually, keeping the service available while the new version rolls out. Kubernetes tracks the rollout status and exposes it so you can verify before moving on.",
    score: 0.68,
    collection: "calibre",
    sourceType: "pdf",
    sourcePath: "C:\\Users\\you\\Calibre Library\\Luksa\\Kubernetes in Action\\Kubernetes in Action - Luksa.pdf",
    metadata: { page: 214, authors: ["Marko Lukša"] },
  },
  {
    title: "Rolling update checklist",
    content: "Before you roll: backups done, migration run, feature flag off by default, metrics page open. During the rollout: watch rollout status, error rate, and p99. After: leave the flag on and delete the old ReplicaSet.",
    score: 0.63,
    collection: "notes",
    sourceType: "markdown",
    sourcePath: "C:\\Users\\you\\Documents\\notes\\projects\\k8s-upgrade\\checklist.md",
    metadata: { tags: ["kubernetes"] },
  },
  {
    title: "Invoice 2026-06 — hosting and monitoring",
    content: "Thanks for your business. This invoice covers June hosting and monitoring for the cluster, including the Kubernetes control-plane nodes and the canary environments. Payment is due in 14 days.",
    score: 0.44,
    collection: "email",
    sourceType: "email",
    sourcePath: "C:\\Users\\you\\mail\\2026\\orders.mbox",
    metadata: { sender: "billing@hosting.example" },
  },
];

// ---------------------------------------------------------------------------
// Stub handler
// ---------------------------------------------------------------------------

export async function stub(request) {
  const url = request.url();

  // The wails vite plugin injects /wails/custom.js; under plain `vite` (no
  // wails dev backend) it 404s. Fulfill it with an empty script so the
  // pipeline doesn't flag a failed request.
  if (url.includes("/wails/custom.js")) {
    return { body: "", contentType: "application/javascript" };
  }

  if (!url.includes("/wails/runtime")) return null;

  let post;
  try {
    post = request.postDataJSON() ?? {};
  } catch {
    return null;
  }

  // Only backend method calls (object = 0, method = 0) carry a methodID.
  const methodID = post.args?.methodID;

  switch (methodID) {
    case M.GetStatus:
      return { body: status };
    case M.GetCPUCount:
      // Number of logical CPUs — the Settings thread slider's ceiling.
      return { body: 8 };
    case M.GetVersion:
      // The dev-stub middleware sends string bodies raw, so JSON-encode the
      // value — the runtime's res.json() needs `"v0.1.0"` (quoted).
      return { body: JSON.stringify("v0.1.0") };
    case M.ListCollections:
      return { body: collections };
    case M.ListSources: {
      const colId = post.args?.args?.[0];
      return { body: sources.filter((s) => s.collectionId === colId) };
    }
    case M.ListDocuments: {
      const [colId, srcId] = post.args?.args ?? [];
      return {
        body: documents.filter(
          (d) => (srcId ? d.sourceId === srcId : d.collectionId === colId),
        ),
      };
    }
    case M.Search:
      // Small artificial latency so the skeleton state is exercised, like the
      // real embedder + RRF pipeline.
      await new Promise((r) => setTimeout(r, 120));
      return { body: searchResults };
    case M.GetConfig:
      return { body: config };
    case M.GetMCPStatus:
      return { body: mcp };
    case M.StartServer: {
      const port = post.args?.args?.[0] ?? 31123;
      mcp = { running: true, port, url: `http://127.0.0.1:${port}/sse` };
      // String return: JSON-encode so the runtime's res.json() gets a quoted value.
      return { body: JSON.stringify(mcp.url) };
    }
    case M.StopServer:
      mcp = { ...mcp, running: false, url: "" };
      return { body: true };
    case M.GetIndexingState:
      // The screenshot stub is idle; the real backend reports an active run so
      // a freshly loaded frontend can rebuild the indexing UI after a reload.
      return { body: { active: false, all: false, collections: {} } };
    // Index runs return a bool (started or already-running). The dev-stub
    // simulates the indexing events client-side (see dev-stub.mjs), so the
    // middleware only needs to answer the boolean; screenshots never click
    // these buttons.
    case M.IndexCollection:
    case M.IndexAll:
    case M.CancelIndexing:
      return { body: true };
    case M.ListModels:
      return { body: models };
    case M.ImportModel: {
      const srcPath = post.args?.args?.[0];
      const base = String(srcPath).split(/[\\/]/).pop() ?? "model";
      const name = base.replace(/\.gguf$/i, "");
      const m = {
        id: models.length + 1,
        name,
        path: "C:\\Users\\you\\AppData\\Roaming\\vectile\\models\\" + base,
        dimensions: 1024,
        contextWindow: 2048,
        batchSize: 32,
        threads: 0,
        isActive: false,
        created: new Date().toISOString().slice(0, 10),
      };
      models.push(m);
      return { body: m };
    }
    case M.SetActiveModel: {
      const [path, force] = post.args?.args ?? [];
      const m = models.find((x) => x.path === path);
      if (!m) return { body: { needsRebuild: false, model: models[0] } };
      const prev = models.find((x) => x.isActive);
      const dimChange = prev && prev.dimensions !== m.dimensions;
      if (dimChange && !force) return { body: { needsRebuild: true, model: m } };
      models.forEach((x) => (x.isActive = x.path === path));
      config.active_model = path;
      status.modelName = m.name + (m.dimensions ? " · " + m.dimensions + "d" : "");
      return { body: { needsRebuild: false, model: m } };
    }
    case M.DeleteModel: {
      const path = post.args?.args?.[0];
      const idx = models.findIndex((x) => x.path === path);
      if (idx !== -1) models.splice(idx, 1);
      return { body: true };
    }
    case M.UpdateModelSettings: {
      const [id, contextWindow, batchSize, threads] = post.args?.args ?? [];
      const m = models.find((x) => x.id === id);
      if (m) {
        m.contextWindow = contextWindow;
        m.batchSize = batchSize;
        m.threads = threads;
      }
      return { body: true };
    }
    case M.ListRecommendedModels:
      return { body: recommendedModels };
    case M.GetDownloadState:
      return { body: downloadState };
    case M.DownloadModel: {
      const key = post.args?.args?.[0] ?? "";
      downloadState = { active: true, key, status: "downloading", downloaded: 0, total: 36700000, percent: 0, speed: 0, error: "" };
      return { body: true };
    }
    case M.CancelModelDownload:
      downloadState = { active: false, key: downloadState.key, status: "cancelled", downloaded: 0, total: 0, percent: 0, speed: 0, error: "" };
      return { body: true };
    case M.DeleteSource: {
      const sourceId = post.args?.args?.[0];
      const s = sources.find((x) => x.id === sourceId);
      if (!s) return { body: 0 };
      const removed = documents.filter((d) => d.sourceId === s.id).length;
      documents = documents.filter((d) => d.sourceId !== s.id);
      sources = sources.filter((x) => x.id !== sourceId);
      return { body: removed };
    }
    case M.DeleteDocuments: {
      const ids = new Set(post.args?.args?.[0] ?? []);
      if (ids.size === 0) return { body: 0 };
      const before = documents.length;
      documents = documents.filter((d) => !ids.has(d.id));
      return { body: before - documents.length };
    }
    case M.DeleteCollection: {
      const name = post.args?.args?.[0];
      const idx = collections.findIndex((c) => c.name === name);
      if (idx === -1) return { body: 0 };
      const coll = collections[idx];
      const removed = documents.filter((d) => d.collectionId === coll.id).length;
      documents = documents.filter((d) => d.collectionId !== coll.id);
      sources = sources.filter((s) => s.collectionId !== coll.id);
      collections.splice(idx, 1);
      // Mirror the backend: drop the config entry too so it doesn't come back.
      if (name === "obsidian") config.obsidian_vaults = [];
      else if (name === "calibre") config.calibre_libraries = [];
      else {
        delete config.projects[name];
        delete config.repositories[name];
      }
      return { body: removed };
    }
    default:
      // Harmless ack for anything else (void methods, unknown IDs).
      return { body: true };
  }
}
