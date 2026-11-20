import type { CrowdSec, Gatus, Kubernetes, Pbs, Proxmox, TrueNAS } from "../types";
import { formatBytes } from "../lib/format";
import { ChevronDownIcon } from "./icons";
import { StatusDot } from "./StatusBits";

function BlockLabel({ children }: { children: React.ReactNode }) {
  return <p className="font-mono text-[11px] lowercase text-ink-3">{children}</p>;
}

export function KubernetesBlock({
  kube,
  expanded,
  onToggle,
}: {
  kube: Kubernetes;
  expanded: boolean;
  onToggle: () => void;
}) {
  const attention = kube.podsNotReady.length + kube.deploymentsNotReady.length;
  return (
    <div className="min-w-0">
      <BlockLabel>kubernetes · {kube.namespaces} namespaces</BlockLabel>
      <button type="button" onClick={onToggle} className="mt-1 flex items-center gap-2 text-left" aria-expanded={expanded}>
        <span className="font-mono stat-value text-[34px] leading-none font-medium tracking-tight text-ink">
          {kube.nodesReady}
          <span className="text-lg text-ink-3">/{kube.nodesTotal} nodes</span>
        </span>
        <ChevronDownIcon size={16} className={`text-ink-3 transition-transform ${expanded ? "rotate-180" : ""}`} />
      </button>
      <p className="mt-1.5 text-[13px] text-ink-2">
        {kube.podsReady}/{kube.podsTotal} pods · {kube.deploymentsReady}/{kube.deploymentsTotal} deployments
        {attention > 0 && (
          <span className="text-slow"> · {attention} {attention === 1 ? "needs" : "need"} attention</span>
        )}
      </p>
      {kube.argo.available && (
        <p className="mt-0.5 text-[13px] text-ink-2">
          Argo CD: {kube.argo.synced}/{kube.argo.total} synced · {kube.argo.healthy}/{kube.argo.total} healthy
        </p>
      )}
    </div>
  );
}

export interface KubeActionHandlers {
  actionsEnabled: boolean;
  onArgoSync: (app: string) => void;
  onRestart: (namespace: string, name: string) => void;
}

