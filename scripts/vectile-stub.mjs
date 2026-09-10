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
  ListSourcesPage: 757967130,
  ListDocumentsPage: 1199931038,
  GetDocument: 4208047110,
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
  GetCacheStats: 84102949,
  ClearCache: 1916537253,
};

const MODEL_NAME = "bge-m3";
const MODEL_PATH = "C:\\Users\\you\\AppData\\Roaming\\vectile\\models\\bge-m3-Q4_K_M.gguf";

// ---------------------------------------------------------------------------
// Status + config
// ---------------------------------------------------------------------------

const status = {
  collections: 4,
  sources: 20,
  chunks: 7029,
  dbSize: 20971520, // ~20 MB
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
  project_exclude_folders: ["node_modules", "references"],
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

// Query-vector cache state for the Settings Cache panel. ClearCache zeroes it,
// and repeating a search flips the "cached" chip on the results header.
let cacheStats = { entries: 128, bytes: 524288 };
let lastQuery = "";

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
  { id: 1, name: "calibre", type: "system", description: "ebook library — titles, authors, and text, chunked for search", sources: 2, chunks: 1130, created: "2025-11-02", enabled: true, lastIndexed: "2025-11-02" },
  { id: 2, name: "obsidian", type: "system", description: "Obsidian vault — daily notes, zettels, project journals", sources: 8, chunks: 812, created: "2025-06-30", enabled: true, lastIndexed: "2026-08-24" },
  { id: 3, name: "field-notes", type: "project", description: "project folder — runbooks, retrospectives, working notes", sources: 4, chunks: 256, created: "2026-05-18", enabled: true, lastIndexed: "2026-08-24" },
  { id: 4, name: "vectile", type: "code", description: "this repo — backend, frontend, and git history", sources: 6, chunks: 4831, created: "2026-08-24", enabled: true, lastIndexed: "2026-08-24" },
];

