import {
  IconHome,
  IconPlayerPlay,
  IconDeviceTv,
  IconHistory,
} from "@tabler/icons-react";
import { css } from "mintcss";
import type { ReactNode } from "react";
import type { Snapshot } from "../api";
import { RippleLink } from "./Ripple";

export function Layout({
  active,
  recording,
  children,
}: {
  active: string;
  recording?: Snapshot;
  children: ReactNode;
}) {
  return (
    <div className={styles}>
      <header className="topbar">
        <RippleLink className="brand" to="/">
          <span className="brand-mark">
            <IconPlayerPlay size={16} fill="currentColor" aria-hidden="true" />
          </span>
          <span>placidus</span>
        </RippleLink>
        <div className="top-status">
          <span
            aria-hidden="true"
            className={`dot ${recording && recording.Occupied >= recording.Limit ? "warn" : ""}`}
          />
          {recording?.Occupied
            ? `${recording.Occupied} tuner service(s) recording`
            : "Ready for live TV"}
        </div>
      </header>
      <nav className="rail" aria-label="Primary navigation">
        {[
          { to: "/", key: "home", icon: IconHome, text: "Home" },
          { to: "/", key: "live", icon: IconDeviceTv, text: "Live" },
          {
            to: "/reservations",
            key: "reservations",
            icon: IconHistory,
            text: "Library",
          },
        ].map(({ to, key, icon: Icon, text }) => (
          <RippleLink
            key={key}
            to={to}
            className={active === key ? "active" : ""}
            aria-current={active === key ? "page" : undefined}
          >
            <Icon
              className="rail-icon"
              size={22}
              stroke={1.8}
              aria-hidden="true"
            />
            {text}
          </RippleLink>
        ))}
      </nav>
      <main className="page" id="main">
        {children}
      </main>
    </div>
  );
}

const styles = css({
  "& .topbar": {
    position: "fixed",
    inset: "0 0 auto 0",
    height: "64px",
    zIndex: "20",
    display: "flex",
    alignItems: "center",
    gap: "24px",
    padding: "0 24px",
    background: "rgba(15,15,15,.94)",
    backdropFilter: "blur(14px)",
    borderBottom: "1px solid #222",
  },
  "& .brand": {
    display: "flex",
    alignItems: "center",
    gap: "10px",
    fontSize: "20px",
    fontWeight: "800",
    letterSpacing: "-.5px",
  },
  "& .brand-mark": {
    display: "grid",
    placeItems: "center",
    width: "34px",
    height: "24px",
    borderRadius: "8px",
    background: "linear-gradient(135deg,var(--accent),var(--accent2))",
    fontSize: "12px",
  },
  "& .top-status": {
    marginLeft: "auto",
    display: "flex",
    alignItems: "center",
    gap: "8px",
    color: "var(--muted)",
    fontSize: "13px",
  },
  "& .dot": {
    width: "8px",
    height: "8px",
    borderRadius: "50%",
    background: "var(--ok)",
  },
  "& .dot.warn": {
    background: "var(--warn)",
  },
  "& .rail": {
    position: "fixed",
    top: "64px",
    bottom: "0",
    left: "0",
    width: "92px",
    padding: "18px 10px",
    background: "var(--bg)",
    zIndex: "10",
  },
  "& .rail a": {
    display: "flex",
    minHeight: "62px",
    margin: "4px 0",
    padding: "10px 4px",
    flexDirection: "column",
    alignItems: "center",
    justifyContent: "center",
    gap: "6px",
    borderRadius: "12px",
    color: "var(--muted)",
    fontSize: "11px",
  },
  "& .rail a:hover, & .rail a.active": {
    background: "var(--panel2)",
    color: "var(--text)",
  },
  "& .rail-icon": {
    fontSize: "20px",
  },
  "& .page": {
    maxWidth: "1600px",
    margin: "0 auto",
    padding: "30px 34px 60px 126px",
  },
  // Keep this range disjoint from mobile: mintcss sorts conditional rule keys.
  "@media (min-width:621px) and (max-width:900px)": {
    "& .page": {
      paddingLeft: "112px",
    },
  },
  "@media (max-width:620px)": {
    "& .topbar": {
      height: "56px",
      padding: "0 14px",
    },
    "& .top-status": {
      display: "none",
    },
    "& .rail": {
      top: "auto",
      bottom: "0",
      width: "100%",
      height: "64px",
      padding: "4px 8px",
      display: "grid",
      gridTemplateColumns: "repeat(3,1fr)",
      background: "rgba(15,15,15,.97)",
      borderTop: "1px solid #282828",
    },
    "& .rail a": {
      minHeight: "54px",
      margin: "0",
      flexDirection: "row",
      fontSize: "12px",
    },
    "& .rail-icon": {
      fontSize: "18px",
    },
    "& .page": {
      padding: "22px 14px 90px",
    },
  },
});
