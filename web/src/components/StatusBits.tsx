import type { HealthSample, HealthState, ServiceHealth } from "../types";

const STATE_LABEL: Record<HealthState, string> = {
  up: "Up",
  down: "Down",
  slow: "Slow",
  unknown: "Unknown",
};

export function StatusDot({ state, pulse = false }: { state: HealthState; pulse?: boolean }) {
  const color =
    state === "up"
      ? "bg-up"
      : state === "down"
        ? "bg-down"
        : state === "slow"
          ? "bg-slow"
          : "bg-ink-3";
  return (
    <span className="relative inline-flex h-2 w-2 shrink-0" title={STATE_LABEL[state]} role="img" aria-label={STATE_LABEL[state]}>
      {pulse && state === "down" && (
        <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-down opacity-60" aria-hidden="true" />
      )}
      <span className={`relative inline-flex h-2 w-2 rounded-full ${color}`} aria-hidden="true" />
      <span className="sr-only">{STATE_LABEL[state]}</span>
    </span>
  );
}

export function stateTextClass(state: HealthState): string {
  return state === "up"
    ? "text-up"
    : state === "down"
      ? "text-down"
      : state === "slow"
        ? "text-slow"
        : "text-ink-3";
}

/** Row of history ticks, oldest to newest. */
export function UptimeTicks({ history, limit = 30 }: { history: HealthSample[]; limit?: number }) {
  const samples = history.slice(-limit);
  if (samples.length < 3) return null;
  return (
    <span className="flex items-end gap-[2px]" aria-hidden="true">
      {samples.map((sample, i) => (
        <span
          key={`${sample.at}-${i}`}
          className={`w-[3px] rounded-[1px] ${
            sample.state === "up"
              ? "bg-up/70"
              : sample.state === "slow"
                ? "bg-slow/80"
                : sample.state === "down"
                  ? "bg-down"
                  : "bg-ink-3/50"
          }`}
          style={{ height: sample.state === "down" ? 12 : 8 }}
        />
      ))}
    </span>
  );
}

/** Tiny latency sparkline as an inline SVG polyline. */
export function Sparkline({ history, width = 120, height = 28 }: { history: HealthSample[]; width?: number; height?: number }) {
  const points = history.slice(-40);
  if (points.length < 2) return null;

  const max = Math.max(...points.map((p) => p.latencyMs), 50);
  const stepX = width / (points.length - 1);
  const coords = points
    .map((p, i) => `${(i * stepX).toFixed(1)},${(height - 3 - (p.latencyMs / max) * (height - 6)).toFixed(1)}`)
    .join(" ");

  return (
    <svg width={width} height={height} className="text-accent" aria-hidden="true">
      <polyline points={coords} fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
    </svg>
  );
}

export function healthSummary(statuses: Record<string, ServiceHealth>): {
  up: number;
  down: number;
  slow: number;
  total: number;
} {
  let up = 0;
  let down = 0;
  let slow = 0;
  const values = Object.values(statuses);
  for (const status of values) {
    if (status.state === "up") up++;
    else if (status.state === "down") down++;
    else if (status.state === "slow") slow++;
  }
  return { up, down, slow, total: values.length };
}