let sources = [
  // calibre
  { id: 11, collectionId: 1, sourceType: "epub", path: "C:\\Users\\you\\Calibre Library\\Bronson\\The Nudist on the Late Shift", chunks: 310, lastIndexed: "2025-11-02" },
  { id: 12, collectionId: 1, sourceType: "pdf", path: "C:\\Users\\you\\Calibre Library\\Luksa\\Kubernetes in Action", chunks: 820, lastIndexed: "2025-11-02" },
  // obsidian vault
  { id: 21, collectionId: 2, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\zettelkasten", chunks: 412, lastIndexed: "2026-08-24" },
  { id: 22, collectionId: 2, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\inbox", chunks: 57, lastIndexed: "2026-08-24" },
  { id: 23, collectionId: 2, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\projects\\k8s-upgrade", chunks: 96, lastIndexed: "2026-08-22" },
  { id: 24, collectionId: 2, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\camera", chunks: 88, lastIndexed: "2026-08-20" },
  { id: 25, collectionId: 2, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\cooking", chunks: 71, lastIndexed: "2026-08-18" },
  { id: 26, collectionId: 2, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\reading", chunks: 39, lastIndexed: "2026-08-15" },
  { id: 27, collectionId: 2, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\work", chunks: 26, lastIndexed: "2026-08-12" },
  { id: 28, collectionId: 2, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\notes\\travel", chunks: 23, lastIndexed: "2026-08-09" },
  // field-notes (project folder)
  { id: 31, collectionId: 3, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\field-notes\\deploy-runbook.md", chunks: 64, lastIndexed: "2026-08-24" },
  { id: 32, collectionId: 3, sourceType: "plaintext", path: "C:\\Users\\you\\Documents\\field-notes\\backup-checklist.txt", chunks: 18, lastIndexed: "2026-08-24" },
  { id: 33, collectionId: 3, sourceType: "markdown", path: "C:\\Users\\you\\Documents\\field-notes\\retro-2026-06.md", chunks: 33, lastIndexed: "2026-08-24" },
  { id: 34, collectionId: 3, sourceType: "pdf", path: "C:\\Users\\you\\Documents\\field-notes\\team-handbook.pdf", chunks: 141, lastIndexed: "2026-08-24" },
  // vectile (code)
  { id: 41, collectionId: 4, sourceType: "code", path: "C:\\Users\\you\\code\\vectile\\backend\\search", chunks: 402, lastIndexed: "2026-08-24" },
  { id: 42, collectionId: 4, sourceType: "code", path: "C:\\Users\\you\\code\\vectile\\backend\\indexer", chunks: 611, lastIndexed: "2026-08-24" },
  { id: 43, collectionId: 4, sourceType: "code", path: "C:\\Users\\you\\code\\vectile\\backend\\chunker", chunks: 318, lastIndexed: "2026-08-24" },
  { id: 44, collectionId: 4, sourceType: "code", path: "C:\\Users\\you\\code\\vectile\\frontend\\src", chunks: 502, lastIndexed: "2026-08-24" },
  { id: 45, collectionId: 4, sourceType: "code", path: "C:\\Users\\you\\code\\vectile\\third_party\\llama-go", chunks: 204, lastIndexed: "2026-08-24" },
  { id: 46, collectionId: 4, sourceType: "commit", path: "C:\\Users\\you\\code\\vectile\\.git", chunks: 2794, lastIndexed: "2026-08-24" },
];

const SAMPLE_DOCS = [
  // calibre — epub
  { id: 1111, sourceId: 11, collectionId: 1, chunkIndex: 0, title: "Prologue — the late shift", content: "The late shift in the Valley started quietly: a handful of engineers in a rented office, shipping while the rest of the industry slept. Nobody set out to make a culture of it. It just turned out that the work got done at night, and the morning was for arguing about what had been built.", metadata: { page: 3 } },
  { id: 1112, sourceId: 11, collectionId: 1, chunkIndex: 1, title: "Chapter 1 — two founders", content: "Both founders came from support desks. That was the whole trick, they said: they knew what the users typed when they were stuck. So the product was built from search logs, not from a vision deck.", metadata: { page: 17 } },
  { id: 1113, sourceId: 11, collectionId: 1, chunkIndex: 2, title: "Chapter 2 — growth without a plan", content: "Growth came from one feature that shipped three weeks before anyone asked for it. The team kept a rule: if a customer mentioned the same pain twice, it was already a spec.", metadata: { page: 41 } },
  // calibre — pdf
  { id: 1211, sourceId: 12, collectionId: 1, chunkIndex: 0, title: "Rolling updates and rollbacks", content: "A rolling update replaces the old ReplicaSet gradually. The Deployment controller keeps the service available the whole time, and you can pause, resume, or roll back from the rollout status.", metadata: { page: 214 } },
  { id: 1212, sourceId: 12, collectionId: 1, chunkIndex: 1, title: "Choosing a strategy", content: "Pick a strategy by blast radius, not by fashion. Rolling suits stateless services. Canary suits services with real traffic you can measure. Blue-green doubles your cost while the switch happens.", metadata: { page: 221 } },
  // obsidian — zettelkasten
  { id: 2101, sourceId: 21, collectionId: 2, chunkIndex: 0, title: "The exposure triangle, revisited", content: "Shutter, aperture, ISO are a triangle only in the sense that moving one forces the others to move. The real question is always: what do you want frozen, blurred, or clean at this light level?", metadata: { tags: ["photography"] } },
  { id: 2102, sourceId: 21, collectionId: 2, chunkIndex: 1, title: "Notes on quorum and Raft", content: "Raft needs a majority for both election and commit. That is the whole trick — you trade availability during partitions for a guarantee that two leaders never both think they own the term.", metadata: { tags: ["distributed-systems"] } },
  // obsidian — k8s-upgrade
  { id: 2301, sourceId: 23, collectionId: 2, chunkIndex: 0, title: "Rollout strategies — 2026-06-12", content: "Weighed rolling, blue-green, and canary for the k8s upgrade. Rolling wins on simplicity: one command, no extra infra, and rollout status gives a clean view of progress. Set maxUnavailable to 25% so we keep headroom during the drain.", metadata: { tags: ["kubernetes", "deploy"] } },
  { id: 2302, sourceId: 23, collectionId: 2, chunkIndex: 1, title: "Rolling update checklist", content: "Before you roll: backups done, migration run, feature flag off by default, metrics page open. During the rollout: watch rollout status, error rate, and p99. After: leave the flag on and delete the old ReplicaSet.", metadata: { tags: ["kubernetes"] } },
  { id: 2303, sourceId: 23, collectionId: 2, chunkIndex: 2, title: "Blue-green for the API", content: "If the API breaks again, use blue-green. Point traffic at green, run the old blue beside it, and keep the rollback one DNS flip away. Costs double while both stacks are up — fine for a weekend.", metadata: { tags: ["kubernetes"] } },
  // field-notes — deploy runbook
  { id: 3101, sourceId: 31, collectionId: 3, chunkIndex: 0, title: "Deploy runbook — k8s control-plane upgrade", content: "Sequence for the upgrade window. Confirm the nightly backup finished, snapshot the AMI, then roll the control plane first. Workers follow in batches of three, waiting for a clean rollout status and a flat error rate between each batch.", metadata: { tags: ["kubernetes", "ops"] } },
  { id: 3102, sourceId: 31, collectionId: 3, chunkIndex: 1, title: "Rollback rules", content: "If rollout status shows an unhealthy replica set, pause the rollout and look at the pod events before deciding. Roll back in the same order you rolled forward: last batch first, keep the old deployment intact until the incident is closed.", metadata: { tags: ["kubernetes"] } },
  // vectile — git history
  { id: 4601, sourceId: 46, collectionId: 4, chunkIndex: 0, title: "docs: rollout strategy notes", content: "Add notes weighing rolling, canary, and blue-green for the control-plane upgrade. Rolling chosen for the nodes, blue-green reserved for the API. No code changes in this commit.", metadata: { author: "Jesse" } },
];

// The hand-written chunks above carry the demo's reading-pane copy. Every other
// chunk is generated, so a collection really holds the number of chunks its row
// claims, which is what makes paging (and the window cap) real in the browser.
const FILLER_BY_TYPE = {
  epub: [
    "The chapter widens into a detour: how the team measured what they built, and what the numbers refused to say.",
    "A short aside on the people who wrote the tooling, and the meetings that decided it.",
  ],
  pdf: [
    "The section works through the configuration, then hands the reader a checklist and a caution.",
    "A worked example follows, with the caveats that usually get skipped in the summary.",
  ],
  markdown: [
    "Notes from the session: what we tried, what broke, and the one thing worth keeping.",
    "A working note, kept short on purpose so it stays true next month.",
  ],
  code: [
    "The block below is the shortest version of the same logic, with the guards the longer one hid.",
    "Edge cases first, then the happy path the tests cover.",
  ],
  plaintext: [
    "A line from the checklist, kept because it has been useful twice.",
    "Plain text on purpose: no structure to maintain, nothing to reformat.",
  ],
  commit: [
    "Commit message and a summary of the change, with the issue it closes.",
    "Part of a series: the mechanical half of the change, no behaviour yet.",
  ],
};

let nextDocId = 100000;

function generatedDoc(s, i) {
  const lines = FILLER_BY_TYPE[s.sourceType] ?? FILLER_BY_TYPE.plaintext;
  const name = s.path.split(/[\\/]/).pop() || s.path;
  return {
    id: nextDocId++,
    sourceId: s.id,
    collectionId: s.collectionId,
    chunkIndex: i,
    title: `${name} — ${i + 1}`,
    content: `Chunk ${i + 1} of ${s.chunks} from ${name}. ${lines[i % lines.length]}`,
    metadata: {},
  };
}

// Ordered by (sourceId, chunkIndex): the stream order the backend pages in.
let documents = [];
for (const s of sources) {
  const samples = new Map(
    SAMPLE_DOCS.filter((d) => d.sourceId === s.id).map((d) => [d.chunkIndex, d]),
  );
  for (let i = 0; i < s.chunks; i++) documents.push(samples.get(i) ?? generatedDoc(s, i));
}

// ---------------------------------------------------------------------------
// Keyset paging, mirroring backend/db/documents.go
// ---------------------------------------------------------------------------

const PAGE_SIZE = 100;

const cmpDocCursor = (a, b) => {
  const [as, ac] = String(a).split(":").map(Number);
  const [bs, bc] = String(b).split(":").map(Number);
  return as - bs || ac - bc;
};

const cmpPath = (a, b) => (a < b ? -1 : a > b ? 1 : 0);

/** One keyset page of an ascending list: the limit+1 probe, the cursors for the
    neighbouring pages, and "" on a side that has ended. */
function keysetPage(all, cursor, backward, cmp, keyOf, extract) {
  if (backward && cursor === "") return { items: [], before: "", after: "" };
  let rows = all;
  if (cursor !== "") {
    rows = all.filter((x) =>
      backward ? cmp(keyOf(x), cursor) < 0 : cmp(keyOf(x), cursor) > 0,
    );
  }
  const more = rows.length > PAGE_SIZE;
  const page = more ? (backward ? rows.slice(rows.length - PAGE_SIZE) : rows.slice(0, PAGE_SIZE)) : rows;
  const moreBefore = backward ? more : cursor !== "";
  const moreAfter = backward ? cursor !== "" : more;
  return {
    items: page.map(extract),
    before: moreBefore && page.length ? keyOf(page[0]) : "",
    after: moreAfter && page.length ? keyOf(page[page.length - 1]) : "",
  };
}

const docCursor = (d) => `${d.sourceId}:${d.chunkIndex}`;

function pageDocuments(collectionId, cursor, backward) {
  const all = documents
    .filter((d) => d.collectionId === collectionId)
    .sort((a, b) => a.sourceId - b.sourceId || a.chunkIndex - b.chunkIndex);
  const page = keysetPage(all, cursor, backward, cmpDocCursor, docCursor, (d) => {
    const s = sources.find((x) => x.id === d.sourceId);
    return {
      id: d.id,
      sourceId: d.sourceId,
      collectionId: d.collectionId,
      chunkIndex: d.chunkIndex,
      title: d.title,
      sourcePath: s?.path ?? "",
      sourceType: s?.sourceType ?? "",
    };
  });
  return { documents: page.items, before: page.before, after: page.after };
}

function pageSources(collectionId, cursor, backward) {
  const all = sources
    .filter((s) => s.collectionId === collectionId)
    .sort((a, b) => cmpPath(a.path, b.path));
  const page = keysetPage(all, cursor, backward, cmpPath, (s) => s.path, (s) => s);
  return { sources: page.items, before: page.before, after: page.after };
}

// ---------------------------------------------------------------------------
// Search results for the demo query ("kubernetes rollout")
// ---------------------------------------------------------------------------

const searchResults = [
  {
    title: "Rollout strategies for the k8s upgrade",
    content: "We weighed rolling, blue-green, and canary for the k8s upgrade. A rolling update won on simplicity: one command, no extra infra, and rollout status gives a clean way to watch progress. Set maxUnavailable to 25% and keep headroom during the node drain.",
    score: 0.92,
    collection: "obsidian",
    sourceType: "markdown",
    sourcePath: "C:\\Users\\you\\Documents\\notes\\projects\\k8s-upgrade\\rollout-strategies.md",
    metadata: { tags: ["kubernetes", "deploy"] },
  },
  {
    title: "Rolling update checklist",
    content: "Before you roll: backups done, migration run, feature flag off by default, metrics page open. During the rollout: watch rollout status, error rate, and p99. After: leave the flag on and delete the old ReplicaSet.",
    score: 0.83,
    collection: "obsidian",
    sourceType: "markdown",
    sourcePath: "C:\\Users\\you\\Documents\\notes\\projects\\k8s-upgrade\\checklist.md",
    metadata: { tags: ["kubernetes"] },
  },
  {
    title: "Kubernetes in Action — Rolling updates and rollbacks",
    content: "Rolling updates replace pods gradually, keeping the service available while the new version rolls out. Kubernetes tracks the rollout status and exposes it so you can verify before moving on.",
    score: 0.76,
    collection: "calibre",
    sourceType: "pdf",
    sourcePath: "C:\\Users\\you\\Calibre Library\\Luksa\\Kubernetes in Action\\Kubernetes in Action - Luksa.pdf",
    metadata: { page: 214, authors: ["Marko Lukša"] },
  },
  {
    title: "Deploy runbook — k8s control-plane upgrade",
    content: "Sequence for the upgrade window. Confirm the backup finished, snapshot the AMI, then roll the control plane first. Workers follow in batches of three, waiting for a clean rollout status and a flat error rate between each batch.",
    score: 0.68,
    collection: "field-notes",
    sourceType: "markdown",
    sourcePath: "C:\\Users\\you\\Documents\\field-notes\\deploy-runbook.md",
    metadata: { tags: ["kubernetes", "ops"] },
  },
  {
    title: "Blue-green for the API",
    content: "If the API breaks again, use blue-green. Point traffic at green, run the old blue beside it, and keep the rollback one DNS flip away. Costs double while both stacks are up, which is fine for a weekend.",
    score: 0.61,
    collection: "obsidian",
    sourceType: "markdown",
    sourcePath: "C:\\Users\\you\\Documents\\notes\\projects\\k8s-upgrade\\blue-green-api.md",
    metadata: { tags: ["kubernetes"] },
  },
  {
    title: "docs: rollout strategy notes",
    content: "Notes weighing rolling, canary, and blue-green for the control-plane upgrade. Rolling chosen for the nodes, blue-green reserved for the API. No code changes in this commit.",
    score: 0.52,
    collection: "vectile",
    sourceType: "commit",
    sourcePath: "C:\\Users\\you\\code\\vectile",
    metadata: { author: "Jesse" },
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
      // value — the runtime's res.json() needs `"v0.3.1"` (quoted).
      return { body: JSON.stringify("v0.3.1") };
    case M.ListCollections:
      return { body: collections };
    case M.ListSourcesPage: {
      const [colId, cursor, backward] = post.args?.args ?? [];
      return { body: pageSources(colId, cursor, backward) };
    }
    case M.ListDocumentsPage: {
      const [colId, cursor, backward] = post.args?.args ?? [];
      return { body: pageDocuments(colId, cursor, backward) };
    }
    case M.GetDocument: {
      const id = post.args?.args?.[0];
      return { body: documents.find((d) => d.id === id) ?? null };
    }
    case M.Search: {
      // Small artificial latency so the skeleton state is exercised, like the
      // real embedder + RRF pipeline. A repeated query reports a cache hit so
      // the results header's "cached" chip is reachable in the browser.
      const query = post.args?.args?.[0] ?? "";
      await new Promise((r) => setTimeout(r, 120));
      const cached = query !== "" && query === lastQuery;
      lastQuery = query;
      return { body: { results: searchResults, cached } };
    }
    case M.GetConfig:
      return { body: config };
    case M.GetCacheStats:
      return { body: cacheStats };
    case M.ClearCache: {
      const removed = cacheStats.entries;
      cacheStats = { entries: 0, bytes: 0 };
      return { body: removed };
    }
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
