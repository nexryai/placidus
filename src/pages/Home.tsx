import {
  IconCalendarWeek,
  IconServer,
  IconCpu,
  IconDeviceTv,
} from "@tabler/icons-react";
import { css } from "mintcss";
import type { HomeData } from "../api";
import { clock } from "../api";
import { RippleLink, buttonLinkClass } from "../components/Ripple";
import { Notice } from "../components/Notice";
import { ReserveButton } from "../components/ReserveButton";

export function Home({ data }: { data: HomeData }) {
  const services = data.Services ?? [];
  return (
    <div className={styles}>
      <div className="page-head">
        <div>
          <p className="eyebrow">Your television</p>
          <h1>On air now</h1>
        </div>
        <RippleLink className={buttonLinkClass} to="/#guide">
          <IconCalendarWeek size={18} aria-hidden="true" />
          Program guide
        </RippleLink>
      </div>
      {data.BackendError && (
        <Notice kind="error">
          <strong>Mirakurun is unavailable.</strong> {data.BackendError}
        </Notice>
      )}
      {data.Capabilities.Warning && (
        <Notice>
          <strong>Encoding notice.</strong> {data.Capabilities.Warning}
        </Notice>
      )}
      {data.Page.Recording.Occupied > 0 && (
        <Notice>
          <strong>Recording has priority.</strong> When all tuners are occupied,
          only a service already being recorded can be watched.
        </Notice>
      )}
      <section className="cards" aria-label="Current programs">
        {services
          .filter((service) => service.Current)
          .map((service) => (
            <article className="card" key={service.ID}>
              <RippleLink className="thumb" to={`/live/${service.ID}`}>
                <span className="channel-badge">{service.Number || "TV"}</span>
                <span className="thumb-title">{service.Current!.Name}</span>
                <span className="live-pill">LIVE</span>
              </RippleLink>
              <div className="card-meta">
                <div className="avatar">{service.Number || "TV"}</div>
                <div>
                  <h3 className="card-title">
                    <RippleLink to={`/live/${service.ID}`}>
                      {service.Current!.Name}
                    </RippleLink>
                  </h3>
                  <div className="card-sub">
                    {service.Name} · until {clock(service.Current!.EndAt)}
                  </div>
                </div>
              </div>
            </article>
          ))}
        {!services.length && (
          <div className="empty">No services are available from Mirakurun.</div>
        )}
      </section>
      <h2 id="guide">Program guide</h2>
      <section className="guide" aria-label="Program guide">
        {services.map((service) => (
          <article className="guide-row" key={service.ID}>
            <div className="guide-channel">
              <span className="channel-logo">{service.Number || "TV"}</span>
              <div>
                {service.Name}
                <div className="muted">{service.Type}</div>
              </div>
            </div>
            <div className="guide-programs">
              {service.Programs?.length ? (
                service.Programs.map((program) => (
                  <div className="program" key={program.ID}>
                    <div className="program-time">
                      {clock(program.StartAt)}–{clock(program.EndAt)}
                    </div>
                    <p>
                      <strong>{program.Name}</strong>
                    </p>
                    <ReserveButton programID={program.ID} small />
                  </div>
                ))
              ) : (
                <div className="program muted">No upcoming EPG data</div>
              )}
            </div>
          </article>
        ))}
      </section>
      <h2>System</h2>
      <section className="system-grid">
        <div className="system-card">
          <strong>
            <IconServer size={18} aria-hidden="true" /> Mirakurun
          </strong>
          <span>
            {data.BackendError
              ? "Disconnected"
              : `Connected · ${services.length} services`}
          </span>
        </div>
        <div className="system-card">
          <strong>
            <IconCpu size={18} aria-hidden="true" /> FFmpeg
          </strong>
          <span>
            {data.Capabilities.Available
              ? `${data.Capabilities.Selected.Name} · ${data.Capabilities.Selected.Hardware ? "hardware" : "software"}`
              : "Unavailable"}
          </span>
        </div>
        <div className="system-card">
          <strong>
            <IconDeviceTv size={18} aria-hidden="true" /> Recording capacity
          </strong>
          <span>
            {data.Page.Recording.Occupied} of {data.Page.Recording.Limit}{" "}
            configured tuner services occupied
          </span>
        </div>
      </section>
    </div>
  );
}

