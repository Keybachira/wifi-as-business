import { useEffect, useMemo, useRef, useState } from "react";
import type { MouseEvent as MEvent, ReactNode } from "react";
import { SvgIcon } from "./icons";
import type { Bandwidth } from "./data";

export interface ToastAction {
  label: string;
  onClick: () => void;
}
export interface ToastItem {
  id: number;
  message: string;
  type: "ok" | "error" | "warn";
  action?: ToastAction;
}
export interface CommandItem {
  id: string;
  label: string;
  icon?: string;
  type?: "page" | "user" | string;
}

export function Sparkline({
  series,
  color = "#3b82f6",
}: {
  series: number[];
  color?: string;
}) {
  const w = 88;
  const h = 28;
  const max = Math.max(...series);
  const min = Math.min(...series);
  const pts = series
    .map((v, i) => {
      const x = (i / (series.length - 1)) * w;
      const y = h - ((v - min) / (max - min || 1)) * (h - 4) - 2;
      return `${x},${y}`;
    })
    .join(" ");
  return (
    <svg className="spark" viewBox={`0 0 ${w} ${h}`} aria-hidden="true">
      <polyline points={pts} fill="none" stroke={color} strokeWidth="1.6" />
    </svg>
  );
}

export function AreaChart({
  data,
  hover,
  setHover,
}: {
  data: Bandwidth[];
  hover: number | null;
  setHover: (i: number | null) => void;
}) {
  const wrap = useRef<HTMLDivElement | null>(null);
  const w = 640;
  const h = 220;
  const pad = { l: 36, r: 12, t: 12, b: 28 };
  const max = Math.max(...data.flatMap((d) => [d.download, d.upload])) * 1.1;
  const x = (i: number) =>
    pad.l + (i / (data.length - 1)) * (w - pad.l - pad.r);
  const y = (v: number) => pad.t + (1 - v / max) * (h - pad.t - pad.b);
  const path = (key: "download" | "upload") =>
    data.map((d, i) => `${i === 0 ? "M" : "L"} ${x(i)} ${y(d[key])}`).join(" ");
  const area = (key: "download" | "upload") =>
    `${path(key)} L ${x(data.length - 1)} ${y(0)} L ${x(0)} ${y(0)} Z`;

  function onMove(e: MEvent<HTMLDivElement>) {
    const rect = wrap.current!.getBoundingClientRect();
    const rel = (e.clientX - rect.left) / rect.width;
    const i = Math.min(
      data.length - 1,
      Math.max(0, Math.round(rel * (data.length - 1))),
    );
    setHover(i);
  }

  const i = hover ?? data.length - 1;
  const d = data[i];

  return (
    <div
      className="chart-wrap"
      ref={wrap}
      onMouseMove={onMove}
      onMouseLeave={() => setHover(null)}
    >
      <svg
        viewBox={`0 0 ${w} ${h}`}
        role="img"
        aria-label="Bandwidth usage over 24 hours"
      >
        {[0, 0.5, 1].map((t) => (
          <g key={t}>
            <line
              x1={pad.l}
              x2={w - pad.r}
              y1={y(max * t)}
              y2={y(max * t)}
              stroke="rgba(255,255,255,0.05)"
            />
            <text x={4} y={y(max * t) + 4} fill="#5c6169" fontSize="10">
              {Math.round(max * t)}
            </text>
          </g>
        ))}
        {data.map((pt, idx) =>
          idx % 4 === 0 ? (
            <text
              key={pt.hour}
              x={x(idx) - 10}
              y={h - 8}
              fill="#5c6169"
              fontSize="10"
            >
              {pt.hour}
            </text>
          ) : null,
        )}
        <path d={area("download")} fill="rgba(59,130,246,0.12)" />
        <path
          d={path("download")}
          fill="none"
          stroke="#3b82f6"
          strokeWidth="1.8"
        />
        <path
          d={path("upload")}
          fill="none"
          stroke="#22c55e"
          strokeWidth="1.8"
          strokeDasharray="4 3"
        />
        <line
          x1={x(i)}
          x2={x(i)}
          y1={pad.t}
          y2={h - pad.b}
          stroke="rgba(255,255,255,0.18)"
        />
        <circle cx={x(i)} cy={y(d.download)} r="3.5" fill="#3b82f6" />
        <circle cx={x(i)} cy={y(d.upload)} r="3.5" fill="#22c55e" />
      </svg>
      {hover != null && (
        <div
          className="tooltip"
          style={{
            left: Math.min(
              (x(i) * (wrap.current?.clientWidth || w)) / w + 8,
              (wrap.current?.clientWidth || 280) - 140,
            ),
            top: 16,
          }}
        >
          <strong>{d.hour}</strong>
          <div>Download {d.download} Mbps</div>
          <div>Upload {d.upload} Mbps</div>
        </div>
      )}
    </div>
  );
}

