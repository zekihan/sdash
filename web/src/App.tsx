import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type {
  CalendarData,
  Dashboard,
  DockerInfo,
  CrowdSec,
  Gatus,
  Kubernetes,
  Me,
  MediaRailData as MediaRailType,
  Pbs,
  Proxmox,
  TrueNAS,
  Service,
  ServiceHealth,
  ServiceWidget,
  SystemInfo,
  Weather,
} from "./types";
import { ApiError, api, subscribeEvents } from "./lib/api";
import {
  applyTheme,
  loadPrefs,
  pushRecent,
  resolveTheme,
  sanitizePrefs,
  searchShortcutLabel,
  savePrefs,
  setPrefsErrorListener,
  type Prefs,
} from "./lib/prefs";
import { TopBar } from "./components/TopBar";
import { ClockBlock, DockerBlock, DockerList, ServicesBlock, SystemBlock, WeatherBlock } from "./components/Widgets";
import {
  CrowdSecBlock,
  EnvironBlock,
  GatusBlock,
  GatusDetail,
  KubernetesBlock,
  KubernetesDetail,
  PbsBlock,
  PbsDetail,
  ProxmoxBlock,
  ProxmoxDetail,
  TrueNASBlock,
  TrueNASDetail,
} from "./components/ClusterWidgets";
import { AttentionSection, buildAttentionItems } from "./components/Attention";
import { CalendarRail, MediaRails } from "./components/Media";
import { ServiceCard } from "./components/ServiceCard";
import { CommandPalette, type PaletteAction } from "./components/CommandPalette";
import { SettingsDrawer } from "./components/SettingsDrawer";
import { LoginScreen } from "./components/LoginScreen";

type LoadState = "loading" | "ready" | "unauthorized" | "error";