export function KubernetesDetail({ kube, actions }: { kube: Kubernetes; actions: KubeActionHandlers }) {
  return (
    <div className="rise border-t border-line">
      <div className="grid grid-cols-1 gap-0 lg:grid-cols-3">
        <DetailSection title="nodes">
          {kube.nodes.map((node) => (
            <DetailRow key={node.name}>
              <StatusDot state={node.ready ? "up" : "down"} />
              <span className="min-w-0 flex-1 truncate text-ink">
                {node.name}
                {node.rebootPending && <span className="ml-2 text-slow">reboot pending</span>}
              </span>
              <span className="font-mono text-[11px] text-ink-3">
                {kube.metrics.available
                  ? `cpu ${node.cpuPercent.toFixed(0)}% · mem ${node.memPercent.toFixed(0)}%`
                  : `${node.roles} · ${node.version}`}
              </span>
            </DetailRow>
          ))}
        </DetailSection>

        <DetailSection title="needs attention">
          {kube.podsNotReady.length === 0 && kube.deploymentsNotReady.length === 0 && (
            <p className="px-4 py-2 text-[13px] text-ink-3">All workloads ready</p>
          )}
          {kube.podsNotReady.map((pod) => (
            <DetailRow key={`pod-${pod.namespace}-${pod.name}`}>
              <StatusDot state="down" />
              <span className="min-w-0 flex-1 truncate text-ink">
                {pod.namespace}/{pod.name}
              </span>
              <span className="font-mono text-[11px] text-ink-3">
                {pod.detail} {pod.ready}/{pod.desired}
                {pod.restarts ? ` · ↻${pod.restarts}` : ""}
              </span>
            </DetailRow>
          ))}
          {kube.deploymentsNotReady.map((deploy) => (
            <DetailRow key={`deploy-${deploy.namespace}-${deploy.name}`}>
              <StatusDot state="slow" />
              <span className="min-w-0 flex-1 truncate text-ink">
                {deploy.namespace}/{deploy.name}
              </span>
              <span className="font-mono text-[11px] text-ink-3">
                {deploy.ready}/{deploy.desired} replicas
              </span>
              {actions.actionsEnabled && (
                <ActionButton onClick={() => actions.onRestart(deploy.namespace, deploy.name)}>restart</ActionButton>
              )}
            </DetailRow>
          ))}
        </DetailSection>

        <DetailSection title="argo cd">
          {!kube.argo.available && <p className="px-4 py-2 text-[13px] text-ink-3">Argo CD not detected</p>}
          {kube.argo.available && kube.argo.attention.length === 0 && (
            <p className="px-4 py-2 text-[13px] text-ink-3">All applications synced and healthy</p>
          )}
          {kube.argo.attention.map((app) => (
            <DetailRow key={app.name}>
              <StatusDot state={app.health === "Healthy" ? "slow" : "down"} />
              <span className="min-w-0 flex-1 truncate text-ink">{app.name}</span>
              <span className="font-mono text-[11px] text-ink-3">
                {app.sync} · {app.health}
              </span>
              {actions.actionsEnabled && app.sync !== "Synced" && (
                <ActionButton onClick={() => actions.onArgoSync(app.name)}>sync</ActionButton>
              )}
            </DetailRow>
          ))}
        </DetailSection>
      </div>

      {(kube.metrics.available || kube.certificatesAvailable || kube.longhorn.available) && (
        <div className="grid grid-cols-1 gap-0 border-t border-line lg:grid-cols-3">
          <DetailSection title="top pods (cpu)">
            {!kube.metrics.available && <p className="px-4 py-2 text-[13px] text-ink-3">metrics unavailable</p>}
            {kube.metrics.topPods.map((pod) => (
              <DetailRow key={`${pod.namespace}-${pod.name}`}>
                <span className="min-w-0 flex-1 truncate text-ink">
                  {pod.namespace}/{pod.name}
                </span>
                <span className="font-mono text-[11px] text-ink-3">
                  {pod.cpuMillis}m · {formatBytes(pod.memBytes)}
                </span>
              </DetailRow>
            ))}
            {kube.metrics.namespaces.slice(0, 3).map((ns) => (
              <DetailRow key={`ns-${ns.name}`}>
                <span className="min-w-0 flex-1 truncate text-ink-3">ns {ns.name}</span>
                <span className="font-mono text-[11px] text-ink-3">
                  {ns.cpuMillis}m · {formatBytes(ns.memBytes)} · {ns.pods} pods
                </span>
              </DetailRow>
            ))}
          </DetailSection>

          <DetailSection title={`certificates · ${kube.certificatesReady}/${kube.certificatesTotal} ready`}>
            {!kube.certificatesAvailable && <p className="px-4 py-2 text-[13px] text-ink-3">cert-manager not detected</p>}
            {kube.certificatesAvailable && kube.certificates.length === 0 && (
              <p className="px-4 py-2 text-[13px] text-ink-3">None expiring within 14 days</p>
            )}
            {kube.certificates.map((cert) => (
              <DetailRow key={`${cert.namespace}-${cert.name}`}>
                <StatusDot state={!cert.ready || cert.daysLeft < 0 ? "down" : "slow"} />
                <span className="min-w-0 flex-1 truncate text-ink">
                  {cert.namespace}/{cert.name}
                </span>
                <span className="font-mono text-[11px] text-ink-3">
                  {cert.ready ? `${cert.daysLeft}d left` : "not ready"}
                </span>
              </DetailRow>
            ))}
            <p className="px-4 py-1.5 font-mono text-[11px] text-ink-3">
              traefik: {kube.ingresses} ingresses · {kube.ingressRoutes} ingressroutes
            </p>
          </DetailSection>

          <DetailSection title="longhorn volumes">
            {!kube.longhorn.available && <p className="px-4 py-2 text-[13px] text-ink-3">Longhorn not detected</p>}
            {kube.longhorn.available && (
              <p className="px-4 py-2 text-[13px] text-ink-2">
                {kube.longhorn.healthy}/{kube.longhorn.total} healthy
                {kube.longhorn.degraded > 0 && <span className="text-slow"> · {kube.longhorn.degraded} degraded</span>}
              </p>
            )}
            {kube.longhorn.attention.map((volume) => (
              <DetailRow key={volume.name}>
                <StatusDot state="slow" />
                <span className="min-w-0 flex-1 truncate text-ink">{volume.name}</span>
                <span className="font-mono text-[11px] text-ink-3">{volume.detail}</span>
              </DetailRow>
            ))}
          </DetailSection>
        </div>
      )}
    </div>
  );
}

