use std::{
    sync::Arc,
    time::{SystemTime, UNIX_EPOCH},
};

use browser_control::ControlService;
use browser_core::{CreateIdentity, SystemSnapshot, TerminationKind};
use browser_network::{NetworkMode, ProxyProtocol};
use browser_persona::{LocalePolicy, PersonaConfig, RegionPreset, TimezonePolicy, WebRtcPolicy};
use gpui_kit::component::button::{Button, ButtonVariants};
use gpui_kit::component::input::{Input, InputEvent, InputState};
use gpui_kit::component::menu::{DropdownMenu, PopupMenu, PopupMenuItem};
use gpui_kit::component::notification::Notification;
use gpui_kit::component::tab::{Tab, TabBar};
use gpui_kit::component::*;
use gpui_kit::prelude::FluentBuilder as _;
use gpui_kit::*;
use realbrowser_desktop::{
    CommandFailure, IdentityView, SystemView, apply_locale, apply_preset, apply_scale,
    apply_screen, apply_timezone, apply_viewport, apply_webrtc, apply_window, egress_label,
    field_label, locale_label, network_direct, network_proxy, open_control, parse_identity_id,
    persona_is_native, persona_status_label, profile_label, readonly_field_value, region_label,
    relative_time, reset_native, runtime_label, timezone_label, webrtc_label,
};

pub fn run() {
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::try_from_default_env()
                .unwrap_or_else(|_| tracing_subscriber::EnvFilter::new("info")),
        )
        .with_target(false)
        .compact()
        .init();

    let control = match open_control() {
        Ok(control) => Arc::new(control),
        Err(message) => {
            eprintln!("error: {message}");
            std::process::exit(1);
        }
    };

    gpui_kit::application()
        .with_assets(gpui_kit::assets::Assets)
        .run(move |cx| {
            gpui_kit::init(cx);
            let control = Arc::clone(&control);
            let options = WindowOptions {
                window_bounds: Some(WindowBounds::centered(size(px(1280.), px(820.)), cx)),
                ..TitleBar::window_options()
            };
            gpui_kit::open_window(options, cx, move |window, cx| {
                let control = Arc::clone(&control);
                cx.new(|cx| Workspace::new(control, window, cx))
            })
            .expect("failed to open window");
        });
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum Scope {
    Active,
    Archived,
}

struct Workspace {
    control: Arc<ControlService>,
    scope: Scope,
    records: Vec<IdentityView>,
    system: SystemView,
    running_only: bool,
    collapsed: bool,
    banner: Option<String>,
    search: Entity<InputState>,
    _search_subscription: Subscription,
}

impl Workspace {
    fn new(control: Arc<ControlService>, window: &mut Window, cx: &mut Context<Self>) -> Self {
        let search = cx.new(|cx| InputState::new(window, cx).placeholder("搜索名称、编号或网址"));
        let subscription = cx.subscribe(&search, |_, _, event: &InputEvent, cx| {
            if matches!(event, InputEvent::Change) {
                cx.notify();
            }
        });
        let mut workspace = Self {
            control,
            scope: Scope::Active,
            records: Vec::new(),
            system: SystemView::from(SystemSnapshot {
                platform: String::new(),
                browser: None,
                canvas_kernel_observed: false,
            }),
            running_only: false,
            collapsed: false,
            banner: None,
            search,
            _search_subscription: subscription,
        };
        workspace.reload();
        workspace
    }

    fn reload(&mut self) {
        self.system = SystemView::from(self.control.system_snapshot());
        let listed = match self.scope {
            Scope::Active => self.control.list(),
            Scope::Archived => self.control.list_archived(),
        };
        match listed {
            Ok(records) => {
                self.records = records.into_iter().map(IdentityView::from).collect();
                self.banner = None;
            }
            Err(error) => {
                self.banner = Some(CommandFailure::from(error).message);
            }
        }
    }

    fn query(&self, cx: &App) -> String {
        self.search.read(cx).value().to_lowercase()
    }

    fn visible<'a>(&'a self, cx: &App) -> Vec<&'a IdentityView> {
        let query = self.query(cx);
        self.records
            .iter()
            .filter(|record| {
                if self.running_only
                    && self.scope == Scope::Active
                    && record.runtime_state != "running"
                {
                    return false;
                }
                if query.is_empty() {
                    return true;
                }
                let haystack = format!(
                    "{} {} {} {}",
                    record.name,
                    record.display_code,
                    record.id,
                    record.startup_url.as_deref().unwrap_or("")
                )
                .to_lowercase();
                haystack.contains(&query)
            })
            .collect()
    }

    fn failure(window: &mut Window, cx: &mut App, error: browser_control::ControlError) {
        let failure = CommandFailure::from(error);
        let message = failure
            .field_issues
            .first()
            .map(|issue| issue.message.clone())
            .unwrap_or(failure.message);
        window.push_notification(Notification::error(message), cx);
    }

    fn open_create(&mut self, _: &ClickEvent, window: &mut Window, cx: &mut Context<Self>) {
        let parent = cx.entity().downgrade();
        let control = Arc::clone(&self.control);
        let system = self.system.clone();
        let dialog = cx.new(|cx| CreateDialog::new(parent, control, system, window, cx));
        window.open_dialog(cx, move |dialog_el, _, _| {
            dialog_el
                .title("新建环境")
                .width(px(480.))
                .child(dialog.clone())
        });
    }

    fn open_rename(&mut self, record: IdentityView, window: &mut Window, cx: &mut Context<Self>) {
        let parent = cx.entity().downgrade();
        let control = Arc::clone(&self.control);
        let dialog = cx.new(|cx| RenameDialog::new(parent, control, record, window, cx));
        window.open_dialog(cx, move |dialog_el, _, _| {
            dialog_el
                .title("重命名")
                .width(px(440.))
                .child(dialog.clone())
        });
    }

    fn open_network(&mut self, record: IdentityView, window: &mut Window, cx: &mut Context<Self>) {
        let parent = cx.entity().downgrade();
        let control = Arc::clone(&self.control);
        let sheet = cx.new(|cx| NetworkSheet::new(parent, control, record, window, cx));
        window.open_sheet(cx, move |sheet_el, _, _| {
            sheet_el
                .title("网络出口")
                .size(px(480.))
                .child(sheet.clone())
        });
    }

    fn open_persona(&mut self, record: IdentityView, window: &mut Window, cx: &mut Context<Self>) {
        let parent = cx.entity().downgrade();
        let control = Arc::clone(&self.control);
        let system = self.system.clone();
        let sheet = cx.new(|_| PersonaSheet::new(parent, control, system, record));
        window.open_sheet(cx, move |sheet_el, _, _| {
            sheet_el
                .title("指纹设置")
                .size(px(560.))
                .child(sheet.clone())
        });
    }

    fn run_action(
        &mut self,
        id: &str,
        action: &'static str,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) {
        let Ok(identity_id) = parse_identity_id(id) else {
            window.push_notification(Notification::error("浏览器环境编号无效"), cx);
            return;
        };
        let revision = self
            .records
            .iter()
            .find(|record| record.id == id)
            .map(|record| record.revision);
        let result = match action {
            "start" => self.control.start(identity_id).map(|_| "环境已打开"),
            "stop" => self.control.stop(identity_id).map(|result| {
                if result.termination == TerminationKind::Forced {
                    "RealBrowser 未及时退出，已强制停止"
                } else {
                    "环境已停止"
                }
            }),
            "archive" => {
                let Some(revision) = revision else {
                    window.push_notification(Notification::error("找不到这个浏览器环境"), cx);
                    return;
                };
                match self.control.archive(identity_id, revision) {
                    Ok(()) => Ok("环境已移入归档，Profile 数据仍保留"),
                    Err(error) => Err(error),
                }
            }
            "restore" => {
                let Some(revision) = revision else {
                    window.push_notification(Notification::error("找不到这个浏览器环境"), cx);
                    return;
                };
                self.control
                    .restore(identity_id, revision)
                    .map(|_| "环境已恢复")
            }
            _ => return,
        };
        match result {
            Ok(message) => {
                self.reload();
                cx.notify();
                if action == "stop" && message.contains("强制") {
                    window.push_notification(Notification::warning(message), cx);
                } else {
                    window.push_notification(Notification::success(message), cx);
                }
            }
            Err(error) => Self::failure(window, cx, error),
        }
    }
}

