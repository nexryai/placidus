import { expect, test } from "@playwright/test";
import { home, library, live } from "../src/test/fixtures";

test("preserves guide, reservations, SPA navigation, and ripple feedback", async ({
  page,
}) => {
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  let cancelled = false;
  await page.route("**/api/**", async (route) => {
    const path = new URL(route.request().url()).pathname;
    let data: unknown = home;
    if (route.request().method() === "POST") {
      cancelled = path.endsWith("/cancel");
      data = {
        message: cancelled ? "Reservation cancelled." : "Recording reserved.",
      };
    } else if (path === "/api/reservations") {
      data = cancelled
        ? {
            ...library,
            Reservations: library.Reservations!.map((item) => ({
              ...item,
              state: "cancelled",
            })),
          }
        : library;
    } else if (path === "/api/live/100") {
      data = {
        ...live,
        Page: {
          ...live.Page,
          Error:
            "Tuners are occupied by recording. Try a service already being recorded.",
        },
      };
    }
    await route.fulfill({ json: data });
  });
  await page.goto("/");
  await expect(page.getByRole("heading", { name: "On air now" })).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Program guide" }),
  ).toBeVisible();
  if (test.info().project.name === "mobile") {
    expect(
      await page
        .getByRole("main")
        .evaluate((element) => getComputedStyle(element).paddingLeft),
    ).toBe("14px");
  }
  await page.screenshot({
    path: `test-results/home-${test.info().project.name}.png`,
  });
  await page.evaluate(() => {
    (window as Window & { spaMarker?: boolean }).spaMarker = true;
  });
  const libraryLink = page
    .getByRole("navigation")
    .getByRole("link", { name: "Library" });
  await libraryLink.focus();
  await page.keyboard.press("Enter");
  await expect(page.locator("[data-ripple]")).toHaveCount(1);
  await expect(page.locator("[data-ripple]")).toHaveCount(0);
  await expect(
    page.getByRole("heading", { name: "Reservations & recordings" }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => (window as Window & { spaMarker?: boolean }).spaMarker,
    ),
  ).toBe(true);
  await page.getByRole("link", { name: "Browse guide" }).click();
  await page.getByRole("button", { name: "Reserve" }).click();
  await expect(page.getByRole("status")).toHaveText("Recording reserved.");
  await page.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByText("cancelled", { exact: true })).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Reservations & recordings" }),
  ).toBeVisible();
  await page.goto("/live/100");
  await expect(page.getByRole("alert")).toContainText("Tuners are occupied");
  await expect(
    page.getByRole("heading", { name: "Morning News" }),
  ).toBeVisible();
  await expect(page.getByLabel("Live television player")).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: `test-results/live-${test.info().project.name}.png`,
  });
  expect(errors).toEqual([]);
});

test("reduced motion keeps ripple feedback brief", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.route("**/api/**", (route) => route.fulfill({ json: home }));
  await page.goto("/");
  await page.getByRole("heading", { name: "On air now" }).waitFor();
  await page.getByRole("link", { name: "Program guide", exact: true }).click();
  await expect(page.locator("[data-ripple]")).toHaveCount(0);
});
