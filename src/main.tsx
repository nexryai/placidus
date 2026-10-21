import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { globalCss } from "mintcss";
import { App } from "./App";

// Only the document margin needs a global reset; all visual styles belong to components.
globalCss({ body: { margin: 0 } });
createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <App />
    </BrowserRouter>
  </StrictMode>,
);
