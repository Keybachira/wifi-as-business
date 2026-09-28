import type { ReactNode } from "react";

const paths: Record<string, ReactNode> = {
  overview: (
    <>
      <rect x="3.5" y="3.5" width="7" height="8" rx="1.4" />
      <rect x="13.5" y="3.5" width="7" height="5" rx="1.4" />
      <rect x="13.5" y="11.5" width="7" height="9" rx="1.4" />
      <rect x="3.5" y="14.5" width="7" height="6" rx="1.4" />
    </>
  ),
  users: (
    <>
      <circle cx="9" cy="7.5" r="3.25" />
      <path d="M3.5 19.5v-1.4A4.6 4.6 0 0 1 8.1 13.5h1.8A4.6 4.6 0 0 1 14.5 18.1v1.4" />
      <circle cx="16.75" cy="8.25" r="2.5" />
      <path d="M20.5 19.5v-1.1a3.9 3.9 0 0 0-2.7-3.7" />
    </>
  ),
  ticket: (
    <>
      <path d="M4 9.2V7.5A2.5 2.5 0 0 1 6.5 5h11A2.5 2.5 0 0 1 20 7.5v1.7a2.2 2.2 0 0 0 0 4.4v1.9A2.5 2.5 0 0 1 17.5 18h-11A2.5 2.5 0 0 1 4 15.5v-1.9a2.2 2.2 0 0 0 0-4.4Z" />
      <path d="M9 5.5v13" strokeDasharray="1.6 2.2" />
    </>
  ),
  plans: (
    <>
      <rect x="4" y="4" width="16" height="5.2" rx="1.4" />
      <rect x="4" y="11.4" width="16" height="3.8" rx="1.2" />
      <rect x="4" y="17.2" width="16" height="2.8" rx="1" />
    </>
  ),
  network: (
    <>
      <circle cx="12" cy="5.5" r="2.1" />
      <circle cx="5.2" cy="18.4" r="2.1" />
      <circle cx="18.8" cy="18.4" r="2.1" />
      <circle cx="12" cy="12.2" r="1.7" />
      <path d="M12 7.6v2.9M6.8 16.8 10.6 13.4M17.2 16.8 13.4 13.4" />
    </>
  ),
  devices: (
    <>
      <rect x="3.5" y="4" width="17" height="11.5" rx="1.8" />
      <path d="M9 20.2h6M12 15.5v4.7" />
    </>
  ),
  sessions: (
    <>
      <circle cx="12" cy="12" r="8.2" />
      <path d="M12 7.6v4.6l3.2 1.9" />
    </>
  ),
  analytics: (
    <>
      <path d="M4.5 19.5V5.5" />
      <path d="M4.5 19.5h15" />
      <path d="M8.2 15.4V11" />
      <path d="M12 15.4V8.2" />
      <path d="M15.8 15.4V6.8" />
    </>
  ),
  portal: (
    <>
      <rect x="7.5" y="2.5" width="9" height="19" rx="2.2" />
      <path d="M10.6 5.2h2.8" />
      <circle cx="12" cy="18.4" r="0.7" fill="currentColor" stroke="none" />
    </>
  ),
  team: (
    <>
      <circle cx="12" cy="7.4" r="2.8" />
      <path d="M7.2 19.5v-1.3A4.1 4.1 0 0 1 11.3 14h1.4a4.1 4.1 0 0 1 4.1 4.2v1.3" />
      <circle cx="5.4" cy="9.2" r="2.1" />
      <path d="M3.2 19.5v-.9A3.2 3.2 0 0 1 5.8 15.6" />
      <circle cx="18.6" cy="9.2" r="2.1" />
      <path d="M20.8 19.5v-.9a3.2 3.2 0 0 0-2.6-3" />
    </>
  ),
  settings: (
    <>
      <circle cx="12" cy="12" r="3.1" />
      <path d="M12 3.4v1.8M12 18.8v1.8M20.6 12h-1.8M5.2 12H3.4M18.1 5.9l-1.3 1.3M7.2 16.8l-1.3 1.3M18.1 18.1l-1.3-1.3M7.2 7.2 5.9 5.9" />
    </>
  ),
  search: (
    <>
      <circle cx="11" cy="11" r="6.2" />
      <path d="m16.2 16.2 4.1 4.1" />
    </>
  ),
  bell: (
    <>
      <path d="M6.4 9.2a5.6 5.6 0 1 1 11.2 0c0 6.2 2.4 7.8 2.4 7.8H4s2.4-1.6 2.4-7.8" />
      <path d="M10 19.6a2 2 0 0 0 4 0" />
    </>
  ),
  plus: (
    <>
      <path d="M12 5.2v13.6M5.2 12h13.6" />
    </>
  ),
  chevron: (
    <>
      <path d="m7 9.5 5 5 5-5" />
    </>
  ),
  wifi: (
    <>
      <path d="M4.8 10.4c4-4 10.4-4 14.4 0" />
      <path d="M7.6 13.6c2.5-2.4 6.3-2.4 8.8 0" />
      <path d="M10.2 16.7c1-.9 2.6-.9 3.6 0" />
      <circle cx="12" cy="19.2" r="1.05" fill="currentColor" stroke="none" />
    </>
  ),
  menu: (
    <>
      <path d="M4.5 7h15M4.5 12h15M4.5 17h15" />
    </>
  ),
  close: (
    <>
      <path d="M6.4 6.4 17.6 17.6M17.6 6.4 6.4 17.6" />
    </>
  ),
  download: (
    <>
      <path d="M12 4.5v11.2M7.6 11.4 12 15.8l4.4-4.4" />
      <path d="M5 19.5h14" />
    </>
  ),
  print: (
    <>
      <path d="M7.2 8.2V4.5h9.6v3.7" />
      <rect x="7.2" y="13.4" width="9.6" height="6.1" rx="0.8" />
      <path d="M7.2 16.2H5.4A1.9 1.9 0 0 1 3.5 14.3v-3.4A1.9 1.9 0 0 1 5.4 9h13.2a1.9 1.9 0 0 1 1.9 1.9v3.4a1.9 1.9 0 0 1-1.9 1.9h-1.8" />
    </>
  ),
  more: (
    <>
      <circle cx="6" cy="12" r="1.15" fill="currentColor" stroke="none" />
      <circle cx="12" cy="12" r="1.15" fill="currentColor" stroke="none" />
      <circle cx="18" cy="12" r="1.15" fill="currentColor" stroke="none" />
    </>
  ),
  check: (
    <>
      <path d="M5.2 12.2 9.8 16.8 18.8 7.2" />
    </>
  ),
  alert: (
    <>
      <circle cx="12" cy="12" r="8.2" />
      <path d="M12 8.2v5" />
      <circle cx="12" cy="16.4" r="0.7" fill="currentColor" stroke="none" />
    </>
  ),
  live: (
    <>
      <circle cx="12" cy="12" r="2.2" />
      <path d="M7.4 7.4a6.5 6.5 0 0 0 0 9.2M16.6 7.4a6.5 6.5 0 0 1 0 9.2" />
      <path d="M5 5a9.4 9.4 0 0 0 0 14M19 5a9.4 9.4 0 0 1 0 14" />
    </>
  ),
  router: (
    <>
      <rect x="3.5" y="10" width="17" height="8.5" rx="1.8" />
      <path d="M7.2 10V7.4M12 10V5.6M16.8 10V7.4" />
      <circle cx="8" cy="14.4" r="0.85" fill="currentColor" stroke="none" />
      <circle cx="11.2" cy="14.4" r="0.85" fill="currentColor" stroke="none" />
    </>
  ),
  export: (
    <>
      <path d="M12 14.5V4.8M8 8.2l4-3.7 4 3.7" />
      <path d="M5 14.8v3.2A1.5 1.5 0 0 0 6.5 19.5h11a1.5 1.5 0 0 0 1.5-1.5v-3.2" />
    </>
  ),
};

export function SvgIcon({
  name,
  size = 16,
  stroke = 1.6,
}: {
  name: string;
  size?: number;
  stroke?: number;
}) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={stroke}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      className="ui-icon"
    >
      {paths[name] || paths.search}
    </svg>
  );
}
