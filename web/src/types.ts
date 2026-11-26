export type HealthState = "up" | "down" | "slow" | "unknown";

export interface HealthSample {
  at: string;
  state: HealthState;
  latencyMs: number;
  code: number;
  error?: string;
}

export interface ServiceHealth {
  key: string;
  state: HealthState;
  checkedAt: string;
  latencyMs: number;
  code: number;
  error?: string;
  uptime: number;
  history: HealthSample[];
}

export interface Service {
  key: string;
  title: string;
  description?: string;
  url: string;
  icon?: string;
  group: string;
  location?: string;
  cluster?: string;
  tags: string[];
  openInNewTab: boolean;
  widget?: { type: string; options?: Record<string, unknown> };
  health?: ServiceHealth;
}

export interface Group {
  key: string;
  title: string;
  order: number;
  services: Service[];
}

export interface Settings {
  title: string;
  subtitle?: string;
  searchEngine: string;
  searchEngineName?: string;
  background?: string;
  defaultTheme: "dark" | "light" | "system";
  defaultAccent: string;
  defaultDensity: "comfortable" | "compact";
  openInNewTab: boolean;
  showClock: boolean;
}

export interface Dashboard {
  settings: Settings;
  features: {
    health: boolean;
    system: boolean;
    docker: boolean;
    weather: boolean;
    auth: boolean;
    kubernetes: boolean;
    proxmox: boolean;
    pbs: boolean;
    truenas: boolean;
    crowdsec: boolean;
    gatus: boolean;
    widgets: boolean;
    actions: boolean;
    prefsSync: boolean;
  };
  groups: Group[];
}

export interface Me {
  authEnabled: boolean;
  authenticated: boolean;
  providers: { key: string; default: boolean }[];
}

export interface SystemInfo {
  hostname: string;
  os: string;
  arch: string;
  cpus: number;
  cpuPercent: number;
  memTotal: number;
  memUsed: number;
  memPercent: number;
  diskPath: string;
  diskTotal: number;
  diskUsed: number;
  diskPercent: number;
  load1: number;
  load5: number;
  load15: number;
  uptimeSec: number;
  available: boolean;
}

export interface DockerContainer {
  id: string;
  name: string;
  image: string;
  state: string;
  status: string;
  created: number;
  ports: { PrivatePort: number; PublicPort: number; Type: string }[];
}

export interface DockerInfo {
  available: boolean;
  containers: DockerContainer[];
  error?: string;
}

export interface KubeNode {
  name: string;
  roles: string;
  version: string;
  ready: boolean;
}

export interface KubeWorkload {
  namespace: string;
  name: string;
  detail: string;
  ready: number;
  desired: number;
  restarts?: number;
}

export interface KubeCertificate {
  namespace: string;
  name: string;
  notAfter: string;
  daysLeft: number;
  ready: boolean;
}

export interface Kubernetes {
  nodes: (KubeNode & {
    cpuPercent: number;
    memPercent: number;
    cpuMillis: number;
    memUsedBytes: number;
    allocMemBytes: number;
    rebootPending: boolean;
  })[];
  podsNotReady: KubeWorkload[];
  deploymentsNotReady: KubeWorkload[];
  certificates: KubeCertificate[];
  rebootRequired: string[];
  longhorn: {
    attention: KubeWorkload[];
    healthy: number;
    degraded: number;
    total: number;
    available: boolean;
  };
  metrics: {
    topPods: { namespace: string; name: string; cpuMillis: number; memBytes: number }[];
    namespaces: { name: string; cpuMillis: number; memBytes: number; pods: number }[];
    available: boolean;
  };
  argo: {
    attention: { name: string; sync: string; health: string }[];
    total: number;
    synced: number;
    healthy: number;
    available: boolean;
  };
  nodesReady: number;
  nodesTotal: number;
  podsReady: number;
  podsTotal: number;
  deploymentsReady: number;
  deploymentsTotal: number;
  namespaces: number;
  certificatesTotal: number;
  certificatesReady: number;
  ingresses: number;
  ingressRoutes: number;
  certificatesAvailable: boolean;
  available: boolean;
}

export interface Pbs {
  datastores: { name: string; usedPercent: number; used: number; total: number }[];
  used: number;
  total: number;
  lastBackupAt: number;
  recentTotal: number;
  recentFailures: number;
  lastBackupOk: boolean;
  hasBackups: boolean;
  tasksUnavailable: boolean;
  available: boolean;
}

export interface CrowdSec {
  groups: { origin: string; type: string; count: number }[];
  decisions: number;
  available: boolean;
}

export interface Proxmox {
  nodes: { name: string; cpuPercent: number; memPercent: number; memUsed: number; memTotal: number; online: boolean }[];
  storage: { name: string; usedPercent: number; used: number; total: number }[];
  stoppedGuests: { name: string; node: string; kind: string; status: string; vmid: number }[];
  wakeHosts: string[];
  nodesOnline: number;
  nodesTotal: number;
  guestsRunning: number;
  guestsTotal: number;
  storageUsed: number;
  storageTotal: number;
  available: boolean;
}

export interface TrueNAS {
  hostname: string;
  version: string;
  pools: { name: string; status: string; usedPercent: number; used: number; total: number; healthy: boolean }[];
  poolsHealthy: number;
  poolsTotal: number;
  alerts: number;
  alertsUnavailable?: boolean;
  used: number;
  total: number;
  available: boolean;
}

export interface Gatus {
  endpoints: { name: string; group: string; uptime: number; latencyMs: number; up: boolean }[];
  total: number;
  up: number;
  down: number;
  available: boolean;
}

export interface ServiceWidget {
  type: string;
  stats: { label: string; value: string }[];
  lines: string[];
  controls: { id: string; label: string; state: string; on: boolean }[];
  sensors: { id: string; label: string; value: string; unit: string; kind: string }[];
}

export interface CalendarData {
  items: { title: string; detail: string; source: string; airDate: string; hasFile: boolean }[];
  available: boolean;
}

export interface MediaRailData {
  jellyfin: { id: string; title: string; detail: string; image: string }[];
  immich: { id: string; title: string; detail: string; image: string }[];
  available: boolean;
}

export interface ArrLookupResult {
  title: string;
  overview: string;
  poster: string;
  externalId: number;
  year: number;
  added: boolean;
}

export interface Weather {
  label: string;
  units: string;
  temperature: number;
  feelsLike: number;
  humidity: number;
  windSpeed: number;
  code: number;
  description: string;
  isDay: boolean;
  daily: { date: string; code: number; max: number; min: number }[];
  fetchedAt: string;
}