impl Render for Workspace {
    fn render(&mut self, window: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let query_active = !self.query(cx).is_empty() || self.running_only;
        let visible = self.visible(cx);
        let running = self
            .records
            .iter()
            .filter(|record| record.runtime_state == "running")
            .count();
        let count = self.records.len();
        let archived = self.scope == Scope::Archived;
        let browser = self
            .system
            .browser_version
            .clone()
            .map(|version| format!("RealBrowser {version}"))
            .unwrap_or_else(|| "RealBrowser 内核未检测".to_owned());
        let scope_label = if archived {
            "归档"
        } else {
            "使用中环境"
        };

        v_flex()
            .size_full()
            .bg(cx.theme().background)
            .text_color(cx.theme().foreground)
            .child(TitleBar::new().child("RealBrowser"))
            .child(
                h_flex()
                    .flex_1()
                    .min_h_0()
                    .items_stretch()
                    .child(self.sidebar(cx))
                    .child(
                        v_flex()
                            .flex_1()
                            .min_w_0()
                            .h_full()
                            .child(self.toolbar(scope_label, count, running, window, cx))
                            .child(self.body(visible, query_active, archived, window, cx))
                            .child(self.status_bar(count, browser, cx)),
                    ),
            )
    }
}

impl Workspace {
    fn sidebar(&self, cx: &mut Context<Self>) -> impl IntoElement {
        let collapsed = self.collapsed;
        v_flex()
            .h_full()
            .w(if collapsed { px(72.) } else { px(216.) })
            .border_r_1()
            .border_color(cx.theme().border)
            .bg(cx.theme().muted)
            .p_3()
            .gap_3()
            .child(
                h_flex()
                    .items_center()
                    .gap_2()
                    .child(
                        div()
                            .flex()
                            .items_center()
                            .justify_center()
                            .size_8()
                            .rounded(cx.theme().radius)
                            .bg(cx.theme().primary)
                            .text_color(cx.theme().primary_foreground)
                            .child(Icon::new(IconName::Globe)),
                    )
                    .when(!collapsed, |this| {
                        this.child(div().font_bold().child("RealBrowser"))
                    })
                    .when(!collapsed, |this| {
                        this.child(
                            Button::new("collapse-nav")
                                .ghost()
                                .icon(IconName::PanelLeftClose)
                                .on_click(cx.listener(|this, _, _, cx| {
                                    this.collapsed = true;
                                    cx.notify();
                                })),
                        )
                    }),
            )
            .child(nav_button(
                "nav-identities",
                "环境管理",
                IconName::LayoutDashboard,
                true,
                collapsed,
            ))
            .child(nav_button(
                "nav-activity",
                "运行记录",
                IconName::ChartPie,
                false,
                collapsed,
            ))
            .child(nav_button(
                "nav-health",
                "系统诊断",
                IconName::Info,
                false,
                collapsed,
            ))
            .child(nav_button(
                "nav-settings",
                "偏好设置",
                IconName::Settings,
                false,
                collapsed,
            ))
            .child(div().flex_1())
            .when(collapsed, |this| {
                this.child(
                    Button::new("expand-nav")
                        .ghost()
                        .icon(IconName::PanelLeftOpen)
                        .on_click(cx.listener(|this, _, _, cx| {
                            this.collapsed = false;
                            cx.notify();
                        })),
                )
            })
            .child(
                h_flex()
                    .items_center()
                    .gap_2()
                    .child(div().size_2().rounded_full().bg(cx.theme().success))
                    .when(!collapsed, |this| this.child("服务正常")),
            )
    }