const styles = css({
  "& .cards": {
    display: "grid",
    gridTemplateColumns: "repeat(auto-fill,minmax(250px,1fr))",
    gap: "24px 18px",
  },
  "& .card": {
    minWidth: "0",
  },
  "& .thumb": {
    position: "relative",
    display: "flex",
    aspectRatio: "16/9",
    alignItems: "flex-end",
    overflow: "hidden",
    padding: "18px",
    borderRadius: "14px",
    background:
      "radial-gradient(circle at 80% 20%,rgba(255,56,92,.35),transparent 32%),linear-gradient(145deg,#393939,#171717)",
    transition: "transform .18s ease",
  },
  "& .thumb:hover": {
    transform: "translateY(-2px)",
  },
  "& .channel-badge": {
    position: "absolute",
    left: "14px",
    top: "14px",
    display: "grid",
    placeItems: "center",
    width: "44px",
    height: "44px",
    borderRadius: "50%",
    background: "#fff",
    color: "#111",
    fontSize: "15px",
    fontWeight: "900",
  },
  "& .live-pill": {
    position: "absolute",
    right: "10px",
    bottom: "10px",
    padding: "4px 7px",
    borderRadius: "5px",
    background: "var(--accent2)",
    fontSize: "11px",
    fontWeight: "800",
  },
  "& .thumb-title": {
    maxWidth: "80%",
    fontSize: "17px",
    fontWeight: "750",
    textShadow: "0 2px 12px #000",
  },
  "& .card-meta": {
    display: "flex",
    gap: "12px",
    padding: "12px 4px",
  },
  "& .card-meta .avatar": {
    flex: "0 0 36px",
    height: "36px",
    display: "grid",
    placeItems: "center",
    borderRadius: "50%",
    background: "var(--panel2)",
    fontWeight: "800",
  },
  "& .card-title": {
    margin: "0 0 4px",
    fontSize: "15px",
    lineHeight: "1.35",
  },
  "& .card-sub": {
    color: "var(--muted)",
    fontSize: "13px",
  },
  "& .guide": {
    display: "flex",
    flexDirection: "column",
    gap: "12px",
  },
  "& .guide-row": {
    display: "grid",
    gridTemplateColumns: "190px 1fr",
    gap: "12px",
    padding: "12px",
    borderRadius: "14px",
    background: "var(--panel)",
  },
  "& .guide-channel": {
    display: "flex",
    alignItems: "center",
    gap: "12px",
    fontWeight: "750",
  },
  "& .guide-programs": {
    display: "flex",
    gap: "10px",
    overflowX: "auto",
  },
  "& .program": {
    flex: "0 0 260px",
    padding: "12px",
    border: "1px solid var(--line)",
    borderRadius: "10px",
    background: "#202020",
  },
  "& .program.current": {
    borderColor: "var(--accent)",
    background: "#291a1e",
  },
  "& .program-time": {
    color: "var(--muted)",
    fontSize: "12px",
  },
  "& .program p": {
    margin: "6px 0",
    fontSize: "14px",
  },
  "& .channel-logo": {
    display: "grid",
    placeItems: "center",
    width: "42px",
    height: "42px",
    borderRadius: "50%",
    background: "#eee",
    color: "#111",
    fontWeight: "900",
  },
  "& .system-grid": {
    display: "grid",
    gridTemplateColumns: "repeat(auto-fit,minmax(230px,1fr))",
    gap: "12px",
  },
  "& .system-card": {
    padding: "18px",
    borderRadius: "14px",
    background: "var(--panel)",
  },
  "& .system-card strong": {
    display: "flex",
    alignItems: "center",
    gap: "8px",
    marginBottom: "5px",
  },
  "& .system-card span": {
    fontSize: "13px",
    color: "var(--muted)",
  },
  "@media (max-width:900px)": {
    "& .guide-row": {
      gridTemplateColumns: "1fr",
    },
  },
  "@media (max-width:620px)": {
    "& .cards": {
      gap: "18px 10px",
    },
    "& .thumb": {
      borderRadius: "9px",
      padding: "10px",
    },
    "& .channel-badge": {
      left: "8px",
      top: "8px",
      width: "34px",
      height: "34px",
    },
    "& .thumb-title": {
      fontSize: "14px",
    },
    "& .card-meta .avatar": {
      display: "none",
    },
    "& .card-meta": {
      padding: "8px 2px",
    },
    "& .guide-row": {
      padding: "8px",
    },
    "& .program": {
      flexBasis: "230px",
    },
  },
  "@media (min-width:421px) and (max-width:620px)": {
    "& .cards": { gridTemplateColumns: "1fr 1fr" },
  },
  "@media (max-width:420px)": {
    "& .cards": {
      gridTemplateColumns: "1fr",
    },
  },
});
