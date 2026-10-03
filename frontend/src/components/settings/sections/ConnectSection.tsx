import { createSignal, For, Show } from "solid-js";
import { useAppStore } from "../../../lib/store";
import type { MCPTransport } from "../../../lib/types";
import { CheckIcon, CopyIcon, PlugIcon } from "../../ui/icons";
import { InfoTip, Toggle } from "../../ui/primitives";
import { FieldList, NumField, Section, SubHeading } from "../fields";
import { useSettings } from "../context";
import { STATIC_BOUNDS } from "../bounds";

const copyText = async (text: string): Promise<boolean> => {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(text);
      return true;
    }
  } catch {
    /* fall through to the textarea fallback */
  }
  try {
    const ta = document.createElement("textarea");
    ta.value = text;
    ta.style.position = "fixed";
    ta.style.opacity = "0";
    document.body.appendChild(ta);
    ta.select();
    const ok = document.execCommand("copy");
    document.body.removeChild(ta);
    return ok;
  } catch {
    return false;
  }
};

/* ---- MCP client matrix: what to paste, and where it goes ---- */

/**
 * One connectable MCP client. The payload is the only per-client difference
 * worth showing; `where` carries the part a user cannot guess (which file, or
 * which panel names a remote server).
 */
type MCPClient = {
  id: string;
  /** Tab label. */
  label: string;
  /** Panel heading when the tab label alone would be ambiguous. */
  title?: string;
  /** Where the setup goes, in plain words. */
  where: string;
  /** The exact text to copy. Varies by transport where the client says so. */
  payload: (url: string, transport: MCPTransport) => string;
  /** An extra step the client needs before it works. */
  then?: string;
};

/** The name those clients use for each transport in their own config. */
const clientTransportName = (t: MCPTransport) => (t === "sse" ? "sse" : "http");

const MCP_CLIENTS: MCPClient[] = [
  {
    id: "claude-desktop",
    label: "Claude Desktop",
    where: "Add this to claude_desktop_config.json.",
    payload: (url) => `{\n  "mcpServers": {\n    "vectile": {\n      "url": "${url}"\n    }\n  }\n}`,
    then: "Edit it from Settings → Developer → Edit Config, then quit Claude completely and reopen it.",
  },
  {
    id: "cursor",
    label: "Cursor",
    where: "Add this to ~/.cursor/mcp.json for every project, or .cursor/mcp.json for one.",
    payload: (url) => `{\n  "mcpServers": {\n    "vectile": {\n      "url": "${url}"\n    }\n  }\n}`,
  },
  {
    id: "windsurf",
    label: "Windsurf",
    where: "Add this to mcp_config.json.",
    payload: (url) => `{\n  "mcpServers": {\n    "vectile": {\n      "serverUrl": "${url}"\n    }\n  }\n}`,
    then: "Windsurf also takes this from Settings → Cascade → MCP Servers → Add remote server.",
  },
  {
    id: "vscode",
    label: "VS Code",
    title: "VS Code / GitHub Copilot",
    where: "Add this to .vscode/mcp.json for one workspace, or to your user mcp.json for every workspace.",
    payload: (url, t) =>
      `{\n  "servers": {\n    "vectile": {\n      "type": "${clientTransportName(t)}",\n      "url": "${url}"\n    }\n  }\n}`,
    then: "In VS Code the wrapper key is servers, not mcpServers, and it needs the type field.",
  },
  {
    id: "cline",
    label: "Cline",
    where: "Open the MCP Servers panel in Cline and add a remote server with this URL.",
    payload: (url) => url,
  },
  {
    id: "claude-code",
    label: "Claude Code",
    where: "Run this in your terminal.",
    payload: (url, t) => `claude mcp add vectile --transport ${clientTransportName(t)} ${url}`,
  },
  {
    id: "other",
    label: "Other",
    where: "Use this URL with any MCP client that supports Streamable HTTP or SSE.",
    payload: (url) => url,
  },
];

/**
 * The two MCP transports. Streamable HTTP is what current clients probe first;
 * SSE is kept for assistants that only speak the older one.
 */