    fn toolbar(
        &self,
        scope_label: &'static str,
        count: usize,
        running: usize,
        _window: &mut Window,
        cx: &mut Context<Self>,
    ) -> impl IntoElement {
        let workspace = cx.entity();
        let scope = self.scope;
        h_flex()
            .items_center()
            .gap_2()
            .px_4()
            .py_3()
            .child(
                Button::new("scope")
                    .outline()
                    .icon(if scope == Scope::Archived {
                        IconName::FolderClosed
                    } else {
                        IconName::FolderOpen
                    })
                    .label(format!("{scope_label} {count}"))
                    .dropdown_menu(move |menu, _, _| {
                        let active = workspace.clone();
                        let archived = workspace.clone();
                        menu.item(
                            PopupMenuItem::new("使用中环境")
                                .checked(scope == Scope::Active)
                                .on_click(move |_, _, cx| {
                                    active.update(cx, |this, cx| {
                                        this.scope = Scope::Active;
                                        this.running_only = false;
                                        this.reload();
                                        cx.notify();
                                    });
                                }),
                        )
                        .item(
                            PopupMenuItem::new("归档")
                                .checked(scope == Scope::Archived)
                                .on_click(move |_, _, cx| {
                                    archived.update(cx, |this, cx| {
                                        this.scope = Scope::Archived;
                                        this.running_only = false;
                                        this.reload();
                                        cx.notify();
                                    });
                                }),
                        )
                    }),
            )
            .child(div().flex_1().min_w_0().child(Input::new(&self.search)))
            .when(self.scope == Scope::Active, |this| {
                let pressed = self.running_only;
                this.child(
                    Button::new("running-filter")
                        .outline()
                        .selected(pressed)
                        .label(format!("运行中 {running}"))
                        .on_click(cx.listener(|this, _, _, cx| {
                            this.running_only = !this.running_only;
                            cx.notify();
                        })),
                )
            })
            .when(self.scope == Scope::Active, |this| {
                this.child(
                    Button::new("create-identity")
                        .primary()
                        .icon(IconName::Plus)
                        .label("新建环境")
                        .on_click(cx.listener(Workspace::open_create)),
                )
            })
            .child(
                Button::new("refresh")
                    .outline()
                    .icon(IconName::RefreshCw)
                    .label("刷新")
                    .on_click(cx.listener(|this, _, window, cx| {
                        this.reload();
                        cx.notify();
                        window.push_notification(Notification::success("已刷新"), cx);
                    })),
            )
    }

    fn body(
        &self,
        visible: Vec<&IdentityView>,
        filtered: bool,
        archived: bool,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) -> impl IntoElement {
        let banner = self.banner.clone();
        v_flex()
            .flex_1()
            .min_h_0()
            .px_4()
            .when_some(banner, |this, banner| {
                this.child(
                    div()
                        .mb_2()
                        .px_3()
                        .py_2()
                        .rounded(cx.theme().radius)
                        .bg(cx.theme().danger)
                        .text_color(cx.theme().danger_foreground)
                        .child(banner),
                )
            })
            .child(if visible.is_empty() {
                self.empty(filtered, archived, cx).into_any_element()
            } else {
                self.table(visible, archived, window, cx).into_any_element()
            })
    }

    fn empty(&self, filtered: bool, archived: bool, cx: &mut Context<Self>) -> impl IntoElement {
        let title = if filtered {
            "没有匹配的环境"
        } else if archived {
            "暂无归档环境"
        } else {
            "创建第一个浏览器环境"
        };
        let detail = if filtered {
            "换个关键词，或清除筛选。"
        } else if archived {
            "归档会保留 Profile。"
        } else {
            "每个环境使用独立的 RealBrowser Profile。"
        };
        v_flex()
            .flex_1()
            .items_center()
            .justify_center()
            .gap_2()
            .text_color(cx.theme().muted_foreground)
            .child(
                div()
                    .text_color(cx.theme().foreground)
                    .font_bold()
                    .child(title),
            )
            .child(detail)
            .child(if filtered {
                Button::new("clear-filter")
                    .outline()
                    .label("清除筛选")
                    .on_click(cx.listener(|this, _, window, cx| {
                        this.running_only = false;
                        this.search.update(cx, |input, cx| {
                            input.set_value("", window, cx);
                        });
                        cx.notify();
                    }))
            } else if archived {
                Button::new("back-active")
                    .outline()
                    .label("返回使用中")
                    .on_click(cx.listener(|this, _, _, cx| {
                        this.scope = Scope::Active;
                        this.reload();
                        cx.notify();
                    }))
            } else {
                Button::new("empty-create")
                    .primary()
                    .label("新建环境")
                    .on_click(cx.listener(Workspace::open_create))
            })
    }

    fn table(
        &self,
        visible: Vec<&IdentityView>,
        archived: bool,
        _window: &mut Window,
        cx: &mut Context<Self>,
    ) -> impl IntoElement {
        let now = now_ms();
        v_flex()
            .flex_1()
            .min_h_0()
            .border_1()
            .border_color(cx.theme().border)
            .rounded(cx.theme().radius)
            .overflow_hidden()
            .child(header_row(cx))
            .child(
                div().flex_1().min_h_0().overflow_y_scroll().children(
                    visible
                        .into_iter()
                        .map(|record| self.row(record, archived, now, cx)),
                ),
            )
    }

