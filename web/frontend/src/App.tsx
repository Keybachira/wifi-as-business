import { useCallback, useEffect, useMemo, useState } from "react"
import { SvgIcon } from "./icons"
import { AreaChart, Bars, CommandMenu, Drawer, Modal, SkeletonBlock, Sparkline, Toasts } from "./components"
import type { CommandItem, ToastItem } from "./components"
import {
  bandwidth,
  company,
  deviceShare,
  devices as devicesSeed,
  generateCodes,
  hourlyUsers,
  kpis,
  liveUsers as liveSeed,
  networks,
  plans as plansSeed,
  planShare,
  portalUsers,
  revenueSeries,
  sessions as sessionsSeed,
  team,
  vouchersSeed,
} from "./data"
import type { Device, LiveUser, Network, Plan, Voucher } from "./data"

export type PageId =
  | "overview"
  | "live"
  | "vouchers"
  | "plans"
  | "networks"
  | "devices"
  | "sessions"
  | "analytics"
  | "portal"
  | "users"
  | "team"
  | "settings"

type NavItem =
  | { section: string }
  | { id: PageId; label: string; icon: string }

interface ConfirmState {
  title: string;
  body: string;
  danger: boolean;
  action: string;
  onConfirm: () => void;
}

interface GenForm {
  qty: number | string
  plan: string
  speed: string
  data: string
  devices: number
  expiration: string
}

const NAV: NavItem[] = [
  { section: "Operate" },
  { id: "overview", label: "Overview", icon: "overview" },
  { id: "live", label: "Live Users", icon: "live" },
  { id: "vouchers", label: "Vouchers", icon: "ticket" },
  { id: "plans", label: "Plans", icon: "plans" },
  { section: "Infrastructure" },
  { id: "networks", label: "Networks", icon: "network" },
  { id: "devices", label: "Devices", icon: "devices" },
  { id: "sessions", label: "Sessions", icon: "sessions" },
  { id: "analytics", label: "Analytics", icon: "analytics" },
  { section: "Access" },
  { id: "portal", label: "Portal", icon: "portal" },
  { id: "users", label: "Users", icon: "users" },
  { id: "team", label: "Team", icon: "team" },
  { id: "settings", label: "Settings", icon: "settings" },
]

const MOBILE_NAV: { id: PageId; label: string; icon: string }[] = [
  { id: "overview", label: "Home", icon: "overview" },
  { id: "live", label: "Live", icon: "live" },
  { id: "vouchers", label: "Vouchers", icon: "ticket" },
  { id: "analytics", label: "Analytics", icon: "analytics" },
  { id: "settings", label: "More", icon: "settings" },
]

function greeting() {
  const h = new Date().getHours()
  if (h < 12) return "Good morning"
  if (h < 18) return "Good afternoon"
  return "Good evening"
}

let toastSeq = 0