const MCP_TRANSPORTS: { id: MCPTransport; label: string; path: string; desc: string }[] = [
  {
    id: "streamable-http",
    label: "Streamable HTTP",
    path: "/mcp",
    desc: "The current MCP transport, and what most assistants expect. Recommended.",
  },
  {
    id: "sse",
    label: "SSE",
    path: "/sse",
    desc: "The older transport. Pick this only if your assistant cannot use Streamable HTTP.",
  },
];

/** Transport picker. Arrow keys, Home, and End move between options, per the ARIA radio-group pattern. */
function TransportTabs(props: { value: MCPTransport; onChange: (id: MCPTransport) => void }) {
  let els: HTMLButtonElement[] = [];
  const onKey = (e: KeyboardEvent, i: number) => {
    const n = MCP_TRANSPORTS.length;
    let next = -1;
    if (e.key === "ArrowRight") next = (i + 1) % n;
    else if (e.key === "ArrowLeft") next = (i - 1 + n) % n;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = n - 1;
    if (next < 0) return;
    e.preventDefault();
    props.onChange(MCP_TRANSPORTS[next].id);
    els[next]?.focus();
  };
  return (
    <div
      role="radiogroup"
      aria-label="MCP transport"
      class="grid grid-cols-1 gap-1 rounded-control border border-line-control bg-surface p-1 min-[430px]:grid-cols-2"
    >
      <For each={MCP_TRANSPORTS}>
        {(t, i) => {
          const active = () => props.value === t.id;
          return (
            <button
              ref={(el) => (els[i()] = el)}
              type="button"
              role="radio"
              aria-checked={active()}
              tabindex={active() ? 0 : -1}
              onClick={() => props.onChange(t.id)}
              onKeyDown={(e) => onKey(e, i())}
              class={`flex min-w-0 items-center gap-2 rounded-[7px] px-2.5 py-2 text-[13px] font-medium transition-colors duration-150 ease-snappy ${
                active() ? "bg-indigo text-white" : "text-ink-soft hover:bg-surface-2 hover:text-ink"
              }`}
            >
              <span class="min-w-0 flex-1 truncate text-left">{t.label}</span>
              <span
                class={`data shrink-0 rounded-full px-1.5 py-0.5 font-mono text-[11px] ${
                  active() ? "bg-surface text-ink" : "bg-paper text-muted"
                }`}
              >
                {t.path}
              </span>
            </button>
          );
        }}
      </For>
    </div>
  );
}

/** Client picker. Arrow keys, Home and End move between tabs, per the ARIA tabs pattern. */
function ClientTabs(props: { value: string; onChange: (id: string) => void }) {
  let els: HTMLButtonElement[] = [];
  const onKey = (e: KeyboardEvent, i: number) => {
    const n = MCP_CLIENTS.length;
    let next = -1;
    if (e.key === "ArrowRight") next = (i + 1) % n;
    else if (e.key === "ArrowLeft") next = (i - 1 + n) % n;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = n - 1;
    if (next < 0) return;
    e.preventDefault();
    props.onChange(MCP_CLIENTS[next].id);
    els[next]?.focus();
  };
  return (
    <div
      role="tablist"
      aria-label="MCP client"
      class="flex flex-wrap items-center gap-x-0.5 border-b border-line px-2.5 pt-1.5"
    >
      <For each={MCP_CLIENTS}>
        {(c, i) => (
          <button
            ref={(el) => (els[i()] = el)}
            role="tab"
            id={`mcp-tab-${c.id}`}
            aria-selected={props.value === c.id}
            aria-controls="mcp-client-panel"
            tabindex={props.value === c.id ? 0 : -1}
            class={`-mb-px whitespace-nowrap border-b-2 px-2 py-2 text-[12px] outline-offset-2 transition-colors focus-visible:outline-2 focus-visible:outline-leaf-deep ${
              props.value === c.id
                ? "border-leaf-deep font-semibold text-leaf-deep"
                : "border-transparent text-muted hover:text-ink-soft"
            }`}
            onClick={() => props.onChange(c.id)}
            onKeyDown={(e) => onKey(e, i())}
          >
            {c.label}
          </button>
        )}
      </For>
    </div>
  );
}

