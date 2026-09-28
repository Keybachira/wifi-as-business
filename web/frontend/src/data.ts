// Tipos do domínio
export interface Company {
  name: string;
  role: string;
  initials: string;
}
export interface Network {
  id: string;
  name: string;
}
export interface KPI {
  id: string;
  label: string;
  value: string;
  delta: string;
  up: boolean;
  series: number[];
}

export interface Bandwidth {
  hour: string;
  download: number;
  upload: number;
}

export type LiveStatus = "Online" | "Offline";
export interface LiveUser {
  id: number;
  user: string;
  device: string;
  ip: string;
  plan: string;
  speed: string;
  remaining: string;
  status: LiveStatus;
  mac: string;
}

export type VoucherStatus = "Available" | "Active" | "Expired" | "Revoked";
export interface Voucher {
  code: string;
  plan: string;
  duration: string;
  device: string;
  created: string;
  status: VoucherStatus;
}

export interface Plan {
  id: string;
  name: string;
  price: string;
  duration: string;
  speed: string;
  data: string;
  devices: number;
  used: number;
}

export type DeviceStatus = "Online" | "Offline";
export type DeviceType = "Router" | "Switch" | "Access Point" | "Internet";
export interface Device {
  id: string;
  type: DeviceType;
  name: string;
  ip: string;
  status: DeviceStatus;
  uptime: string;
  clients: number | string;
  bandwidth: string;
  firmware: string;
  cpu: string;
  memory: string;
  lastSeen: string;
}

export type SessionStatus = "Live" | "Ended";
export interface Session {
  id: string;
  user: string;
  started: string;
  duration: string;
  data: string;
  plan: string;
  status: SessionStatus;
}

export interface Share {
  name: string;
  value: number;
}

export interface TeamMember {
  name: string;
  role: string;
  email: string;
  last: string;
}

export type UserStatus = "Active" | "Disabled";
export interface PortalUser {
  name: string;
  email: string;
  role: string;
  status: UserStatus;
}

export const company: Company = {
  name: "Kimbo Networks",
  role: "Owner",
  initials: "KN",
};

export const networks: Network[] = [
  { id: "main", name: "Main Network" },
  { id: "hotel", name: "Hotel Lobby" },
  { id: "cafe", name: "Cafe Guest" },
];

export const kpis: KPI[] = [
  {
    id: "users",
    label: "Online Users",
    value: "47",
    delta: "+12%",
    up: true,
    series: [18, 22, 19, 28, 31, 36, 41, 39, 44, 47],
  },
  {
    id: "vouchers",
    label: "Active Vouchers",
    value: "326",
    delta: "+4%",
    up: true,
    series: [280, 290, 301, 297, 310, 318, 322, 319, 324, 326],
  },
  {
    id: "bw",
    label: "Bandwidth",
    value: "184 Mbps",
    delta: "-8%",
    up: false,
    series: [120, 140, 160, 210, 190, 175, 200, 188, 170, 184],
  },
  {
    id: "rev",
    label: "Today's Revenue",
    value: "128,500 Kz",
    delta: "+18%",
    up: true,
    series: [40, 55, 48, 70, 82, 90, 105, 112, 120, 128],
  },
];

export const bandwidth: Bandwidth[] = Array.from({ length: 24 }, (_, i) => {
  const hour = i;
  const download =
    40 +
    Math.round(
      Math.sin(i / 3) * 50 + (i > 8 && i < 22 ? 70 : 10) + (i % 4) * 8,
    );
  const upload =
    12 + Math.round(Math.cos(i / 2.4) * 18 + (i > 8 && i < 22 ? 22 : 4));
  return { hour: `${String(hour).padStart(2, "0")}:00`, download, upload };
});

export const liveUsers: LiveUser[] = [
  {
    id: 1,
    user: "+244 923 441 882",
    device: "iPhone",
    ip: "192.168.1.24",
    plan: "1 Hour",
    speed: "5 Mbps",
    remaining: "42 min",
    status: "Online",
    mac: "A4:83:E7:12:90:21",
  },
  {
    id: 2,
    user: "Guest-029",
    device: "Android",
    ip: "192.168.1.31",
    plan: "3 Hours",
    speed: "10 Mbps",
    remaining: "2h 14m",
    status: "Online",
    mac: "C8:3A:35:90:11:AA",
  },
  {
    id: 3,
    user: "+244 912 778 104",
    device: "Laptop",
    ip: "192.168.1.18",
    plan: "1 Day",
    speed: "20 Mbps",
    remaining: "18h 02m",
    status: "Online",
    mac: "00:1B:44:11:3A:B7",
  },
  {
    id: 4,
    user: "Guest-044",
    device: "Android",
    ip: "192.168.1.47",
    plan: "1 Hour",
    speed: "5 Mbps",
    remaining: "11 min",
    status: "Online",
    mac: "F0:9F:C2:33:10:08",
  },
  {
    id: 5,
    user: "+244 935 220 019",
    device: "iPhone",
    ip: "192.168.1.52",
    plan: "7 Days",
    speed: "20 Mbps",
    remaining: "5d 4h",
    status: "Online",
    mac: "3C:22:FB:88:09:C1",
  },
  {
    id: 6,
    user: "Staff-02",
    device: "Windows",
    ip: "192.168.1.10",
    plan: "Staff",
    speed: "50 Mbps",
    remaining: "Unlimited",
    status: "Online",
    mac: "B8:27:EB:44:12:90",
  },
  {
    id: 7,
    user: "Guest-061",
    device: "Tablet",
    ip: "192.168.1.63",
    plan: "3 Hours",
    speed: "10 Mbps",
    remaining: "48 min",
    status: "Online",
    mac: "DC:A6:32:01:77:2E",
  },
  {
    id: 8,
    user: "+244 941 003 552",
    device: "Android",
    ip: "192.168.1.71",
    plan: "1 Day",
    speed: "10 Mbps",
    remaining: "6h 40m",
    status: "Online",
    mac: "E4:5F:01:AB:32:19",
  },
];

