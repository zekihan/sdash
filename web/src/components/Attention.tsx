import { useState } from "react";
import type { Gatus, Kubernetes, Pbs, Proxmox, Service, TrueNAS } from "../types";
import { humanizeHealthError } from "../lib/format";
import { StatusDot } from "./StatusBits";
import { ActionButton } from "./ClusterWidgets";

export interface AttentionItem {
  id: string;
  severity: "down" | "slow";
  source: string;
  label: string;
  detail: string;
  rawDetail?: string;
  serviceKey?: string;
  href?: string;
  openInNewTab?: boolean;
  action?: { label: string; run: () => void };
}

const ATTENTION_PREVIEW = 6;

function failingSince(service: Service): string {
  const health = service.health;
  if (!health || health.history.length === 0) return "";
  let earliest = health.history[health.history.length - 1];
  let reachedStart = false;
  for (let i = health.history.length - 1; i >= 0; i--) {
    if (health.history[i].state !== health.state) break;
    earliest = health.history[i];
    if (i === 0) reachedStart = true;
  }
  const minutes = Math.max(0, Math.floor((Date.now() - new Date(earliest.at).getTime()) / 60000));
  let duration: string;
  if (minutes < 1) duration = "just now";
  else if (minutes < 60) duration = `${minutes}m`;
  else {
    const hours = Math.floor(minutes / 60);
    duration = hours < 24 ? `${hours}h` : `${Math.floor(hours / 24)}d`;
  }
  // The whole retained history is failing: the outage is at least this long.
  return reachedStart && minutes >= 1 ? `≥${duration}` : duration;
}

interface Inputs {
  services: Service[];
  kube: Kubernetes | null;
  gatusData: Gatus | null;
  proxmox: Proxmox | null;
  truenas: TrueNAS | null;
  pbs: Pbs | null;
  actionsEnabled: boolean;
  onArgoSync: (app: string) => void;
}

/** Builds the unified "needs attention" feed from every live source. */
export function buildAttentionItems(input: Inputs): AttentionItem[] {
  const items: AttentionItem[] = [];

  for (const service of input.services) {
    if (service.health?.state === "down") {
      const since = failingSince(service);
      items.push({
        id: `svc-${service.key}`,
        severity: "down",
        source: "service",
        label: service.title,
        detail: `${humanizeHealthError(service.health.error, service.health.code)}${since ? ` · down ${since}` : ""}`,
        rawDetail: service.health.error,
        serviceKey: service.key,
        href: service.url,
        openInNewTab: service.openInNewTab,
      });
    } else if (service.health?.state === "slow") {
      items.push({
        id: `svc-${service.key}`,
        severity: "slow",
        source: "service",
        label: service.title,
        detail: `slow · ${service.health.latencyMs}ms`,
        serviceKey: service.key,
        href: service.url,
        openInNewTab: service.openInNewTab,
      });
    }
  }

  const kube = input.kube;
  if (kube?.available) {
    for (const pod of kube.podsNotReady) {
      items.push({
        id: `pod-${pod.namespace}-${pod.name}`,
        severity: "down",
        source: "k8s pod",
        label: `${pod.namespace}/${pod.name}`,
        detail: `${pod.detail} ${pod.ready}/${pod.desired}`,
      });
    }
    for (const deploy of kube.deploymentsNotReady) {
      items.push({
        id: `deploy-${deploy.namespace}-${deploy.name}`,
        severity: "slow",
        source: "k8s deploy",
        label: `${deploy.namespace}/${deploy.name}`,
        detail: `${deploy.ready}/${deploy.desired} replicas`,
      });
    }
    for (const app of kube.argo.attention) {
      items.push({
        id: `argo-${app.name}`,
        severity: app.health === "Healthy" ? "slow" : "down",
        source: "argo cd",
        label: app.name,
        detail: `${app.sync} · ${app.health}`,
        action:
          input.actionsEnabled && app.sync !== "Synced"
            ? { label: "sync", run: () => input.onArgoSync(app.name) }
            : undefined,
      });
    }
    for (const cert of kube.certificates) {
      items.push({
        id: `cert-${cert.namespace}-${cert.name}`,
        severity: !cert.ready || cert.daysLeft < 3 ? "down" : "slow",
        source: "certificate",
        label: `${cert.namespace}/${cert.name}`,
        detail: cert.ready ? `expires in ${cert.daysLeft}d` : "not ready",
      });
    }
    for (const volume of kube.longhorn.attention) {
      items.push({
        id: `lh-${volume.name}`,
        severity: "slow",
        source: "longhorn",
        label: volume.name,
        detail: volume.detail,
      });
    }
    for (const node of kube.rebootRequired) {
      items.push({
        id: `reboot-${node}`,
        severity: "slow",
        source: "kured",
        label: node,
        detail: "reboot pending",
      });
    }
  }

  if (input.gatusData?.available) {
    for (const endpoint of input.gatusData.endpoints.filter((e) => !e.up)) {
      items.push({
        id: `gatus-${endpoint.group}-${endpoint.name}`,
        severity: "down",
        source: "gatus",
        label: endpoint.name,
        detail: `${endpoint.group} · uptime ${endpoint.uptime.toFixed(0)}%`,
      });
    }
  }

  if (input.proxmox?.available) {
    for (const node of input.proxmox.nodes.filter((n) => !n.online)) {
      items.push({
        id: `pve-${node.name}`,
        severity: "down",
        source: "proxmox",
        label: node.name,
        detail: "host offline",
      });
    }
  }

  if (input.truenas?.available) {
    for (const pool of input.truenas.pools.filter((p) => !p.healthy)) {
      items.push({
        id: `pool-${pool.name}`,
        severity: "down",
        source: "truenas",
        label: pool.name,
        detail: pool.status,
      });
    }
    if (input.truenas.alerts > 0) {
      items.push({
        id: "truenas-alerts",
        severity: "slow",
        source: "truenas",
        label: "Active alerts",
        detail: `${input.truenas.alerts} undismissed`,
      });
    }
  }

  if (input.pbs?.available && input.pbs.hasBackups && !input.pbs.lastBackupOk) {
    items.push({
      id: "pbs-last-backup",
      severity: "down",
      source: "backups",
      label: "Last backup failed",
      detail: new Date(input.pbs.lastBackupAt * 1000).toLocaleString(),
    });
  }

  return items.sort((a, b) => (a.severity === b.severity ? a.label.localeCompare(b.label) : a.severity === "down" ? -1 : 1));
}