/** The selected client's setup: what it is, where it goes, and the payload. */
function ClientSetup(props: { client: MCPClient; url: string; transport: MCPTransport }) {
  const [copied, setCopied] = createSignal(false);
  const payload = () => props.client.payload(props.url, props.transport);
  const copy = async () => {
    if (await copyText(payload())) {
      setCopied(true);
      setTimeout(() => setCopied(false), 1600);
    }
  };
  return (
    <div>
      <div class="flex items-start gap-3 px-4 pb-3 pt-3.5">
        <div class="min-w-0 flex-1">
          <p class="text-[13px] font-medium leading-5 text-ink">{props.client.title ?? props.client.label}</p>
          <p class="mt-0.5 text-[12.5px] leading-4 text-muted">{props.client.where}</p>
        </div>
        <button
          class="flex shrink-0 items-center gap-1.5 rounded-control border border-line-control px-2 py-1.5 text-[12px] text-ink-soft outline-offset-2 transition-colors hover:border-leaf hover:text-leaf-deep focus-visible:outline-2 focus-visible:outline-leaf-deep"
          onClick={() => void copy()}
          aria-label={`Copy setup for ${props.client.label}`}
          title="Copy"
        >
          {copied() ? <CheckIcon size={14} /> : <CopyIcon size={14} />}
          {copied() ? "copied" : "copy"}
        </button>
      </div>
      <pre class="select-text overflow-x-auto border-t border-line px-4 py-3 font-mono text-[12.5px] leading-[1.6] text-ink">
        <code>{payload()}</code>
      </pre>
      <Show when={props.client.then}>
        <p class="border-t border-line px-4 py-2.5 text-[12px] leading-4 text-muted">{props.client.then}</p>
      </Show>
      <p class="sr-only" role="status" aria-live="polite">
        {copied() ? `${props.client.label} setup copied` : ""}
      </p>
    </div>
  );
}

const MCP_TOOLS: { name: string; desc: string; kind: "read" | "write" }[] = [
  {
    name: "vectile_search",
    desc: "Hybrid semantic + keyword search, filterable by collection, source type, path, and date.",
    kind: "read",
  },
  {
    name: "vectile_list_collections",
    desc: "List your collections with file and chunk counts.",
    kind: "read",
  },
  {
    name: "vectile_collection_info",
    desc: "Details for one collection: counts, source types, and sample titles.",
    kind: "read",
  },
  {
    name: "vectile_index",
    desc: "Index one collection: Obsidian vault, Calibre library, or a project/repo group.",
    kind: "write",
  },
  {
    name: "vectile_prune",
    desc: "Remove stale entries for files or books that no longer exist.",
    kind: "write",
  },
];

