import { describe, expect, it } from "vitest";

import { resolveLanguage, resolveTheme } from "./preferences";

describe("interface preference resolution", () => {
  it("resolves explicit and system languages", () => {
    expect(resolveLanguage("en", "zh-CN")).toBe("en");
    expect(resolveLanguage("system", "zh-CN")).toBe("zh-CN");
    expect(resolveLanguage("system", "en-US")).toBe("en");
  });

  it("resolves explicit and system themes", () => {
    expect(resolveTheme("light", true)).toBe("light");
    expect(resolveTheme("dark", false)).toBe("dark");
    expect(resolveTheme("system", true)).toBe("dark");
  });
});