export function Bars({
  values,
  labels,
}: {
  values: number[];
  labels: string[];
}) {
  const max = Math.max(...values);
  const w = 640;
  const h = 180;
  const bw = (w - 40) / values.length - 6;
  return (
    <svg viewBox={`0 0 ${w} ${h}`} role="img" aria-label="Users per hour">
      {values.map((v, i) => {
        const bh = (v / max) * (h - 40);
        return (
          <g key={i}>
            <rect
              x={24 + i * ((w - 40) / values.length)}
              y={h - 20 - bh}
              width={bw}
              height={bh}
              rx="2"
              fill={i === 13 ? "#3b82f6" : "rgba(59,130,246,0.45)"}
            />
            {i % 3 === 0 && (
              <text
                x={24 + i * ((w - 40) / values.length)}
                y={h - 6}
                fill="#5c6169"
                fontSize="9"
              >
                {labels[i]}
              </text>
            )}
          </g>
        );
      })}
    </svg>
  );
}

export function Modal({
  title,
  hint,
  children,
  onClose,
}: {
  title: string;
  hint?: string;
  children: ReactNode;
  onClose: () => void;
}) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);
  return (
    <div className="modal-bg" onClick={onClose} role="presentation">
      <div
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
        onClick={(e) => e.stopPropagation()}
      >
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            alignItems: "flex-start",
          }}
        >
          <div>
            <h2 id="modal-title">{title}</h2>
            {hint && <p className="hint">{hint}</p>}
          </div>
          <button className="icon-btn" onClick={onClose} aria-label="Close">
            <SvgIcon name="close" />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

export function Drawer({
  title,
  children,
  onClose,
}: {
  title: string;
  children: ReactNode;
  onClose: () => void;
}) {
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);
  return (
    <div className="drawer-bg" onClick={onClose}>
      <aside
        className="drawer"
        role="dialog"
        aria-label={title}
        onClick={(e) => e.stopPropagation()}
      >
        <div
          style={{
            display: "flex",
            justifyContent: "space-between",
            marginBottom: 16,
          }}
        >
          <strong>{title}</strong>
          <button
            className="icon-btn"
            onClick={onClose}
            aria-label="Close panel"
          >
            <SvgIcon name="close" />
          </button>
        </div>
        {children}
      </aside>
    </div>
  );
}

export function Toasts({
  items,
  dismiss,
}: {
  items: ToastItem[];
  dismiss: (id: number) => void;
}) {
  if (!items.length) return null;
  return (
    <div className="toasts" aria-live="polite">
      {items.map((t) => (
        <div className="toast" key={t.id} role="status">
          <span
            className={`dot ${t.type === "error" ? "danger" : t.type === "warn" ? "warn" : ""}`}
          />
          <div style={{ flex: 1 }}>
            <div>{t.message}</div>
            {t.action && (
              <button
                className="btn ghost sm"
                style={{ marginTop: 6 }}
                onClick={() => {
                  t.action?.onClick();
                  dismiss(t.id);
                }}
              >
                {t.action.label}
              </button>
            )}
          </div>
          <button
            className="icon-btn"
            aria-label="Dismiss"
            onClick={() => dismiss(t.id)}
          >
            <SvgIcon name="close" size={14} />
          </button>
        </div>
      ))}
    </div>
  );
}

export function CommandMenu({
  open,
  onClose,
  items,
  onSelect,
}: {
  open: boolean;
  onClose: () => void;
  items: CommandItem[];
  onSelect: (item: CommandItem) => void;
}) {
  const [q, setQ] = useState("");
  const [idx, setIdx] = useState(0);
  const filtered = useMemo(
    () => items.filter((i) => i.label.toLowerCase().includes(q.toLowerCase())),
    [items, q],
  );
  useEffect(() => {
    if (open) {
      setQ("");
      setIdx(0);
    }
  }, [open]);
  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setIdx((n) => Math.min(filtered.length - 1, n + 1));
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setIdx((n) => Math.max(0, n - 1));
      }
      if (e.key === "Enter" && filtered[idx]) {
        e.preventDefault();
        onSelect(filtered[idx]);
      }
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, filtered, idx, onClose, onSelect]);
  if (!open) return null;
  return (
    <div className="cmd-bg" onClick={onClose}>
      <div
        className="cmd"
        onClick={(e) => e.stopPropagation()}
        role="dialog"
        aria-label="Command menu"
      >
        <input
          autoFocus
          placeholder="Search pages, users, vouchers…"
          value={q}
          onChange={(e) => {
            setQ(e.target.value);
            setIdx(0);
          }}
        />
        <div className="cmd-list">
          {filtered.length === 0 && (
            <div className="empty" style={{ padding: 24 }}>
              No results
            </div>
          )}
          {filtered.map((item, i) => (
            <button
              key={item.id}
              className={`cmd-item ${i === idx ? "active" : ""}`}
              onMouseEnter={() => setIdx(i)}
              onClick={() => onSelect(item)}
            >
              <SvgIcon name={item.icon || "search"} />
              {item.label}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

export function SkeletonBlock() {
  return (
    <div className="kpi-grid">
      {[1, 2, 3, 4].map((n) => (
        <div className="card kpi" key={n}>
          <div className="skeleton" style={{ height: 12, width: 80 }} />
          <div className="skeleton" style={{ height: 28, width: 120 }} />
          <div className="skeleton" style={{ height: 12, width: 60 }} />
        </div>
      ))}
    </div>
  );
}
