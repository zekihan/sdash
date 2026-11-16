export function formatBytes(bytes: number): string {
  if (!bytes) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB", "PB"];
  const i = Math.min(units.length - 1, Math.floor(Math.log(bytes) / Math.log(1024)));
  const value = bytes / 1024 ** i;
  return `${value >= 100 ? Math.round(value) : value.toFixed(1)} ${units[i]}`;
}

export function formatUptime(seconds: number): string {
  if (!seconds) return "—";
  const days = Math.floor(seconds / 86400);
  const hours = Math.floor((seconds % 86400) / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  if (days > 0) return `${days}d ${hours}h`;
  if (hours > 0) return `${hours}h ${mins}m`;
  return `${mins}m`;
}

export function hostOf(url: string): string {
  try {
    return new URL(url).host;
  } catch {
    return url;
  }
}

export function timeAgo(iso: string): string {
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return "";
  const diff = Math.max(0, Math.floor((Date.now() - then) / 1000));
  if (diff < 5) return "just now";
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  return `${Math.floor(diff / 3600)}h ago`;
}

/** Short, human reason for a failed health probe; raw text stays in tooltips. */
export function humanizeHealthError(error: string | undefined, code: number): string {
  if (!error) return code ? `HTTP ${code}` : "Unreachable";
  const text = error.toLowerCase();
  if (text.includes("no such host") || text.includes("server misbehaving")) return "DNS lookup failed";
  if (text.includes("connection refused")) return "Connection refused";
  if (text.includes("deadline exceeded") || text.includes("timeout") || text.includes("timed out")) return "Timed out";
  if (text.includes("x509") || text.includes("certificate")) return "TLS certificate error";
  if (text.includes("no route to host") || text.includes("network is unreachable")) return "Network unreachable";
  if (text.includes("connection reset")) return "Connection reset";
  if (text.includes("eof")) return "Connection closed early";
  const http = error.match(/\b([1-5]\d\d)\b/);
  if (http) return `HTTP ${http[1]}`;
  return "Unreachable";
}

export function greeting(date = new Date()): string {
  const h = date.getHours();
  if (h < 5) return "Up late";
  if (h < 12) return "Good morning";
  if (h < 18) return "Good afternoon";
  return "Good evening";
}
