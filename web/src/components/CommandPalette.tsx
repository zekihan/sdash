import { useEffect, useMemo, useRef, useState } from "react";
import type { ArrLookupResult, Service } from "../types";
import { api, ApiError } from "../lib/api";
import { useFocusTrap } from "../lib/useFocusTrap";
import { fuzzyAny } from "../lib/fuzzy";
import { hostOf } from "../lib/format";
import { GlobeIcon, SearchIcon } from "./icons";
import { StatusDot } from "./StatusBits";

export interface PaletteAction {
  id: string;
  label: string;
  hint?: string;
  run: () => void;
}

interface Props {
  open: boolean;
  services: Service[];
  groupTitles: Record<string, string>;
  searchEngine: string;
  searchEngineName: string;
  actions: PaletteAction[];
  actionsEnabled: boolean;
  onClose: () => void;
  onOpenService: (service: Service) => void;
  onRecord: (service: Service) => void;
  onToast: (message: string) => void;
}

type Mode = "root" | "radarr" | "sonarr";

type Item =
  | { kind: "service"; service: Service }
  | { kind: "action"; action: PaletteAction }
  | { kind: "web"; query: string }
  | { kind: "magnet"; magnet: string }
  | { kind: "mode"; arrType: Exclude<Mode, "root">; label: string }
  | { kind: "lookup"; result: ArrLookupResult };

const MAX_RESULTS = 14;