    fn row(
        &self,
        record: &IdentityView,
        archived: bool,
        now: i64,
        cx: &mut Context<Self>,
    ) -> impl IntoElement {
        let id = record.id.clone();
        let name = record.name.clone();
        let running = record.runtime_state == "running";
        let persona_record = record.clone();
        let network_record = record.clone();
        h_flex()
            .id(("identity", record.id.clone()))
            .items_center()
            .px_3()
            .py_2()
            .gap_2()
            .border_t_1()
            .border_color(cx.theme().border)
            .child(
                div()
                    .w_24()
                    .text_sm()
                    .text_color(cx.theme().muted_foreground)
                    .child(record.display_code.clone()),
            )
            .child(
                v_flex()
                    .flex_1()
                    .min_w_0()
                    .overflow_hidden()
                    .child(div().font_bold().child(name))
                    .child(
                        div()
                            .text_xs()
                            .text_color(cx.theme().muted_foreground)
                            .child(
                                record
                                    .startup_url
                                    .clone()
                                    .unwrap_or_else(|| "启动后打开空白页".to_owned()),
                            ),
                    ),
            )
            .child(div().w_16().text_sm().child(profile_label()))
            .child(if archived {
                div()
                    .w_20()
                    .text_sm()
                    .child(persona_status_label(record))
                    .into_any_element()
            } else {
                Button::new(("persona", record.id.clone()))
                    .ghost()
                    .small()
                    .label(persona_status_label(record))
                    .on_click(cx.listener(move |this, _, window, cx| {
                        this.open_persona(persona_record.clone(), window, cx);
                    }))
                    .into_any_element()
            })
            .child(if archived {
                div()
                    .w_16()
                    .text_sm()
                    .child(egress_label(record.egress_mode))
                    .into_any_element()
            } else {
                Button::new(("network", record.id.clone()))
                    .ghost()
                    .small()
                    .label(egress_label(record.egress_mode))
                    .on_click(cx.listener(move |this, _, window, cx| {
                        this.open_network(network_record.clone(), window, cx);
                    }))
                    .into_any_element()
            })
            .child(
                div().w_28().text_sm().child(
                    record
                        .browser_version
                        .clone()
                        .unwrap_or_else(|| runtime_label(record.runtime_state).to_owned()),
                ),
            )
            .child(
                div()
                    .w_24()
                    .text_xs()
                    .text_color(cx.theme().muted_foreground)
                    .child(relative_time(record.updated_at_ms, now)),
            )
            .child(if archived {
                let restore_id = id.clone();
                Button::new(("restore", id))
                    .outline()
                    .small()
                    .label("恢复")
                    .on_click(cx.listener(move |this, _, window, cx| {
                        this.run_action(&restore_id, "restore", window, cx);
                    }))
                    .into_any_element()
            } else {
                let action_id = id.clone();
                let menu_id = id.clone();
                let menu_persona = record.clone();
                let menu_network = record.clone();
                let menu_rename = record.clone();
                let archive_id = record.id.clone();
                let workspace = cx.entity();
                h_flex()
                    .gap_1()
                    .child(
                        Button::new(("toggle", id))
                            .small()
                            .when(running, |this| this.outline())
                            .when(!running, |this| this.primary())
                            .icon(if running {
                                IconName::Square
                            } else {
                                IconName::Play
                            })
                            .label(if running { "停止" } else { "打开" })
                            .on_click(cx.listener(move |this, _, window, cx| {
                                this.run_action(
                                    &action_id,
                                    if running { "stop" } else { "start" },
                                    window,
                                    cx,
                                );
                            })),
                    )
                    .child(
                        Button::new(("more", menu_id))
                            .ghost()
                            .small()
                            .icon(IconName::Ellipsis)
                            .dropdown_menu(move |menu, _, _| {
                                let persona = menu_persona.clone();
                                let network = menu_network.clone();
                                let rename_record = menu_rename.clone();
                                let archive_id = archive_id.clone();
                                let for_persona = workspace.clone();
                                let for_network = workspace.clone();
                                let for_rename = workspace.clone();
                                let for_archive = workspace.clone();
                                menu.item(PopupMenuItem::new("指纹设置").on_click(
                                    move |_, window, cx| {
                                        for_persona.update(cx, |this, cx| {
                                            this.open_persona(persona.clone(), window, cx);
                                        });
                                    },
                                ))
                                .item(PopupMenuItem::new("代理设置").on_click(
                                    move |_, window, cx| {
                                        for_network.update(cx, |this, cx| {
                                            this.open_network(network.clone(), window, cx);
                                        });
                                    },
                                ))
                                .item(PopupMenuItem::new("重命名").disabled(running).on_click(
                                    move |_, window, cx| {
                                        for_rename.update(cx, |this, cx| {
                                            this.open_rename(rename_record.clone(), window, cx);
                                        });
                                    },
                                ))
                                .separator()
                                .item(
                                    PopupMenuItem::new("移入归档").disabled(running).on_click(
                                        move |_, window, cx| {
                                            for_archive.update(cx, |this, cx| {
                                                this.run_action(&archive_id, "archive", window, cx);
                                            });
                                        },
                                    ),
                                )
                            }),
                    )
                    .into_any_element()
            })
    }

    fn status_bar(&self, count: usize, browser: String, cx: &App) -> impl IntoElement {
        h_flex()
            .h_10()
            .items_center()
            .gap_3()
            .px_4()
            .border_t_1()
            .border_color(cx.theme().border)
            .text_xs()
            .text_color(cx.theme().muted_foreground)
            .child("服务正常")
            .child(browser)
            .child(format!("{count} 个环境"))
            .child(format!(
                "GPUI Kit · {}",
                platform_label(&self.system.platform)
            ))
    }
}

fn header_row(cx: &App) -> impl IntoElement {
    h_flex()
        .items_center()
        .px_3()
        .py_2()
        .gap_2()
        .text_xs()
        .text_color(cx.theme().muted_foreground)
        .bg(cx.theme().muted)
        .child(div().w_24().child("编号"))
        .child(div().flex_1().child("环境名称"))
        .child(div().w_16().child("Profile"))
        .child(div().w_20().child("Persona"))
        .child(div().w_16().child("网络"))
        .child(div().w_28().child("RealBrowser"))
        .child(div().w_24().child("最近更新"))
        .child(div().w_32().child("操作"))
}

fn nav_button(
    id: &'static str,
    label: &'static str,
    icon: IconName,
    active: bool,
    collapsed: bool,
) -> Button {
    Button::new(id)
        .ghost()
        .icon(icon)
        .selected(active)
        .disabled(!active)
        .when(!collapsed, |this| this.label(label))
}

fn platform_label(platform: &str) -> &'static str {
    match platform {
        "macos" => "macOS",
        "windows" => "Windows",
        "linux" => "Linux",
        _ => "本机",
    }
}

fn now_ms() -> i64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_millis() as i64)
        .unwrap_or(0)
}

struct CreateDialog {
    parent: WeakEntity<Workspace>,
    control: Arc<ControlService>,
    system: SystemView,
    name: Entity<InputState>,
    startup_url: Entity<InputState>,
    error: Option<String>,
}

impl CreateDialog {
    fn new(
        parent: WeakEntity<Workspace>,
        control: Arc<ControlService>,
        system: SystemView,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) -> Self {
        Self {
            parent,
            control,
            system,
            name: cx.new(|cx| InputState::new(window, cx).placeholder("例如：欧洲站 · 客服")),
            startup_url: cx
                .new(|cx| InputState::new(window, cx).placeholder("https://seller.example.com")),
            error: None,
        }
    }

    fn submit(&mut self, _: &ClickEvent, window: &mut Window, cx: &mut Context<Self>) {
        let name = self.name.read(cx).value().to_string();
        let startup_url = self.startup_url.read(cx).value().to_string();
        let startup_url = if startup_url.trim().is_empty() {
            None
        } else {
            Some(startup_url)
        };
        match self.control.create(CreateIdentity { name, startup_url }) {
            Ok(_) => {
                if let Some(parent) = self.parent.upgrade() {
                    parent.update(cx, |workspace, cx| {
                        workspace.scope = Scope::Active;
                        workspace.reload();
                        cx.notify();
                    });
                }
                window.close_dialog(cx);
                window.push_notification(Notification::success("浏览器环境已创建"), cx);
            }
            Err(error) => {
                let failure = CommandFailure::from(error);
                self.error = Some(
                    failure
                        .field_issues
                        .first()
                        .map(|issue| issue.message.clone())
                        .unwrap_or(failure.message),
                );
                cx.notify();
            }
        }
    }
}

