import {
  IconAlertTriangle,
  IconCircleCheck,
  IconInfoCircle,
} from "@tabler/icons-react";
import type { ReactNode } from "react";
import { css } from "mintcss";

const notice = css({
  display: "flex",
  alignItems: "flex-start",
  gap: 10,
  "& > svg": { flexShrink: 0, marginTop: 1 },
  margin: "0 0 20px",
  padding: "14px 16px",
  border: "1px solid #725f19",
  borderRadius: 12,
  background: "#2b2718",
  color: "#ffe49a",
  "&.error": {
    borderColor: "#743434",
    background: "#301b1b",
    color: "#ffb4b4",
  },
  "&.success": {
    borderColor: "#286342",
    background: "#172b21",
    color: "#aef0c8",
  },
});
export function Notice({
  children,
  kind = "",
}: {
  children: ReactNode;
  kind?: "" | "error" | "success";
}) {
  const Icon =
    kind === "error"
      ? IconAlertTriangle
      : kind === "success"
        ? IconCircleCheck
        : IconInfoCircle;
  return (
    <div
      className={`${notice} ${kind}`}
      role={kind === "error" ? "alert" : "status"}
    >
      <Icon size={20} aria-hidden="true" />
      <div>{children}</div>
    </div>
  );
}
