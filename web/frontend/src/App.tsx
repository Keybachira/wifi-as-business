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
  { section: "Operação" },
  { id: "overview", label: "Resumo", icon: "overview" },
  { id: "live", label: "Clientes Ligados", icon: "live" },
  { id: "vouchers", label: "Vales", icon: "ticket" },
  { id: "plans", label: "Planos", icon: "plans" },
  { section: "Infraestrutura" },
  { id: "networks", label: "Redes", icon: "network" },
  { id: "devices", label: "Equipamentos", icon: "devices" },
  { id: "sessions", label: "Sessões", icon: "sessions" },
  { id: "analytics", label: "Análises", icon: "analytics" },
  { section: "Acesso" },
  { id: "portal", label: "Portal", icon: "portal" },
  { id: "users", label: "Utilizadores", icon: "users" },
  { id: "team", label: "Equipa", icon: "team" },
  { id: "settings", label: "Definições", icon: "settings" },
]

const MOBILE_NAV: { id: PageId; label: string; icon: string }[] = [
  { id: "overview", label: "Início", icon: "overview" },
  { id: "live", label: "Ligados", icon: "live" },
  { id: "vouchers", label: "Vales", icon: "ticket" },
  { id: "analytics", label: "Análises", icon: "analytics" },
  { id: "settings", label: "Mais", icon: "settings" },
]