export function ActionButton({ children, onClick }: { children: React.ReactNode; onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="shrink-0 rounded-md border border-line bg-surface px-2 py-0.5 font-mono text-[11px] text-ink-2 transition-colors hover:border-accent hover:text-ink"
    >
      {children}
    </button>
  );
}

function DetailSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="border-line max-lg:border-b lg:border-r lg:last:border-r-0">
      <p className="px-4 pt-3 font-mono text-[11px] lowercase text-ink-3">{title}</p>
      <div className="py-1.5">{children}</div>
    </div>
  );
}

function DetailRow({ children }: { children: React.ReactNode }) {
  return <div className="flex items-center gap-2 px-4 py-1.5 text-[13px]">{children}</div>;
}

export function ProxmoxBlock({
  proxmox,
  expanded,
  onToggle,
}: {
  proxmox: Proxmox;
  expanded: boolean;
  onToggle: () => void;
}) {
  const storagePct = proxmox.storageTotal > 0 ? (proxmox.storageUsed / proxmox.storageTotal) * 100 : 0;
  return (
    <div className="min-w-0">
      <BlockLabel>proxmox ve</BlockLabel>
      <button type="button" onClick={onToggle} className="mt-1 flex items-center gap-2 text-left" aria-expanded={expanded}>
        <span className="font-mono stat-value text-[34px] leading-none font-medium tracking-tight text-ink">
          {proxmox.nodesOnline}
          <span className="text-lg text-ink-3">/{proxmox.nodesTotal} hosts</span>
        </span>
        <ChevronDownIcon size={16} className={`text-ink-3 transition-transform ${expanded ? "rotate-180" : ""}`} />
      </button>
      <p className="mt-1.5 text-[13px] text-ink-2">
        {proxmox.guestsRunning}/{proxmox.guestsTotal} guests running · storage {storagePct.toFixed(0)}% used
      </p>
    </div>
  );
}

export interface ProxmoxActionHandlers {
  actionsEnabled: boolean;
  onStart: (node: string, kind: string, vmid: number, name: string) => void;
  onWake: (name: string) => void;
}

export function ProxmoxDetail({ proxmox, actions }: { proxmox: Proxmox; actions: ProxmoxActionHandlers }) {
  return (
    <div className="rise border-t border-line">
      <div className="grid grid-cols-1 gap-0 lg:grid-cols-3">
        <DetailSection title="hosts">
          {proxmox.nodes.map((node) => (
            <DetailRow key={node.name}>
              <StatusDot state={node.online ? "up" : "down"} />
              <span className="min-w-0 flex-1 truncate text-ink">{node.name}</span>
              <span className="font-mono text-[11px] text-ink-3">
                cpu {node.cpuPercent.toFixed(0)}% · mem {formatBytes(node.memUsed)}/{formatBytes(node.memTotal)}
              </span>
              {actions.actionsEnabled && !node.online && proxmox.wakeHosts.includes(node.name) && (
                <ActionButton onClick={() => actions.onWake(node.name)}>wake</ActionButton>
              )}
            </DetailRow>
          ))}
        </DetailSection>
        <DetailSection title="storage">
          {proxmox.storage.map((store) => (
            <DetailRow key={store.name}>
              <span className="min-w-0 flex-1 truncate text-ink">{store.name}</span>
              <span className="font-mono text-[11px] text-ink-3">
                {formatBytes(store.used)}/{formatBytes(store.total)} · {store.usedPercent.toFixed(0)}%
              </span>
            </DetailRow>
          ))}
        </DetailSection>
        <DetailSection title="stopped guests">
          {proxmox.stoppedGuests.length === 0 && (
            <p className="px-4 py-2 text-[13px] text-ink-3">Everything is running</p>
          )}
          {proxmox.stoppedGuests.map((guest) => (
            <DetailRow key={guest.vmid}>
              <StatusDot state="unknown" />
              <span className="min-w-0 flex-1 truncate text-ink">
                {guest.vmid} {guest.name}
              </span>
              <span className="font-mono text-[11px] text-ink-3">
                {guest.kind} · {guest.node} · {guest.status}
              </span>
              {actions.actionsEnabled && (
                <ActionButton onClick={() => actions.onStart(guest.node, guest.kind, guest.vmid, guest.name)}>
                  start
                </ActionButton>
              )}
            </DetailRow>
          ))}
        </DetailSection>
      </div>
    </div>
  );
}