export const vouchersSeed: Voucher[] = [
  {
    code: "ZG-82LX-PA",
    plan: "1 Hour",
    duration: "1h",
    device: "1 device",
    created: "Today",
    status: "Available",
  },
  {
    code: "ZG-91KQ-TR",
    plan: "3 Hours",
    duration: "3h",
    device: "1 device",
    created: "Today",
    status: "Active",
  },
  {
    code: "ZG-44NM-QX",
    plan: "1 Day",
    duration: "24h",
    device: "2 devices",
    created: "Yesterday",
    status: "Active",
  },
  {
    code: "ZG-18VB-YH",
    plan: "1 Hour",
    duration: "1h",
    device: "1 device",
    created: "Yesterday",
    status: "Expired",
  },
  {
    code: "ZG-77CD-LK",
    plan: "7 Days",
    duration: "7d",
    device: "2 devices",
    created: "12 Sep",
    status: "Available",
  },
  {
    code: "ZG-03AF-WE",
    plan: "3 Hours",
    duration: "3h",
    device: "1 device",
    created: "12 Sep",
    status: "Revoked",
  },
  {
    code: "ZG-55PL-OK",
    plan: "1 Day",
    duration: "24h",
    device: "1 device",
    created: "11 Sep",
    status: "Available",
  },
  {
    code: "ZG-29HJ-UI",
    plan: "1 Hour",
    duration: "1h",
    device: "1 device",
    created: "11 Sep",
    status: "Active",
  },
];

export const plans: Plan[] = [
  {
    id: "1h",
    name: "1 Hour",
    price: "100 Kz",
    duration: "1 hour",
    speed: "5 Mbps",
    data: "1 GB",
    devices: 1,
    used: 842,
  },
  {
    id: "3h",
    name: "3 Hours",
    price: "200 Kz",
    duration: "3 hours",
    speed: "10 Mbps",
    data: "3 GB",
    devices: 1,
    used: 511,
  },
  {
    id: "1d",
    name: "1 Day",
    price: "500 Kz",
    duration: "24 hours",
    speed: "20 Mbps",
    data: "8 GB",
    devices: 2,
    used: 268,
  },
  {
    id: "7d",
    name: "7 Days",
    price: "2,000 Kz",
    duration: "7 days",
    speed: "20 Mbps",
    data: "30 GB",
    devices: 2,
    used: 94,
  },
];