impl Render for CreateDialog {
    fn render(&mut self, _: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let kernel = self
            .system
            .browser_version
            .clone()
            .map(|version| format!("RealBrowser {version}"))
            .unwrap_or_else(|| "未检测到 RealBrowser 内核".to_owned());
        v_flex()
            .gap_3()
            .p_4()
            .w_full()
            .child(field_label_text("环境名称", cx))
            .child(Input::new(&self.name))
            .child(field_label_text("启动网址", cx))
            .child(Input::new(&self.startup_url))
            .child(
                div()
                    .text_sm()
                    .text_color(cx.theme().muted_foreground)
                    .child(format!(
                        "Profile {} · Persona {} · 网络{}",
                        profile_label(),
                        realbrowser_desktop::persona_mode_label(self.system.default_persona_mode),
                        egress_label(self.system.default_egress_mode)
                    )),
            )
            .child(div().text_sm().child(kernel))
            .when_some(self.error.clone(), |this, error| {
                this.child(div().text_color(cx.theme().danger).child(error))
            })
            .child(
                h_flex().justify_end().gap_2().child(
                    Button::new("create-submit")
                        .primary()
                        .label("创建环境")
                        .on_click(cx.listener(CreateDialog::submit)),
                ),
            )
    }
}

struct RenameDialog {
    parent: WeakEntity<Workspace>,
    control: Arc<ControlService>,
    id: String,
    revision: u64,
    name: Entity<InputState>,
}

impl RenameDialog {
    fn new(
        parent: WeakEntity<Workspace>,
        control: Arc<ControlService>,
        record: IdentityView,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) -> Self {
        let name = cx.new(|cx| InputState::new(window, cx).default_value(record.name));
        Self {
            parent,
            control,
            id: record.id,
            revision: record.revision,
            name,
        }
    }

    fn submit(&mut self, _: &ClickEvent, window: &mut Window, cx: &mut Context<Self>) {
        let Ok(identity_id) = parse_identity_id(&self.id) else {
            return;
        };
        let name = self.name.read(cx).value().to_string();
        match self.control.rename(identity_id, self.revision, &name) {
            Ok(_) => {
                if let Some(parent) = self.parent.upgrade() {
                    parent.update(cx, |workspace, cx| {
                        workspace.reload();
                        cx.notify();
                    });
                }
                window.close_dialog(cx);
                window.push_notification(Notification::success("环境名称已更新"), cx);
            }
            Err(error) => Workspace::failure(window, cx, error),
        }
    }
}

impl Render for RenameDialog {
    fn render(&mut self, _: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        v_flex().gap_3().p_4().child(Input::new(&self.name)).child(
            h_flex().justify_end().child(
                Button::new("rename-submit")
                    .primary()
                    .label("保存")
                    .on_click(cx.listener(RenameDialog::submit)),
            ),
        )
    }
}

struct NetworkSheet {
    parent: WeakEntity<Workspace>,
    control: Arc<ControlService>,
    id: String,
    revision: u64,
    running: bool,
    mode: NetworkMode,
    protocol: ProxyProtocol,
    host: Entity<InputState>,
    port: Entity<InputState>,
}

impl NetworkSheet {
    fn new(
        parent: WeakEntity<Workspace>,
        control: Arc<ControlService>,
        record: IdentityView,
        window: &mut Window,
        cx: &mut Context<Self>,
    ) -> Self {
        let proxy = record.network.proxy.clone();
        let host = proxy
            .as_ref()
            .map(|proxy| proxy.host.clone())
            .unwrap_or_default();
        let port = proxy
            .as_ref()
            .map(|proxy| proxy.port.to_string())
            .unwrap_or_else(|| "8080".to_owned());
        let protocol = proxy
            .as_ref()
            .map(|proxy| proxy.protocol)
            .unwrap_or(ProxyProtocol::Http);
        Self {
            parent,
            control,
            id: record.id,
            revision: record.revision,
            running: record.runtime_state == "running",
            mode: record.network.mode,
            protocol,
            host: cx.new(|cx| {
                InputState::new(window, cx)
                    .default_value(host)
                    .placeholder("127.0.0.1")
            }),
            port: cx.new(|cx| {
                InputState::new(window, cx)
                    .default_value(port)
                    .placeholder("8080")
            }),
        }
    }

    fn save(&mut self, _: &ClickEvent, window: &mut Window, cx: &mut Context<Self>) {
        if self.running {
            return;
        }
        let Ok(identity_id) = parse_identity_id(&self.id) else {
            return;
        };
        let network = if self.mode == NetworkMode::Direct {
            network_direct()
        } else {
            let host = self.host.read(cx).value().to_string();
            let port = self.port.read(cx).value().parse::<u16>().unwrap_or(0);
            network_proxy(self.protocol, host, port)
        };
        match self
            .control
            .update_network(identity_id, self.revision, network)
        {
            Ok(_) => {
                if let Some(parent) = self.parent.upgrade() {
                    parent.update(cx, |workspace, cx| {
                        workspace.reload();
                        cx.notify();
                    });
                }
                window.close_sheet(cx);
                window.push_notification(Notification::success("网络出口已保存"), cx);
            }
            Err(error) => Workspace::failure(window, cx, error),
        }
    }
}

impl Render for NetworkSheet {
    fn render(&mut self, _: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let running = self.running;
        let mode = self.mode;
        let protocol = self.protocol;
        v_flex()
            .size_full()
            .gap_3()
            .p_4()
            .child(
                h_flex()
                    .gap_2()
                    .child(
                        Button::new("mode-direct")
                            .small()
                            .selected(mode == NetworkMode::Direct)
                            .disabled(running)
                            .label("直连")
                            .on_click(cx.listener(|this, _, _, cx| {
                                this.mode = NetworkMode::Direct;
                                cx.notify();
                            })),
                    )
                    .child(
                        Button::new("mode-proxy")
                            .small()
                            .selected(mode == NetworkMode::Proxy)
                            .disabled(running)
                            .label("代理")
                            .on_click(cx.listener(|this, _, _, cx| {
                                this.mode = NetworkMode::Proxy;
                                cx.notify();
                            })),
                    ),
            )
            .when(mode == NetworkMode::Proxy, |this| {
                this.child(protocol_picker(protocol, running, cx))
                    .child(field_label_text("主机", cx))
                    .child(Input::new(&self.host))
                    .child(field_label_text("端口", cx))
                    .child(Input::new(&self.port))
                    .child(
                        div()
                            .text_sm()
                            .text_color(cx.theme().muted_foreground)
                            .child("认证：无认证 / IP 白名单"),
                    )
            })
            .child(
                div()
                    .text_xs()
                    .text_color(cx.theme().muted_foreground)
                    .child(if running {
                        "运行中只读"
                    } else {
                        "保存后在下次打开时生效"
                    }),
            )
            .child(
                h_flex().justify_end().child(
                    Button::new("network-save")
                        .primary()
                        .disabled(running)
                        .label(if running { "请先停止" } else { "保存" })
                        .on_click(cx.listener(NetworkSheet::save)),
                ),
            )
    }
}