export function TrueNASBlock({
  truenas,
  expanded,
  onToggle,
}: {
  truenas: TrueNAS;
  expanded: boolean;
  onToggle: () => void;
}) {
  const usedPct = truenas.total > 0 ? (truenas.used / truenas.total) * 100 : 0;
  return (
    <div className="min-w-0">
      <BlockLabel>truenas{truenas.hostname ? ` · ${truenas.hostname}` : ""}</BlockLabel>
      <button type="button" onClick={onToggle} className="mt-1 flex items-center gap-2 text-left" aria-expanded={expanded}>
        <span className="font-mono stat-value text-[34px] leading-none font-medium tracking-tight text-ink">
          {truenas.poolsHealthy}
          <span className="text-lg text-ink-3">/{truenas.poolsTotal} pools ok</span>
        </span>
        <ChevronDownIcon size={16} className={`text-ink-3 transition-transform ${expanded ? "rotate-180" : ""}`} />
      </button>
      <p className="mt-1.5 text-[13px] text-ink-2">
        {formatBytes(truenas.used)}/{formatBytes(truenas.total)} ({usedPct.toFixed(0)}%)
        {truenas.alertsUnavailable ? (
          <span className="text-slow"> · alerts unavailable</span>
        ) : truenas.alerts > 0 && (
          <span className="text-slow"> · {truenas.alerts} {truenas.alerts === 1 ? "alert" : "alerts"}</span>
        )}
      </p>
    </div>
  );
}

export function TrueNASDetail({ truenas }: { truenas: TrueNAS }) {
  return (
    <div className="rise border-t border-line px-4 py-3">
      <p className="font-mono text-[11px] lowercase text-ink-3">
        pools{truenas.version ? ` · ${truenas.version}` : ""}
      </p>
      <div className="mt-1">
        {truenas.pools.map((pool) => (
          <DetailRow key={pool.name}>
            <StatusDot state={pool.healthy ? "up" : "down"} />
            <span className="min-w-0 flex-1 truncate text-ink">{pool.name}</span>
            <span className="font-mono text-[11px] text-ink-3">
              {pool.status} · {formatBytes(pool.used)}/{formatBytes(pool.total)} · {pool.usedPercent.toFixed(0)}%
            </span>
          </DetailRow>
        ))}
      </div>
    </div>
  );
}

export function PbsBlock({ pbs, expanded, onToggle }: { pbs: Pbs; expanded: boolean; onToggle: () => void }) {
  const usedPct = pbs.total > 0 ? (pbs.used / pbs.total) * 100 : 0;
  return (
    <div className="min-w-0">
      <BlockLabel>backups · proxmox bs</BlockLabel>
      <button type="button" onClick={onToggle} className="mt-1 flex items-center gap-2 text-left" aria-expanded={expanded}>
        <span className="font-mono stat-value text-[34px] leading-none font-medium tracking-tight text-ink">
          {pbs.hasBackups ? (pbs.lastBackupOk ? "ok" : "fail") : "—"}
          <span className="text-lg text-ink-3"> last backup</span>
        </span>
        <ChevronDownIcon size={16} className={`text-ink-3 transition-transform ${expanded ? "rotate-180" : ""}`} />
      </button>
      <p className="mt-1.5 text-[13px] text-ink-2">
        {formatBytes(pbs.used)}/{formatBytes(pbs.total)} ({usedPct.toFixed(0)}%)
        {pbs.recentFailures > 0 && (
          <span className="text-down">
            {" "}
            · {pbs.recentFailures}/{pbs.recentTotal} recent failed
          </span>
        )}
      </p>
    </div>
  );
}

export function PbsDetail({ pbs }: { pbs: Pbs }) {
  return (
    <div className="rise border-t border-line px-4 py-3">
      <p className="font-mono text-[11px] lowercase text-ink-3">
        datastores{pbs.lastBackupAt > 0 ? ` · last backup ${new Date(pbs.lastBackupAt * 1000).toLocaleString()}` : ""}
      </p>
      <div className="mt-1">
        {pbs.datastores.map((store) => (
          <DetailRow key={store.name}>
            <span className="min-w-0 flex-1 truncate text-ink">{store.name}</span>
            <span className="font-mono text-[11px] text-ink-3">
              {formatBytes(store.used)}/{formatBytes(store.total)} · {store.usedPercent.toFixed(0)}%
            </span>
          </DetailRow>
        ))}
      </div>
    </div>
  );
}