export function ConnectSection() {
  const store = useAppStore();
  const { draft, setMCP } = useSettings();

  const running = () => store.mcpStatus()?.running ?? false;
  const mcpWriteAllowed = () => draft()?.mcp.allow_write ?? false;
  /** The draft's transport, and the path clients reach it on. */
  const mcpTransport = (): MCPTransport => draft()!.mcp.transport;
  const mcpPath = () => (mcpTransport() === "sse" ? "/sse" : "/mcp");
  const mcpUrl = () => `http://127.0.0.1:${draft()!.mcp.port}${mcpPath()}`;
  /** What the running server is actually speaking, which differs from the
      draft while a transport change is still unsaved. */
  const liveTransportLabel = () => (store.mcpStatus()?.transport === "sse" ? "SSE" : "Streamable HTTP");

  /* The plate answers for the draft as well as the saved server, so flipping the
     switch below it can never leave the plate stating the opposite. */
  const mcpPending = () => !!draft()?.mcp.enabled && !running() && store.settingsDirty();
  const mcpStopping = () => !draft()?.mcp.enabled && running() && store.settingsDirty();
  const mcpStateLabel = () =>
    mcpStopping()
      ? "stops when you save"
      : running()
        ? "running"
        : mcpPending()
          ? "starts when you save"
          : "stopped";
  const mcpDot = () =>
    running() && !mcpStopping() ? "bg-indigo" : mcpPending() || mcpStopping() ? "bg-amber" : "bg-faint";
  const mcpStateText = () =>
    running() && !mcpStopping()
      ? "text-indigo-deep"
      : mcpPending() || mcpStopping()
        ? "text-amber-deep"
        : "text-muted";
  const mcpStateNote = () => {
    if (mcpStopping()) return "Still running. Save settings to stop the server.";
    if (running()) return "AI tools on this machine can connect now.";
    if (mcpPending()) return "Switched on. Save settings to start the server.";
    if (!draft()?.mcp.enabled) return "Turn on Share your library below, then save settings.";
    return "Not running. Save settings to start it again.";
  };
  /** The URL a client can actually reach: the live server's while it runs, the
      draft's while nothing is running, so editing the port never hands out a URL
      that is not answering yet. */
  const mcpConnectUrl = () => (running() ? store.mcpStatus()?.url || mcpUrl() : mcpUrl());
  /** The transport a client can actually reach, kept in step with
      mcpConnectUrl so a copied snippet never names a transport the URL beside
      it is not serving yet. */
  const mcpConnectTransport = (): MCPTransport =>
    running() ? (store.mcpStatus()?.transport ?? mcpTransport()) : mcpTransport();
  /** What the server would expose right now, so the size of the decision is visible. */
  const mcpScope = () => {
    const s = store.status();
    const chunks = s?.chunks ?? 0;
    if (chunks === 0) return "Nothing indexed yet, so there is nothing to share";
    const readable = mcpWriteAllowed() ? "readable and writable" : "readable";
    return `${s?.collections ?? 0} collections · ${chunks.toLocaleString()} chunks ${readable}`;
  };
  const [mcpClient, setMcpClient] = createSignal(MCP_CLIENTS[0].id);
  const activeClient = () => MCP_CLIENTS.find((c) => c.id === mcpClient()) ?? MCP_CLIENTS[0];
  const [urlCopied, setUrlCopied] = createSignal(false);
  const copyUrl = async () => {
    if (await copyText(mcpConnectUrl())) {
      setUrlCopied(true);
      setTimeout(() => setUrlCopied(false), 1600);
    }
  };

  return (
    <Section
      icon={<PlugIcon size={16} />}
      title="Connect"
      note="Let AI assistants on this machine search your library."
    >
      <div class="space-y-6">
        <div class="rounded-control border border-line bg-paper-warm px-4 py-3.5">
          <div class="flex items-center gap-2">
            <span class="relative flex h-2 w-2 shrink-0">
              <span class={`h-2 w-2 rounded-full ${mcpDot()}`} />
            </span>
            <span class={`text-[12px] font-semibold leading-none ${mcpStateText()}`}>
              {mcpStateLabel()}
            </span>
            <Show when={running()}>
              <span class="data shrink-0 text-[11px] text-muted">{liveTransportLabel()}</span>
            </Show>
            <Show when={running()}>
              <button
                class="ml-auto -mr-1 flex h-6 shrink-0 items-center gap-1 rounded-control px-1.5 text-[11.5px] text-muted outline-offset-2 transition-colors hover:bg-surface-2 hover:text-indigo focus-visible:outline-2 focus-visible:outline-leaf-deep"
                onClick={() => void copyUrl()}
                aria-label="Copy server URL"
                title="Copy URL"
              >
                {urlCopied() ? <CheckIcon size={13} /> : <CopyIcon size={13} />}
                {urlCopied() ? "copied" : "copy URL"}
              </button>
            </Show>
          </div>
          <Show when={running()}>
            <p class="data mt-2 truncate text-ink-soft" title={mcpConnectUrl()}>
              {mcpConnectUrl()}
            </p>
          </Show>
          <p class="mt-2 text-[12px] leading-4 text-muted">{mcpStateNote()}</p>
          <p class="mt-2.5 text-[12px] leading-4 text-ink-soft">
            {mcpScope()}. Binds to 127.0.0.1, so nothing leaves this machine.
          </p>
        </div>

        <FieldList>
          <Toggle
            checked={draft()!.mcp.enabled}
            onChange={(v) => setMCP({ enabled: v })}
            label="Share your library with AI tools"
            description="Serve search tools over MCP on 127.0.0.1."
            hint="Starts a local MCP server that AI assistants on this machine can connect to. Applies when you save settings. The server answers only on your machine."
          />
          <Show when={draft()!.mcp.enabled}>
            <div class="py-3.5">
              <p class="mb-2 flex items-center gap-1.5 text-[13.5px] text-ink-soft">
                Transport
                <InfoTip text="How AI clients talk to the server. Streamable HTTP is the current standard and what most assistants probe first; SSE is the older transport. Applies when you save." />
              </p>
              <TransportTabs
                value={mcpTransport()}
                onChange={(t) => setMCP({ transport: t })}
              />
              <p class="note mt-2 text-[12.5px] leading-4 text-muted">
                {MCP_TRANSPORTS.find((t) => t.id === mcpTransport())?.desc}
              </p>
            </div>
            <NumField
              label="Port"
              value={draft()!.mcp.port}
              onChange={(n) => setMCP({ port: n })}
              hint={`The port the MCP server listens on. Clients connect to http://127.0.0.1:<port>${mcpPath()}. Applies when you save.`}
              min={STATIC_BOUNDS.mcp_port.min}
              max={STATIC_BOUNDS.mcp_port.max}
              step={STATIC_BOUNDS.mcp_port.step}
            />
            <Toggle
              checked={draft()!.mcp.allow_write}
              onChange={(v) => setMCP({ allow_write: v })}
              label="Allow write tools"
              description="Let AI tools index and prune your library."
              hint="Off by default. When on, vectile_index and vectile_prune become callable. The server still binds to 127.0.0.1 only."
            />
          </Show>
        </FieldList>

        <div>
          <SubHeading>What your AI can do</SubHeading>
          <p class="note mb-2 mt-0.5 text-[12.5px] leading-4 text-muted">
            {mcpWriteAllowed()
              ? "Search, plus index and prune, scoped to your library."
              : "Read-only search now. Turn on Allow write tools to let an AI index and prune."}
          </p>
          <ul class="divide-y divide-line overflow-hidden rounded-control border border-line bg-paper-warm pb-1.5">
            <For each={MCP_TOOLS}>
              {(t) => (
                <li class="flex items-start gap-3 px-3 py-2">
                  <span class="data mt-px shrink-0 text-[11.5px] text-ink-soft">{t.name}</span>
                  <span class="text-[12.5px] leading-5 text-muted">{t.desc}</span>
                  <Show when={t.kind === "write"}>
                    <span
                      class={`ml-auto shrink-0 rounded-full px-2 py-0.5 font-mono text-[11px] ${
                        mcpWriteAllowed() ? "bg-amber-soft text-amber-deep" : "bg-surface-2 text-muted"
                      }`}
                    >
                      write
                    </span>
                  </Show>
                </li>
              )}
            </For>
          </ul>
        </div>

        <div>
          <SubHeading>How to connect</SubHeading>
          <p class="note mb-2.5 mt-0.5 text-[12.5px] leading-4 text-muted">
            Pick the app you're connecting, then paste the setup into it.
          </p>
          <Show when={mcpTransport() === "streamable-http"}>
            <p class="note mb-2.5 text-[12.5px] leading-4 text-muted">
              Set vectile up before? Paste the setup again once. The address changed
              from /sse to /mcp.
            </p>
          </Show>
          <div class="overflow-hidden rounded-control border border-line bg-surface">
            <ClientTabs value={mcpClient()} onChange={setMcpClient} />
            <div
              role="tabpanel"
              id="mcp-client-panel"
              aria-labelledby={`mcp-tab-${mcpClient()}`}
              tabindex={0}
              class="-outline-offset-2 focus-visible:outline-2 focus-visible:outline-leaf-deep"
            >
              <ClientSetup
                client={activeClient()}
                url={mcpConnectUrl()}
                transport={mcpConnectTransport()}
              />
            </div>
          </div>
        </div>
      </div>
    </Section>
  );
}