fn protocol_picker(
    current: ProxyProtocol,
    running: bool,
    cx: &mut Context<NetworkSheet>,
) -> impl IntoElement {
    let this = cx.entity();
    Button::new("proxy-protocol")
        .small()
        .outline()
        .disabled(running)
        .label(match current {
            ProxyProtocol::Http => "HTTP",
            ProxyProtocol::Https => "HTTPS",
            ProxyProtocol::Socks5 => "SOCKS5",
        })
        .dropdown_menu(move |menu, _, _| {
            menu.item(protocol_item(
                this.clone(),
                ProxyProtocol::Http,
                "HTTP",
                current,
            ))
            .item(protocol_item(
                this.clone(),
                ProxyProtocol::Https,
                "HTTPS",
                current,
            ))
            .item(protocol_item(
                this,
                ProxyProtocol::Socks5,
                "SOCKS5",
                current,
            ))
        })
}

fn protocol_item(
    sheet: Entity<NetworkSheet>,
    protocol: ProxyProtocol,
    label: &'static str,
    current: ProxyProtocol,
) -> PopupMenuItem {
    PopupMenuItem::new(label)
        .checked(current == protocol)
        .on_click(move |_, _, cx| {
            sheet.update(cx, |this, cx| {
                this.protocol = protocol;
                cx.notify();
            });
        })
}

struct PersonaSheet {
    parent: WeakEntity<Workspace>,
    control: Arc<ControlService>,
    system: SystemView,
    id: String,
    revision: u64,
    running: bool,
    name: String,
    panel: usize,
    draft: PersonaConfig,
    baseline: PersonaConfig,
}

impl PersonaSheet {
    fn new(
        parent: WeakEntity<Workspace>,
        control: Arc<ControlService>,
        system: SystemView,
        record: IdentityView,
    ) -> Self {
        Self {
            parent,
            control,
            system,
            id: record.id,
            revision: record.revision,
            running: record.runtime_state == "running",
            name: record.name,
            panel: 0,
            draft: record.persona.clone(),
            baseline: record.persona,
        }
    }

    fn save(&mut self, _: &ClickEvent, window: &mut Window, cx: &mut Context<Self>) {
        if self.running {
            return;
        }
        let Ok(identity_id) = parse_identity_id(&self.id) else {
            return;
        };
        match self
            .control
            .update_persona(identity_id, self.revision, self.draft.clone())
        {
            Ok(_) => {
                if let Some(parent) = self.parent.upgrade() {
                    parent.update(cx, |workspace, cx| {
                        workspace.reload();
                        cx.notify();
                    });
                }
                window.close_sheet(cx);
                window.push_notification(Notification::success("指纹设置已保存"), cx);
            }
            Err(error) => Workspace::failure(window, cx, error),
        }
    }
}

const PERSONA_TABS: [&str; 6] = ["基础", "地区", "设备", "图形", "媒体", "隐私"];

impl Render for PersonaSheet {
    fn render(&mut self, _: &mut Window, cx: &mut Context<Self>) -> impl IntoElement {
        let panel = self.panel;
        let running = self.running;
        let dirty = self.draft != self.baseline;
        let canvas_kernel = self.system.persona_capabilities.iter().any(|capability| {
            capability.field == "graphics.canvas" && capability.backend == "custom_kernel"
        });
        let sheet = cx.entity();
        v_flex()
            .size_full()
            .gap_3()
            .p_4()
            .child(
                div()
                    .text_sm()
                    .text_color(cx.theme().muted_foreground)
                    .child(self.name.clone()),
            )
            .child(
                TabBar::new("persona-tabs")
                    .selected_index(panel)
                    .on_click(move |index, _, cx| {
                        sheet.update(cx, |this, cx| {
                            this.panel = *index;
                            cx.notify();
                        });
                    })
                    .children(PERSONA_TABS.map(Tab::from)),
            )
            .child(
                div()
                    .flex_1()
                    .min_h_0()
                    .overflow_y_scroll()
                    .child(self.panel_body(canvas_kernel, cx)),
            )
            .child(
                h_flex()
                    .justify_between()
                    .items_center()
                    .child(
                        div()
                            .text_xs()
                            .text_color(cx.theme().muted_foreground)
                            .child(if running {
                                "运行中只读"
                            } else if persona_is_native(&self.draft) {
                                "系统原生"
                            } else {
                                "已配置"
                            }),
                    )
                    .child(
                        h_flex()
                            .gap_2()
                            .child(
                                Button::new("persona-reset")
                                    .ghost()
                                    .small()
                                    .disabled(running)
                                    .label("恢复原生")
                                    .on_click(cx.listener(|this, _, _, cx| {
                                        this.draft = reset_native(&this.draft);
                                        cx.notify();
                                    })),
                            )
                            .child(
                                Button::new("persona-save")
                                    .primary()
                                    .small()
                                    .disabled(running || !dirty)
                                    .label(if running { "请先停止" } else { "保存" })
                                    .on_click(cx.listener(PersonaSheet::save)),
                            ),
                    ),
            )
    }
}

impl PersonaSheet {
    fn panel_body(&self, canvas_kernel: bool, cx: &mut Context<Self>) -> impl IntoElement {
        match self.panel {
            0 => self.overview(cx).into_any_element(),
            1 => self.region_panel(cx).into_any_element(),
            2 => self.device_panel(cx).into_any_element(),
            5 => self.privacy_panel(cx).into_any_element(),
            panel => self
                .readonly_group(group_for_panel(panel), canvas_kernel, cx)
                .into_any_element(),
        }
    }

    fn overview(&self, cx: &mut Context<Self>) -> impl IntoElement {
        let preset = self.draft.region_preset;
        v_flex()
            .gap_2()
            .child(summary_line(
                "模式",
                if persona_is_native(&self.draft) {
                    "系统原生"
                } else {
                    "自定义"
                },
                cx,
            ))
            .child(summary_line(
                "Schema",
                &format!("v{}", self.draft.schema_version),
                cx,
            ))
            .child(summary_line(
                "稳定种子",
                &format!("{:08X}", self.draft.seed),
                cx,
            ))
            .child(summary_line("地区预设", region_label(preset), cx))
            .child(summary_line("语言", locale_label(self.draft.locale), cx))
            .child(summary_line(
                "时区",
                &timezone_label(&self.draft.timezone),
                cx,
            ))
            .child(summary_line(
                "窗口",
                &format!("{} × {}", self.draft.window_width, self.draft.window_height),
                cx,
            ))
            .child(summary_line("WebRTC", webrtc_label(self.draft.webrtc), cx))
            .child(preset_row(preset, self.running, cx))
    }