export default function App() {
  const [state, setState] = useState<LoadState>("loading");
  const [me, setMe] = useState<Me | null>(null);
  const [dashboard, setDashboard] = useState<Dashboard | null>(null);
  const [health, setHealth] = useState<Record<string, ServiceHealth>>({});
  const [system, setSystem] = useState<SystemInfo | null>(null);
  const [docker, setDocker] = useState<DockerInfo | null>(null);
  const [weather, setWeather] = useState<Weather | null>(null);
  const [kube, setKube] = useState<Kubernetes | null>(null);
  const [proxmox, setProxmox] = useState<Proxmox | null>(null);
  const [truenas, setTruenas] = useState<TrueNAS | null>(null);
  const [pbs, setPbs] = useState<Pbs | null>(null);
  const [crowdsec, setCrowdsec] = useState<CrowdSec | null>(null);
  const [gatusData, setGatusData] = useState<Gatus | null>(null);
  const [calendar, setCalendar] = useState<CalendarData | null>(null);
  const [mediaRail, setMediaRail] = useState<MediaRailType | null>(null);
  const [serviceWidgets, setServiceWidgets] = useState<Record<string, ServiceWidget>>({});
  const [prefs, setPrefs] = useState<Prefs>(() => loadPrefs());
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [settingsOpen, setSettingsOpen] = useState(false);
  const [dockerOpen, setDockerOpen] = useState(false);
  const [kubeOpen, setKubeOpen] = useState(false);
  const [proxmoxOpen, setProxmoxOpen] = useState(false);
  const [truenasOpen, setTruenasOpen] = useState(false);
  const [pbsOpen, setPbsOpen] = useState(false);
  const prefsSyncTimer = useRef<number | null>(null);
  const prefsSyncWarned = useRef(false);
  const [prefsSync, setPrefsSync] = useState(false);
  const [gatusOpen, setGatusOpen] = useState(false);
  const [groupFilter, setGroupFilter] = useState<string>("all");
  const [downOnly, setDownOnly] = useState(false);
  const [refreshing, setRefreshing] = useState(false);
  const [toast, setToast] = useState<string | null>(null);
  const toastTimer = useRef<number | null>(null);

  const showToast = useCallback((message: string) => {
    setToast(message);
    if (toastTimer.current) window.clearTimeout(toastTimer.current);
    toastTimer.current = window.setTimeout(() => setToast(null), 3200);
  }, []);

  useEffect(() => setPrefsErrorListener(showToast), [showToast]);

  const updatePrefs = useCallback(
    (next: Prefs) => {
      setPrefs(next);
      savePrefs(next);
      applyTheme(next);
      if (prefsSync) {
        if (prefsSyncTimer.current) window.clearTimeout(prefsSyncTimer.current);
        prefsSyncTimer.current = window.setTimeout(() => {
          api.putPreferences(next).catch(() => {
            if (!prefsSyncWarned.current) {
              prefsSyncWarned.current = true;
              showToast("Preferences saved locally only (server sync failed)");
            }
          });
        }, 600);
      }
    },
    [prefsSync, showToast],
  );

  const loadDashboard = useCallback(async () => {
    const data = await api.dashboard();
    setDashboard(data);
    const seeded: Record<string, ServiceHealth> = {};
    for (const group of data.groups) {
      for (const service of group.services) {
        if (service.health) seeded[service.key] = service.health;
      }
    }
    setHealth((prev) => ({ ...seeded, ...prev }));
    document.title = data.settings.title;
    return data;
  }, []);

  const loadWidgets = useCallback(async (features: Dashboard["features"]) => {
    const jobs: Promise<unknown>[] = [];
    if (features.system) jobs.push(api.system().then(setSystem).catch(() => undefined));
    if (features.docker) jobs.push(api.docker().then(setDocker).catch(() => undefined));
    if (features.weather) jobs.push(api.weather().then(setWeather).catch(() => undefined));
    if (features.kubernetes) jobs.push(api.kubernetes().then(setKube).catch(() => undefined));
    if (features.proxmox) jobs.push(api.proxmox().then(setProxmox).catch(() => undefined));
    if (features.truenas) jobs.push(api.truenas().then(setTruenas).catch(() => undefined));
    if (features.pbs) jobs.push(api.pbs().then(setPbs).catch(() => undefined));
    if (features.crowdsec) jobs.push(api.crowdsec().then(setCrowdsec).catch(() => undefined));
    if (features.gatus) jobs.push(api.gatus().then(setGatusData).catch(() => undefined));
    if (features.widgets) jobs.push(api.serviceWidgets().then(setServiceWidgets).catch(() => undefined));
    if (features.widgets) jobs.push(api.calendar().then(setCalendar).catch(() => undefined));
    if (features.widgets) jobs.push(api.mediaRail().then(setMediaRail).catch(() => undefined));
    await Promise.allSettled(jobs);
  }, []);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const meData = await api.me();
        if (cancelled) return;
        setMe(meData);
        if (meData.authEnabled && !meData.authenticated) {
          setState("unauthorized");
          return;
        }
        const data = await loadDashboard();
        if (cancelled) return;
        if (data.features.prefsSync) {
          try {
            const remote = await api.getPreferences();
            if (!cancelled && remote.enabled) {
              setPrefsSync(true);
              if (remote.prefs !== null && remote.prefs !== undefined) {
                const seeded = loadPrefs({
                  theme: data.settings.defaultTheme,
                  accent: data.settings.defaultAccent,
                  density: data.settings.defaultDensity,
                });
                const next = sanitizePrefs(remote.prefs, seeded);
                setPrefs(next);
                savePrefs(next);
                applyTheme(next);
                setState("ready");
                void loadWidgets(data.features);
                return;
              }
            }
          } catch {
            // Preference sync is best-effort; local prefs still apply.
          }
        }
        if (cancelled) return;
        setPrefs((current) => {
          const hasStored = localStorage.getItem("sdash:prefs:v1") !== null;
          const next = hasStored
            ? current
            : loadPrefs({
                theme: data.settings.defaultTheme,
                accent: data.settings.defaultAccent,
                density: data.settings.defaultDensity,
              });
          applyTheme(next);
          return next;
        });
        setState("ready");
        void loadWidgets(data.features);
      } catch (err) {
        if (cancelled) return;
        setState(err instanceof ApiError && err.status === 401 ? "unauthorized" : "error");
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [loadDashboard, loadWidgets]);

  // Live updates: health rounds and dashboard.yaml reloads over SSE.
  useEffect(() => {
    if (state !== "ready") return;
    return subscribeEvents((event) => {
      if (event.type === "health") setHealth(event.data);
      else loadDashboard().catch(() => undefined);
    });
  }, [state, loadDashboard]);

  // Poll widget data the SSE stream does not carry.
  useEffect(() => {
    if (state !== "ready" || !dashboard) return;
    const { features } = dashboard;
    const timers: number[] = [];
    if (features.system) timers.push(window.setInterval(() => api.system().then(setSystem).catch(() => undefined), 5000));
    if (features.docker) timers.push(window.setInterval(() => api.docker().then(setDocker).catch(() => undefined), 15000));
    if (features.kubernetes) timers.push(window.setInterval(() => api.kubernetes().then(setKube).catch(() => undefined), 20000));
    if (features.proxmox) timers.push(window.setInterval(() => api.proxmox().then(setProxmox).catch(() => undefined), 30000));
    if (features.truenas) timers.push(window.setInterval(() => api.truenas().then(setTruenas).catch(() => undefined), 60000));
    if (features.pbs) timers.push(window.setInterval(() => api.pbs().then(setPbs).catch(() => undefined), 60000));
    if (features.crowdsec) timers.push(window.setInterval(() => api.crowdsec().then(setCrowdsec).catch(() => undefined), 30000));
    if (features.gatus) timers.push(window.setInterval(() => api.gatus().then(setGatusData).catch(() => undefined), 30000));
    if (features.widgets) timers.push(window.setInterval(() => api.serviceWidgets().then(setServiceWidgets).catch(() => undefined), 30000));
    if (features.widgets) timers.push(window.setInterval(() => api.calendar().then(setCalendar).catch(() => undefined), 300000));
    if (features.widgets) timers.push(window.setInterval(() => api.mediaRail().then(setMediaRail).catch(() => undefined), 600000));
    if (features.weather) timers.push(window.setInterval(() => api.weather().then(setWeather).catch(() => undefined), 600000));
    return () => timers.forEach((t) => window.clearInterval(t));
  }, [state, dashboard]);

  // Follow OS theme changes while theme = system.
  useEffect(() => {
    if (prefs.theme !== "system") return;
    const media = window.matchMedia("(prefers-color-scheme: light)");
    const onChange = () => applyTheme(prefs);
    media.addEventListener("change", onChange);
    return () => media.removeEventListener("change", onChange);
  }, [prefs]);

  // Lock page scroll while a dialog is open.
  useEffect(() => {
    if (!paletteOpen && !settingsOpen) return;
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = previous;
    };
  }, [paletteOpen, settingsOpen]);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const typing = target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable;
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        setPaletteOpen((v) => !v);
      } else if (e.key === "/" && !typing) {
        e.preventDefault();
        setPaletteOpen(true);
      } else if (e.key === "Escape") {
        setSettingsOpen(false);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const services = useMemo<Service[]>(() => {
    if (!dashboard) return [];
    return dashboard.groups.flatMap((group) =>
      group.services.map((service) => ({ ...service, health: health[service.key] ?? service.health })),
    );
  }, [dashboard, health]);

  const byKey = useMemo(() => new Map(services.map((s) => [s.key, s])), [services]);
  const groupTitles = useMemo(
    () => Object.fromEntries((dashboard?.groups ?? []).map((g) => [g.key, g.title])),
    [dashboard],
  );

  const recordRecent = useCallback(
    (service: Service) => {
      updatePrefs({ ...prefs, recents: pushRecent(prefs.recents, service.key) });
    },
    [prefs, updatePrefs],
  );

  const openService = useCallback(
    (service: Service) => {
      recordRecent(service);
      if (service.openInNewTab) window.open(service.url, "_blank", "noopener");
      else window.location.href = service.url;
    },
    [recordRecent],
  );

  const showDownServices = useCallback(() => {
    setGroupFilter("all");
    setDownOnly(true);
    requestAnimationFrame(() => {
      document.getElementById("service-filters")?.scrollIntoView({ behavior: "smooth", block: "start" });
    });
  }, []);

  const togglePin = useCallback(
    (key: string) => {
      const pinned = prefs.pinned.includes(key)
        ? prefs.pinned.filter((k) => k !== key)
        : [...prefs.pinned, key];
      updatePrefs({ ...prefs, pinned });
    },
    [prefs, updatePrefs],
  );

  const copyText = useCallback(
    async (text: string) => {
      try {
        await navigator.clipboard.writeText(text);
        showToast("Copied to clipboard");
      } catch {
        showToast("Copy failed — clipboard unavailable");
      }
    },
    [showToast],
  );

  const refresh = useCallback(async () => {
    setRefreshing(true);
    try {
      const data = await loadDashboard();
      await loadWidgets(data.features);
      const fresh = await api.health().catch(() => null);
      if (fresh) setHealth(fresh);
    } finally {
      setRefreshing(false);
    }
  }, [loadDashboard, loadWidgets]);

  const runAction = useCallback(
    async (confirmText: string | null, successText: string, call: () => Promise<void>, after?: () => void) => {
      if (confirmText && !window.confirm(confirmText)) return;
      try {
        await call();
        showToast(successText);
        if (after) after();
        else void refresh();
      } catch (err) {
        showToast(err instanceof ApiError ? err.message : "Action failed");
      }
    },
    [refresh, showToast],
  );

  const onArgoSync = useCallback(
    (app: string) =>
      void runAction(`Sync Argo CD application “${app}”?`, `Sync triggered for ${app}`, () =>
        api.action("/api/actions/argo-sync", { app }),
      ),
    [runAction],
  );

  const onRestart = useCallback(
    (namespace: string, name: string) =>
      void runAction(`Restart deployment ${namespace}/${name}?`, `Restarting ${name}`, () =>
        api.action("/api/actions/restart", { namespace, name }),
      ),
    [runAction],
  );

  const onStartGuest = useCallback(
    (node: string, kind: string, vmid: number, name: string) =>
      void runAction(`Start ${kind} ${vmid} (${name}) on ${node}?`, `Starting ${name}`, () =>
        api.action("/api/actions/proxmox-start", { node, kind, vmid }),
      ),
    [runAction],
  );

  const onWake = useCallback(
    (name: string) =>
      void runAction(`Send Wake-on-LAN to ${name}?`, `Wake packet sent to ${name}`, () =>
        api.action("/api/actions/wol", { name }),
      ),
    [runAction],
  );

  const onToggleControl = useCallback(
    (entityId: string, label: string) =>
      void runAction(
        null,
        `Toggled ${label}`,
        () => api.action("/api/actions/homeassistant", { entityId }),
        () => {
          window.setTimeout(() => {
            api.serviceWidgets().then(setServiceWidgets).catch(() => undefined);
          }, 900);
        },
      ),
    [runAction],
  );

  const logout = useCallback(async () => {
    await api.logout();
    window.location.reload();
  }, []);

  const actions = useMemo<PaletteAction[]>(
    () => [
      {
        id: "toggle-theme",
        label: "Toggle dark / light theme",
        run: () => updatePrefs({ ...prefs, theme: resolveTheme(prefs.theme) === "dark" ? "light" : "dark" }),
      },
      {
        id: "toggle-layout",
        label: "Toggle grid / list layout",
        run: () => updatePrefs({ ...prefs, layout: prefs.layout === "grid" ? "list" : "grid" }),
      },
      { id: "settings", label: "Open settings", run: () => setSettingsOpen(true) },
      { id: "refresh", label: "Refresh dashboard data", run: () => void refresh() },
      ...(dashboard?.features.docker
        ? [{ id: "docker", label: "Toggle container list", run: () => setDockerOpen((v) => !v) }]
        : []),
    ],
    [prefs, updatePrefs, refresh, dashboard],
  );

  if (state === "loading") {
    return (
      <main className="flex min-h-screen items-center justify-center">
        <p className="font-mono text-[13px] text-ink-3">loading dashboard…</p>
      </main>
    );
  }

  if (state === "unauthorized") {
    return <LoginScreen me={me ?? { authEnabled: true, authenticated: false, providers: [] }} title={dashboard?.settings.title ?? "SDash"} />;
  }

  if (state === "error" || !dashboard) {
    return (
      <main className="flex min-h-screen items-center justify-center px-4">
        <div className="text-center">
          <p className="font-medium text-ink">Could not load the dashboard</p>
          <p className="mt-1 text-[13px] text-ink-2">The server did not return dashboard data.</p>
          <button
            type="button"
            onClick={() => window.location.reload()}
            className="mt-4 rounded-lg bg-accent px-4 py-2 text-[13px] font-medium text-accent-ink"
          >
            Retry
          </button>
        </div>
      </main>
    );
  }

  const { settings, features } = dashboard;
  const haSensors = Object.values(serviceWidgets).find((widget) => widget.type === "homeassistant")?.sensors ?? [];
  const effectiveBackground =
    settings.background ||
    (prefs.immichBackground && mediaRail?.immich[0]
      ? `${mediaRail.immich[0].image}?size=preview`
      : "");
  const pinnedServices = prefs.pinned.map((k) => byKey.get(k)).filter((s): s is Service => Boolean(s));
  const recentServices = prefs.recents
    .map((k) => byKey.get(k))
    .filter((s): s is Service => Boolean(s))
    .filter((s) => !prefs.pinned.includes(s.key));
  const visibleGroups = dashboard.groups
    .filter((g) => groupFilter === "all" || g.key === groupFilter)
    .map((g) => ({
      ...g,
      services: g.services
        .map((s) => byKey.get(s.key) ?? s)
        .filter((s) => !downOnly || s.health?.state === "down"),
    }))
    .filter((g) => g.services.length > 0);

  const gridClass =
    prefs.layout === "list"
      ? "grid grid-cols-1 gap-2 md:grid-cols-2 2xl:grid-cols-3"
      : prefs.density === "compact"
        ? "grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4"
        : "grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3";

  const card = (service: Service) => (
    <ServiceCard
      key={service.key}
      service={service}
      liveWidget={serviceWidgets[service.key]}
      actionsEnabled={features.actions}
      onToggleControl={onToggleControl}
      density={prefs.density}
      layout={prefs.layout}
      pinned={prefs.pinned.includes(service.key)}
      showHealth={features.health}
      onTogglePin={togglePin}
      onOpen={openService}
      onRecord={recordRecent}
      onCopy={(text) => void copyText(text)}
    />
  );

  return (
    <div
      className={effectiveBackground ? "bg-photo min-h-screen" : "min-h-screen"}
      style={effectiveBackground ? { backgroundImage: `url(${effectiveBackground})` } : undefined}
    >
      <div className="relative">
        <TopBar
          title={settings.title}
          subtitle={settings.subtitle}
          theme={resolveTheme(prefs.theme)}
          layout={prefs.layout}
          authEnabled={features.auth}
          refreshing={refreshing}
          onSearch={() => setPaletteOpen(true)}
          onToggleTheme={() =>
            updatePrefs({ ...prefs, theme: resolveTheme(prefs.theme) === "dark" ? "light" : "dark" })
          }
          onToggleLayout={() => updatePrefs({ ...prefs, layout: prefs.layout === "grid" ? "list" : "grid" })}
          onOpenSettings={() => setSettingsOpen(true)}
          onRefresh={() => void refresh()}
          onLogout={() => void logout()}
        />

        <main className="mx-auto max-w-[1400px] px-4 pb-16 sm:px-6">
          <section className="mt-5 overflow-hidden rounded-xl border border-line bg-surface">
            <div className="grid grid-cols-[repeat(auto-fit,minmax(14rem,1fr))] gap-px bg-line">
              {[
                ...(settings.showClock
                  ? [<ClockBlock key="clock" title={settings.title} />]
                  : []),
                ...(features.health
                  ? [
                      <ServicesBlock
                        key="services"
                        statuses={health}
                        total={services.length}
                        onShowDown={showDownServices}
                      />,
                    ]
                  : []),
                ...(features.kubernetes && kube?.available
                  ? [
                      <KubernetesBlock
                        key="kubernetes"
                        kube={kube}
                        expanded={kubeOpen}
                        onToggle={() => setKubeOpen((v) => !v)}
                      />,
                    ]
                  : []),
                ...(features.proxmox && proxmox?.available
                  ? [
                      <ProxmoxBlock
                        key="proxmox"
                        proxmox={proxmox}
                        expanded={proxmoxOpen}
                        onToggle={() => setProxmoxOpen((v) => !v)}
                      />,
                    ]
                  : []),
                ...(features.pbs && pbs?.available
                  ? [<PbsBlock key="pbs" pbs={pbs} expanded={pbsOpen} onToggle={() => setPbsOpen((v) => !v)} />]
                  : []),
                ...(features.truenas && truenas?.available
                  ? [
                      <TrueNASBlock
                        key="truenas"
                        truenas={truenas}
                        expanded={truenasOpen}
                        onToggle={() => setTruenasOpen((v) => !v)}
                      />,
                    ]
                  : []),
                ...(features.crowdsec && crowdsec?.available
                  ? [<CrowdSecBlock key="crowdsec" crowdsec={crowdsec} />]
                  : []),
                ...(haSensors.length > 0 ? [<EnvironBlock key="environ" sensors={haSensors} />] : []),
                ...(features.gatus && gatusData?.available
                  ? [
                      <GatusBlock
                        key="gatus"
                        gatus={gatusData}
                        expanded={gatusOpen}
                        onToggle={() => setGatusOpen((v) => !v)}
                      />,
                    ]
                  : []),
                ...(features.system && system?.available
                  ? [<SystemBlock key="system" system={system} />]
                  : []),
                ...(features.weather && weather ? [<WeatherBlock key="weather" weather={weather} />] : []),
                ...(features.docker && docker && prefs.showDocker
                  ? [
                      <DockerBlock
                        key="docker"
                        docker={docker}
                        expanded={dockerOpen}
                        onToggle={() => setDockerOpen((v) => !v)}
                      />,
                    ]
                  : []),
              ].map((block, i) => (
                <div key={block.key ?? i} className={`bg-surface ${prefs.density === "compact" ? "p-3" : "p-4"}`}>
                  {block}
                </div>
              ))}
            </div>
            {kubeOpen && kube && (
              <KubernetesDetail
                kube={kube}
                actions={{ actionsEnabled: features.actions, onArgoSync, onRestart }}
              />
            )}
            {proxmoxOpen && proxmox && (
              <ProxmoxDetail
                proxmox={proxmox}
                actions={{ actionsEnabled: features.actions, onStart: onStartGuest, onWake }}
              />
            )}
            {pbsOpen && pbs && <PbsDetail pbs={pbs} />}
            {truenasOpen && truenas && <TrueNASDetail truenas={truenas} />}
            {gatusOpen && gatusData && <GatusDetail gatus={gatusData} />}
            {dockerOpen && docker && <DockerList docker={docker} />}
          </section>

          <AttentionSection
            items={buildAttentionItems({
              services,
              kube,
              gatusData,
              proxmox,
              truenas,
              pbs,
              actionsEnabled: features.actions,
              onArgoSync,
            })}
            onRecord={(serviceKey) => {
              const service = byKey.get(serviceKey);
              if (service) recordRecent(service);
            }}
          />

          {calendar && <CalendarRail calendar={calendar} />}
          {mediaRail && <MediaRails rail={mediaRail} />}

          {recentServices.length > 0 && (
            <section className="mt-6">
              <h2 className="font-mono text-[11px] lowercase text-ink-3">recent</h2>
              <div className="mt-2 flex flex-wrap gap-1.5">
                {recentServices.map((service) => (
                  <a
                    key={service.key}
                    href={service.url}
                    target={service.openInNewTab ? "_blank" : undefined}
                    rel="noopener"
                    onClick={() => recordRecent(service)}
                    className="flex items-center gap-2 rounded-full border border-line bg-surface px-3 py-1.5 text-[13px] text-ink-2 transition-colors hover:border-line-strong hover:text-ink"
                  >
                    {service.icon && <img src={service.icon} alt="" width={14} height={14} className="rounded-sm object-contain" />}
                    {service.title}
                  </a>
                ))}
              </div>
            </section>
          )}

          {pinnedServices.length > 0 && (
            <section className="mt-8">
              <h2 className="text-[15px] font-semibold text-ink">Favorites</h2>
              <div className={`mt-3 ${gridClass}`}>{pinnedServices.map(card)}</div>
            </section>
          )}

          <nav id="service-filters" className="mt-8 flex flex-wrap items-center gap-1.5 scroll-mt-20" aria-label="Filter by group">
            <FilterChip active={groupFilter === "all" && !downOnly} onClick={() => { setGroupFilter("all"); setDownOnly(false); }}>
              All
            </FilterChip>
            {dashboard.groups.map((group) => (
              <FilterChip
                key={group.key}
                active={groupFilter === group.key && !downOnly}
                onClick={() => { setGroupFilter(group.key); setDownOnly(false); }}
              >
                {group.title}
              </FilterChip>
            ))}
            {features.health && (
              <FilterChip
                active={downOnly}
                onClick={() => {
                  if (!downOnly) setGroupFilter("all");
                  setDownOnly(!downOnly);
                }}
              >
                Down only
              </FilterChip>
            )}
          </nav>

          {visibleGroups.length === 0 && (
            <p className="mt-10 text-center text-[13px] text-ink-3">
              {downOnly ? "Nothing is down right now." : "No services here. Edit dashboard.yaml on the server to add some."}
            </p>
          )}

          {visibleGroups.map((group) => (
            <section key={group.key} className="mt-8">
              <div className="flex items-baseline gap-2.5">
                <h2 className="text-[15px] font-semibold text-ink">{group.title}</h2>
                <span className="font-mono text-[11px] text-ink-3">{group.services.length}</span>
                <span className="h-px flex-1 bg-line" />
              </div>
              <div className={`mt-3 ${gridClass}`}>{group.services.map(card)}</div>
            </section>
          ))}

          <footer className="mt-14 flex flex-wrap items-center justify-between gap-2 border-t border-line pt-4 font-mono text-[11px] text-ink-3">
            <span>sdash · {services.length} services · {dashboard.groups.length} groups</span>
            <span>{searchShortcutLabel()} to search</span>
          </footer>
        </main>

        <CommandPalette
          open={paletteOpen}
          services={services}
          groupTitles={groupTitles}
          searchEngine={settings.searchEngine}
          searchEngineName={settings.searchEngineName || "the web"}
          actions={actions}
          actionsEnabled={features.actions}
          onClose={() => setPaletteOpen(false)}
          onOpenService={openService}
          onRecord={recordRecent}
          onToast={showToast}
        />
        <SettingsDrawer open={settingsOpen} prefs={prefs} prefsSync={prefsSync} onChange={updatePrefs} onClose={() => setSettingsOpen(false)} />

        {toast && (
          <div className="rise fixed bottom-4 left-1/2 z-50 -translate-x-1/2 rounded-lg border border-line-strong bg-surface-2 px-3.5 py-2 text-[13px] text-ink shadow-xl" role="status">
            {toast}
          </div>
        )}
      </div>
    </div>
  );
}

function FilterChip({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={`rounded-full border px-3 py-1.5 text-[13px] transition-colors outline-none focus-visible:ring-2 focus-visible:ring-accent ${
        active
          ? "border-accent bg-accent-soft font-medium text-ink"
          : "border-line bg-surface text-ink-2 hover:border-line-strong hover:text-ink"
      }`}
    >
      {children}
    </button>
  );
}
