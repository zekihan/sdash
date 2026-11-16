import type {
  ArrLookupResult,
  CalendarData,
  Dashboard,
  DockerInfo,
  Gatus,
  CrowdSec,
  MediaRailData,
  Kubernetes,
  Me,
  Pbs,
  Proxmox,
  TrueNAS,
  ServiceHealth,
  ServiceWidget,
  SystemInfo,
  Weather,
} from "../types";

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function getJSON<T>(path: string): Promise<T> {
  const res = await fetch(path, { headers: { Accept: "application/json" } });
  if (!res.ok) {
    throw new ApiError(res.status, `${path} failed with ${res.status}`);
  }
  return (await res.json()) as T;
}

export const api = {
  me: () => getJSON<Me>("/api/me"),
  dashboard: () => getJSON<Dashboard>("/api/dashboard"),
  health: () => getJSON<Record<string, ServiceHealth>>("/api/health/services"),
  system: () => getJSON<SystemInfo>("/api/system"),
  docker: () => getJSON<DockerInfo>("/api/docker"),
  weather: () => getJSON<Weather>("/api/weather"),
  kubernetes: () => getJSON<Kubernetes>("/api/kubernetes"),
  proxmox: () => getJSON<Proxmox>("/api/proxmox"),
  pbs: () => getJSON<Pbs>("/api/pbs"),
  truenas: () => getJSON<TrueNAS>("/api/truenas"),
  crowdsec: () => getJSON<CrowdSec>("/api/crowdsec"),
  calendar: () => getJSON<CalendarData>("/api/calendar"),
  mediaRail: () => getJSON<MediaRailData>("/api/media-rail"),
  arrLookup: (type: string, term: string) =>
    getJSON<ArrLookupResult[]>(`/api/arr/lookup?type=${encodeURIComponent(type)}&term=${encodeURIComponent(term)}`),
  getPreferences: () => getJSON<{ enabled: boolean; prefs: unknown }>("/api/preferences"),
  putPreferences: async (prefs: unknown) => {
    const res = await fetch("/api/preferences", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(prefs),
    });
    if (!res.ok) throw new ApiError(res.status, "preference sync failed");
  },
  action: async (path: string, body: unknown) => {
    const res = await fetch(path, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    if (!res.ok) {
      let message = `action failed with ${res.status}`;
      try {
        const parsed = (await res.json()) as { error?: string };
        if (parsed.error) message = parsed.error;
      } catch {
        // keep the generic message
      }
      throw new ApiError(res.status, message);
    }
  },
  gatus: () => getJSON<Gatus>("/api/gatus"),
  serviceWidgets: () => getJSON<Record<string, ServiceWidget>>("/api/service-widgets"),
  logout: async () => {
    await fetch("/api/logout", { method: "POST" });
  },
};

export type LiveEvent =
  | { type: "health"; data: Record<string, ServiceHealth> }
  | { type: "config"; data: unknown };

/** Subscribe to the server-sent event stream. Returns an unsubscribe fn. */
export function subscribeEvents(onEvent: (event: LiveEvent) => void): () => void {
  const source = new EventSource("/api/events");

  source.addEventListener("health", (raw) => {
    try {
      onEvent({ type: "health", data: JSON.parse((raw as MessageEvent).data) });
    } catch {
      // malformed frame: next round resyncs
    }
  });
  source.addEventListener("config", () => onEvent({ type: "config", data: null }));

  return () => source.close();
}
