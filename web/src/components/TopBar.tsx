import { GearIcon, GridIcon, ListIcon, LogoutIcon, MoonIcon, RefreshIcon, SearchIcon, SunIcon } from "./icons";
import { searchShortcutLabel, type LayoutPref } from "../lib/prefs";

interface Props {
  title: string;
  subtitle?: string;
  theme: "dark" | "light";
  layout: LayoutPref;
  authEnabled: boolean;
  refreshing: boolean;
  onSearch: () => void;
  onToggleTheme: () => void;
  onToggleLayout: () => void;
  onOpenSettings: () => void;
  onRefresh: () => void;
  onLogout: () => void;
}

function IconButton({
  title,
  onClick,
  children,
  spinning = false,
}: {
  title: string;
  onClick: () => void;
  children: React.ReactNode;
  spinning?: boolean;
}) {
  return (
    <button
      type="button"
      title={title}
      aria-label={title}
      onClick={onClick}
      className="rounded-lg border border-line bg-surface p-2 text-ink-2 transition-colors outline-none hover:border-line-strong hover:text-ink focus-visible:ring-2 focus-visible:ring-accent"
    >
      <span className={spinning ? "block animate-spin" : "block"}>{children}</span>
    </button>
  );
}

export function TopBar(p: Props) {
  return (
    <header className="sticky top-0 z-30 border-b border-line bg-bg/90 backdrop-blur-sm">
      <div className="mx-auto flex max-w-[1400px] items-center gap-3 px-4 py-3 sm:px-6">
        <div className="flex min-w-0 shrink-0 items-center gap-2.5">
          <img src="/favicon.svg" alt="" width={26} height={26} className="rounded-[6px]" />
          <div className="hidden min-w-0 leading-tight min-[560px]:block">
            <p className="truncate font-semibold text-ink">{p.title}</p>
            {p.subtitle && <p className="truncate text-[12px] text-ink-3">{p.subtitle}</p>}
          </div>
        </div>

        <button
          type="button"
          onClick={p.onSearch}
          className="mx-auto flex w-full min-w-0 max-w-md flex-1 items-center gap-2 rounded-lg border border-line bg-surface px-3 py-2 text-left text-[13px] text-ink-3 transition-colors hover:border-line-strong hover:text-ink-2"
        >
          <SearchIcon size={15} />
          <span className="flex-1 truncate sm:hidden">Search…</span>
          <span className="hidden flex-1 truncate sm:inline">Search services, tags, the web…</span>
          <kbd className="hidden rounded border border-line bg-surface-2 px-1.5 font-mono text-[10px] text-ink-2 sm:block">{searchShortcutLabel()}</kbd>
        </button>

        <div className="flex shrink-0 items-center gap-2">
          <IconButton title="Refresh data" onClick={p.onRefresh} spinning={p.refreshing}>
            <RefreshIcon size={16} />
          </IconButton>
          <span className="hidden sm:block">
            <IconButton title={p.layout === "grid" ? "List layout" : "Grid layout"} onClick={p.onToggleLayout}>
              {p.layout === "grid" ? <ListIcon size={16} /> : <GridIcon size={16} />}
            </IconButton>
          </span>
          <IconButton title={p.theme === "dark" ? "Light theme" : "Dark theme"} onClick={p.onToggleTheme}>
            {p.theme === "dark" ? <SunIcon size={16} /> : <MoonIcon size={16} />}
          </IconButton>
          <IconButton title="Settings" onClick={p.onOpenSettings}>
            <GearIcon size={16} />
          </IconButton>
          {p.authEnabled && (
            <IconButton title="Sign out" onClick={p.onLogout}>
              <LogoutIcon size={16} />
            </IconButton>
          )}
        </div>
      </div>
    </header>
  );
}
