import { useEffect, useRef, useState } from "react";
import { css } from "mintcss";
import Hls from "hls.js";
import { request, type LiveStatus } from "../api";
import { Notice } from "./Notice";

const shell = css({
  overflow: "hidden",
  aspectRatio: "16/9",
  borderRadius: 14,
  background: "#000",
  boxShadow: "0 14px 50px rgba(0,0,0,.35)",
  "& video": { width: "100%", height: "100%", background: "#000" },
  "@media (max-width:620px)": { margin: "0 -14px", borderRadius: 0 },
});

export function Player({
  serviceID,
  initial,
  blocked,
}: {
  serviceID: number;
  initial: LiveStatus;
  blocked: boolean;
}) {
  const video = useRef<HTMLVideoElement>(null);
  const [status, setStatus] = useState(initial);
  const [error, setError] = useState("");
  useEffect(() => {
    if (blocked || !video.current) return;
    const element = video.current;
    const controller = new AbortController();
    const source = `/hls/${serviceID}/index.m3u8`;
    let hls: Hls | undefined;
    let retry: ReturnType<typeof setTimeout> | undefined;
    let poll: ReturnType<typeof setTimeout> | undefined;
    if (element.canPlayType("application/vnd.apple.mpegurl")) {
      element.src = source;
    } else if (Hls.isSupported()) {
      hls = new Hls({
        liveSyncDurationCount: 3,
        maxLiveSyncPlaybackRate: 1.2,
        enableWorker: false,
      });
      hls.loadSource(source);
      hls.attachMedia(element);
      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (!data.fatal) return;
        setError("Live playback was interrupted. Retrying…");
        clearTimeout(retry);
        retry = setTimeout(() => hls?.loadSource(source), 1500);
      });
      hls.on(Hls.Events.FRAG_LOADED, () => setError(""));
    } else {
      setError("This browser cannot play HLS video.");
    }
    const refresh = async () => {
      try {
        const updated = await request<LiveStatus>(
          `/api/live/${serviceID}/status`,
          { signal: controller.signal },
        );
        if (!controller.signal.aborted) setStatus(updated);
      } catch {
        // Keep the last encoder warning visible when diagnostics cannot be refreshed.
      }
      if (!controller.signal.aborted) poll = setTimeout(refresh, 2000);
    };
    void refresh();
    return () => {
      controller.abort();
      clearTimeout(poll);
      clearTimeout(retry);
      hls?.destroy();
      // Explicitly release native HLS requests when changing channels or leaving the player.
      element.pause();
      element.removeAttribute("src");
      element.load();
    };
  }, [serviceID, blocked]);
  return (
    <>
      {status.Warning && <Notice>{status.Warning}</Notice>}
      {status.Error && <Notice kind="error">{status.Error}</Notice>}
      {error && <Notice kind="error">{error}</Notice>}
      <div className={shell}>
        <video
          ref={video}
          controls
          autoPlay
          playsInline
          aria-label="Live television player"
          onError={() =>
            setError(
              "Live video could not be loaded. Check the tuner and encoder status, then retry.",
            )
          }
        />
      </div>
      <p className="muted">Encoder: {status.Encoder || "Unavailable"}</p>
    </>
  );
}
