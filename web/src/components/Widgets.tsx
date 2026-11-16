import { useEffect, useState } from "react";
import type { DockerInfo, ServiceHealth, SystemInfo, Weather } from "../types";
import { formatBytes, formatUptime, greeting } from "../lib/format";
import { BoxIcon, ChevronDownIcon } from "./icons";
import { healthSummary } from "./StatusBits";

function useNow(intervalMs = 1000): Date {
  const [now, setNow] = useState(() => new Date());
  useEffect(() => {
    const id = window.setInterval(() => setNow(new Date()), intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs]);
  return now;
}

function Meter({ label, percent, detail }: { label: string; percent: number; detail: string }) {
  const clamped = Math.min(100, Math.max(0, percent));
  return (
    <div>
      <div className="flex items-baseline justify-between font-mono text-[11px]">
        <span className="text-ink-3">{label}</span>
        <span className="text-ink-2">
          {detail} <span className="text-ink">{clamped.toFixed(0)}%</span>
        </span>
      </div>
      <div
        className="mt-1 h-1 overflow-hidden rounded-full bg-line"
        role="meter"
        aria-label={label}
        aria-valuenow={Math.round(clamped)}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div
          className={`h-full rounded-full transition-[width] duration-500 ${clamped >= 90 ? "bg-down" : clamped >= 75 ? "bg-slow" : "bg-accent"}`}
          style={{ width: `${clamped}%` }}
        />
      </div>
    </div>
  );
}

function BlockLabel({ children }: { children: React.ReactNode }) {
  return <p className="font-mono text-[11px] lowercase text-ink-3">{children}</p>;
}

export function ClockBlock({ title }: { title: string }) {
  const now = useNow();
  const pad = (value: number) => String(value).padStart(2, "0");
  const time = `${pad(now.getHours())}:${pad(now.getMinutes())}`;
  const seconds = pad(now.getSeconds());
  const date = now.toLocaleDateString([], { weekday: "long", month: "long", day: "numeric" });
  return (
    <div className="min-w-0">
      <BlockLabel>
        {greeting(now)} · {title}
      </BlockLabel>
      <p className="mt-1 font-mono stat-value text-[34px] leading-none font-medium tracking-tight text-ink">
        {time}
        <span className="ml-1 text-base text-ink-3">{seconds}</span>
      </p>
      <p className="mt-1.5 text-[13px] text-ink-2">{date}</p>
    </div>
  );
}

export function WeatherBlock({ weather }: { weather: Weather }) {
  const unit = weather.units === "imperial" ? "°F" : "°C";
  const windUnit = weather.units === "imperial" ? "mph" : "km/h";
  return (
    <div className="min-w-0">
      <BlockLabel>weather{weather.label ? ` · ${weather.label}` : ""}</BlockLabel>
      <p className="mt-1 font-mono stat-value text-[34px] leading-none font-medium tracking-tight text-ink">
        {Math.round(weather.temperature)}
        <span className="text-lg text-ink-3">{unit}</span>
      </p>
      <p className="mt-1.5 text-[13px] text-ink-2">
        {weather.description} · feels {Math.round(weather.feelsLike)}{unit} · {Math.round(weather.humidity)}% humidity ·{" "}
        {Math.round(weather.windSpeed)} {windUnit} wind
      </p>
      {weather.daily.length > 1 && (
        <p className="mt-1 font-mono text-[11px] text-ink-3">
          {weather.daily.slice(0, 4).map((day) => {
            const label = new Date(`${day.date}T12:00:00`).toLocaleDateString([], { weekday: "short" });
            return `${label} ${Math.round(day.min)}–${Math.round(day.max)}${unit}`;
          }).join("  ·  ")}
        </p>
      )}
    </div>
  );
}

export function SystemBlock({ system }: { system: SystemInfo }) {
  return (
    <div className="min-w-0">
      <BlockLabel>
        host · {system.hostname || "this machine"} · up {formatUptime(system.uptimeSec)}
      </BlockLabel>
      <div className="mt-2.5 space-y-2.5">
        <Meter label={`cpu ×${system.cpus}`} percent={system.cpuPercent} detail={`load ${system.load1.toFixed(2)}`} />
        <Meter
          label="memory"
          percent={system.memPercent}
          detail={`${formatBytes(system.memUsed)} / ${formatBytes(system.memTotal)}`}
        />
        <Meter
          label={`disk ${system.diskPath}`}
          percent={system.diskPercent}
          detail={`${formatBytes(system.diskUsed)} / ${formatBytes(system.diskTotal)}`}
        />
      </div>
    </div>
  );
}

export function ServicesBlock({
  statuses,
  total,
  onShowDown,
}: {
  statuses: Record<string, ServiceHealth>;
  total: number;
  onShowDown: () => void;
}) {
  const summary = healthSummary(statuses);
  return (
    <div className="min-w-0">
      <BlockLabel>services</BlockLabel>
      <p className="mt-1 font-mono stat-value text-[34px] leading-none font-medium tracking-tight text-ink">
        {summary.up}
        <span className="text-lg text-ink-3">/{total} up</span>
      </p>
      {total > 0 && (
        <div className="mt-2 flex h-1 overflow-hidden rounded-full bg-line" aria-hidden="true">
          <span className="bg-up" style={{ width: `${(summary.up / total) * 100}%` }} />
          <span className="bg-slow" style={{ width: `${(summary.slow / total) * 100}%` }} />
          <span className="bg-down" style={{ width: `${(summary.down / total) * 100}%` }} />
        </div>
      )}
      <p className="mt-1.5 text-[13px] text-ink-2">
        {summary.down > 0 ? (
          <button type="button" onClick={onShowDown} className="font-medium text-down underline decoration-down/50 underline-offset-2 hover:decoration-down">
            {summary.down} down
          </button>
        ) : (
          <span>0 down</span>
        )}
        {summary.slow > 0 && <span className="text-slow"> · {summary.slow} slow</span>}
        <span className="text-ink-3"> · live-probed by the server</span>
      </p>
    </div>
  );
}

export function DockerBlock({ docker, expanded, onToggle }: { docker: DockerInfo; expanded: boolean; onToggle: () => void }) {
  const running = docker.containers.filter((c) => c.state === "running").length;
  return (
    <div className="min-w-0">
      <BlockLabel>containers</BlockLabel>
      <button type="button" onClick={onToggle} className="mt-1 flex items-center gap-2 text-left" aria-expanded={expanded}>
        <span className="font-mono stat-value text-[34px] leading-none font-medium tracking-tight text-ink">
          {running}
          <span className="text-lg text-ink-3">/{docker.containers.length} running</span>
        </span>
        <ChevronDownIcon size={16} className={`text-ink-3 transition-transform ${expanded ? "rotate-180" : ""}`} />
      </button>
      <p className="mt-1.5 text-[13px] text-ink-2">{docker.available ? "Docker engine reachable" : (docker.error ?? "unavailable")}</p>
    </div>
  );
}

export function DockerList({ docker }: { docker: DockerInfo }) {
  if (!docker.available) return null;
  return (
    <div className="rise overflow-x-auto border-t border-line">
      <table className="w-full text-left text-[13px]">
        <thead>
          <tr className="font-mono text-[11px] text-ink-3">
            <th className="px-4 py-2 font-normal">container</th>
            <th className="px-4 py-2 font-normal">image</th>
            <th className="px-4 py-2 font-normal">state</th>
            <th className="px-4 py-2 font-normal">status</th>
            <th className="px-4 py-2 font-normal">ports</th>
          </tr>
        </thead>
        <tbody>
          {docker.containers.map((container) => (
            <tr key={container.id} className="border-t border-line/60">
              <td className="px-4 py-2 font-medium whitespace-nowrap text-ink">
                <span className="flex items-center gap-2">
                  <BoxIcon size={13} className="text-ink-3" />
                  {container.name}
                </span>
              </td>
              <td className="max-w-56 truncate px-4 py-2 font-mono text-[12px] whitespace-nowrap text-ink-2">{container.image}</td>
              <td className="px-4 py-2">
                <span
                  className={`rounded-full px-1.5 py-px font-mono text-[10px] ${
                    container.state === "running" ? "bg-accent-soft text-accent" : "bg-surface-2 text-ink-3"
                  }`}
                >
                  {container.state}
                </span>
              </td>
              <td className="px-4 py-2 whitespace-nowrap text-ink-2">{container.status}</td>
              <td className="px-4 py-2 font-mono text-[12px] whitespace-nowrap text-ink-3">
                {container.ports
                  .filter((p) => p.PublicPort)
                  .map((p) => `${p.PublicPort}→${p.PrivatePort}/${p.Type}`)
                  .join(" ") || "—"}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