export function CommandPalette(p: Props) {
  const [query, setQuery] = useState("");
  const [index, setIndex] = useState(0);
  const [mode, setMode] = useState<Mode>("root");
  const [lookupResults, setLookupResults] = useState<ArrLookupResult[]>([]);
  const [lookupState, setLookupState] = useState<"idle" | "loading" | "ready" | "empty" | "error">("idle");
  const [busy, setBusy] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const dialogRef = useRef<HTMLDivElement>(null);
  useFocusTrap(dialogRef, p.open, p.onClose, () => {
    if (mode !== "root") {
      setMode("root");
      setQuery("");
      setLookupResults([]);
    } else {
      p.onClose();
    }
  });

  useEffect(() => {
    if (p.open) {
      setBusy(false);
      setQuery("");
      setIndex(0);
      setMode("root");
      setLookupResults([]);
      requestAnimationFrame(() => inputRef.current?.focus());
    }
  }, [p.open]);

  // Debounced *arr lookup while in add mode. Stale rows are cleared as soon
  // as the query changes; loading/empty/error render distinctly.
  useEffect(() => {
    if (mode === "root") return;
    const term = query.trim();
    setLookupResults([]);
    if (term.length < 2) {
      setLookupState("idle");
      return;
    }
    setLookupState("loading");
    let cancelled = false;
    const timer = window.setTimeout(() => {
      api
        .arrLookup(mode, term)
        .then((results) => {
          if (cancelled) return;
          setLookupResults(results);
          setLookupState(results.length > 0 ? "ready" : "empty");
        })
        .catch(() => {
          if (!cancelled) setLookupState("error");
        });
    }, 300);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [mode, query]);

  const items = useMemo<Item[]>(() => {
    if (mode !== "root") {
      return lookupResults.map((result): Item => ({ kind: "lookup", result }));
    }

    const q = query.trim();
    const out: Item[] = [];

    if (q.startsWith("magnet:?")) {
      out.push({ kind: "magnet", magnet: q });
      return out;
    }

    const scored = p.services
      .map((service) => ({
        service,
        score: q
          ? fuzzyAny(q, [service.title, service.key, service.description, service.location, service.cluster, ...service.tags, p.groupTitles[service.group]])
          : 1,
      }))
      .filter((entry) => entry.score >= 0)
      .sort((a, b) => b.score - a.score)
      .slice(0, MAX_RESULTS)
      .map((entry): Item => ({ kind: "service", service: entry.service }));

    const actions: Item[] = p.actions
      .filter((action) => !q || fuzzyAny(q, [action.label]) >= 0)
      .map((action): Item => ({ kind: "action", action }));

    if (p.actionsEnabled) {
      const modeItems: Extract<Item, { kind: "mode" }>[] = [
        { kind: "mode", arrType: "radarr", label: "Add movie (Radarr)…" },
        { kind: "mode", arrType: "sonarr", label: "Add show (Sonarr)…" },
      ];
      const modes: Item[] = modeItems.filter((item) => !q || fuzzyAny(q, [item.label]) >= 0);
      actions.push(...modes);
    }

    out.push(...scored, ...actions);
    if (q) out.push({ kind: "web", query: q });
    return out;
  }, [mode, query, lookupResults, p.services, p.actions, p.actionsEnabled, p.groupTitles]);

  useEffect(() => setIndex(0), [query, mode]);

  useEffect(() => {
    listRef.current
      ?.querySelector(`[data-index="${index}"]`)
      ?.scrollIntoView({ block: "nearest" });
  }, [index]);

  if (!p.open) return null;

  const fail = (err: unknown) => {
    p.onToast(err instanceof ApiError ? err.message : "Action failed");
    setBusy(false);
  };

  const succeed = (message: string) => {
    setBusy(false);
    p.onToast(message);
    p.onClose();
  };

  const activate = (item: Item | undefined, viaKeyboard = false) => {
    if (!item || busy) return;
    if (item.kind === "service") {
      p.onRecord(item.service);
      p.onClose();
      if (viaKeyboard) p.onOpenService(item.service);
    } else if (item.kind === "action") {
      item.action.run();
      p.onClose();
    } else if (item.kind === "mode") {
      setMode(item.arrType);
      setQuery("");
      setLookupResults([]);
      requestAnimationFrame(() => inputRef.current?.focus());
    } else if (item.kind === "magnet") {
      if (!window.confirm("Add this magnet link to qBittorrent?")) return;
      setBusy(true);
      api
        .action("/api/actions/qbittorrent-add", { magnet: item.magnet })
        .then(() => succeed("Torrent added"))
        .catch(fail);
    } else if (item.kind === "lookup") {
      const result = item.result;
      if (result.added) {
        p.onToast(`“${result.title}” is already in your library`);
        return;
      }
      if (!window.confirm(`Add “${result.title}” (${result.year}) to ${mode === "radarr" ? "Radarr" : "Sonarr"} and search now?`)) return;
      setBusy(true);
      api
        .action("/api/actions/arr-add", { type: mode, externalId: result.externalId, title: result.title })
        .then(() => succeed(`Added “${result.title}”`))
        .catch(fail);
    } else {
      window.open(p.searchEngine.replace("%s", encodeURIComponent(item.query)), "_blank", "noopener");
      p.onClose();
    }
  };

  const placeholder =
    mode === "radarr"
      ? "Search movies to add…"
      : mode === "sonarr"
        ? "Search shows to add…"
        : "Search services, or paste a magnet link…";

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center bg-black/55 px-4 pt-[12vh]"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) p.onClose();
      }}
      role="dialog"
      aria-modal="true"
      aria-label="Command palette"
    >
      <div ref={dialogRef} tabIndex={-1} className="rise w-full max-w-xl overflow-hidden rounded-xl border border-line-strong bg-surface shadow-2xl outline-none">
        <div className="flex items-center gap-2.5 border-b border-line px-4">
          <SearchIcon size={17} className="shrink-0 text-ink-3" />
          {mode !== "root" && (
            <span className="shrink-0 rounded-full bg-accent-soft px-2 py-0.5 font-mono text-[10px] text-ink">
              add to {mode}
            </span>
          )}
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown") {
                e.preventDefault();
                setIndex((i) => Math.min(items.length - 1, i + 1));
              } else if (e.key === "ArrowUp") {
                e.preventDefault();
                setIndex((i) => Math.max(0, i - 1));
              } else if (e.key === "Enter") {
                e.preventDefault();
                activate(items[index], true);
              }
              // Escape is owned by the focus trap's onEscape (back in add
              // mode, close otherwise) — handling it here too would fire
              // both and close the palette from add mode.
            }}
            placeholder={placeholder}
            className="w-full bg-transparent py-3.5 text-[15px] text-ink outline-none placeholder:text-ink-3"
            aria-label="Search"
            role="combobox"
            aria-expanded={items.length > 0}
            aria-controls="palette-results"
            aria-activedescendant={items[index] ? `palette-item-${index}` : undefined}
            aria-autocomplete="list"
          />
          <kbd className="shrink-0 rounded border border-line bg-surface-2 px-1.5 font-mono text-[10px] text-ink-2">esc</kbd>
        </div>

        <div ref={listRef} id="palette-results" role="listbox" className="max-h-[46vh] overflow-y-auto p-1.5">
          {items.length === 0 && (
            <p className="px-3 py-6 text-center text-[13px] text-ink-3">
              {mode === "root"
                ? "No matches"
                : lookupState === "loading"
                  ? "Searching…"
                  : lookupState === "error"
                    ? `Could not reach ${mode === "radarr" ? "Radarr" : "Sonarr"} — check it is configured`
                    : lookupState === "empty"
                      ? "No results"
                      : "Type at least 2 characters to search"}
            </p>
          )}
          {items.map((item, i) => {
            const active = i === index;
            const rowClass = `flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left ${active ? "bg-accent-soft" : ""}`;
            if (item.kind === "service") {
              const s = item.service;
              return (
                <a key={s.key} href={s.url} target={s.openInNewTab ? "_blank" : undefined} rel="noopener" data-index={i} id={`palette-item-${i}`} role="option" aria-selected={active} className={rowClass} onMouseEnter={() => setIndex(i)} onClick={() => activate(item)}>
                  {s.icon ? (
                    <img src={s.icon} alt="" width={22} height={22} className="shrink-0 rounded object-contain" />
                  ) : (
                    <span className="flex h-[22px] w-[22px] shrink-0 items-center justify-center rounded bg-surface-2 font-mono text-[10px] text-ink-2 ring-1 ring-line">
                      {s.title.slice(0, 2).toUpperCase()}
                    </span>
                  )}
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[14px] font-medium text-ink">{s.title}</span>
                    <span className="block truncate font-mono text-[11px] text-ink-3">
                      {p.groupTitles[s.group] ?? s.group} · {hostOf(s.url)}
                    </span>
                  </span>
                  {s.health && <StatusDot state={s.health.state} />}
                </a>
              );
            }
            if (item.kind === "action") {
              return (
                <button key={item.action.id} type="button" data-index={i} id={`palette-item-${i}`} role="option" aria-selected={active} className={rowClass} onMouseEnter={() => setIndex(i)} onClick={() => activate(item)}>
                  <span className="flex-1 text-[14px] text-ink">{item.action.label}</span>
                  {item.action.hint && <span className="font-mono text-[11px] text-ink-3">{item.action.hint}</span>}
                </button>
              );
            }
            if (item.kind === "mode") {
              return (
                <button key={`mode-${item.arrType}`} type="button" data-index={i} id={`palette-item-${i}`} role="option" aria-selected={active} className={rowClass} onMouseEnter={() => setIndex(i)} onClick={() => activate(item)}>
                  <span className="flex-1 text-[14px] text-ink">{item.label}</span>
                  <span className="font-mono text-[11px] text-ink-3">action</span>
                </button>
              );
            }
            if (item.kind === "magnet") {
              return (
                <button key="magnet" type="button" data-index={i} id={`palette-item-${i}`} role="option" aria-selected={active} className={rowClass} onMouseEnter={() => setIndex(i)} onClick={() => activate(item)}>
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[14px] font-medium text-ink">Add magnet to qBittorrent</span>
                    <span className="block truncate font-mono text-[11px] text-ink-3">{item.magnet.slice(0, 64)}…</span>
                  </span>
                </button>
              );
            }
            if (item.kind === "lookup") {
              const result = item.result;
              return (
                <button
                  key={`${result.externalId}-${result.title}`}
                  type="button"
                  data-index={i}
                  id={`palette-item-${i}`}
                  role="option"
                  aria-selected={active}
                  className={rowClass}
                  onMouseEnter={() => setIndex(i)}
                  onClick={() => activate(item)}
                >
                  {result.poster ? (
                    <img src={result.poster} alt="" width={26} height={39} className="h-[39px] w-[26px] shrink-0 rounded object-cover" />
                  ) : null}
                  <span className="min-w-0 flex-1">
                    <span className="block truncate text-[14px] font-medium text-ink">
                      {result.title} <span className="font-mono text-[11px] text-ink-3">{result.year || ""}</span>
                    </span>
                    <span className="line-clamp-2 block text-[12px] text-ink-3">{result.overview}</span>
                  </span>
                  {result.added && <span className="shrink-0 font-mono text-[10px] text-up">in library</span>}
                </button>
              );
            }
            return (
              <button key="web" type="button" data-index={i} id={`palette-item-${i}`} role="option" aria-selected={active} className={rowClass} onMouseEnter={() => setIndex(i)} onClick={() => activate(item)}>
                <GlobeIcon size={16} className="shrink-0 text-ink-3" />
                <span className="flex-1 truncate text-[14px] text-ink">
                  Search {p.searchEngineName} for “{item.query}”
                </span>
              </button>
            );
          })}
        </div>

        <div className="flex items-center gap-3 border-t border-line px-4 py-2 font-mono text-[10px] text-ink-3">
          <span>↑↓ navigate</span>
          <span>↵ {mode === "root" ? "open" : "add"}</span>
          <span>esc {mode === "root" ? "close" : "back"}</span>
          {busy && <span className="ml-auto">working…</span>}
        </div>
      </div>
    </div>
  );
}
