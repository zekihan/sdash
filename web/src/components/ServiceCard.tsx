import { useState } from "react";
import type { Service, ServiceWidget } from "../types";
import { hostOf, humanizeHealthError } from "../lib/format";
import { CopyIcon, ExternalIcon, PinIcon } from "./icons";
import { Sparkline, StatusDot, UptimeTicks, stateTextClass } from "./StatusBits";

interface Props {
  service: Service;
  liveWidget?: ServiceWidget;
  density: "comfortable" | "compact";
  layout: "grid" | "list";
  pinned: boolean;
  showHealth: boolean;
  actionsEnabled: boolean;
  onToggleControl: (entityId: string, label: string) => void;
  onTogglePin: (key: string) => void;
  onOpen: (service: Service) => void;
  onRecord: (service: Service) => void;
  onCopy: (text: string) => void;
}

function initials(title: string): string {
  return title
    .split(/\s+/)
    .map((w) => w.replace(/[^\p{L}\p{N}]/gu, ""))
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("");
}

function ServiceIcon({ service, size }: { service: Service; size: number }) {
  const [broken, setBroken] = useState(false);
  if (service.icon && !broken) {
    return (
      <img
        src={service.icon}
        alt=""
        width={size}
        height={size}
        loading="lazy"
        onError={() => setBroken(true)}
        className="shrink-0 rounded-[6px] object-contain"
        style={{ width: size, height: size }}
      />
    );
  }
  return (
    <span
      className="flex shrink-0 items-center justify-center rounded-[6px] bg-surface-2 font-mono font-medium text-ink-2 ring-1 ring-line"
      style={{ width: size, height: size, fontSize: size * 0.36 }}
      aria-hidden="true"
    >
      {initials(service.title)}
    </span>
  );
}