export default function App() {
  const [page, setPage] = useState<PageId>("overview")
  const [network, setNetwork] = useState<Network>(networks[0])
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [cmdOpen, setCmdOpen] = useState(false)
  const [toasts, setToasts] = useState<ToastItem[]>([])
  const [loading, setLoading] = useState(true)
  const [hoverBw, setHoverBw] = useState<number | null>(null)
  const [live, setLive] = useState<LiveUser[]>(liveSeed)
  const [vouchers, setVouchers] = useState<Voucher[]>(vouchersSeed)
  const [plans, setPlans] = useState<Plan[]>(plansSeed)
  const [voucherFilter, setVoucherFilter] = useState<string>("All")
  const [genOpen, setGenOpen] = useState(false)
  const [genForm, setGenForm] = useState<GenForm>({ qty: 100, plan: "1 Hour — 100 Kz", speed: "5 Mbps", data: "1 GB", devices: 1, expiration: "30 days" })
  const [genDone, setGenDone] = useState<number | null>(null)
  const [planOpen, setPlanOpen] = useState(false)
  const [device, setDevice] = useState<Device | null>(null)
  const [confirm, setConfirm] = useState<ConfirmState | null>(null)
  const [range, setRange] = useState<string>("Today")
  const [portal, setPortal] = useState({
    name: "WiFi do Kimbo",
    message: "Enter your voucher",
    bg: "#0b1220",
    color: "#3b82f6",
    terms: "By connecting you accept the terms of use of this network.",
    success: "You are online. Enjoy your session.",
  })
  const [notifyOpen, setNotifyOpen] = useState(false)

  const toast = useCallback((message: string, type: ToastItem["type"] = "ok", action?: ToastItem["action"]) => {
    const id = ++toastSeq
    setToasts((t) => [...t, { id, message, type, action }])
    setTimeout(() => setToasts((t) => t.filter((x) => x.id !== id)), 4200)
  }, [])

  const dismiss = (id: number) => setToasts((t) => t.filter((x) => x.id !== id))

  useEffect(() => {
    const t = setTimeout(() => setLoading(false), 700)
    return () => clearTimeout(t)
  }, [page])

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === "k") {
        e.preventDefault()
        setCmdOpen(true)
      }
    }
    window.addEventListener("keydown", onKey)
    return () => window.removeEventListener("keydown", onKey)
  }, [])

  function go(id: PageId) {
    setLoading(true)
    setPage(id)
    setSidebarOpen(false)
  }

  function disconnectUser(u: LiveUser) {
    setConfirm({
      title: "Disconnect user",
      body: `Disconnect ${u.user} from the network? The session will end immediately.`,
      danger: true,
      action: "Disconnect",
      onConfirm: () => {
        setLive((list) => list.filter((x) => x.id !== u.id))
        toast(`${u.user} disconnected`)
        setConfirm(null)
      },
    })
  }

  function blockUser(u: LiveUser) {
    setConfirm({
      title: "Block device",
      body: `Block ${u.user} (${u.mac}) from this network?`,
      danger: true,
      action: "Block",
      onConfirm: () => {
        setLive((list) => list.filter((x) => x.id !== u.id))
        toast(`${u.user} blocked`, "warn")
        setConfirm(null)
      },
    })
  }

  function extendUser(u: LiveUser) {
    setLive((list) => list.map((x) => (x.id === u.id ? { ...x, remaining: "Extended +1h" } : x)))
    toast(`Session extended for ${u.user}`)
  }

  function generateVouchers() {
    const n = Number(genForm.qty) || 0
    if (n < 1 || n > 1000) {
      toast("Quantity must be between 1 and 1000", "error")
      return
    }
    const codes = generateCodes(n)
    const planName = genForm.plan.split(" — ")[0]
    const created: Voucher[] = codes.map((code) => ({
      code,
      plan: planName,
      duration: planName.includes("Hour") ? planName : planName,
      device: `${genForm.devices} device${genForm.devices > 1 ? "s" : ""}`,
      created: "Today",
      status: "Available",
    }))
    setVouchers((v) => [...created, ...v])
    setGenDone(n)
    toast(`${n} vouchers generated successfully.`)
  }

  const filteredVouchers = useMemo(() => {
    if (voucherFilter === "All") return vouchers
    return vouchers.filter((v) => v.status === voucherFilter)
  }, [vouchers, voucherFilter])

  const cmdItems: CommandItem[] = [
    ...NAV.filter((n): n is { id: PageId; label: string; icon: string } => !("section" in n)).map((n) => ({ id: n.id, label: n.label, icon: n.icon, type: "page" })),
    ...live.map((u) => ({ id: `u-${u.id}`, label: u.user, icon: "users", type: "user" })),
  ]

  const aps = devicesSeed.filter((d) => d.type === "Access Point")
  const onlineAps = aps.filter((d) => d.status === "Online").length

  return (
    <div className="app">
      {sidebarOpen && <div className="overlay-sidebar" onClick={() => setSidebarOpen(false)} />}
      <aside className={`sidebar ${sidebarOpen ? "open" : ""}`}>
        <div className="sidebar-brand">
          <div className="brand-row">
            <div className="brand-mark">
              <SvgIcon name="wifi" size={17} stroke={1.8} />
            </div>
            <div className="brand-name">WiFi Controller</div>
          </div>
          <div className="system-online">
            <span className="dot" /> System Online
          </div>
        </div>
        <nav className="nav" aria-label="Primary">
          {NAV.map((item, i) =>
            "section" in item ? (
              <div className="nav-section" key={item.section + i}>
                {item.section}
              </div>
            ) : (
              <button key={item.id} className={`nav-item ${page === item.id ? "active" : ""}`} onClick={() => go(item.id)}>
                <SvgIcon name={item.icon} />
                {item.label}
              </button>
            )
          )}
        </nav>
        <div className="sidebar-foot">
          <div className="company-row">
            <div className="avatar">{company.initials}</div>
            <div className="company-meta">
              <div className="company-name">{company.name}</div>
              <div className="company-role">{company.role}</div>
            </div>
          </div>
          <button
            className="select-like"
            onClick={() => {
              const i = networks.findIndex((n) => n.id === network.id)
              const next = networks[(i + 1) % networks.length]
              setNetwork(next)
              toast(`Switched to ${next.name}`)
            }}
            aria-label="Select network"
          >
            <span>{network.name}</span>
            <SvgIcon name="chevron" size={14} />
          </button>
          <button className="select-like" onClick={() => go("settings")}>
            <span>Settings</span>
            <SvgIcon name="settings" size={14} />
          </button>
        </div>
      </aside>

      <div className="main">
        <header className="topbar">
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <button className="icon-btn menu-btn" onClick={() => setSidebarOpen(true)} aria-label="Open menu">
              <SvgIcon name="menu" />
            </button>
            <div className="crumb">
              Network <span className="crumb-sep">/</span> <strong>{network.name}</strong>
            </div>
          </div>
          <div className="top-actions">
            <button className="search-chip" onClick={() => setCmdOpen(true)} aria-label="Search">
              <SvgIcon name="search" size={14} />
              Search
              <kbd>⌘K</kbd>
            </button>
            <button className="icon-btn notif" aria-label="Notifications" onClick={() => setNotifyOpen((v) => !v)}>
              <SvgIcon name="bell" />
            </button>
            <div className="mikrotik">
              <span className="dot" />
              <span className="label">MikroTik Connected</span>
            </div>
            <div className="avatar sm" title={company.name}>
              {company.initials}
            </div>
          </div>
        </header>

        {notifyOpen && (
          <div className="card" style={{ position: "absolute", right: 28, top: 56, width: 300, zIndex: 50, padding: 12 }}>
            <strong style={{ fontSize: 13 }}>Notifications</strong>
            <div style={{ marginTop: 10, fontSize: 12.5, color: "var(--text-muted)", display: "flex", flexDirection: "column", gap: 10 }}>
              <div>AP-03 Patio went offline 2h ago</div>
              <div>42 min remaining on Guest-044</div>
              <div>Daily revenue crossed 100,000 Kz</div>
            </div>
          </div>
        )}

        <main className="content" id="main">
          {page === "overview" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">{greeting()}, Admin</h1>
                  <p className="page-sub">Here's what's happening on your network today.</p>
                </div>
                <div className="head-actions">
                  <button className="btn primary" onClick={() => { setGenOpen(true); setGenDone(null) }}>
                    <SvgIcon name="plus" size={14} /> Create Voucher
                  </button>
                </div>
              </div>
              {loading ? (
                <SkeletonBlock />
              ) : (
                <div className="kpi-grid">
                  {kpis.map((k) => (
                    <article className="card kpi" key={k.id}>
                      <div className="kpi-top">
                        {k.label}
                        <span className={`dot ${k.up ? "" : "warn"}`} />
                      </div>
                      <div className="kpi-value">{k.value}</div>
                      <div className="kpi-foot">
                        <span className={`delta ${k.up ? "up" : "down"}`}>{k.delta} vs yesterday</span>
                        <Sparkline series={k.series} color={k.up ? "#22c55e" : "#ef4444"} />
                      </div>
                    </article>
                  ))}
                </div>
              )}
              <div className="split">
                <section className="card panel">
                  <div className="panel-head">
                    <h2 className="panel-title">Bandwidth Usage</h2>
                    <div className="legend">
                      <span><i className="swatch" style={{ background: "#3b82f6" }} /> Download</span>
                      <span><i className="swatch" style={{ background: "#22c55e" }} /> Upload</span>
                    </div>
                  </div>
                  <AreaChart data={bandwidth} hover={hoverBw} setHover={setHoverBw} />
                </section>
                <section className="card panel">
                  <div className="panel-head">
                    <h2 className="panel-title">Network Status</h2>
                  </div>
                  <div className="status-list">
                    <div className="status-row">
                      <div className="status-left">Internet</div>
                      <div className="status-right"><span className="dot" /> Operational</div>
                    </div>
                    <div className="status-row">
                      <div className="status-left">Router</div>
                      <div className="status-right"><span className="dot" /> Connected</div>
                    </div>
                    <div className="status-row">
                      <div className="status-left">Access Points</div>
                      <div className="status-right"><span className={`dot ${onlineAps === aps.length ? "" : "warn"}`} /> {onlineAps} / {aps.length} Online</div>
                    </div>
                    <div className="status-row">
                      <div className="status-left">RADIUS</div>
                      <div className="status-right"><span className="dot" /> Operational</div>
                    </div>
                  </div>
                </section>
              </div>
              <section className="card">
                <div className="panel panel-head" style={{ marginBottom: 0 }}>
                  <h2 className="panel-title">Live Users</h2>
                  <button className="btn ghost sm" onClick={() => go("live")}>View all</button>
                </div>
                <UserTable users={live.slice(0, 5)} onView={(u) => toast(`Session ${u.user} · ${u.ip}`)} onDisconnect={disconnectUser} onBlock={blockUser} onExtend={extendUser} />
              </section>
            </>
          )}

          {page === "live" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">Live Users</h1>
                  <p className="page-sub">{live.length} clients currently authenticated on {network.name}.</p>
                </div>
              </div>
              {live.length === 0 ? (
                <div className="card empty">
                  <h3>No live users</h3>
                  <p>When clients authenticate, they will appear here.</p>
                </div>
              ) : (
                <div className="card">
                  <UserTable users={live} onView={(u) => toast(`${u.user} · ${u.mac}`)} onDisconnect={disconnectUser} onBlock={blockUser} onExtend={extendUser} />
                </div>
              )}
            </>
          )}

          {page === "vouchers" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">Vouchers</h1>
                  <p className="page-sub">Create, manage and monitor access vouchers.</p>
                </div>
                <div className="head-actions">
                  <button className="btn primary" onClick={() => { setGenOpen(true); setGenDone(null) }}>Generate vouchers</button>
                  <button className="btn" onClick={() => toast("Export started")}>Export</button>
                  <button className="btn" onClick={() => toast("Sent to printer")}><SvgIcon name="print" size={14} /> Print</button>
                </div>
              </div>
              <div className="tabs" role="tablist">
                {["All", "Available", "Active", "Expired", "Revoked"].map((f) => (
                  <button key={f} className={`tab ${voucherFilter === f ? "active" : ""}`} onClick={() => setVoucherFilter(f)} role="tab" aria-selected={voucherFilter === f}>
                    {f}
                  </button>
                ))}
              </div>
              {filteredVouchers.length === 0 ? (
                <div className="card empty">
                  <h3>No vouchers in this filter</h3>
                  <p>Generate a batch to start selling access.</p>
                  <button className="btn primary" style={{ marginTop: 12 }} onClick={() => setGenOpen(true)}>Generate vouchers</button>
                </div>
              ) : (
                <div className="card table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>Code</th>
                        <th>Plan</th>
                        <th>Duration</th>
                        <th>Device</th>
                        <th>Created</th>
                        <th>Status</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredVouchers.map((v) => (
                        <tr key={v.code}>
                          <td className="mono">{v.code}</td>
                          <td>{v.plan}</td>
                          <td>{v.duration}</td>
                          <td>{v.device}</td>
                          <td>{v.created}</td>
                          <td><span className={`badge ${v.status.toLowerCase()}`}>{v.status}</span></td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </>
          )}

          {page === "plans" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">Plans</h1>
                  <p className="page-sub">Duration, speed and price for guest access.</p>
                </div>
                <button className="btn primary" onClick={() => setPlanOpen(true)}>
                  <SvgIcon name="plus" size={14} /> Create plan
                </button>
              </div>
              <div className="plans-grid">
                {plans.map((p) => (
                  <article className="card plan-card" key={p.id}>
                    <div className="kpi-top">
                      {p.name}
                      <span className="badge available">{p.used} used</span>
                    </div>
                    <div className="plan-price">{p.price.split(" ")[0]} <span>Kz</span></div>
                    <div className="meta-list">
                      <div><span>Duration</span><strong style={{ color: "var(--text)" }}>{p.duration}</strong></div>
                      <div><span>Speed</span><strong style={{ color: "var(--text)" }}>{p.speed}</strong></div>
                      <div><span>Data limit</span><strong style={{ color: "var(--text)" }}>{p.data}</strong></div>
                      <div><span>Devices</span><strong style={{ color: "var(--text)" }}>{p.devices}</strong></div>
                    </div>
                  </article>
                ))}
              </div>
            </>
          )}

          {page === "networks" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">Networks</h1>
                  <p className="page-sub">Physical topology of {network.name}.</p>
                </div>
              </div>
              <div className="card">
                <div className="topo">
                  <button className="topo-node" onClick={() => setDevice({ id: "isp", type: "Internet", name: "Upstream ISP", ip: "41.223.x.x", status: "Online", uptime: "99.98%", clients: "—", bandwidth: "200 Mbps", firmware: "—", cpu: "—", memory: "—", lastSeen: "Just now" })}>
                    <div>
                      <div style={{ fontSize: 12, color: "var(--text-muted)" }}>Internet</div>
                      <strong>ISP Upstream</strong>
                    </div>
                    <span className="badge online"><span className="dot" /> Operational</span>
                  </button>
                  <div className="topo-line" />
                  <button className="topo-node" onClick={() => setDevice(devicesSeed[0])}>
                    <div>
                      <div style={{ fontSize: 12, color: "var(--text-muted)" }}>Router</div>
                      <strong>MikroTik RB4011</strong>
                      <div className="stat-inline">192.168.88.1</div>
                    </div>
                    <span className="badge online"><span className="dot" /> Online</span>
                  </button>
                  <div className="topo-line" />
                  <button className="topo-node" onClick={() => setDevice(devicesSeed[1])}>
                    <div>
                      <div style={{ fontSize: 12, color: "var(--text-muted)" }}>Switch</div>
                      <strong>CRS328-24P</strong>
                    </div>
                    <span className="badge online"><span className="dot" /> Online</span>
                  </button>
                  <div className="topo-line" />
                  <div className="ap-row">
                    {aps.map((ap) => (
                      <button key={ap.id} className={`ap-card ${ap.status === "Offline" ? "offline" : ""} ${device?.id === ap.id ? "selected" : ""}`} onClick={() => setDevice(ap)}>
                        <strong>{ap.name.split(" ")[0]}</strong>
                        <div style={{ marginTop: 8, fontSize: 12 }} className={ap.status === "Online" ? "badge online" : "badge danger"}>
                          <span className={`dot ${ap.status === "Online" ? "" : "danger"}`} /> {ap.status}
                        </div>
                      </button>
                    ))}
                  </div>
                </div>
              </div>
            </>
          )}

          {page === "devices" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">Devices</h1>
                  <p className="page-sub">Routers, switches and access points under management.</p>
                </div>
              </div>
              <div className="card table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Device</th>
                      <th>Type</th>
                      <th>IP</th>
                      <th>Clients</th>
                      <th>Bandwidth</th>
                      <th>Firmware</th>
                      <th>Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {devicesSeed.map((d) => (
                      <tr key={d.id} onClick={() => setDevice(d)} style={{ cursor: "pointer" }}>
                        <td>{d.name}</td>
                        <td>{d.type}</td>
                        <td className="mono">{d.ip}</td>
                        <td>{d.clients}</td>
                        <td>{d.bandwidth}</td>
                        <td className="mono">{d.firmware}</td>
                        <td><span className={`badge ${d.status === "Online" ? "online" : "danger"}`}>{d.status}</span></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}

          {page === "sessions" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">Sessions</h1>
                  <p className="page-sub">Active and recent authentications.</p>
                </div>
              </div>
              <div className="card table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>User</th>
                      <th>Started</th>
                      <th>Duration</th>
                      <th>Data</th>
                      <th>Plan</th>
                      <th>Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sessionsSeed.map((s) => (
                      <tr key={s.id}>
                        <td>{s.user}</td>
                        <td>{s.started}</td>
                        <td>{s.duration}</td>
                        <td>{s.data}</td>
                        <td>{s.plan}</td>
                        <td><span className={`badge ${s.status === "Live" ? "online" : "offline"}`}>{s.status}</span></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}

          {page === "analytics" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">Analytics</h1>
                  <p className="page-sub">Traffic, vouchers and revenue for {network.name}.</p>
                </div>
                <div className="tabs" style={{ marginBottom: 0 }}>
                  {["Today", "7 days", "30 days", "Custom"].map((r) => (
                    <button key={r} className={`tab ${range === r ? "active" : ""}`} onClick={() => { setRange(r); if (r === "Custom") toast("Pick a custom range from the calendar") }}>
                      {r}
                    </button>
                  ))}
                </div>
              </div>
              <div className="analytics-grid">
                <section className="card panel">
                  <div className="panel-head"><h2 className="panel-title">Users per hour</h2></div>
                  <div className="chart-wrap" style={{ height: 180 }}>
                    <Bars values={hourlyUsers} labels={hourlyUsers.map((_, i) => `${i}h`)} />
                  </div>
                </section>
                <section className="card panel">
                  <div className="panel-head"><h2 className="panel-title">Plans most used</h2></div>
                  {planShare.map((p) => (
                    <div className="bar-row" key={p.name}>
                      <span style={{ width: 72 }}>{p.name}</span>
                      <div className="bar-track"><div className="bar-fill" style={{ width: `${p.value}%` }} /></div>
                      <span className="stat-inline">{p.value}%</span>
                    </div>
                  ))}
                </section>
              </div>
              <div className="mini-charts">
                <section className="card panel">
                  <div className="panel-head"><h2 className="panel-title">Bandwidth</h2></div>
                  <AreaChart data={bandwidth} hover={hoverBw} setHover={setHoverBw} />
                </section>
                <section className="card panel">
                  <div className="panel-head"><h2 className="panel-title">Revenue</h2></div>
                  <Sparkline series={revenueSeries} color="#22c55e" />
                  <div className="kpi-value" style={{ fontSize: 22, marginTop: 12 }}>128,500 Kz</div>
                  <div className="delta up">+18% vs previous</div>
                </section>
                <section className="card panel">
                  <div className="panel-head"><h2 className="panel-title">Devices</h2></div>
                  {deviceShare.map((p) => (
                    <div className="bar-row" key={p.name}>
                      <span style={{ width: 72 }}>{p.name}</span>
                      <div className="bar-track"><div className="bar-fill" style={{ width: `${p.value}%`, background: "#60a5fa" }} /></div>
                      <span className="stat-inline">{p.value}%</span>
                    </div>
                  ))}
                  <div style={{ marginTop: 16, color: "var(--text-muted)", fontSize: 12 }}>Avg session 54 min · Data 1.1 GB</div>
                </section>
              </div>
            </>
          )}

          {page === "portal" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">Portal</h1>
                  <p className="page-sub">Visual editor for the captive portal guests see.</p>
                </div>
                <button className="btn primary" onClick={() => toast("Portal published")}>Publish</button>
              </div>
              <div className="portal-layout">
                <section className="card panel">
                  <div className="field">
                    <label htmlFor="pname">Network name</label>
                    <input id="pname" value={portal.name} onChange={(e) => setPortal({ ...portal, name: e.target.value })} />
                  </div>
                  <div className="field">
                    <label htmlFor="pmsg">Message</label>
                    <input id="pmsg" value={portal.message} onChange={(e) => setPortal({ ...portal, message: e.target.value })} />
                  </div>
                  <div className="grid-2">
                    <div className="field">
                      <label htmlFor="pbg">Background</label>
                      <input id="pbg" type="color" value={portal.bg} onChange={(e) => setPortal({ ...portal, bg: e.target.value })} />
                    </div>
                    <div className="field">
                      <label htmlFor="pcol">Accent</label>
                      <input id="pcol" type="color" value={portal.color} onChange={(e) => setPortal({ ...portal, color: e.target.value })} />
                    </div>
                  </div>
                  <div className="field">
                    <label htmlFor="pterms">Terms of use</label>
                    <textarea id="pterms" value={portal.terms} onChange={(e) => setPortal({ ...portal, terms: e.target.value })} />
                  </div>
                  <div className="field">
                    <label htmlFor="psuccess">Success page</label>
                    <textarea id="psuccess" value={portal.success} onChange={(e) => setPortal({ ...portal, success: e.target.value })} />
                  </div>
                </section>
                <section className="card panel">
                  <h2 className="panel-title" style={{ marginBottom: 14 }}>Portal Preview</h2>
                  <div className="phone">
                    <div className="phone-screen" style={{ background: portal.bg }}>
                      <div className="phone-logo" style={{ background: portal.color }}>W</div>
                      <strong>{portal.name}</strong>
                      <div style={{ fontSize: 12, color: "rgba(255,255,255,0.7)" }}>{portal.message}</div>
                      <input defaultValue="ZG-82LX-PA" readOnly aria-label="Voucher code" />
                      <button className="connect" style={{ background: portal.color }}>CONNECT</button>
                      <div style={{ fontSize: 10, color: "rgba(255,255,255,0.45)", lineHeight: 1.4 }}>{portal.terms}</div>
                    </div>
                  </div>
                </section>
              </div>
            </>
          )}

          {page === "users" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">Users</h1>
                  <p className="page-sub">Operator accounts for this controller.</p>
                </div>
              </div>
              <div className="card table-wrap">
                <table>
                  <thead>
                    <tr><th>Name</th><th>Email</th><th>Role</th><th>Status</th></tr>
                  </thead>
                  <tbody>
                    {portalUsers.map((u) => (
                      <tr key={u.email}>
                        <td>{u.name}</td>
                        <td>{u.email}</td>
                        <td>{u.role}</td>
                        <td><span className={`badge ${u.status === "Active" ? "online" : "offline"}`}>{u.status}</span></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}

          {page === "team" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">Team</h1>
                  <p className="page-sub">People with access to Kimbo Networks.</p>
                </div>
                <button className="btn primary" onClick={() => toast("Invite sent")}>Invite member</button>
              </div>
              <div className="card table-wrap">
                <table>
                  <thead>
                    <tr><th>Name</th><th>Role</th><th>Email</th><th>Last seen</th></tr>
                  </thead>
                  <tbody>
                    {team.map((m) => (
                      <tr key={m.email}>
                        <td>
                          <div className="user-cell">
                            <div className="avatar sm">{m.name.split(" ").map((p) => p[0]).join("").slice(0, 2)}</div>
                            {m.name}
                          </div>
                        </td>
                        <td>{m.role}</td>
                        <td>{m.email}</td>
                        <td>{m.last}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}

          {page === "settings" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">Settings</h1>
                  <p className="page-sub">Controller, RADIUS and branding.</p>
                </div>
              </div>
              <div className="split">
                <section className="card panel">
                  <h2 className="panel-title" style={{ marginBottom: 14 }}>General</h2>
                  <div className="field">
                    <label htmlFor="org">Organization</label>
                    <input id="org" defaultValue="Kimbo Networks" />
                  </div>
                  <div className="field">
                    <label htmlFor="tz">Timezone</label>
                    <select id="tz" defaultValue="Africa/Luanda">
                      <option>Africa/Luanda</option>
                      <option>UTC</option>
                    </select>
                  </div>
                  <div className="field">
                    <label htmlFor="curr">Currency</label>
                    <input id="curr" defaultValue="Kz — Angolan Kwanza" />
                  </div>
                  <button className="btn primary" onClick={() => toast("Settings saved")}>Save changes</button>
                </section>
                <section className="card panel">
                  <h2 className="panel-title" style={{ marginBottom: 14 }}>Router</h2>
                  <div className="field">
                    <label htmlFor="host">MikroTik host</label>
                    <input id="host" defaultValue="192.168.88.1" className="mono" />
                  </div>
                  <div className="field">
                    <label htmlFor="api">API port</label>
                    <input id="api" defaultValue="8728" />
                  </div>
                  <div className="status-row">
                    <span>Connection</span>
                    <span className="status-right"><span className="dot" /> Connected</span>
                  </div>
                </section>
              </div>
            </>
          )}
        </main>
      </div>

      <nav className="mobile-nav" aria-label="Mobile">
        {MOBILE_NAV.map((n) => (
          <button key={n.id} className={page === n.id ? "active" : ""} onClick={() => go(n.id)}>
            <SvgIcon name={n.icon} />
            {n.label}
          </button>
        ))}
      </nav>

      {genOpen && (
        <Modal title="Generate vouchers" hint="Batch access codes for guests." onClose={() => setGenOpen(false)}>
          {!genDone ? (
            <>
              <div className="grid-2">
                <div className="field">
                  <label htmlFor="qty">Quantity</label>
                  <input id="qty" type="number" min="1" max="1000" value={genForm.qty} onChange={(e) => setGenForm({ ...genForm, qty: e.target.value })} />
                </div>
                <div className="field">
                  <label htmlFor="plan">Plan</label>
                  <select id="plan" value={genForm.plan} onChange={(e) => setGenForm({ ...genForm, plan: e.target.value })}>
                    <option>1 Hour — 100 Kz</option>
                    <option>3 Hours — 200 Kz</option>
                    <option>1 Day — 500 Kz</option>
                    <option>7 Days — 2,000 Kz</option>
                  </select>
                </div>
              </div>
              <div className="grid-2">
                <div className="field">
                  <label htmlFor="speed">Speed</label>
                  <input id="speed" value={genForm.speed} onChange={(e) => setGenForm({ ...genForm, speed: e.target.value })} />
                </div>
                <div className="field">
                  <label htmlFor="data">Data limit</label>
                  <input id="data" value={genForm.data} onChange={(e) => setGenForm({ ...genForm, data: e.target.value })} />
                </div>
              </div>
              <div className="grid-2">
                <div className="field">
                  <label htmlFor="dev">Devices</label>
                  <input id="dev" type="number" min="1" value={genForm.devices} onChange={(e) => setGenForm({ ...genForm, devices: Number(e.target.value) })} />
                </div>
                <div className="field">
                  <label htmlFor="exp">Expiration</label>
                  <input id="exp" value={genForm.expiration} onChange={(e) => setGenForm({ ...genForm, expiration: e.target.value })} />
                </div>
              </div>
              <button className="btn primary" style={{ width: "100%", height: 40, marginTop: 8 }} onClick={generateVouchers}>
                Generate {genForm.qty} vouchers
              </button>
            </>
          ) : (
            <>
              <p className="hint" style={{ color: "var(--success)" }}>{genDone} vouchers generated successfully.</p>
              <div className="head-actions">
                <button className="btn" onClick={() => toast("CSV downloaded")}><SvgIcon name="download" size={14} /> Download CSV</button>
                <button className="btn" onClick={() => toast("Print job queued")}><SvgIcon name="print" size={14} /> Print vouchers</button>
              </div>
            </>
          )}
        </Modal>
      )}

      {planOpen && (
        <Modal title="Create plan" hint="Define duration, speed and price." onClose={() => setPlanOpen(false)}>
          <div className="field">
            <label htmlFor="pn">Name</label>
            <input id="pn" placeholder="2 Hours" />
          </div>
          <div className="grid-2">
            <div className="field">
              <label htmlFor="pp">Price (Kz)</label>
              <input id="pp" defaultValue="150" />
            </div>
            <div className="field">
              <label htmlFor="ps">Speed</label>
              <input id="ps" defaultValue="10 Mbps" />
            </div>
          </div>
          <button
            className="btn primary"
            style={{ width: "100%" }}
            onClick={() => {
              setPlans((p) => [...p, { id: `p${p.length}`, name: "2 Hours", price: "150 Kz", duration: "2 hours", speed: "10 Mbps", data: "2 GB", devices: 1, used: 0 }])
              setPlanOpen(false)
              toast("Plan created")
            }}
          >
            Create plan
          </button>
        </Modal>
      )}

      {confirm && (
        <Modal title={confirm.title} onClose={() => setConfirm(null)}>
          <p className="hint">{confirm.body}</p>
          <div className="confirm-actions">
            <button className="btn ghost" onClick={() => setConfirm(null)}>Cancel</button>
            <button className={`btn ${confirm.danger ? "danger" : "primary"}`} onClick={confirm.onConfirm}>{confirm.action}</button>
          </div>
        </Modal>
      )}

      {device && (
        <Drawer title={device.name} onClose={() => setDevice(null)}>
          <div className="meta-list">
            {[
              ["Status", device.status],
              ["Uptime", device.uptime],
              ["IP", device.ip],
              ["Clients", device.clients],
              ["Bandwidth", device.bandwidth],
              ["Firmware", device.firmware],
              ["CPU", device.cpu],
              ["Memory", device.memory],
              ["Last seen", device.lastSeen],
            ].map(([k, v]) => (
              <div key={k}>
                <span>{k}</span>
                <strong style={{ color: "var(--text)" }}>{v}</strong>
              </div>
            ))}
          </div>
        </Drawer>
      )}

      <CommandMenu
        open={cmdOpen}
        onClose={() => setCmdOpen(false)}
        items={cmdItems}
        onSelect={(item) => {
          setCmdOpen(false)
          if (item.type === "page") go(item.id as PageId)
          else toast(item.label)
        }}
      />
      <Toasts items={toasts} dismiss={dismiss} />
    </div>
  )
}

function UserTable({
  users,
  onView,
  onDisconnect,
  onBlock,
  onExtend,
}: {
  users: LiveUser[]
  onView: (u: LiveUser) => void
  onDisconnect: (u: LiveUser) => void
  onBlock: (u: LiveUser) => void
  onExtend: (u: LiveUser) => void
}) {
  return (
    <div className="table-wrap">
      <table>
        <thead>
          <tr>
            <th>User</th>
            <th>Device</th>
            <th>IP</th>
            <th>Plan</th>
            <th>Speed</th>
            <th>Remaining</th>
            <th>Status</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {users.map((u) => (
            <tr key={u.id}>
              <td>{u.user}</td>
              <td>{u.device}</td>
              <td className="mono">{u.ip}</td>
              <td>{u.plan}</td>
              <td>{u.speed}</td>
              <td className="stat-inline">{u.remaining}</td>
              <td><span className="badge online"><span className="dot" /> Online</span></td>
              <td>
                <div className="row-actions">
                  <button className="btn sm ghost" onClick={() => onView(u)}>View</button>
                  <button className="btn sm ghost" onClick={() => onExtend(u)}>Extend</button>
                  <button className="btn sm ghost" onClick={() => onDisconnect(u)}>Disconnect</button>
                  <button className="btn sm danger" onClick={() => onBlock(u)}>Block</button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