function greeting() {
  const h = new Date().getHours()
  if (h < 12) return "Bom dia"
  if (h < 18) return "Boa tarde"
  return "Boa noite"
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
  const [voucherFilter, setVoucherFilter] = useState<string>("Todos")
  const [genOpen, setGenOpen] = useState(false)
  const [genForm, setGenForm] = useState<GenForm>({ qty: 100, plan: "1 Hora — 100", speed: "5 Mbps", data: "1 GB", devices: 1, expiration: "30 dias" })
  const [genDone, setGenDone] = useState<number | null>(null)
  const [planOpen, setPlanOpen] = useState(false)
  const [device, setDevice] = useState<Device | null>(null)
  const [confirm, setConfirm] = useState<ConfirmState | null>(null)
  const [range, setRange] = useState<string>("Hoje")
  const [portal, setPortal] = useState({
    name: "WiFi Controller",
    message: "Introduza o seu código de acesso",
    bg: "#0b1220",
    color: "#3b82f6",
    terms: "Ao ligar-se aceita os termos de utilização desta rede.",
    success: "Está ligado. Aproveite a sua sessão.",
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
      title: "Desligar cliente",
      body: `Desligar ${u.user} da rede? A sessão termina imediatamente.`,
      danger: true,
      action: "Desligar",
      onConfirm: () => {
        setLive((list) => list.filter((x) => x.id !== u.id))
        toast(`${u.user} desligado`)
        setConfirm(null)
      },
    })
  }

  function blockUser(u: LiveUser) {
    setConfirm({
      title: "Bloquear equipamento",
      body: `Bloquear ${u.user} (${u.mac}) nesta rede?`,
      danger: true,
      action: "Bloquear",
      onConfirm: () => {
        setLive((list) => list.filter((x) => x.id !== u.id))
        toast(`${u.user} bloqueado`, "warn")
        setConfirm(null)
      },
    })
  }

  function extendUser(u: LiveUser) {
    setLive((list) => list.map((x) => (x.id === u.id ? { ...x, remaining: "Estendida +1h" } : x)))
    toast(`Sessão estendida para ${u.user}`)
  }

  function generateVouchers() {
    const n = Number(genForm.qty) || 0
    if (n < 1 || n > 1000) {
      toast("A quantidade deve estar entre 1 e 1000", "error")
      return
    }
    const codes = generateCodes(n)
    const planName = genForm.plan.split(" — ")[0]
    const created: Voucher[] = codes.map((code) => ({
      code,
      plan: planName,
      duration: planName.includes("Hora") ? planName : planName,
      device: `${genForm.devices} dispositivo${genForm.devices > 1 ? "s" : ""}`,
      created: "Hoje",
      status: "Disponível",
    }))
    setVouchers((v) => [...created, ...v])
    setGenDone(n)
    toast(`${n} vales gerados com sucesso.`)
  }

  const filteredVouchers = useMemo(() => {
    if (voucherFilter === "Todos") return vouchers
    return vouchers.filter((v) => v.status === voucherFilter)
  }, [vouchers, voucherFilter])

  const cmdItems: CommandItem[] = [
    ...NAV.filter((n): n is { id: PageId; label: string; icon: string } => !("section" in n)).map((n) => ({ id: n.id, label: n.label, icon: n.icon, type: "page" })),
    ...live.map((u) => ({ id: `u-${u.id}`, label: u.user, icon: "users", type: "user" })),
  ]

  const aps = devicesSeed.filter((d) => d.type === "Access Point")
  const onlineAps = aps.filter((d) => d.status === "Online").length
  const traduzir = (t: string) =>
    ({
      Online: "Ligado",
      Offline: "Desligado",
      Live: "Ativa",
      Ended: "Terminada",
      Available: "Disponível",
      Active: "Ativo",
      Expired: "Expirado",
      Revoked: "Revogado",
      Operator: "Operador",
      Support: "Suporte",
      Owner: "Responsável",
    } as Record<string, string>)[t] ?? t
  const traduzirTipo = (t: string) =>
    ({
      Router: "Router",
      Switch: "Switch",
      "Access Point": "Ponto de Acesso",
      Firewall: "Firewall",
      Internet: "Internet",
    } as Record<string, string>)[t] ?? t

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
            <span className="dot" /> Sistema Online
          </div>
        </div>
        <nav className="nav" aria-label="Principal">
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
              toast(`Mudou para ${next.name}`)
            }}
            aria-label="Selecionar rede"
          >
            <span>{network.name}</span>
            <SvgIcon name="chevron" size={14} />
          </button>
          <button className="select-like" onClick={() => go("settings")}>
            <span>Definições</span>
            <SvgIcon name="settings" size={14} />
          </button>
        </div>
      </aside>

      <div className="main">
        <header className="topbar">
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <button className="icon-btn menu-btn" onClick={() => setSidebarOpen(true)} aria-label="Abrir menu">
              <SvgIcon name="menu" />
            </button>
            <div className="crumb">
              Rede <span className="crumb-sep">/</span> <strong>{network.name}</strong>
            </div>
          </div>
          <div className="top-actions">
            <button className="search-chip" onClick={() => setCmdOpen(true)} aria-label="Pesquisar">
              <SvgIcon name="search" size={14} />
              Pesquisar
              <kbd>⌘K</kbd>
            </button>
            <button className="icon-btn notif" aria-label="Notificações" onClick={() => setNotifyOpen((v) => !v)}>
              <SvgIcon name="bell" />
            </button>
            <div className="mikrotik">
              <span className="dot" />
              <span className="label">MikroTik Ligado</span>
            </div>
            <div className="avatar sm" title={company.name}>
              {company.initials}
            </div>
          </div>
        </header>

        {notifyOpen && (
          <div className="card" style={{ position: "absolute", right: 28, top: 56, width: 300, zIndex: 50, padding: 12 }}>
            <strong style={{ fontSize: 13 }}>Notificações</strong>
            <div style={{ marginTop: 10, fontSize: 12.5, color: "var(--text-muted)", display: "flex", flexDirection: "column", gap: 10 }}>
              <div>AP-03 Pátio desligou-se há 2h</div>
              <div>42 min restantes para Guest-044</div>
              <div>Receita do dia passou 100,000</div>
            </div>
          </div>
        )}

        <main className="content" id="main">
          {page === "overview" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">{greeting()}, Admin</h1>
                  <p className="page-sub">Eis o que se passa na sua rede hoje.</p>
                </div>
                <div className="head-actions">
                  <button className="btn primary" onClick={() => { setGenOpen(true); setGenDone(null) }}>
                    <SvgIcon name="plus" size={14} /> Criar Vale
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
                        <span className={`delta ${k.up ? "up" : "down"}`}>{k.delta} vs ontem</span>
                        <Sparkline series={k.series} color={k.up ? "#22c55e" : "#ef4444"} />
                      </div>
                    </article>
                  ))}
                </div>
              )}
              <div className="split">
                <section className="card panel">
                  <div className="panel-head">
                    <h2 className="panel-title">Uso de Banda</h2>
                    <div className="legend">
                      <span><i className="swatch" style={{ background: "#3b82f6" }} /> Descarga</span>
                      <span><i className="swatch" style={{ background: "#22c55e" }} /> Carregamento</span>
                    </div>
                  </div>
                  <AreaChart data={bandwidth} hover={hoverBw} setHover={setHoverBw} />
                </section>
                <section className="card panel">
                  <div className="panel-head">
                    <h2 className="panel-title">Estado da Rede</h2>
                  </div>
                  <div className="status-list">
                    <div className="status-row">
                      <div className="status-left">Internet</div>
                      <div className="status-right"><span className="dot" /> Operacional</div>
                    </div>
                    <div className="status-row">
                      <div className="status-left">Router</div>
                      <div className="status-right"><span className="dot" /> Ligado</div>
                    </div>
                    <div className="status-row">
                      <div className="status-left">Pontos de Acesso</div>
                      <div className="status-right"><span className={`dot ${onlineAps === aps.length ? "" : "warn"}`} /> {onlineAps} / {aps.length} Ligados</div>
                    </div>
                    <div className="status-row">
                      <div className="status-left">RADIUS</div>
                      <div className="status-right"><span className="dot" /> Operacional</div>
                    </div>
                  </div>
                </section>
              </div>
              <section className="card">
                <div className="panel panel-head" style={{ marginBottom: 0 }}>
                  <h2 className="panel-title">Clientes Ligados</h2>
                  <button className="btn ghost sm" onClick={() => go("live")}>Ver todos</button>
                </div>
                <UserTable users={live.slice(0, 5)} onView={(u) => toast(`Session ${u.user} · ${u.ip}`)} onDisconnect={disconnectUser} onBlock={blockUser} onExtend={extendUser} />
              </section>
            </>
          )}

          {page === "live" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">Clientes Ligados</h1>
                  <p className="page-sub">{live.length} clientes atualmente autenticados em {network.name}.</p>
                </div>
              </div>
              {live.length === 0 ? (
                <div className="card empty">
                  <h3>Nenhum cliente ligado</h3>
                  <p>Quando os clientes se autenticarem, aparecerão aqui.</p>
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
                  <h1 className="page-title">Vales</h1>
                  <p className="page-sub">Criar, gerir e acompanhar os vales de acesso.</p>
                </div>
                <div className="head-actions">
                  <button className="btn primary" onClick={() => { setGenOpen(true); setGenDone(null) }}>Gerar vales</button>
                  <button className="btn" onClick={() => toast("Exportação iniciada")}>Exportar</button>
                  <button className="btn" onClick={() => toast("Enviado para a impressora")}><SvgIcon name="print" size={14} /> Imprimir</button>
                </div>
              </div>
              <div className="tabs" role="tablist">
                {["Todos", "Disponível", "Ativo", "Expirado", "Revogado"].map((f) => (
                  <button key={f} className={`tab ${voucherFilter === f ? "active" : ""}`} onClick={() => setVoucherFilter(f)} role="tab" aria-selected={voucherFilter === f}>
                    {f}
                  </button>
                ))}
              </div>
              {filteredVouchers.length === 0 ? (
                <div className="card empty">
                  <h3>Nenhum vale neste filtro</h3>
                  <p>Gere um lote para começar a vender acesso.</p>
                  <button className="btn primary" style={{ marginTop: 12 }} onClick={() => setGenOpen(true)}>Gerar vales</button>
                </div>
              ) : (
                <div className="card table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>Código</th>
                        <th>Plano</th>
                        <th>Duração</th>
                        <th>Dispositivos</th>
                        <th>Criado</th>
                        <th>Estado</th>
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
                          <td><span className={`badge ${v.status.toLowerCase()}`}>{traduzir(v.status)}</span></td>
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
                  <h1 className="page-title">Planos</h1>
                  <p className="page-sub">Duração, velocidade e preço do acesso aos clientes.</p>
                </div>
                <button className="btn primary" onClick={() => setPlanOpen(true)}>
                  <SvgIcon name="plus" size={14} /> Criar plano
                </button>
              </div>
              <div className="plans-grid">
                {plans.map((p) => (
                  <article className="card plan-card" key={p.id}>
                    <div className="kpi-top">
                      {p.name}
                      <span className="badge available">{p.used} usados</span>
                    </div>
                    <div className="plan-price">{p.price.split(" ")[0]}</div>
                    <div className="meta-list">
                      <div><span>Duração</span><strong style={{ color: "var(--text)" }}>{p.duration}</strong></div>
                      <div><span>Velocidade</span><strong style={{ color: "var(--text)" }}>{p.speed}</strong></div>
                      <div><span>Limite de dados</span><strong style={{ color: "var(--text)" }}>{p.data}</strong></div>
                      <div><span>Dispositivos</span><strong style={{ color: "var(--text)" }}>{p.devices}</strong></div>
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
                  <h1 className="page-title">Redes</h1>
                  <p className="page-sub">Topologia física de {network.name}.</p>
                </div>
              </div>
              <div className="card">
                <div className="topo">
                  <button className="topo-node" onClick={() => setDevice({ id: "isp", type: "Internet", name: "Upstream ISP", ip: "41.223.x.x", status: "Online", uptime: "99.98%", clients: "—", bandwidth: "200 Mbps", firmware: "—", cpu: "—", memory: "—", lastSeen: "Just now" })}>
                    <div>
                      <div style={{ fontSize: 12, color: "var(--text-muted)" }}>Internet</div>
                      <strong>Fornecedor (ISP)</strong>
                    </div>
                    <span className="badge online"><span className="dot" /> Operacional</span>
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
                  <h1 className="page-title">Equipamentos</h1>
                  <p className="page-sub">Routers, switches e pontos de acesso em gestão.</p>
                </div>
              </div>
              <div className="card table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Equipamento</th>
                      <th>Tipo</th>
                      <th>IP</th>
                      <th>Clientes</th>
                      <th>Banda</th>
                      <th>Firmware</th>
                      <th>Estado</th>
                    </tr>
                  </thead>
                  <tbody>
                    {devicesSeed.map((d) => (
                      <tr key={d.id} onClick={() => setDevice(d)} style={{ cursor: "pointer" }}>
                        <td>{d.name}</td>
                        <td>{traduzirTipo(d.type)}</td>
                        <td className="mono">{d.ip}</td>
                        <td>{d.clients}</td>
                        <td>{d.bandwidth}</td>
                        <td className="mono">{d.firmware}</td>
                        <td><span className={`badge ${d.status === "Online" ? "online" : "danger"}`}>{traduzir(d.status)}</span></td>
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
                  <h1 className="page-title">Sessões</h1>
                  <p className="page-sub">Autenticações ativas e recentes.</p>
                </div>
              </div>
              <div className="card table-wrap">
                <table>
                  <thead>
                    <tr>
                      <th>Utilizador</th>
                      <th>Início</th>
                      <th>Duração</th>
                      <th>Dados</th>
                      <th>Plano</th>
                      <th>Estado</th>
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
                        <td><span className={`badge ${s.status === "Live" ? "online" : "offline"}`}>{traduzir(s.status)}</span></td>
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
                  <h1 className="page-title">Análises</h1>
                  <p className="page-sub">Tráfego, vales e receita de {network.name}.</p>
                </div>
                <div className="tabs" style={{ marginBottom: 0 }}>
                  {["Hoje", "7 dias", "30 dias", "Personalizado"].map((r) => (
                    <button key={r} className={`tab ${range === r ? "active" : ""}`} onClick={() => { setRange(r); if (r === "Personalizado") toast("Escolha um intervalo no calendário") }}>
                      {r}
                    </button>
                  ))}
                </div>
              </div>
              <div className="analytics-grid">
                <section className="card panel">
                  <div className="panel-head"><h2 className="panel-title">Utilizadores por hora</h2></div>
                  <div className="chart-wrap" style={{ height: 180 }}>
                    <Bars values={hourlyUsers} labels={hourlyUsers.map((_, i) => `${i}h`)} />
                  </div>
                </section>
                <section className="card panel">
                  <div className="panel-head"><h2 className="panel-title">Planos mais usados</h2></div>
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
                  <div className="panel-head"><h2 className="panel-title">Banda</h2></div>
                  <AreaChart data={bandwidth} hover={hoverBw} setHover={setHoverBw} />
                </section>
                <section className="card panel">
                  <div className="panel-head"><h2 className="panel-title">Receita</h2></div>
                  <Sparkline series={revenueSeries} color="#22c55e" />
                  <div className="kpi-value" style={{ fontSize: 22, marginTop: 12 }}>128,500</div>
                  <div className="delta up">+18% vs período anterior</div>
                </section>
                <section className="card panel">
                  <div className="panel-head"><h2 className="panel-title">Equipamentos</h2></div>
                  {deviceShare.map((p) => (
                    <div className="bar-row" key={p.name}>
                      <span style={{ width: 72 }}>{p.name}</span>
                      <div className="bar-track"><div className="bar-fill" style={{ width: `${p.value}%`, background: "#60a5fa" }} /></div>
                      <span className="stat-inline">{p.value}%</span>
                    </div>
                  ))}
                  <div style={{ marginTop: 16, color: "var(--text-muted)", fontSize: 12 }}>Sessão média 54 min · Dados 1,1 GB</div>
                </section>
              </div>
            </>
          )}

          {page === "portal" && (
            <>
              <div className="page-head">
                <div>
                  <h1 className="page-title">Portal</h1>
                  <p className="page-sub">Editor visual do portal cativo que os clientes veem.</p>
                </div>
                <button className="btn primary" onClick={() => toast("Portal publicado")}>Publicar</button>
              </div>
              <div className="portal-layout">
                <section className="card panel">
                  <div className="field">
                    <label htmlFor="pname">Nome da rede</label>
                    <input id="pname" value={portal.name} onChange={(e) => setPortal({ ...portal, name: e.target.value })} />
                  </div>
                  <div className="field">
                    <label htmlFor="pmsg">Mensagem</label>
                    <input id="pmsg" value={portal.message} onChange={(e) => setPortal({ ...portal, message: e.target.value })} />
                  </div>
                  <div className="grid-2">
                    <div className="field">
                      <label htmlFor="pbg">Fundo</label>
                      <input id="pbg" type="color" value={portal.bg} onChange={(e) => setPortal({ ...portal, bg: e.target.value })} />
                    </div>
                    <div className="field">
                      <label htmlFor="pcol">Cor de destaque</label>
                      <input id="pcol" type="color" value={portal.color} onChange={(e) => setPortal({ ...portal, color: e.target.value })} />
                    </div>
                  </div>
                  <div className="field">
                    <label htmlFor="pterms">Termos de utilização</label>
                    <textarea id="pterms" value={portal.terms} onChange={(e) => setPortal({ ...portal, terms: e.target.value })} />
                  </div>
                  <div className="field">
                    <label htmlFor="psuccess">Página de sucesso</label>
                    <textarea id="psuccess" value={portal.success} onChange={(e) => setPortal({ ...portal, success: e.target.value })} />
                  </div>
                </section>
                <section className="card panel">
                  <h2 className="panel-title" style={{ marginBottom: 14 }}>Pré-visualização do Portal</h2>
                  <div className="phone">
                    <div className="phone-screen" style={{ background: portal.bg }}>
                      <div className="phone-logo" style={{ background: portal.color }}>W</div>
                      <strong>{portal.name}</strong>
                      <div style={{ fontSize: 12, color: "rgba(255,255,255,0.7)" }}>{portal.message}</div>
                      <input defaultValue="VA-82LX-PA" readOnly aria-label="Voucher code" />
                      <button className="connect" style={{ background: portal.color }}>LIGAR</button>
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
                  <h1 className="page-title">Utilizadores</h1>
                  <p className="page-sub">Contas de operador deste controlador.</p>
                </div>
              </div>
              <div className="card table-wrap">
                <table>
                  <thead>
                    <tr><th>Nome</th><th>Email</th><th>Função</th><th>Estado</th></tr>
                  </thead>
                  <tbody>
                    {portalUsers.map((u) => (
                      <tr key={u.email}>
                        <td>{u.name}</td>
                        <td>{u.email}</td>
                        <td>{traduzir(u.role)}</td>
                        <td><span className={`badge ${u.status === "Ativo" ? "online" : "offline"}`}>{traduzir(u.status)}</span></td>
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
                  <h1 className="page-title">Equipa</h1>
                  <p className="page-sub">Pessoas com acesso a este sistema.</p>
                </div>
                <button className="btn primary" onClick={() => toast("Convite enviado")}>Convidar membro</button>
              </div>
              <div className="card table-wrap">
                <table>
                  <thead>
                    <tr><th>Nome</th><th>Função</th><th>Email</th><th>Visto por último</th></tr>
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
                        <td>{traduzir(m.role)}</td>
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
                  <h1 className="page-title">Definições</h1>
                  <p className="page-sub">Controlador, RADIUS e identidade visual.</p>
                </div>
              </div>
              <div className="split">
                <section className="card panel">
                  <h2 className="panel-title" style={{ marginBottom: 14 }}>Geral</h2>
                  <div className="field">
                    <label htmlFor="org">Organização</label>
                    <input id="org" defaultValue="WiFi Controller" />
                  </div>
                  <div className="field">
                    <label htmlFor="tz">Fuso horário</label>
                    <select id="tz" defaultValue="Africa/Luanda">
                      <option>Africa/Luanda</option>
                      <option>UTC</option>
                    </select>
                  </div>
                  <div className="field">
                    <label htmlFor="curr">Moeda</label>
                    <input id="curr" defaultValue="(configurada via WIFIC_MOEDA)" />
                  </div>
                  <button className="btn primary" onClick={() => toast("Definições gravadas")}>Gravar alterações</button>
                </section>
                <section className="card panel">
                  <h2 className="panel-title" style={{ marginBottom: 14 }}>Router</h2>
                  <div className="field">
                    <label htmlFor="host">Endereço do MikroTik</label>
                    <input id="host" defaultValue="192.168.88.1" className="mono" />
                  </div>
                  <div className="field">
                    <label htmlFor="api">Porta da API</label>
                    <input id="api" defaultValue="8728" />
                  </div>
                  <div className="status-row">
                    <span>Ligação</span>
                    <span className="status-right"><span className="dot" /> Ligado</span>
                  </div>
                </section>
              </div>
            </>
          )}
        </main>
      </div>

      <nav className="mobile-nav" aria-label="Móvel">
        {MOBILE_NAV.map((n) => (
          <button key={n.id} className={page === n.id ? "active" : ""} onClick={() => go(n.id)}>
            <SvgIcon name={n.icon} />
            {n.label}
          </button>
        ))}
      </nav>

      {genOpen && (
        <Modal title="Gerar vales" hint="Lote de códigos de acesso para clientes." onClose={() => setGenOpen(false)}>
          {!genDone ? (
            <>
              <div className="grid-2">
                <div className="field">
                  <label htmlFor="qty">Quantidade</label>
                  <input id="qty" type="number" min="1" max="1000" value={genForm.qty} onChange={(e) => setGenForm({ ...genForm, qty: e.target.value })} />
                </div>
                <div className="field">
                  <label htmlFor="plan">Plano</label>
                  <select id="plan" value={genForm.plan} onChange={(e) => setGenForm({ ...genForm, plan: e.target.value })}>
                    <option>1 Hora — 100</option>
                    <option>3 Horas — 200</option>
                    <option>1 Dia — 500</option>
                    <option>7 Dias — 2,000</option>
                  </select>
                </div>
              </div>
              <div className="grid-2">
                <div className="field">
                  <label htmlFor="speed">Velocidade</label>
                  <input id="speed" value={genForm.speed} onChange={(e) => setGenForm({ ...genForm, speed: e.target.value })} />
                </div>
                <div className="field">
                  <label htmlFor="data">Limite de dados</label>
                  <input id="data" value={genForm.data} onChange={(e) => setGenForm({ ...genForm, data: e.target.value })} />
                </div>
              </div>
              <div className="grid-2">
                <div className="field">
                  <label htmlFor="dev">Dispositivos</label>
                  <input id="dev" type="number" min="1" value={genForm.devices} onChange={(e) => setGenForm({ ...genForm, devices: Number(e.target.value) })} />
                </div>
                <div className="field">
                  <label htmlFor="exp">Validade</label>
                  <input id="exp" value={genForm.expiration} onChange={(e) => setGenForm({ ...genForm, expiration: e.target.value })} />
                </div>
              </div>
              <button className="btn primary" style={{ width: "100%", height: 40, marginTop: 8 }} onClick={generateVouchers}>
                Gerar {genForm.qty} vales
              </button>
            </>
          ) : (
            <>
              <p className="hint" style={{ color: "var(--success)" }}>{genDone} vales gerados com sucesso.</p>
              <div className="head-actions">
                <button className="btn" onClick={() => toast("CSV descarregado")}><SvgIcon name="download" size={14} /> Descarregar CSV</button>
                <button className="btn" onClick={() => toast("Impressão na fila")}><SvgIcon name="print" size={14} /> Imprimir vales</button>
              </div>
            </>
          )}
        </Modal>
      )}

      {planOpen && (
        <Modal title="Criar plano" hint="Defina duração, velocidade e preço." onClose={() => setPlanOpen(false)}>
          <div className="field">
            <label htmlFor="pn">Nome</label>
            <input id="pn" placeholder="2 Horas" />
          </div>
          <div className="grid-2">
            <div className="field">
              <label htmlFor="pp">Preço</label>
              <input id="pp" defaultValue="150" />
            </div>
            <div className="field">
              <label htmlFor="ps">Velocidade</label>
              <input id="ps" defaultValue="10 Mbps" />
            </div>
          </div>
          <button
            className="btn primary"
            style={{ width: "100%" }}
            onClick={() => {
              setPlans((p) => [...p, { id: `p${p.length}`, name: "2 Horas", price: "150", duration: "2 horas", speed: "10 Mbps", data: "2 GB", devices: 1, used: 0 }])
              setPlanOpen(false)
              toast("Plano criado")
            }}
          >
            Criar plano
          </button>
        </Modal>
      )}

      {confirm && (
        <Modal title={confirm.title} onClose={() => setConfirm(null)}>
          <p className="hint">{confirm.body}</p>
          <div className="confirm-actions">
            <button className="btn ghost" onClick={() => setConfirm(null)}>Cancelar</button>
            <button className={`btn ${confirm.danger ? "danger" : "primary"}`} onClick={confirm.onConfirm}>{confirm.action}</button>
          </div>
        </Modal>
      )}

      {device && (
        <Drawer title={device.name} onClose={() => setDevice(null)}>
          <div className="meta-list">
            {[
              ["Estado", traduzir(device.status)],
              ["Atividade", device.uptime],
              ["IP", device.ip],
              ["Clientes", device.clients],
              ["Banda", device.bandwidth],
              ["Firmware", device.firmware],
              ["CPU", device.cpu],
              ["Memória", device.memory],
              ["Visto por último", device.lastSeen],
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
            <th>Utilizador</th>
            <th>Dispositivo</th>
            <th>IP</th>
            <th>Plano</th>
            <th>Velocidade</th>
            <th>Restante</th>
            <th>Estado</th>
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
              <td><span className="badge online"><span className="dot" /> Ligado</span></td>
              <td>
                <div className="row-actions">
                  <button className="btn sm ghost" onClick={() => onView(u)}>Ver</button>
                  <button className="btn sm ghost" onClick={() => onExtend(u)}>Estender</button>
                  <button className="btn sm ghost" onClick={() => onDisconnect(u)}>Desligar</button>
                  <button className="btn sm danger" onClick={() => onBlock(u)}>Bloquear</button>
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
