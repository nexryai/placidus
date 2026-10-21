import { IconCalendarWeek, IconPlayerPlay, IconX } from "@tabler/icons-react";
import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { css } from "mintcss";
import { dateTime, clock, duration, mutate, type LibraryData } from "../api";
import { Button, RippleLink, buttonLinkClass } from "../components/Ripple";
import { Notice } from "../components/Notice";

export function Library({
  data,
  refresh,
}: {
  data: LibraryData;
  refresh: () => void;
}) {
  const navigate = useNavigate();
  const [pending, setPending] = useState("");
  const [error, setError] = useState("");
  async function cancel(id: string) {
    setPending(id);
    setError("");
    try {
      const result = await mutate(`/api/reservations/${id}/cancel`);
      navigate("/reservations", {
        replace: true,
        state: { message: result.message },
      });
      refresh();
    } catch (error) {
      setError((error as Error).message);
    } finally {
      setPending("");
    }
  }
  return (
    <div className={styles}>
      <div className="page-head">
        <div>
          <p className="eyebrow">Library</p>
          <h1>Reservations & recordings</h1>
        </div>
        <RippleLink className={buttonLinkClass} to="/">
          <IconCalendarWeek size={18} aria-hidden="true" />
          Browse guide
        </RippleLink>
      </div>
      {error && <Notice kind="error">{error}</Notice>}
      {data.Page.Recording.Occupied > 0 && (
        <Notice>
          Recording is using tuner capacity. Live viewing is limited to services
          already being recorded when all tuners are occupied.
        </Notice>
      )}
      <section className="reservation-list">
        {data.Reservations?.length ? (
          data.Reservations.map((item) => (
            <article className="reservation" key={item.id}>
              <span className={`state state-${item.state}`}>{item.state}</span>
              <div>
                <h3>{item.title}</h3>
                <p className="muted">
                  {dateTime(item.startAt)}–{clock(item.endAt)} ·{" "}
                  {duration(item.startAt, item.endAt)}
                </p>
                {item.error && <Notice kind="error">{item.error}</Notice>}
              </div>
              <div className="actions">
                {item.state === "completed" && (
                  <a
                    className={buttonLinkClass}
                    href={`/recordings/${item.id}/file`}
                  >
                    <IconPlayerPlay size={16} aria-hidden="true" />
                    Play / download
                  </a>
                )}
                {(item.state === "scheduled" || item.state === "recording") && (
                  <Button
                    className="danger small"
                    disabled={Boolean(pending)}
                    onClick={() => cancel(item.id)}
                  >
                    <IconX size={16} aria-hidden="true" />
                    {pending === item.id ? "Cancelling…" : "Cancel"}
                  </Button>
                )}
              </div>
            </article>
          ))
        ) : (
          <div className="empty">
            <h3>No reservations yet</h3>
            <p>Choose a program from the guide to schedule a recording.</p>
          </div>
        )}
      </section>
    </div>
  );
}

const styles = css({
  "& .actions": {
    display: "flex",
    flexWrap: "wrap",
    gap: "10px",
    marginTop: "12px",
  },
  "& .reservation-list": {
    display: "flex",
    flexDirection: "column",
    gap: "12px",
  },
  "& .reservation": {
    display: "grid",
    gridTemplateColumns: "110px minmax(0,1fr) auto",
    gap: "20px",
    alignItems: "center",
    padding: "18px",
    border: "1px solid var(--line)",
    borderRadius: "14px",
    background: "var(--panel)",
  },
  "& .state": {
    display: "inline-flex",
    justifyContent: "center",
    padding: "6px 9px",
    borderRadius: "20px",
    background: "#333",
    color: "var(--muted)",
    fontSize: "11px",
    fontWeight: "800",
    textTransform: "uppercase",
  },
  "& .state-recording": {
    background: "#5a1420",
    color: "#ffb1bd",
  },
  "& .state-completed": {
    background: "#173828",
    color: "#9be2ba",
  },
  "& .state-failed": {
    background: "#3f2020",
    color: "#ffaaaa",
  },
  "& .state-cancelled": {
    opacity: ".65",
  },
  "@media (max-width:900px)": {
    "& .reservation": {
      gridTemplateColumns: "1fr",
    },
  },
});
