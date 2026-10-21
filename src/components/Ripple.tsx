import {
  useRef,
  useState,
  type ButtonHTMLAttributes,
  type MouseEvent,
  type PointerEvent,
  type ReactNode,
} from "react";
import { Link, type LinkProps } from "react-router-dom";
import { css, keyframes } from "mintcss";

const motion = keyframes({
  from: { transform: "scale(0)", opacity: ".22" },
  to: { transform: "scale(1)", opacity: "0" },
});
const surface = css({
  position: "relative",
  overflow: "hidden",
  isolation: "isolate",
  "&:focus-visible": { outline: "3px solid var(--accent)", outlineOffset: 3 },
});
const wave = css({
  position: "absolute",
  pointerEvents: "none",
  borderRadius: "50%",
  background: "currentColor",
  animation: `${motion} 650ms ease-out forwards`,
  "@media (prefers-reduced-motion: reduce)": { animationDuration: "1ms" },
});
const label = css({
  position: "relative",
  zIndex: 1,
  display: "inline-flex",
  alignItems: "center",
  gap: 8,
});
const button = css({
  display: "inline-flex",
  alignItems: "center",
  justifyContent: "center",
  gap: 8,
  minHeight: 38,
  padding: "0 16px",
  border: 0,
  borderRadius: 20,
  background: "var(--panel2)",
  color: "var(--text)",
  font: "inherit",
  fontWeight: 750,
  cursor: "pointer",
  "&:hover": { filter: "brightness(.88)" },
  "&:disabled": { opacity: ".5", cursor: "wait" },
  "&.small": { minHeight: 32, padding: "0 12px", fontSize: 12 },
  "&.danger": { background: "#432020", color: "#ffbaba" },
});

function useRipple() {
  const sequence = useRef(0);
  const [ripples, setRipples] = useState<
    { id: number; x: number; y: number; size: number }[]
  >([]);
  function add(target: HTMLElement, clientX?: number, clientY?: number) {
    const rect = target.getBoundingClientRect();
    const size = Math.hypot(rect.width, rect.height) * 2;
    setRipples((values) => [
      ...values,
      {
        id: sequence.current++,
        size,
        x:
          (clientX === undefined ? rect.width / 2 : clientX - rect.left) -
          size / 2,
        y:
          (clientY === undefined ? rect.height / 2 : clientY - rect.top) -
          size / 2,
      },
    ]);
  }
  return {
    onPointerDown(event: PointerEvent<HTMLElement>) {
      if (event.button === 0)
        add(event.currentTarget, event.clientX, event.clientY);
    },
    onClick(event: MouseEvent<HTMLElement>) {
      // Keyboard and assistive activation have no pointer coordinates.
      if (event.detail === 0) add(event.currentTarget);
    },
    waves: ripples.map((value) => (
      <span
        key={value.id}
        aria-hidden="true"
        data-ripple
        className={wave}
        style={{
          left: value.x,
          top: value.y,
          width: value.size,
          height: value.size,
        }}
        onAnimationEnd={() =>
          setRipples((values) => values.filter((item) => item.id !== value.id))
        }
      />
    )),
  };
}

export function Button({
  children,
  className = "",
  onClick,
  onPointerDown,
  type = "button",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement>) {
  const ripple = useRipple();
  return (
    <button
      {...props}
      type={type}
      className={`${surface} ${button} ${className}`}
      onPointerDown={(event) => {
        ripple.onPointerDown(event);
        onPointerDown?.(event);
      }}
      onClick={(event) => {
        ripple.onClick(event);
        onClick?.(event);
      }}
    >
      <span className={label}>{children}</span>
      {ripple.waves}
    </button>
  );
}

export function RippleLink({
  children,
  className = "",
  onClick,
  onPointerDown,
  ...props
}: LinkProps & { children: ReactNode }) {
  const ripple = useRipple();
  return (
    <Link
      {...props}
      className={`${surface} ${className}`}
      onPointerDown={(event) => {
        ripple.onPointerDown(event);
        onPointerDown?.(event);
      }}
      onClick={(event) => {
        ripple.onClick(event);
        onClick?.(event);
      }}
    >
      {children}
      {ripple.waves}
    </Link>
  );
}
export const buttonLinkClass = button;
