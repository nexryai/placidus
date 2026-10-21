import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";
import { Button, RippleLink } from "./Ripple";

afterEach(cleanup);
describe("ripple controls", () => {
  it("shows a centered keyboard ripple and removes it after animation", () => {
    const onClick = vi.fn();
    const { container } = render(<Button onClick={onClick}>Reserve</Button>);
    fireEvent.click(screen.getByRole("button"), { detail: 0 });
    expect(container.querySelectorAll("[data-ripple]").length).toBe(1);
    expect(onClick).toHaveBeenCalledOnce();
    fireEvent.animationEnd(container.querySelector("[data-ripple]")!);
    expect(container.querySelector("[data-ripple]")).toBeNull();
  });
  it("does not activate a disabled button", () => {
    const onClick = vi.fn();
    const { container } = render(
      <Button disabled onClick={onClick}>
        Reserve
      </Button>,
    );
    fireEvent.click(screen.getByRole("button"));
    expect(onClick).not.toHaveBeenCalled();
    expect(container.querySelector("[data-ripple]")).toBeNull();
  });
  it("keeps links accessible and navigable", () => {
    render(
      <MemoryRouter>
        <RippleLink to="/reservations">Library</RippleLink>
      </MemoryRouter>,
    );
    expect(screen.getByRole("link").getAttribute("href")).toBe("/reservations");
  });
});
