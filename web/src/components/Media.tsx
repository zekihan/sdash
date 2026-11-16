import type { CalendarData, MediaRailData } from "../types";

function dayLabel(date: Date): string {
  const today = new Date();
  const sameDay = (a: Date, b: Date) =>
    a.getFullYear() === b.getFullYear() && a.getMonth() === b.getMonth() && a.getDate() === b.getDate();
  if (sameDay(date, today)) return "Today";
  const tomorrow = new Date(today);
  tomorrow.setDate(today.getDate() + 1);
  if (sameDay(date, tomorrow)) return "Tomorrow";
  return date.toLocaleDateString([], { weekday: "short", month: "short", day: "numeric" });
}

interface DayGroup {
  label: string;
  items: CalendarData["items"];
}

function groupByDay(items: CalendarData["items"]): DayGroup[] {
  const groups: DayGroup[] = [];
  for (const item of items) {
    const label = dayLabel(new Date(item.airDate));
    const last = groups[groups.length - 1];
    if (last && last.label === label) last.items.push(item);
    else groups.push({ label, items: [item] });
  }
  return groups;
}

/** Sonarr + Radarr "coming this week" rail, one column per day. */
export function CalendarRail({ calendar }: { calendar: CalendarData }) {
  if (!calendar.available || calendar.items.length === 0) return null;
  const days = groupByDay(calendar.items);
  return (
    <section className="mt-8" aria-label="Coming this week">
      <div className="flex items-baseline gap-2.5">
        <h2 className="text-[15px] font-semibold text-ink">Coming this week</h2>
        <span className="font-mono text-[11px] text-ink-3">{calendar.items.length}</span>
        <span className="h-px flex-1 bg-line" />
      </div>
      <div className="mt-3 flex gap-3 overflow-x-auto pb-2">
        {days.map((day) => (
          <div key={day.label} className="w-64 shrink-0 rounded-[10px] border border-line bg-surface p-3">
            <p className="font-mono text-[11px] lowercase text-ink-3">{day.label}</p>
            <ul className="mt-2 space-y-2.5">
              {day.items.map((item, index) => (
                <li key={`${item.title}-${index}`} className="min-w-0">
                  <p className="truncate text-[13px] font-medium text-ink">
                    {item.title}
                    {item.hasFile && <span className="ml-1.5 font-mono text-[10px] text-up">downloaded</span>}
                  </p>
                  <p className="truncate font-mono text-[11px] text-ink-3">
                    {item.source} · {item.detail}
                  </p>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </section>
  );
}

/** Jellyfin recently-added posters and Immich on-this-day photos. */
export function MediaRails({ rail }: { rail: MediaRailData }) {
  if (!rail.available) return null;
  return (
    <>
      {rail.jellyfin.length > 0 && (
        <section className="mt-8" aria-label="Recently added">
          <div className="flex items-baseline gap-2.5">
            <h2 className="text-[15px] font-semibold text-ink">Recently added</h2>
            <span className="font-mono text-[11px] text-ink-3">jellyfin</span>
            <span className="h-px flex-1 bg-line" />
          </div>
          <div className="mt-3 flex gap-3 overflow-x-auto pb-2">
            {rail.jellyfin.map((item) => (
              <figure key={item.id} className="w-28 shrink-0">
                <img
                  src={item.image}
                  alt=""
                  loading="lazy"
                  className="aspect-[2/3] w-full rounded-[8px] border border-line object-cover"
                />
                <figcaption className="mt-1.5 min-w-0">
                  <span className="block truncate text-[12px] font-medium text-ink">{item.title}</span>
                  <span className="block truncate font-mono text-[10px] text-ink-3">{item.detail}</span>
                </figcaption>
              </figure>
            ))}
          </div>
        </section>
      )}

      {rail.immich.length > 0 && (
        <section className="mt-8" aria-label="On this day">
          <div className="flex items-baseline gap-2.5">
            <h2 className="text-[15px] font-semibold text-ink">On this day</h2>
            <span className="font-mono text-[11px] text-ink-3">immich</span>
            <span className="h-px flex-1 bg-line" />
          </div>
          <div className="mt-3 flex gap-3 overflow-x-auto pb-2">
            {rail.immich.map((item) => (
              <figure key={item.id} className="w-40 shrink-0">
                <img
                  src={item.image}
                  alt=""
                  loading="lazy"
                  className="aspect-[4/3] w-full rounded-[8px] border border-line object-cover"
                />
                <figcaption className="mt-1.5 min-w-0">
                  <span className="block truncate text-[12px] font-medium text-ink">{item.title}</span>
                  <span className="block truncate font-mono text-[10px] text-ink-3">{item.detail}</span>
                </figcaption>
              </figure>
            ))}
          </div>
        </section>
      )}
    </>
  );
}
