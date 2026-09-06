// Post-ready interaction for the screenshot pipeline. Runs after the app
// mounts (see capture.mjs `--interact`), before the stability/validation loop.
//
// The job config picks the view: `interactView` = "search", "library",
// "browse", or "settings". The sidebar nav is clicked first so the capture
// is taken on the right page, then view-specific setup runs (typing a query,
// expanding the first collection row).

const NAV_LABEL = {
  search: "Search",
  library: "Library",
  browse: "Browse",
  index: "Index",
  settings: "Settings",
};

export async function interact(page, job) {
  const view = job.interactView ?? "search";

  // Route to the target view via the sidebar nav.
  const label = NAV_LABEL[view];
  if (label) {
    const nav = page.locator(`nav[aria-label="Primary"] button[aria-label="${label}"]`);
    if (await nav.count()) {
      await nav.first().click();
      await page
        .locator(`nav[aria-label="Primary"] button[aria-label="${label}"][aria-current="page"]`)
        .waitFor({ state: "visible", timeout: 3000 })
        .catch(() => {});
    }
  }

  if (view === "search") {
    const query = job.query ?? "kubernetes rollout";
    await page.evaluate((q) => {
      const input = document.querySelector('input[aria-label="Search"]');
      if (!input) return;
      // Set the value through the native setter so Solid's onInput sees it.
      const setter = Object.getOwnPropertyDescriptor(
        HTMLInputElement.prototype,
        "value",
      ).set;
      setter.call(input, q);
      input.dispatchEvent(new Event("input", { bubbles: true }));
    }, query);

    await page
      .locator("main article")
      .first()
      .waitFor({ state: "visible", timeout: 8000 })
      .catch(() => {});
    await page.waitForTimeout(1700);
    return;
  }

  if (view === "library") {
    // Wait for collections, then expand the first row so its sources are
    // visible. The row is a flex div now (toggle + delete button siblings),
    // so target the toggle by its aria-expanded attribute.
    const firstRow = page.locator("ul.divide-y > li:first-child button[aria-expanded]");
    await firstRow.waitFor({ state: "visible", timeout: 8000 }).catch(() => {});
    if (await firstRow.count()) {
      await firstRow.first().click();
      await page
        .locator("ul.divide-y > li:first-child ul li")
        .first()
        .waitFor({ state: "visible", timeout: 8000 })
        .catch(() => {});
    }
    return;
  }

  if (view === "browse") {
    // The tree + preview load via loadLibrary(); the default expands the first
    // collection and its first source, which is what the screenshot shows.
    await page
      .locator('[role="tree"] [role="treeitem"]')
      .first()
      .waitFor({ state: "visible", timeout: 8000 })
      .catch(() => {});
    return;
  }

  // Settings is a rail of sections; only the active one renders. A job that
  // wants a specific card (e.g. `settingsSection: "vexter"`) clicks its rail
  // button. Scoped to the rail nav so hidden mobile chips are never matched.
  if (view === "settings" && job.settingsSection) {
    const btn = page.locator(
      `nav[aria-label="Settings sections"] button:has-text("${job.settingsSection}")`,
    );
    await btn.first().waitFor({ state: "visible", timeout: 8000 }).catch(() => {});
    if (await btn.count()) {
      await btn.first().click();
      await page
        .locator("main h2")
        .filter({ hasText: job.settingsSection })
        .first()
        .waitFor({ state: "visible", timeout: 8000 })
        .catch(() => {});
    }
    return;
  }
}