    fn region_panel(&self, cx: &mut Context<Self>) -> impl IntoElement {
        v_flex()
            .gap_3()
            .child(locale_picker(self.draft.locale, self.running, cx))
            .child(timezone_picker(
                &self.draft.timezone,
                &self.system.timezone_ids,
                self.running,
                cx,
            ))
            .child(readonly_line(
                "地理位置",
                readonly_field_value("region.geolocation", false),
                cx,
            ))
    }

    fn device_panel(&self, cx: &mut Context<Self>) -> impl IntoElement {
        let metrics = self.draft.display_metrics;
        v_flex()
            .gap_3()
            .child(window_picker(
                self.draft.window_width,
                self.draft.window_height,
                self.running,
                cx,
            ))
            .child(viewport_picker(metrics, self.running, cx))
            .child(screen_picker(metrics, self.running, cx))
            .child(scale_picker(metrics, self.running, cx))
            .child(readonly_line(
                "CPU",
                readonly_field_value("hardware.cpu", false),
                cx,
            ))
            .child(readonly_line(
                "内存",
                readonly_field_value("hardware.deviceMemory", false),
                cx,
            ))
            .child(readonly_line(
                "触控",
                readonly_field_value("hardware.touch", false),
                cx,
            ))
    }

    fn privacy_panel(&self, cx: &mut Context<Self>) -> impl IntoElement {
        v_flex()
            .gap_3()
            .child(webrtc_picker(self.draft.webrtc, self.running, cx))
            .child(readonly_line(
                "权限",
                readonly_field_value("privacy.permissions", false),
                cx,
            ))
            .child(readonly_line(
                "Do Not Track",
                readonly_field_value("privacy.doNotTrack", false),
                cx,
            ))
            .child(readonly_line(
                "GPC",
                readonly_field_value("privacy.globalPrivacyControl", false),
                cx,
            ))
    }

    fn readonly_group(&self, group: &str, canvas_kernel: bool, cx: &App) -> impl IntoElement {
        v_flex().gap_2().children(
            self.system
                .persona_capabilities
                .iter()
                .filter(|capability| capability.group == group)
                .map(|capability| {
                    readonly_line(
                        field_label(capability.field),
                        readonly_field_value(capability.field, canvas_kernel),
                        cx,
                    )
                    .into_any_element()
                }),
        )
    }
}

fn group_for_panel(panel: usize) -> &'static str {
    match panel {
        3 => "graphics",
        4 => "media",
        _ => "privacy",
    }
}

fn summary_line(label: &str, value: &str, cx: &App) -> impl IntoElement {
    h_flex()
        .justify_between()
        .text_sm()
        .child(
            div()
                .text_color(cx.theme().muted_foreground)
                .child(label.to_owned()),
        )
        .child(value.to_owned())
}

fn readonly_line(label: &str, value: &str, cx: &App) -> impl IntoElement {
    summary_line(label, value, cx)
}

fn field_label_text(label: &'static str, cx: &App) -> impl IntoElement {
    div()
        .text_sm()
        .text_color(cx.theme().muted_foreground)
        .child(label)
}

fn preset_row(
    current: RegionPreset,
    running: bool,
    cx: &mut Context<PersonaSheet>,
) -> impl IntoElement {
    let presets = [
        RegionPreset::Native,
        RegionPreset::Shanghai,
        RegionPreset::NewYork,
        RegionPreset::Berlin,
        RegionPreset::Moscow,
    ];
    h_flex().gap_1().flex_wrap().children(presets.map(|preset| {
        Button::new(("preset", region_label(preset)))
            .xsmall()
            .selected(current == preset)
            .disabled(running)
            .label(region_label(preset))
            .on_click(cx.listener(move |this, _, _, cx| {
                this.draft = apply_preset(&this.draft, preset);
                cx.notify();
            }))
    }))
}

fn locale_picker(
    current: LocalePolicy,
    running: bool,
    cx: &mut Context<PersonaSheet>,
) -> impl IntoElement {
    let sheet = cx.entity();
    let locales = [
        LocalePolicy::System,
        LocalePolicy::ZhCn,
        LocalePolicy::EnUs,
        LocalePolicy::RuRu,
        LocalePolicy::DeDe,
    ];
    labeled_menu(
        cx,
        "语言",
        locale_label(current),
        running,
        move |menu, _, _| {
            let mut menu = menu;
            for locale in locales {
                let sheet = sheet.clone();
                menu = menu.item(
                    PopupMenuItem::new(locale_label(locale))
                        .checked(current == locale)
                        .on_click(move |_, _, cx| {
                            sheet.update(cx, |this, cx| {
                                this.draft = apply_locale(&this.draft, locale);
                                cx.notify();
                            });
                        }),
                );
            }
            menu
        },
    )
}

fn timezone_picker(
    current: &TimezonePolicy,
    catalog: &[String],
    running: bool,
    cx: &mut Context<PersonaSheet>,
) -> impl IntoElement {
    let sheet = cx.entity();
    let current_label = timezone_label(current);
    let system = current.is_system();
    let current_id = current.cdp_value().unwrap_or("").to_owned();
    labeled_menu(cx, "时区", &current_label, running, move |menu, _, _| {
        let mut menu = menu.scrollable(true).max_h(px(320.));
        let sheet_system = sheet.clone();
        menu = menu.item(PopupMenuItem::new("跟随系统").checked(system).on_click(
            move |_, _, cx| {
                sheet_system.update(cx, |this, cx| {
                    this.draft = apply_timezone(&this.draft, TimezonePolicy::system());
                    cx.notify();
                });
            },
        ));
        for timezone in catalog {
            let sheet = sheet.clone();
            let id = timezone.clone();
            menu = menu.item(
                PopupMenuItem::new(timezone.clone())
                    .checked(current_id == *timezone)
                    .on_click(move |_, _, cx| {
                        if let Ok(policy) = TimezonePolicy::iana(&id) {
                            sheet.update(cx, |this, cx| {
                                this.draft = apply_timezone(&this.draft, policy);
                                cx.notify();
                            });
                        }
                    }),
            );
        }
        menu
    })
}

