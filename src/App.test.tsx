import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, expect, it, vi } from "vitest";
import { App } from "./App";
import { home, library } from "./test/fixtures";

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
function respond(value: unknown, status = 200) {
  return new Response(JSON.stringify(value), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}
function renderApp(path = "/") {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <App />
    </MemoryRouter>,
  );
}
it("reserves authoritative programs and navigates to the library without a document reload", async () => {
  const fetcher = vi.fn(async (path: string, init?: RequestInit) => {
    if (path === "/api/home") return respond(home);
    if (init?.method === "POST")
      return respond({ message: "Recording reserved." });
    return respond(library);
  });
  vi.stubGlobal("fetch", fetcher);
  renderApp();
  await screen.findByRole("heading", { name: "On air now" });
  fireEvent.click(screen.getByRole("button", { name: "Reserve" }));
  await screen.findByRole("heading", { name: "Reservations & recordings" });
  expect(screen.getByText("Recording reserved.")).toBeTruthy();
  const mutation = fetcher.mock.calls.find(
    ([, init]) => init?.method === "POST",
  )!;
  expect(String(mutation[1]?.body)).toBe("program_id=99");
});
it("shows reservation conflicts and allows retry", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_path: string, init?: RequestInit) =>
      respond(
        init?.method === "POST"
          ? { error: "This reservation conflicts with another service." }
          : home,
        init?.method === "POST" ? 400 : 200,
      ),
    ),
  );
  renderApp();
  fireEvent.click(await screen.findByRole("button", { name: "Reserve" }));
  expect((await screen.findByRole("alert")).textContent).toContain("conflicts");
  expect(
    (screen.getByRole("button", { name: "Reserve" }) as HTMLButtonElement)
      .disabled,
  ).toBe(false);
});
it("cancels reservations and refreshes their persisted state", async () => {
  let cancelled = false;
  vi.stubGlobal(
    "fetch",
    vi.fn(async (_path: string, init?: RequestInit) => {
      if (init?.method === "POST") {
        cancelled = true;
        return respond({ message: "Reservation cancelled." });
      }
      return respond(
        cancelled
          ? {
              ...library,
              Reservations: library.Reservations!.map((item) => ({
                ...item,
                state: "cancelled",
              })),
            }
          : library,
      );
    }),
  );
  renderApp("/reservations");
  fireEvent.click(await screen.findByRole("button", { name: "Cancel" }));
  await screen.findByText("cancelled");
  expect(screen.getByRole("status").textContent).toBe("Reservation cancelled.");
  expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
});
it("renders dependency and software fallback warnings", async () => {
  vi.stubGlobal(
    "fetch",
    vi.fn(async () =>
      respond({
        ...home,
        Services: null,
        BackendError: "Check the configured Mirakurun URL.",
        Capabilities: {
          ...home.Capabilities,
          Warning: "Hardware initialization failed; using libx264.",
        },
      }),
    ),
  );
  renderApp();
  expect((await screen.findByRole("alert")).textContent).toContain("Mirakurun");
  expect(screen.getByText(/Hardware initialization failed/)).toBeTruthy();
  expect(
    screen.getByText("No services are available from Mirakurun."),
  ).toBeTruthy();
});
it("reports network failures and retries", async () => {
  const fetcher = vi
    .fn()
    .mockRejectedValueOnce(new Error("Network unavailable"))
    .mockResolvedValueOnce(respond(home));
  vi.stubGlobal("fetch", fetcher);
  renderApp();
  expect((await screen.findByRole("alert")).textContent).toBe(
    "Network unavailable",
  );
  fireEvent.click(screen.getByRole("button", { name: "Retry" }));
  await screen.findByRole("heading", { name: "On air now" });
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
});
