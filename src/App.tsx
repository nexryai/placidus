import { IconRefresh } from "@tabler/icons-react";
import { useEffect, useState } from "react";
import { useLocation } from "react-router-dom";
import { css } from "mintcss";
import { request, type HomeData, type LiveData, type LibraryData } from "./api";
import { Layout } from "./components/Layout";
import { Notice } from "./components/Notice";
import { Button } from "./components/Ripple";
import { Home } from "./pages/Home";
import { Live } from "./pages/Live";
import { Library } from "./pages/Library";

type Page =
  | { kind: "home"; data: HomeData }
  | { kind: "live"; data: LiveData }
  | { kind: "reservations"; data: LibraryData };
const application = css({
  minHeight: "100vh",
  paddingTop: 64,
  background: "var(--bg)",
  color: "var(--text)",
  "--bg": "#0f0f0f",
  "--panel": "#181818",
  "--panel2": "#242424",
  "--line": "#343434",
  "--text": "#f1f1f1",
  "--muted": "#aaa",
  "--accent": "#ff385c",
  "--accent2": "#ff0033",
  "--warn": "#ffca4b",
  "--ok": "#53d38a",
  colorScheme: "dark",
  fontFamily:
    'Inter,ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif',
  "&, & *": { boxSizing: "border-box" },
  "& a": { color: "inherit", textDecoration: "none" },
  "& h1, & h2, & h3, & p": { marginTop: 0 },
  "& h1": { marginBottom: 0, fontSize: 30, letterSpacing: "-.7px" },
  "& h2": { margin: "32px 0 16px", fontSize: 22 },
  "& h3": { marginBottom: 8 },
  "& .muted": { color: "var(--muted)" },
  "& .eyebrow": {
    margin: "0 0 6px",
    color: "var(--accent)",
    fontSize: 12,
    fontWeight: 800,
    textTransform: "uppercase",
    letterSpacing: "1.5px",
  },
  "& .empty": {
    padding: "50px 20px",
    border: "1px dashed var(--line)",
    borderRadius: 14,
    textAlign: "center",
    color: "var(--muted)",
  },
  "& .page-head": {
    display: "flex",
    alignItems: "end",
    justifyContent: "space-between",
    gap: 18,
    marginBottom: 24,
  },
  "@media (max-width:620px)": {
    paddingTop: 56,
    "& .page-head": { alignItems: "start", flexDirection: "column" },
  },
  "@media (prefers-reduced-motion:reduce)": {
    "& *": { scrollBehavior: "auto !important", transition: "none !important" },
  },
});

export function App() {
  const location = useLocation();
  const [page, setPage] = useState<Page>();
  const [error, setError] = useState("");
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setPage(undefined);
    setError("");
    const kind =
      location.pathname === "/"
        ? "home"
        : location.pathname === "/reservations"
          ? "reservations"
          : /^\/live\/\d+$/.test(location.pathname)
            ? "live"
            : undefined;
    if (!kind) {
      setError("Page not found. Use the navigation to return to the guide.");
      return;
    }
    const path = kind === "home" ? "/api/home" : `/api${location.pathname}`;
    request<HomeData | LiveData | LibraryData>(path, {
      signal: controller.signal,
    })
      .then((data) => {
        setPage({ kind, data } as Page);
        document.title = `${data.Page.Title} · placidus`;
      })
      .catch((error) => {
        if (!controller.signal.aborted) setError(error.message);
      });
    return () => controller.abort();
  }, [location.pathname, revision]);
  useEffect(() => {
    if (page && location.hash === "#guide")
      document.getElementById("guide")?.scrollIntoView();
  }, [page, location.hash]);
  const query = new URLSearchParams(location.search);
  return (
    <div className={application}>
      <Layout active={page?.kind ?? ""} recording={page?.data.Page.Recording}>
        {error ? (
          <>
            <Notice kind="error">{error}</Notice>
            <Button onClick={() => setRevision((value) => value + 1)}>
              <IconRefresh size={16} aria-hidden="true" />
              Retry
            </Button>
          </>
        ) : !page ? (
          <p role="status">Loading…</p>
        ) : (
          <>
            {(location.state?.message || query.get("message")) && (
              <Notice kind="success">
                {location.state?.message || query.get("message")}
              </Notice>
            )}
            {query.get("error") && (
              <Notice kind="error">{query.get("error")}</Notice>
            )}
            {page.kind === "home" && <Home data={page.data} />}
            {page.kind === "live" && <Live data={page.data} />}
            {page.kind === "reservations" && (
              <Library
                data={page.data}
                refresh={() => setRevision((value) => value + 1)}
              />
            )}
          </>
        )}
      </Layout>
    </div>
  );
}