fn window_picker(
    width: u16,
    height: u16,
    running: bool,
    cx: &mut Context<PersonaSheet>,
) -> impl IntoElement {
    choice_picker(
        cx,
        "window",
        "窗口尺寸",
        format!("{width} × {height}"),
        running,
        [(1440, 900), (1680, 1050), (1920, 1080), (2560, 1440)],
        move |this, value, cx| {
            this.draft = apply_window(&this.draft, value.0, value.1);
            cx.notify();
        },
    )
}

fn viewport_picker(
    metrics: Option<browser_persona::DisplayMetrics>,
    running: bool,
    cx: &mut Context<PersonaSheet>,
) -> impl IntoElement {
    let label = metrics
        .map(|metrics| format!("{} × {}", metrics.viewport_width, metrics.viewport_height))
        .unwrap_or_else(|| "跟随窗口".to_owned());
    let sheet = cx.entity();
    labeled_menu(cx, "Viewport", &label, running, move |menu, _, _| {
        let mut menu = menu.item(
            PopupMenuItem::new("跟随窗口")
                .checked(metrics.is_none())
                .on_click({
                    let sheet = sheet.clone();
                    move |_, _, cx| {
                        sheet.update(cx, |this, cx| {
                            this.draft = apply_viewport(&this.draft, None, None);
                            cx.notify();
                        });
                    }
                }),
        );
        for (width, height) in [(1280, 720), (1366, 768), (1440, 900), (1920, 1080)] {
            let sheet = sheet.clone();
            menu = menu.item(
                PopupMenuItem::new(format!("{width} × {height}"))
                    .checked(metrics.is_some_and(|metrics| {
                        metrics.viewport_width == width && metrics.viewport_height == height
                    }))
                    .on_click(move |_, _, cx| {
                        sheet.update(cx, |this, cx| {
                            this.draft = apply_viewport(&this.draft, Some(width), Some(height));
                            cx.notify();
                        });
                    }),
            );
        }
        menu
    })
}

fn screen_picker(
    metrics: Option<browser_persona::DisplayMetrics>,
    running: bool,
    cx: &mut Context<PersonaSheet>,
) -> impl IntoElement {
    let Some(metrics) = metrics else {
        return readonly_line("屏幕", "系统屏幕", cx).into_any_element();
    };
    choice_picker(
        cx,
        "screen",
        "屏幕",
        format!("{} × {}", metrics.screen_width, metrics.screen_height),
        running,
        [
            (1366, 768),
            (1440, 900),
            (1680, 1050),
            (1920, 1080),
            (2560, 1440),
            (3840, 2160),
        ],
        move |this, value, cx| {
            this.draft = apply_screen(&this.draft, value.0, value.1);
            cx.notify();
        },
    )
    .into_any_element()
}

fn scale_picker(
    metrics: Option<browser_persona::DisplayMetrics>,
    running: bool,
    cx: &mut Context<PersonaSheet>,
) -> impl IntoElement {
    let Some(metrics) = metrics else {
        return readonly_line("像素比", "系统缩放", cx).into_any_element();
    };
    let current = metrics.device_scale_factor_percent;
    let sheet = cx.entity();
    labeled_menu(
        cx,
        "像素比",
        &format!("{:.2}×", f32::from(current) / 100.0),
        running,
        move |menu, _, _| {
            let mut menu = menu;
            for percent in [100, 125, 150, 175, 200, 250, 300] {
                let sheet = sheet.clone();
                menu = menu.item(
                    PopupMenuItem::new(format!("{:.2}×", f32::from(percent) / 100.0))
                        .checked(current == percent)
                        .on_click(move |_, _, cx| {
                            sheet.update(cx, |this, cx| {
                                this.draft = apply_scale(&this.draft, percent);
                                cx.notify();
                            });
                        }),
                );
            }
            menu
        },
    )
    .into_any_element()
}

fn webrtc_picker(
    current: WebRtcPolicy,
    running: bool,
    cx: &mut Context<PersonaSheet>,
) -> impl IntoElement {
    let sheet = cx.entity();
    labeled_menu(
        cx,
        "WebRTC",
        webrtc_label(current),
        running,
        move |menu, _, _| {
            menu.item(
                PopupMenuItem::new(webrtc_label(WebRtcPolicy::Native))
                    .checked(current == WebRtcPolicy::Native)
                    .on_click({
                        let sheet = sheet.clone();
                        move |_, _, cx| {
                            sheet.update(cx, |this, cx| {
                                this.draft = apply_webrtc(&this.draft, WebRtcPolicy::Native);
                                cx.notify();
                            });
                        }
                    }),
            )
            .item(
                PopupMenuItem::new(webrtc_label(WebRtcPolicy::DisableNonProxiedUdp))
                    .checked(current == WebRtcPolicy::DisableNonProxiedUdp)
                    .on_click(move |_, _, cx| {
                        sheet.update(cx, |this, cx| {
                            this.draft =
                                apply_webrtc(&this.draft, WebRtcPolicy::DisableNonProxiedUdp);
                            cx.notify();
                        });
                    }),
            )
        },
    )
}

fn choice_picker<const N: usize>(
    cx: &mut Context<PersonaSheet>,
    id: &'static str,
    label: &'static str,
    current_label: String,
    running: bool,
    choices: [(u16, u16); N],
    apply: impl Fn(&mut PersonaSheet, (u16, u16), &mut Context<PersonaSheet>) + 'static + Clone,
) -> impl IntoElement {
    let sheet = cx.entity();
    labeled_menu(cx, label, &current_label, running, move |menu, _, _| {
        let mut menu = menu;
        for choice in choices {
            let sheet = sheet.clone();
            let apply = apply.clone();
            menu = menu.item(
                PopupMenuItem::new(format!("{} × {}", choice.0, choice.1)).on_click(
                    move |_, _, cx| {
                        sheet.update(cx, |this, cx| apply(this, choice, cx));
                    },
                ),
            );
        }
        let _ = id;
        menu
    })
}

fn labeled_menu<F>(
    cx: &mut Context<PersonaSheet>,
    label: &'static str,
    value: &str,
    running: bool,
    builder: F,
) -> impl IntoElement
where
    F: Fn(PopupMenu, &mut Window, &mut Context<PopupMenu>) -> PopupMenu + 'static,
{
    h_flex()
        .justify_between()
        .items_center()
        .child(field_label_text(label, cx))
        .child(
            Button::new(("menu", label))
                .small()
                .outline()
                .disabled(running)
                .label(value.to_owned())
                .dropdown_menu(builder),
        )
}
