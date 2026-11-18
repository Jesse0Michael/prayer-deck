import assert from "node:assert/strict";
import test from "node:test";
import { createAnalytics } from "../src/lib/googleTag.ts";

const host = "prayer.example";
const id = "G-TEST000001";

function fixture(settings = {}, origin = `https://${host}`) {
  const scripts = [];
  const browser = {
    location: new URL("/private?token=private-token#private-fragment", origin),
    document: {
      referrer: "https://example.org/private-referrer",
      title: "private-title",
      createElement: () => ({}),
      head: { appendChild: (script) => scripts.push(script) },
    },
  };
  const analytics = createAnalytics(
    { measurementId: id, hostname: host, production: true, debugRequested: false, ...settings },
    browser,
  );
  return { analytics, browser, scripts };
}

test("collection requires the configured production host and ID, or explicit offline development debugging", () => {
  for (const [settings, origin] of [
    [{ measurementId: undefined }], [{ measurementId: "UA-invalid" }], [{ hostname: undefined }],
    [{ production: false }], [{}, "http://prayer.example"], [{}, "https://prayer.example.pages.dev"],
    [{ debugRequested: true }, "https://localhost"],
  ]) {
    const { analytics, browser, scripts } = fixture(settings, origin);
    analytics.trackPage("landing");
    analytics.trackLogin();
    assert.equal(browser.dataLayer, undefined);
    assert.equal(scripts.length, 0);
  }
  const { analytics, browser, scripts } = fixture({ production: false, debugRequested: true }, "http://localhost");
  analytics.trackPage("landing");
  assert.equal(browser.dataLayer.at(-1)[1], "page_view");
  assert.equal(scripts.length, 0);
});

test("the tag receives canonical page metadata and records each navigation once", () => {
  const { analytics, scripts, browser } = fixture();
  analytics.trackPage("landing");
  analytics.trackPage("landing");
  analytics.trackPage("sign-in");
  analytics.trackLogin();
  analytics.trackPage("draw");
  analytics.trackHandDealt("personal", true, 2);
  analytics.trackPage("cards");
  analytics.trackCard("card_created", "personal");

  assert.deepEqual(scripts, [{
    async: true,
    referrerPolicy: "no-referrer",
    src: `https://www.googletagmanager.com/gtag/js?id=${id}`,
  }]);
  const commands = browser.dataLayer.map((command) => Array.from(command));
  const configs = commands.filter(([command]) => command === "config").map(([, , params]) => params);
  assert.deepEqual(configs.map((p) => p.page_location), ["/", "/sign-in", "/draw", "/cards"].map((p) => `https://${host}${p}`));
  assert.ok(configs.every((p) => p.page_referrer === "" && p.send_page_view === false));
  assert.ok(configs.every((p) => p.allow_google_signals === false && p.allow_ad_personalization_signals === false));
  assert.ok(!JSON.stringify(commands).includes("private"));
  const events = commands.filter(([command]) => command === "event").map(([, name, { send_to, ...params }]) => {
    assert.equal(send_to, id);
    return [name, params];
  });
  assert.deepEqual(events, [
    ["page_view", {}], ["page_view", {}], ["login", { method: "google" }], ["page_view", {}],
    ["hand_dealt", { deck_mode: "personal", draw_type: "shuffle", hand_size: 2 }],
    ["page_view", {}], ["card_created", { deck_mode: "personal" }],
  ]);
});

test("a blocked data layer cannot interrupt app actions", () => {
  const { analytics, browser } = fixture();
  browser.dataLayer = Object.freeze([]);
  analytics.trackPage("draw");
  analytics.trackHandDealt("demo", false, 1);
});
