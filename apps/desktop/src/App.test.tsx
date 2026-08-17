import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, test } from "vitest";

import App from "./App";
import { TooltipProvider } from "./components/ui/tooltip";

afterEach(cleanup);

describe("Browser Identity table", () => {
  test("keeps Host-owned identity codes stable when a newer record sorts first", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <App />
      </TooltipProvider>,
    );

    const existingName = await screen.findByText("东南亚店铺 · 主账号");
    expect(within(existingName.closest("tr")!).getByText("RB-0001")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "新建环境" }));
    const dialog = await screen.findByRole("dialog", { name: "新建环境" });
    await user.type(within(dialog).getByLabelText(/环境名称/), "新环境");
    await user.click(within(dialog).getByRole("button", { name: "创建环境" }));

    const newName = await screen.findByText("新环境");
    expect(within(newName.closest("tr")!).getByText("RB-0004")).toBeInTheDocument();
    expect(within(existingName.closest("tr")!).getByText("RB-0001")).toBeInTheDocument();
  });

  test("configures a stopped identity proxy through the network drawer", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <App />
      </TooltipProvider>,
    );

    const identityName = await screen.findByText("欧洲站 · 客服");
    const row = identityName.closest("tr")!;
    await user.click(within(row).getByRole("button", { name: "直连" }));

    const drawer = await screen.findByRole("dialog", { name: "网络出口" });
    await user.click(within(drawer).getByRole("button", { name: "代理" }));
    await user.selectOptions(within(drawer).getByLabelText("协议"), "socks5");
    await user.type(within(drawer).getByLabelText("主机"), "127.0.0.1");
    const port = within(drawer).getByLabelText("端口");
    await user.clear(port);
    await user.type(port, "1080");
    await user.click(within(drawer).getByRole("button", { name: "保存" }));

    expect(await within(row).findByRole("button", { name: "代理" })).toBeInTheDocument();
  });

  test("configures and observes a managed timezone through the persona drawer", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <App />
      </TooltipProvider>,
    );

    const identityName = await screen.findByText("欧洲站 · 客服");
    const row = identityName.closest("tr")!;
    await user.click(within(row).getByRole("button", { name: "原生" }));

    const drawer = await screen.findByRole("dialog", { name: "指纹设置" });
    await user.click(within(drawer).getByRole("button", { name: "地区" }));
    await user.click(within(drawer).getByRole("combobox", { name: "时区" }));
    await user.type(screen.getByRole("combobox", { name: "搜索时区" }), "Tokyo");
    await user.click(screen.getByRole("option", { name: "Asia / Tokyo" }));
    await user.click(within(drawer).getByRole("button", { name: "保存" }));

    expect(await within(row).findByRole("button", { name: "已配置" })).toBeInTheDocument();
    await user.click(within(row).getByRole("button", { name: "打开" }));
    expect(await within(row).findByRole("button", { name: "已观测" })).toBeInTheDocument();

    await user.click(within(row).getByRole("button", { name: "已观测" }));
    const observedDrawer = await screen.findByRole("dialog", { name: "指纹设置" });
    expect(within(observedDrawer).getByText("2 项一致")).toBeInTheDocument();
  });

  test("configures and observes display metrics as one device atom", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <App />
      </TooltipProvider>,
    );

    const identityName = await screen.findByText("北美站 · 售后");
    const row = identityName.closest("tr")!;
    await user.click(within(row).getByRole("button", { name: "原生" }));

    const drawer = await screen.findByRole("dialog", { name: "指纹设置" });
    await user.click(within(drawer).getByRole("button", { name: "设备" }));
    await user.selectOptions(within(drawer).getByLabelText("Viewport"), "1366x768");
    await user.selectOptions(within(drawer).getByLabelText("屏幕"), "1920x1080");
    await user.selectOptions(within(drawer).getByLabelText("像素比"), "125");
    await user.click(within(drawer).getByRole("button", { name: "保存" }));

    expect(await within(row).findByRole("button", { name: "已配置" })).toBeInTheDocument();
    await user.click(within(row).getByRole("button", { name: "打开" }));
    expect(await within(row).findByRole("button", { name: "已观测" })).toBeInTheDocument();

    await user.click(within(row).getByRole("button", { name: "已观测" }));
    const observedDrawer = await screen.findByRole("dialog", { name: "指纹设置" });
    expect(within(observedDrawer).getByText("4 项一致")).toBeInTheDocument();
  });

  test("labels Canvas as CustomKernel only after a running observation", async () => {
    const user = userEvent.setup();
    render(
      <TooltipProvider>
        <App />
      </TooltipProvider>,
    );

    const open = (await screen.findAllByRole("button", { name: "打开" }))[0]!;
    const row = open.closest("tr")!;
    await user.click(open);
    await user.click(await within(row).findByRole("button", { name: "已观测" }));
    const drawer = await screen.findByRole("dialog", { name: "指纹设置" });
    expect(within(drawer).getByText("1 项一致")).toBeInTheDocument();
    await user.click(within(drawer).getByRole("button", { name: "图形" }));
    expect(within(drawer).getByText("种子微扰")).toBeInTheDocument();
    expect(within(drawer).getByLabelText("定制内核")).toBeInTheDocument();
  });
});

describe("Navigation sidebar", () => {
  test("toggles between collapsed and expanded states and persists preference", async () => {
    localStorage.clear();
    const user = userEvent.setup();
    const { unmount } = render(
      <TooltipProvider>
        <App />
      </TooltipProvider>,
    );

    const nav = screen.getByLabelText("主导航");
    expect(nav).not.toHaveClass("nav-rail-expanded");
    expect(within(nav).queryByText("RealBrowser")).not.toBeInTheDocument();

    const expandBtn = screen.getByRole("button", { name: "展开导航" });
    await user.click(expandBtn);

    expect(nav).toHaveClass("nav-rail-expanded");
    expect(within(nav).getByText("RealBrowser")).toBeInTheDocument();
    expect(screen.getByText("环境管理")).toBeInTheDocument();
    expect(localStorage.getItem("realbrowser:sidebar_expanded")).toBe("true");

    const collapseBtn = screen.getByRole("button", { name: "收起导航" });
    await user.click(collapseBtn);

    expect(nav).not.toHaveClass("nav-rail-expanded");
    expect(within(nav).queryByText("RealBrowser")).not.toBeInTheDocument();
    expect(localStorage.getItem("realbrowser:sidebar_expanded")).toBe("false");

    unmount();
  });

  test("loads in expanded state if persisted in localStorage", async () => {
    localStorage.setItem("realbrowser:sidebar_expanded", "true");
    render(
      <TooltipProvider>
        <App />
      </TooltipProvider>,
    );

    const nav = screen.getByLabelText("主导航");
    expect(nav).toHaveClass("nav-rail-expanded");
    expect(within(nav).getByText("RealBrowser")).toBeInTheDocument();
    expect(screen.getByText("环境管理")).toBeInTheDocument();
  });
});