export const devices: Device[] = [
  {
    id: "router",
    type: "Router",
    name: "MikroTik RB4011",
    ip: "192.168.88.1",
    status: "Online",
    uptime: "42d 6h",
    clients: 47,
    bandwidth: "184 Mbps",
    firmware: "7.15.3",
    cpu: "18%",
    memory: "41%",
    lastSeen: "Just now",
  },
  {
    id: "switch",
    type: "Switch",
    name: "CRS328-24P",
    ip: "192.168.88.2",
    status: "Online",
    uptime: "42d 6h",
    clients: 8,
    bandwidth: "184 Mbps",
    firmware: "7.14.2",
    cpu: "6%",
    memory: "22%",
    lastSeen: "Just now",
  },
  {
    id: "ap1",
    type: "Access Point",
    name: "AP-01 Lobby",
    ip: "192.168.88.11",
    status: "Online",
    uptime: "18d 2h",
    clients: 14,
    bandwidth: "52 Mbps",
    firmware: "6.6.55",
    cpu: "11%",
    memory: "34%",
    lastSeen: "Just now",
  },
  {
    id: "ap2",
    type: "Access Point",
    name: "AP-02 Cafe",
    ip: "192.168.88.12",
    status: "Online",
    uptime: "18d 2h",
    clients: 11,
    bandwidth: "41 Mbps",
    firmware: "6.6.55",
    cpu: "9%",
    memory: "31%",
    lastSeen: "Just now",
  },
  {
    id: "ap3",
    type: "Access Point",
    name: "AP-03 Patio",
    ip: "192.168.88.13",
    status: "Offline",
    uptime: "—",
    clients: 0,
    bandwidth: "0 Mbps",
    firmware: "6.5.28",
    cpu: "—",
    memory: "—",
    lastSeen: "2h ago",
  },
  {
    id: "ap4",
    type: "Access Point",
    name: "AP-04 Hall",
    ip: "192.168.88.14",
    status: "Online",
    uptime: "9d 14h",
    clients: 8,
    bandwidth: "28 Mbps",
    firmware: "6.6.55",
    cpu: "7%",
    memory: "29%",
    lastSeen: "Just now",
  },
  {
    id: "ap5",
    type: "Access Point",
    name: "AP-05 Rooms A",
    ip: "192.168.88.15",
    status: "Online",
    uptime: "21d 1h",
    clients: 6,
    bandwidth: "19 Mbps",
    firmware: "6.6.55",
    cpu: "5%",
    memory: "27%",
    lastSeen: "Just now",
  },
  {
    id: "ap6",
    type: "Access Point",
    name: "AP-06 Rooms B",
    ip: "192.168.88.16",
    status: "Online",
    uptime: "21d 1h",
    clients: 4,
    bandwidth: "12 Mbps",
    firmware: "6.6.55",
    cpu: "4%",
    memory: "24%",
    lastSeen: "Just now",
  },
  {
    id: "ap7",
    type: "Access Point",
    name: "AP-07 Kitchen",
    ip: "192.168.88.17",
    status: "Online",
    uptime: "6d 8h",
    clients: 2,
    bandwidth: "8 Mbps",
    firmware: "6.6.55",
    cpu: "3%",
    memory: "22%",
    lastSeen: "Just now",
  },
  {
    id: "ap8",
    type: "Access Point",
    name: "AP-08 Office",
    ip: "192.168.88.18",
    status: "Online",
    uptime: "42d 6h",
    clients: 2,
    bandwidth: "24 Mbps",
    firmware: "6.6.55",
    cpu: "8%",
    memory: "30%",
    lastSeen: "Just now",
  },
];

export const sessions: Session[] = [
  {
    id: "s1",
    user: "+244 923 441 882",
    started: "08:12",
    duration: "42 min",
    data: "420 MB",
    plan: "1 Hour",
    status: "Live",
  },
  {
    id: "s2",
    user: "Guest-029",
    started: "07:01",
    duration: "2h 14m",
    data: "1.2 GB",
    plan: "3 Hours",
    status: "Live",
  },
  {
    id: "s3",
    user: "Guest-018",
    started: "Yesterday 21:40",
    duration: "58 min",
    data: "310 MB",
    plan: "1 Hour",
    status: "Ended",
  },
  {
    id: "s4",
    user: "+244 912 778 104",
    started: "06:18",
    duration: "4h 02m",
    data: "2.8 GB",
    plan: "1 Day",
    status: "Live",
  },
  {
    id: "s5",
    user: "Guest-007",
    started: "Yesterday 16:02",
    duration: "3h 00m",
    data: "890 MB",
    plan: "3 Hours",
    status: "Ended",
  },
];

export const hourlyUsers: number[] = [
  4, 3, 2, 2, 3, 6, 12, 21, 29, 34, 38, 41, 44, 47, 45, 42, 39, 36, 31, 22, 16,
  11, 8, 6,
];
export const revenueSeries: number[] = [
  8, 12, 9, 18, 22, 16, 28, 31, 24, 36, 41, 38,
];
export const planShare: Share[] = [
  { name: "1 Hour", value: 46 },
  { name: "3 Hours", value: 28 },
  { name: "1 Day", value: 18 },
  { name: "7 Days", value: 8 },
];
export const deviceShare: Share[] = [
  { name: "Android", value: 52 },
  { name: "iPhone", value: 31 },
  { name: "Laptop", value: 12 },
  { name: "Other", value: 5 },
];

export const team: TeamMember[] = [
  { name: "Admin Kimbo", role: "Owner", email: "admin@kimbo.ao", last: "Now" },
  {
    name: "Marta Silva",
    role: "Operator",
    email: "marta@kimbo.ao",
    last: "12 min ago",
  },
  {
    name: "Paulo Neto",
    role: "Support",
    email: "paulo@kimbo.ao",
    last: "Yesterday",
  },
];

export const portalUsers: PortalUser[] = [
  { name: "Admin", email: "admin@kimbo.ao", role: "Owner", status: "Active" },
  {
    name: "Front Desk",
    email: "desk@kimbo.ao",
    role: "Operator",
    status: "Active",
  },
  {
    name: "Night Shift",
    email: "night@kimbo.ao",
    role: "Operator",
    status: "Disabled",
  },
];

export function generateCodes(n: number): string[] {
  const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789";
  const make = () =>
    Array.from(
      { length: 8 },
      () => alphabet[Math.floor(Math.random() * alphabet.length)],
    ).join("");
  return Array.from({ length: n }, () => {
    const raw = make();
    return `ZG-${raw.slice(0, 4)}-${raw.slice(4)}`;
  });
}