export function ServiceCard({
  service,
  liveWidget,
  density,
  layout,
  pinned,
  showHealth,
  actionsEnabled,
  onToggleControl,
  onTogglePin,
  onOpen,
  onRecord,
  onCopy,
}: Props) {
  const [expanded, setExpanded] = useState(false);
  const health = showHealth ? service.health : undefined;
  const compact = density === "compact";
  const isList = layout === "list";

  return (
    <article
      className={`group relative flex flex-col gap-3 rounded-[10px] border bg-surface ${compact || isList ? "p-3" : "p-4"} transition-colors hover:bg-surface-2 ${
        health?.state === "down" ? "border-down/50 hover:border-down/70" : "border-line hover:border-line-strong"
      }`}
    >
      <div className="flex min-w-0 items-start gap-3">
        <a
          href={service.url}
          target={service.openInNewTab ? "_blank" : undefined}
          rel="noopener"
          onClick={() => onRecord(service)}
          className="flex min-w-0 flex-1 items-start gap-3 text-left outline-none focus-visible:ring-2 focus-visible:ring-accent rounded-[6px] [@media(hover:none)]:pr-24"
          title={`Open ${service.title}`}
        >
          <ServiceIcon service={service} size={compact || isList ? 28 : 36} />
          <span className="min-w-0">
            <span className="flex items-center gap-2">
              <span className="truncate font-medium text-ink">{service.title}</span>
              {health && <StatusDot state={health.state} />}
              {health && health.state !== "up" && health.state !== "unknown" && (
                <span className={`font-mono text-[10px] ${stateTextClass(health.state)}`}>
                  {health.state}
                </span>
              )}
            </span>
            {!compact && service.description && (
              <span className="mt-0.5 line-clamp-2 block text-[13px] leading-snug text-ink-2">{service.description}</span>
            )}
            <span className="mt-0.5 block truncate font-mono text-[11px] text-ink-3">
              {hostOf(service.url)}
              {service.location ? ` · ${service.location}` : ""}
              {service.cluster ? ` · ${service.cluster}` : ""}
            </span>
            {health?.state === "down" && (
              <span className="mt-0.5 block truncate text-[11px] text-down" title={health.error}>
                {humanizeHealthError(health.error, health.code)}
              </span>
            )}
          </span>
        </a>

        <span className="absolute top-2.5 right-2.5 flex shrink-0 items-center gap-1 rounded-lg bg-surface/90 opacity-100 transition-opacity focus-within:pointer-events-auto focus-within:opacity-100 [@media(hover:hover)]:pointer-events-none [@media(hover:hover)]:opacity-0 [@media(hover:hover)]:group-hover:pointer-events-auto [@media(hover:hover)]:group-hover:opacity-100">
          <button
            type="button"
            onClick={() => onTogglePin(service.key)}
            className={`rounded-md p-1.5 hover:bg-surface ${pinned ? "text-accent opacity-100" : "text-ink-3 hover:text-ink"}`}
            title={pinned ? "Unpin from favorites" : "Pin to favorites"}
            aria-label={pinned ? "Unpin from favorites" : "Pin to favorites"}
            aria-pressed={pinned}
          >
            <PinIcon size={14} />
          </button>
          <button
            type="button"
            onClick={() => onCopy(service.url)}
            className="rounded-md p-1.5 text-ink-3 hover:bg-surface hover:text-ink"
            aria-label="Copy URL"
            title="Copy URL"
          >
            <CopyIcon size={14} />
          </button>
          <button
            type="button"
            onClick={() => onOpen(service)}
            className="rounded-md p-1.5 text-ink-3 hover:bg-surface hover:text-ink"
            aria-label={`Open ${service.title}`}
            title="Open"
          >
            <ExternalIcon size={14} />
          </button>
        </span>
      </div>

      {layout === "grid" && (
        <div className="mt-auto flex items-end justify-between gap-2">
          <span className="flex min-w-0 flex-wrap gap-1">
            {service.tags.slice(0, 3).map((tag) => (
              <span key={tag} className="rounded-full border border-line px-1.5 py-px font-mono text-[10px] text-ink-3">
                {tag}
              </span>
            ))}
          </span>
          {health && (
            <button
              type="button"
              onClick={() => setExpanded((v) => !v)}
              className="flex shrink-0 items-center gap-2 rounded-md px-1 py-0.5 font-mono text-[11px] hover:bg-surface-2"
              title="Health detail"
              aria-expanded={expanded}
            >
              <UptimeTicks history={health.history} limit={density === "compact" ? 16 : 24} />
              <span className={stateTextClass(health.state)}>
                {health.state === "unknown" ? "—" : health.state === "down" ? "down" : `${health.latencyMs}ms`}
              </span>
            </button>
          )}
        </div>
      )}

      {isList &&
        liveWidget &&
        (liveWidget.stats.length > 0 ||
          liveWidget.controls.length > 0 ||
          liveWidget.sensors.length > 0 ||
          liveWidget.lines.length > 0) && (
        <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
          {liveWidget.sensors.slice(0, 3).map((sensor) => (
            <span key={sensor.id} className="font-mono text-[11px] text-ink-3">
              {sensor.label} <span className="text-[12px] text-ink">{sensor.value}</span>
              {sensor.unit ? <span> {sensor.unit}</span> : null}
            </span>
          ))}
          {liveWidget.stats.slice(0, 3).map((stat) => (
            <span key={stat.label} className="font-mono text-[11px] text-ink-3">
              {stat.label} <span className="text-[12px] text-ink">{stat.value}</span>
            </span>
          ))}
          {liveWidget.controls.map((control) => (
            <button
              key={control.id}
              type="button"
              disabled={!actionsEnabled}
              onClick={() => onToggleControl(control.id, control.label)}
              aria-pressed={control.on}
              title={actionsEnabled ? `Toggle ${control.label}` : "Actions are disabled on the server"}
              className={`rounded-full border px-2 py-0.5 text-[11px] ${
                control.on ? "border-accent bg-accent-soft text-ink" : "border-line text-ink-3"
              } ${actionsEnabled ? "" : "cursor-not-allowed opacity-60"}`}
            >
              {control.label} · {control.state}
            </button>
          ))}
          {liveWidget.stats.length === 0 && liveWidget.controls.length === 0 && liveWidget.lines[0] && (
            <span className="min-w-0 flex-1 truncate text-[12px] text-ink-2">{liveWidget.lines[0]}</span>
          )}
        </div>
      )}

      {isList && (
        <div className="mt-auto flex items-end justify-between gap-2 border-t border-line pt-2">
          <span className="flex min-w-0 flex-wrap gap-1">
            {service.tags.slice(0, 2).map((tag) => (
              <span key={tag} className="rounded-full border border-line px-1.5 py-px font-mono text-[10px] text-ink-3">
                {tag}
              </span>
            ))}
          </span>
          {health ? (
            <button
              type="button"
              onClick={() => setExpanded((v) => !v)}
              className="flex shrink-0 items-center gap-2 rounded-md px-1 py-0.5 font-mono text-[11px] hover:bg-surface-2"
              title="Health detail"
              aria-expanded={expanded}
            >
              <UptimeTicks history={health.history} limit={12} />
              <span className={stateTextClass(health.state)}>
                {health.state === "unknown" ? "—" : health.state === "down" ? "down" : `${health.latencyMs}ms`}
              </span>
            </button>
          ) : (
            <span className="font-mono text-[11px] text-ink-3">no health data</span>
          )}
        </div>
      )}

      {layout === "grid" &&
        liveWidget &&
        (liveWidget.stats.length > 0 ||
          liveWidget.lines.length > 0 ||
          liveWidget.controls.length > 0 ||
          liveWidget.sensors.length > 0) && (
        <div className="border-t border-line pt-2.5">
          <div className="flex flex-wrap gap-x-4 gap-y-1">
            {liveWidget.stats.map((stat) => (
              <span key={stat.label} className="font-mono text-[11px] text-ink-3">
                {stat.label} <span className="text-[13px] text-ink">{stat.value}</span>
              </span>
            ))}
          </div>
          {liveWidget.lines.length > 0 && (
            <ul className="mt-1.5 space-y-0.5">
              {liveWidget.lines.map((line) => (
                <li key={line} className="truncate text-[12px] text-ink-2">
                  <span className="mr-1.5 text-accent">▸</span>
                  {line}
                </li>
              ))}
            </ul>
          )}
          {liveWidget.sensors.length > 0 && (
            <div className="mb-1.5 flex flex-wrap gap-x-4 gap-y-1">
              {liveWidget.sensors.map((sensor) => (
                <span key={sensor.id} className="font-mono text-[11px] text-ink-3">
                  {sensor.label} <span className="text-[13px] text-ink">{sensor.value}</span>
                  {sensor.unit ? <span className="text-ink-3"> {sensor.unit}</span> : null}
                </span>
              ))}
            </div>
          )}
          {liveWidget.controls.length > 0 && (
            <div className="mt-2 flex flex-wrap gap-1.5">
              {liveWidget.controls.map((control) => (
                <button
                  key={control.id}
                  type="button"
                  disabled={!actionsEnabled}
                  onClick={() => onToggleControl(control.id, control.label)}
                  aria-pressed={control.on}
                  title={actionsEnabled ? `Toggle ${control.label}` : "Actions are disabled on the server"}
                  className={`rounded-full border px-2.5 py-1 text-[12px] transition-colors ${
                    control.on
                      ? "border-accent bg-accent-soft text-ink"
                      : "border-line text-ink-3 hover:border-line-strong hover:text-ink-2"
                  } ${actionsEnabled ? "" : "cursor-not-allowed opacity-60"}`}
                >
                  {control.label} · {control.state}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {expanded && health && (
        <div className="rise border-t border-line pt-3">
          <div className="flex items-center justify-between gap-3">
            <Sparkline history={health.history} />
            <dl className="grid grid-cols-2 gap-x-4 gap-y-1 font-mono text-[11px]">
              <dt className="text-ink-3">uptime</dt>
              <dd className="text-right text-ink">{health.uptime.toFixed(1)}%</dd>
              <dt className="text-ink-3">status</dt>
              <dd className="text-right text-ink">{health.code || "—"}</dd>
              <dt className="text-ink-3">latency</dt>
              <dd className="text-right text-ink">{health.latencyMs}ms</dd>
              <dt className="text-ink-3">checks</dt>
              <dd className="text-right text-ink">{health.history.length}</dd>
            </dl>
          </div>
          {health.error && <p className="mt-2 break-all font-mono text-[11px] text-down">{health.error}</p>}
        </div>
      )}
    </article>
  );
}
