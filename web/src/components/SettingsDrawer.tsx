import { useRef } from "react";
import { ACCENTS, searchShortcutLabel, type Prefs, type ThemePref } from "../lib/prefs";
import { useFocusTrap } from "../lib/useFocusTrap";
import { CloseIcon } from "./icons";

interface Props {
  open: boolean;
  prefs: Prefs;
  prefsSync: boolean;
  onChange: (prefs: Prefs) => void;
  onClose: () => void;
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 py-3">
      <span className="text-[13px] text-ink-2">{label}</span>
      <span className="flex items-center gap-1.5">{children}</span>
    </div>
  );
}

function Segment<T extends string>({
  value,
  options,
  onPick,
}: {
  value: T;
  options: { value: T; label: string }[];
  onPick: (value: T) => void;
}) {
  return (
    <span className="flex rounded-lg border border-line bg-surface-2 p-0.5">
      {options.map((option) => (
        <button
          key={option.value}
          type="button"
          onClick={() => onPick(option.value)}
          aria-pressed={value === option.value}
          className={`rounded-md px-2.5 py-1 text-[12px] ${
            value === option.value ? "bg-surface font-medium text-ink shadow-sm" : "text-ink-3 hover:text-ink-2"
          }`}
        >
          {option.label}
        </button>
      ))}
    </span>
  );
}

const ACCENT_HEX: Record<string, string> = {
  lime: "#84cc16",
  emerald: "#10b981",
  cyan: "#06b6d4",
  sky: "#0ea5e9",
  amber: "#f59e0b",
  orange: "#f97316",
  rose: "#f43f5e",
};

export function SettingsDrawer({ open, prefs, prefsSync, onChange, onClose }: Props) {
  const asideRef = useRef<HTMLElement>(null);
  useFocusTrap(asideRef, open, onClose);
  if (!open) return null;

  const set = (patch: Partial<Prefs>) => onChange({ ...prefs, ...patch });

  return (
    <div
      className="fixed inset-0 z-50 bg-black/45"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
      role="dialog"
      aria-modal="true"
      aria-label="Settings"
    >
      <aside ref={asideRef} tabIndex={-1} className="rise absolute top-0 right-0 h-full w-full max-w-sm overflow-y-auto border-l border-line-strong bg-surface p-5 shadow-2xl outline-none">
        <div className="flex items-center justify-between">
          <h2 className="font-semibold text-ink">Settings</h2>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close settings"
            className="rounded-lg border border-line bg-surface-2 p-1.5 text-ink-2 hover:text-ink"
          >
            <CloseIcon size={15} />
          </button>
        </div>
        <p className="mt-1 text-[12px] text-ink-3">
          {prefsSync
            ? "Synced to the server and this browser. Services and groups come from the server's dashboard.yaml."
            : "Saved in this browser. Services and groups come from the server's dashboard.yaml."}
        </p>

        <div className="mt-3 divide-y divide-line">
          <Row label="Theme">
            <Segment<ThemePref>
              value={prefs.theme}
              onPick={(theme) => set({ theme })}
              options={[
                { value: "dark", label: "Dark" },
                { value: "light", label: "Light" },
                { value: "system", label: "System" },
              ]}
            />
          </Row>

          <Row label="Accent">
            <span className="flex gap-1.5">
              {ACCENTS.map((accent) => (
                <button
                  key={accent}
                  type="button"
                  title={accent}
                  aria-label={`${accent} accent`}
                  aria-pressed={prefs.accent === accent}
                  onClick={() => set({ accent })}
                  className={`h-6 w-6 rounded-full ${prefs.accent === accent ? "ring-2 ring-ink ring-offset-2 ring-offset-surface" : "ring-1 ring-line"}`}
                  style={{ background: ACCENT_HEX[accent] }}
                />
              ))}
            </span>
          </Row>

          <Row label="Density">
            <Segment
              value={prefs.density}
              onPick={(density) => set({ density })}
              options={[
                { value: "comfortable", label: "Comfortable" },
                { value: "compact", label: "Compact" },
              ]}
            />
          </Row>

          <Row label="Layout">
            <Segment
              value={prefs.layout}
              onPick={(layout) => set({ layout })}
              options={[
                { value: "grid", label: "Grid" },
                { value: "list", label: "List" },
              ]}
            />
          </Row>

          <Row label="Docker widget">
            <Segment
              value={prefs.showDocker ? "on" : "off"}
              onPick={(v) => set({ showDocker: v === "on" })}
              options={[
                { value: "on", label: "Shown" },
                { value: "off", label: "Hidden" },
              ]}
            />
          </Row>

          <Row label="Immich background">
            <Segment
              value={prefs.immichBackground ? "on" : "off"}
              onPick={(v) => set({ immichBackground: v === "on" })}
              options={[
                { value: "off", label: "Off" },
                { value: "on", label: "Memories" },
              ]}
            />
          </Row>
        </div>

        <div className="mt-5 border-t border-line pt-4">
          <button
            type="button"
            onClick={() => set({ pinned: [], recents: [] })}
            className="rounded-lg border border-line bg-surface-2 px-3 py-1.5 text-[12px] text-ink-2 hover:text-ink"
          >
            Clear favorites & recents
          </button>
          <p className="mt-4 font-mono text-[11px] leading-relaxed text-ink-3">
            Shortcuts: {searchShortcutLabel()} or / search · ↑↓ choose · ↵ open · esc close
          </p>
        </div>
      </aside>
    </div>
  );
}
