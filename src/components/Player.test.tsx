import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { Player } from "./Player";
import { live } from "../test/fixtures";

const hls = vi.hoisted(() => ({
  supported: false,
  destroy: vi.fn(),
  loadSource: vi.fn(),
  attachMedia: vi.fn(),
  on: vi.fn(),
}));
vi.mock("hls.js", () => ({
  default: class {
    static isSupported() {
      return hls.supported;
    }
    static Events = { ERROR: "error", FRAG_LOADED: "loaded" };
    destroy = hls.destroy;
    loadSource = hls.loadSource;
    attachMedia = hls.attachMedia;
    on = hls.on;
  },
}));
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
  hls.supported = false;
});
it("displays encoder fallback updates and tears down native playback on exit", async () => {
  vi.spyOn(HTMLMediaElement.prototype, "canPlayType").mockReturnValue(
    "probably",
  );
  const pause = vi
    .spyOn(HTMLMediaElement.prototype, "pause")
    .mockImplementation(() => {});
  const load = vi
    .spyOn(HTMLMediaElement.prototype, "load")
    .mockImplementation(() => {});
  const fetcher = vi.fn(
    async (_path: string, _init?: RequestInit) =>
      new Response(
        JSON.stringify({
          ...live.Live,
          Encoder: "libx264",
          Warning: "NVENC failed. Using software encoding.",
        }),
        { headers: { "Content-Type": "application/json" } },
      ),
  );
  vi.stubGlobal("fetch", fetcher);
  const { unmount } = render(
    <Player serviceID={100} initial={live.Live} blocked={false} />,
  );
  const video = screen.getByLabelText(
    "Live television player",
  ) as HTMLVideoElement;
  expect(video.getAttribute("src")).toBe("/hls/100/index.m3u8");
  expect((await screen.findByRole("status")).textContent).toContain(
    "NVENC failed",
  );
  await screen.findByText("Encoder: libx264");
  unmount();
  expect(pause).toHaveBeenCalledOnce();
  expect(load).toHaveBeenCalledOnce();
  expect(video.hasAttribute("src")).toBe(false);
  expect(fetcher.mock.calls[0][1]?.signal?.aborted).toBe(true);
});
it("does not request HLS or poll status when recording capacity blocks viewing", () => {
  const fetcher = vi.fn();
  vi.stubGlobal("fetch", fetcher);
  render(<Player serviceID={100} initial={live.Live} blocked />);
  expect(
    screen.getByLabelText("Live television player").hasAttribute("src"),
  ).toBe(false);
  expect(fetcher).not.toHaveBeenCalled();
});
it("destroys HLS.js when leaving the channel", async () => {
  hls.supported = true;
  vi.spyOn(HTMLMediaElement.prototype, "canPlayType").mockReturnValue("");
  vi.spyOn(HTMLMediaElement.prototype, "pause").mockImplementation(() => {});
  vi.spyOn(HTMLMediaElement.prototype, "load").mockImplementation(() => {});
  vi.stubGlobal(
    "fetch",
    vi.fn(
      async () =>
        new Response(JSON.stringify(live.Live), {
          headers: { "Content-Type": "application/json" },
        }),
    ),
  );
  const { unmount } = render(
    <Player serviceID={100} initial={live.Live} blocked={false} />,
  );
  await waitFor(() =>
    expect(hls.loadSource).toHaveBeenCalledWith("/hls/100/index.m3u8"),
  );
  expect(hls.attachMedia).toHaveBeenCalledOnce();
  unmount();
  expect(hls.destroy).toHaveBeenCalledOnce();
});
