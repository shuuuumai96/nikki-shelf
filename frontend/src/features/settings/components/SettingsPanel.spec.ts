import { mount } from "@vue/test-utils";
import { beforeEach, describe, expect, it, vi } from "vitest";
import SettingsPanel from "./SettingsPanel.vue";

vi.mock("vue-i18n", () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}));

describe("SettingsPanel", () => {
  beforeEach(() => {
    window.localStorage.clear();
  });

  it("emits confirmed account deletion details", async () => {
    const wrapper = mountPanel();

    await wrapper.get('[data-testid="open-delete-account"]').trigger("click");
    await wrapper
      .get('[data-testid="delete-account-username"]')
      .setValue("nikki");
    await wrapper
      .get('[data-testid="delete-account-password"]')
      .setValue("password123");
    await wrapper.get('[data-testid="delete-account-panel"]').trigger("submit");

    expect(wrapper.emitted("deleteAccount")).toEqual([
      [{ username: "nikki", password: "password123" }],
    ]);
  });

  it("keeps deletion local when the confirmation username does not match", async () => {
    const wrapper = mountPanel();

    await wrapper.get('[data-testid="open-delete-account"]').trigger("click");
    await wrapper
      .get('[data-testid="delete-account-username"]')
      .setValue("someone-else");
    await wrapper
      .get('[data-testid="delete-account-password"]')
      .setValue("password123");
    await wrapper.get('[data-testid="delete-account-panel"]').trigger("submit");

    expect(wrapper.emitted("deleteAccount")).toBeUndefined();
    expect(wrapper.text()).toContain("settings.deleteAccountUsernameMismatch");
  });

  it("shows account deletion API errors in the confirmation panel", async () => {
    const wrapper = mountPanel({ error: "owner still required" });

    await wrapper.get('[data-testid="open-delete-account"]').trigger("click");

    expect(wrapper.text()).toContain("owner still required");
  });
});

function mountPanel(
  props: Partial<InstanceType<typeof SettingsPanel>["$props"]> = {},
) {
  return mount(SettingsPanel, {
    props: {
      user: {
        id: 1,
        role: "owner",
        username: "nikki",
      },
      ...props,
    },
    global: {
      stubs: {
        LanguageSelect: true,
      },
    },
  });
}
