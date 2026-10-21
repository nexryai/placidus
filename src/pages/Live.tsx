import { lazy, Suspense } from "react";
import { css } from "mintcss";
import { clock, type LiveData } from "../api";
import { Notice } from "../components/Notice";
import { RippleLink } from "../components/Ripple";
import { ReserveButton } from "../components/ReserveButton";
const Player = lazy(() =>
  import("../components/Player").then((module) => ({ default: module.Player })),
);

export function Live({ data }: { data: LiveData }) {
  const service = data.Service;
  return (
    <div className={styles}>
      {data.Page.Error && <Notice kind="error">{data.Page.Error}</Notice>}
      {data.BackendError && <Notice kind="error">{data.BackendError}</Notice>}
      <div className="player-layout">
        <section>
          <Suspense fallback={<p role="status">Loading player…</p>}>
            <Player
              key={service.ID}
              serviceID={service.ID}
              initial={data.Live}
              blocked={Boolean(data.Page.Error)}
            />
          </Suspense>
          <h1 className="player-title">
            {service.Current?.Name || service.Name}
          </h1>
          <p className="muted">
            {service.Name}
            {service.Current &&
              ` · ${clock(service.Current.StartAt)}–${clock(service.Current.EndAt)}`}
          </p>
          {service.Current && (
            <>
              <p>{service.Current.Description}</p>
              <ReserveButton programID={service.Current.ID}>
                Record this program
              </ReserveButton>
            </>
          )}
        </section>
        <aside>
          <p className="eyebrow">Channels</p>
          <div className="channel-list">
            {(data.Services ?? []).map((channel) => (
              <RippleLink
                className={`channel-item ${channel.ID === service.ID ? "active" : ""}`}
                key={channel.ID}
                to={`/live/${channel.ID}`}
              >
                <span className="channel-logo">{channel.Number || "TV"}</span>
                <div>
                  <p>
                    <strong>{channel.Name}</strong>
                  </p>
                  <small>
                    {channel.Current?.Name || "No current EPG data"}
                  </small>
                </div>
              </RippleLink>
            ))}
          </div>
        </aside>
      </div>
    </div>
  );
}

const styles = css({
  "& .player-layout": {
    display: "grid",
    gridTemplateColumns: "minmax(0,1fr) 350px",
    gap: "28px",
  },
  "& .player-title": {
    margin: "18px 0 4px",
  },
  "& .channel-list": {
    display: "flex",
    flexDirection: "column",
    gap: "8px",
    maxHeight: "75vh",
    overflow: "auto",
  },
  "& .channel-item": {
    display: "grid",
    gridTemplateColumns: "42px 1fr",
    gap: "10px",
    padding: "9px",
    borderRadius: "10px",
  },
  "& .channel-item:hover, & .channel-item.active": {
    background: "var(--panel2)",
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
  "& .channel-item p": {
    margin: "1px 0",
    fontSize: "13px",
  },
  "& .channel-item small": {
    color: "var(--muted)",
  },
  "@media (max-width:900px)": {
    "& .player-layout": {
      gridTemplateColumns: "1fr",
    },
    "& .channel-list": {
      display: "grid",
      gridTemplateColumns: "repeat(auto-fill,minmax(220px,1fr))",
      maxHeight: "none",
    },
  },
  "@media (max-width:620px)": {
    "& .player-layout": {
      gap: "18px",
    },
  },
});