export function CrowdSecBlock({ crowdsec }: { crowdsec: CrowdSec }) {
  const top = crowdsec.groups[0];
  return (
    <div className="min-w-0">
      <BlockLabel>crowdsec · active bans</BlockLabel>
      <p className="mt-1 font-mono stat-value text-[34px] leading-none font-medium tracking-tight text-ink">{crowdsec.decisions}</p>
      <p className="mt-1.5 truncate text-[13px] text-ink-2">
        {top ? `top: ${top.origin} ${top.type} ×${top.count}` : "no active decisions"}
      </p>
    </div>
  );
}

export function EnvironBlock({ sensors }: { sensors: { id: string; label: string; value: string; unit: string }[] }) {
  return (
    <div className="min-w-0">
      <BlockLabel>environment · home assistant</BlockLabel>
      <div className="mt-2 space-y-1.5">
        {sensors.slice(0, 4).map((sensor) => (
          <p key={sensor.id} className="flex items-baseline justify-between gap-2 text-[13px]">
            <span className="min-w-0 truncate text-ink-2">{sensor.label}</span>
            <span className="shrink-0 font-mono text-[13px] text-ink">
              {sensor.value}
              {sensor.unit ? <span className="text-ink-3"> {sensor.unit}</span> : null}
            </span>
          </p>
        ))}
      </div>
    </div>
  );
}

export function GatusBlock({ gatus, expanded, onToggle }: { gatus: Gatus; expanded: boolean; onToggle: () => void }) {
  return (
    <div className="min-w-0">
      <BlockLabel>gatus · endpoint checks</BlockLabel>
      <button type="button" onClick={onToggle} className="mt-1 flex items-center gap-2 text-left" aria-expanded={expanded}>
        <span className="font-mono stat-value text-[34px] leading-none font-medium tracking-tight text-ink">
          {gatus.up}
          <span className="text-lg text-ink-3">/{gatus.total} up</span>
        </span>
        <ChevronDownIcon size={16} className={`text-ink-3 transition-transform ${expanded ? "rotate-180" : ""}`} />
      </button>
      <p className="mt-1.5 text-[13px] text-ink-2">
        {gatus.down > 0 ? <span className="text-down">{gatus.down} failing checks</span> : "All checks passing"}
      </p>
    </div>
  );
}

export function GatusDetail({ gatus }: { gatus: Gatus }) {
  if (!gatus.available) return null;
  const shown = gatus.endpoints.filter((endpoint) => !endpoint.up);
  const list = shown.length > 0 ? shown : gatus.endpoints;
  return (
    <div className="rise overflow-x-auto border-t border-line">
      <table className="w-full text-left text-[13px]">
        <thead>
          <tr className="font-mono text-[11px] text-ink-3">
            <th className="px-4 py-2 font-normal">endpoint</th>
            <th className="px-4 py-2 font-normal">group</th>
            <th className="px-4 py-2 font-normal">state</th>
            <th className="px-4 py-2 font-normal">latency</th>
            <th className="px-4 py-2 font-normal">uptime</th>
          </tr>
        </thead>
        <tbody>
          {list.map((endpoint) => (
            <tr key={`${endpoint.group}-${endpoint.name}`} className="border-t border-line/60">
              <td className="px-4 py-2 font-medium text-ink">{endpoint.name}</td>
              <td className="px-4 py-2 text-ink-2">{endpoint.group}</td>
              <td className="px-4 py-2">
                <span className="flex items-center gap-2">
                  <StatusDot state={endpoint.up ? "up" : "down"} />
                  <span className={endpoint.up ? "text-ink-2" : "text-down"}>{endpoint.up ? "up" : "down"}</span>
                </span>
              </td>
              <td className="px-4 py-2 font-mono text-[12px] text-ink-3">{endpoint.latencyMs}ms</td>
              <td className="px-4 py-2 font-mono text-[12px] text-ink-3">{endpoint.uptime.toFixed(1)}%</td>
            </tr>
          ))}
        </tbody>
      </table>
      {shown.length === 0 && (
        <p className="border-t border-line px-4 py-2 font-mono text-[11px] text-ink-3">
          showing all endpoints — none are down
        </p>
      )}
    </div>
  );
}