export function AttentionSection({
  items,
  onRecord,
}: {
  items: AttentionItem[];
  onRecord?: (serviceKey: string) => void;
}) {
  const [expanded, setExpanded] = useState(false);
  if (items.length === 0) return null;

  const visible = expanded ? items : items.slice(0, ATTENTION_PREVIEW);
  const groups: { source: string; items: AttentionItem[] }[] = [];
  for (const item of visible) {
    const group = groups.find((g) => g.source === item.source);
    if (group) group.items.push(item);
    else groups.push({ source: item.source, items: [item] });
  }
  const singleSource = groups.length === 1;

  return (
    <section className="mt-5 rounded-xl border border-line bg-surface" aria-label="Needs attention">
      <div className="flex items-baseline gap-2.5 px-4 pt-3">
        <h2 className="text-[15px] font-semibold text-ink">Needs attention</h2>
        <span className="font-mono text-[11px] text-ink-3">{items.length}</span>
        <span className="h-px flex-1 bg-line" />
      </div>
      <div className="py-1.5">
        {groups.map((group) => (
          <div key={group.source}>
            {!singleSource && (
              <p className="px-4 pt-2 font-mono text-[10px] lowercase text-ink-3 first:pt-0">
                {group.source} · {group.items.length}
              </p>
            )}
            <ul>
              {group.items.map((item) => (
                <li key={item.id} className="flex flex-wrap items-center gap-x-2 gap-y-0.5 px-4 py-1.5 text-[13px]">
                  <StatusDot state={item.severity} />
                  {item.href ? (
                    <a
                      href={item.href}
                      target={item.openInNewTab ? "_blank" : undefined}
                      rel="noopener"
                      onClick={() => item.serviceKey && onRecord?.(item.serviceKey)}
                      className="max-w-72 shrink-0 truncate font-medium text-ink underline-offset-2 hover:underline focus-visible:ring-2 focus-visible:ring-accent rounded-sm outline-none"
                      title={`Open ${item.label}`}
                    >
                      {item.label}
                    </a>
                  ) : (
                    <span className="max-w-72 shrink-0 truncate text-ink">{item.label}</span>
                  )}
                  <span
                    className="hidden min-w-0 flex-1 truncate font-mono text-[11px] text-ink-2 sm:block"
                    title={item.rawDetail ?? item.detail}
                  >
                    {item.detail}
                  </span>
                  {item.action && <ActionButton onClick={item.action.run}>{item.action.label}</ActionButton>}
                  <span
                    className="w-full truncate pl-4 font-mono text-[11px] text-ink-2 sm:hidden"
                    title={item.rawDetail ?? item.detail}
                  >
                    {item.detail}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      {items.length > ATTENTION_PREVIEW && (
        <button
          type="button"
          onClick={() => setExpanded((v) => !v)}
          aria-expanded={expanded}
          className="w-full border-t border-line px-4 py-2 text-left font-mono text-[11px] text-ink-2 transition-colors hover:text-ink"
        >
          {expanded ? "Show fewer" : `Show all ${items.length}`}
        </button>
      )}
    </section>
  );
}
