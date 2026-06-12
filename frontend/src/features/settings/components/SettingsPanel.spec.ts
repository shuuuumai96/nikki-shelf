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
    const wrapper = mountPanel();

    await wrapper.get('[data-testid="open-delete-account"]').trigger("click");
    await wrapper
      .get('[data-testid="delete-account-username"]')
      .setValue("nikki");
    await wrapper
      .get('[data-testid="delete-account-password"]')
      .setValue("password123");
    await wrapper.get('[data-testid="delete-account-panel"]').trigger("submit");
    await wrapper.setProps({ error: "owner still required" });

    expect(wrapper.text()).toContain("owner still required");
  });

  it("emits confirmed password change details", async () => {
    const wrapper = mountPanel();

    await wrapper.get('[data-testid="open-change-password"]').trigger("click");
    await wrapper
      .get('[data-testid="change-password-current"]')
      .setValue("password123");
    await wrapper
      .get('[data-testid="change-password-new"]')
      .setValue("new-password-123");
    await wrapper
      .get('[data-testid="change-password-confirm"]')
      .setValue("new-password-123");
    await wrapper
      .get('[data-testid="change-password-panel"]')
      .trigger("submit");

    expect(wrapper.emitted("changePassword")).toEqual([
      [
        {
          currentPassword: "password123",
          newPassword: "new-password-123",
        },
      ],
    ]);
  });

  it("keeps password change local when confirmation does not match", async () => {
    const wrapper = mountPanel();

    await wrapper.get('[data-testid="open-change-password"]').trigger("click");
    await wrapper
      .get('[data-testid="change-password-current"]')
      .setValue("password123");
    await wrapper
      .get('[data-testid="change-password-new"]')
      .setValue("new-password-123");
    await wrapper
      .get('[data-testid="change-password-confirm"]')
      .setValue("different-password");
    await wrapper
      .get('[data-testid="change-password-panel"]')
      .trigger("submit");

    expect(wrapper.emitted("changePassword")).toBeUndefined();
    expect(wrapper.text()).toContain("settings.changePasswordMismatch");
  });

  it("keeps password change local when the new password is unchanged", async () => {
    const wrapper = mountPanel();

    await wrapper.get('[data-testid="open-change-password"]').trigger("click");
    await wrapper
      .get('[data-testid="change-password-current"]')
      .setValue("password123");
    await wrapper
      .get('[data-testid="change-password-new"]')
      .setValue("password123");
    await wrapper
      .get('[data-testid="change-password-confirm"]')
      .setValue("password123");
    await wrapper
      .get('[data-testid="change-password-panel"]')
      .trigger("submit");

    expect(wrapper.emitted("changePassword")).toBeUndefined();
    expect(wrapper.text()).toContain("settings.changePasswordSame");
  });

  it("shows password change API errors only in the change panel", async () => {
    const wrapper = mountPanel();

    await wrapper.get('[data-testid="open-change-password"]').trigger("click");
    await wrapper
      .get('[data-testid="change-password-current"]')
      .setValue("password123");
    await wrapper
      .get('[data-testid="change-password-new"]')
      .setValue("new-password-123");
    await wrapper
      .get('[data-testid="change-password-confirm"]')
      .setValue("new-password-123");
    await wrapper
      .get('[data-testid="change-password-panel"]')
      .trigger("submit");
    await wrapper.setProps({ error: "current password is wrong" });

    expect(wrapper.text()).toContain("current password is wrong");

    await wrapper.get('[data-testid="open-delete-account"]').trigger("click");

    expect(wrapper.text()).not.toContain("current password is wrong");
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
