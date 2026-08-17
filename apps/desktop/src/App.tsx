import {
  Activity,
  AppWindow,
  Archive,
  Check,
  CheckCircle2,
  ChevronDown,
  CircleStop,
  Cpu,
  Fingerprint,
  FolderOpen,
  Gauge,
  Globe,
  Globe2,
  HardDrive,
  Languages,
  MoreHorizontal,
  Monitor,
  PanelLeftClose,
  PanelLeftOpen,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  RadioTower,
  RotateCcw,
  Search,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { toast } from "sonner";
import { RealBrowserIcon } from "./components/RealBrowserIcon";

import { Button } from "./components/ui/button";
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "./components/ui/command";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "./components/ui/dialog";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "./components/ui/dropdown-menu";
import { Input } from "./components/ui/input";
import { Popover, PopoverContent, PopoverTrigger } from "./components/ui/popover";
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "./components/ui/sheet";
import { Tooltip, TooltipContent, TooltipTrigger } from "./components/ui/tooltip";
import {
  browserClient,
  commandFailure,
  defaultNetwork,
  defaultPersona,
  personaMode,
  type DisplayMetrics,
  type IdentityView,
  type NetworkConfig,
  type PersonaCapability,
  type PersonaConfig,
  type PersonaGroup,
  type SystemView,
} from "./lib/browser-client";
import { cn } from "./lib/utils";

export default function App() {
  const [scope, setScope] = useState<"active" | "archived">("active");
  const [records, setRecords] = useState<IdentityView[]>([]);
  const [system, setSystem] = useState<SystemView | null>(null);
  const [query, setQuery] = useState("");
  const [runningOnly, setRunningOnly] = useState(false);
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [renameRecord, setRenameRecord] = useState<IdentityView | null>(null);
  const [personaRecord, setPersonaRecord] = useState<IdentityView | null>(null);
  const [networkRecord, setNetworkRecord] = useState<IdentityView | null>(null);

  const refresh = useCallback(async (targetScope: "active" | "archived" = scope) => {
    setLoading(true);
    try {
      const [nextRecords, nextSystem] = await Promise.all([
        targetScope === "active" ? browserClient.list() : browserClient.listArchived(),
        browserClient.system(),
      ]);
      setRecords(nextRecords);
      setSystem(nextSystem);
    } catch (error) {
      const failure = commandFailure(error);
      toast.error(failure.fieldIssues?.[0]?.message ?? failure.message);
    } finally {
      setLoading(false);
    }
  }, [scope]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const visibleRecords = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase();
    return records.filter((record) => {
      if (runningOnly && record.runtimeState !== "running") return false;
      if (!normalizedQuery) return true;
      return [record.name, record.displayCode, record.id, record.startupUrl ?? ""]
        .join(" ")
        .toLocaleLowerCase()
        .includes(normalizedQuery);
    });
  }, [query, records, runningOnly]);

  const runningCount = records.filter((record) => record.runtimeState === "running").length;

  async function runAction(
    record: IdentityView,
    action: "start" | "stop" | "archive" | "restore",
  ) {
    setBusyId(record.id);
    try {
      if (action === "archive") {
        await browserClient.archive(record.id, record.revision);
        toast.success("环境已移入归档，Profile 数据仍保留");
      } else if (action === "restore") {
        await browserClient.restore(record.id, record.revision);
        toast.success("环境已恢复");
      } else if (action === "stop") {
        const result = await browserClient.stop(record.id);
        if (result.termination === "forced") {
          toast.warning("RealBrowser 未及时退出，已强制停止");
        } else {
          toast.success("环境已停止");
        }
      } else {
        await browserClient.start(record.id);
      }
      await refresh();
    } catch (error) {
      const failure = commandFailure(error);
      toast.error(failure.fieldIssues?.[0]?.message ?? failure.message);
    } finally {
      setBusyId(null);
    }
  }

  return (
    <div className="app-frame">
      <NavigationRail />
      <main className="min-w-0 flex-1 overflow-hidden">
        <div className="flex h-full flex-col px-6 pb-0 pt-6 xl:px-8">
          {browserClient.isPreview && (
            <div className="mb-3 self-start rounded-xl bg-warning-soft px-3 py-1.5 text-xs font-medium text-warning">
              预览模式
            </div>
          )}

          <section className="mb-4 flex items-center gap-2.5">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button type="button" className="scope-control w-[180px] shrink-0 text-left">
                  {scope === "active" ? <FolderOpen className="size-5" /> : <Archive className="size-5" />}
                  <span>{scope === "active" ? "使用中环境" : "归档"}</span>
                  <span className="ml-auto tabular-nums text-muted-foreground">{records.length}</span>
                  <ChevronDown className="size-4 text-muted-foreground" />
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="start" className="w-[180px]">
                <DropdownMenuItem
                  onSelect={() => {
                    setScope("active");
                    setRunningOnly(false);
                  }}
                >
                  <FolderOpen /> 使用中环境
                </DropdownMenuItem>
                <DropdownMenuItem
                  onSelect={() => {
                    setScope("archived");
                    setRunningOnly(false);
                  }}
                >
                  <Archive /> 归档
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
            <label className="search-control min-w-[200px] flex-1">
              <Search className="size-5 shrink-0" />
              <input
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder="搜索名称、编号或网址"
                aria-label="搜索浏览器环境"
              />
              <SlidersHorizontal className="size-[18px] shrink-0 text-foreground" />
            </label>
            {scope === "active" ? (
              <Button
                variant={runningOnly ? "subtle" : "outline"}
                className="h-11 min-w-32 shrink-0 justify-between"
                onClick={() => setRunningOnly((value) => !value)}
                aria-pressed={runningOnly}
              >
                <span className="status-dot status-dot-running" />
                运行中 {runningCount}
                <ChevronDown className="size-4" />
              </Button>
            ) : (
              <Button
                variant="outline"
                className="h-11 min-w-32 shrink-0"
                onClick={() => setScope("active")}
              >
                <RotateCcw /> 使用中
              </Button>
            )}
            {scope === "active" && (
              <Button size="lg" className="shrink-0" onClick={() => setCreateOpen(true)}>
                <Plus className="size-4" />
                新建环境
              </Button>
            )}
            <Button
              size="lg"
              variant="outline"
              className="shrink-0"
              onClick={() => void refresh()}
              disabled={loading}
            >
              <RefreshCw className={cn("size-[18px]", loading && "animate-spin")} />
              刷新
            </Button>
          </section>

          <section className="table-shell min-h-0 flex-1" aria-busy={loading}>
            {loading && records.length === 0 ? (
              <LoadingTable />
            ) : visibleRecords.length === 0 ? (
              <EmptyState
                filtered={records.length > 0}
                archived={scope === "archived"}
                onCreate={() => setCreateOpen(true)}
                onReturn={() => setScope("active")}
                onClear={() => {
                  setQuery("");
                  setRunningOnly(false);
                }}
              />
            ) : (
              <div className="h-full overflow-auto">
                <table className="w-full min-w-[1020px] border-separate border-spacing-0 text-left">
                  <thead className="sticky top-0 z-10 bg-table-head text-xs font-semibold text-muted-foreground">
                    <tr>
                      <th className="w-[108px] px-4 py-3">编号</th>
                      <th className="min-w-[240px] px-3.5 py-3">环境名称</th>
                      <th className="w-[138px] px-3.5 py-3">Profile</th>
                      <th className="w-[110px] px-3.5 py-3">Persona</th>
                      <th className="w-[120px] px-3.5 py-3">网络出口</th>
                      <th className="w-[156px] px-3.5 py-3">RealBrowser</th>
                      <th className="w-[126px] px-3.5 py-3">最近更新</th>
                      <th className="w-[132px] px-4 py-3 text-right">操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {visibleRecords.map((record) => (
                      <IdentityRow
                        key={record.id}
                        record={record}
                        busy={busyId === record.id}
                        archived={scope === "archived"}
                        onAction={(action) => void runAction(record, action)}
                        onRename={() => setRenameRecord(record)}
                        onPersona={() => setPersonaRecord(record)}
                        onNetwork={() => setNetworkRecord(record)}
                      />
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>

          <StatusBar system={system} count={records.length} />
        </div>
      </main>

      <CreateIdentityDialog
        open={createOpen}
        system={system}
        onOpenChange={setCreateOpen}
        onCreated={async () => {
          setScope("active");
          await refresh("active");
        }}
      />
      <RenameIdentityDialog
        record={renameRecord}
        onOpenChange={(open) => !open && setRenameRecord(null)}
        onRenamed={refresh}
      />
      <PersonaSheet
        record={personaRecord}
        system={system}
        onOpenChange={(open) => !open && setPersonaRecord(null)}
        onSaved={refresh}
      />
      <NetworkSheet
        record={networkRecord}
        onOpenChange={(open) => !open && setNetworkRecord(null)}
        onSaved={refresh}
      />
    </div>
  );
}

function NavigationRail() {
  const [isExpanded, setIsExpanded] = useState(() => {
    try {
      return localStorage.getItem("realbrowser:sidebar_expanded") === "true";
    } catch {
      return false;
    }
  });

  const toggleExpanded = () => {
    setIsExpanded((prev) => {
      const next = !prev;
      try {
        localStorage.setItem("realbrowser:sidebar_expanded", String(next));
      } catch {
        // ignore
      }
      return next;
    });
  };

  const items = [
    { label: "环境", expandedLabel: "环境管理", icon: AppWindow, active: true },
    { label: "运行记录", expandedLabel: "运行记录", tag: "即将提供", icon: Activity, active: false },
    { label: "诊断", expandedLabel: "系统诊断", tag: "即将提供", icon: Gauge, active: false },
    { label: "设置", expandedLabel: "偏好设置", tag: "即将提供", icon: Settings, active: false },
  ];

  return (
    <aside
      className={cn("nav-rail", isExpanded && "nav-rail-expanded")}
      data-expanded={isExpanded}
      aria-label="主导航"
    >
      <div className={cn("brand-header", isExpanded ? "brand-header-expanded" : "brand-header-collapsed")}>
        <div className="flex items-center gap-2.5 min-w-0">
          <div className="brand-mark shrink-0" aria-label="RealBrowser">
            <RealBrowserIcon size={isExpanded ? 32 : 40} variant="squircle" />
          </div>
          {isExpanded && (
            <span className="brand-title truncate select-none">RealBrowser</span>
          )}
        </div>
        {isExpanded && (
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                className="nav-toggle-button"
                onClick={toggleExpanded}
                aria-label="收起导航"
              >
                <PanelLeftClose className="size-4" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="right">收起导航</TooltipContent>
          </Tooltip>
        )}
      </div>

      <nav className={cn("nav-list", isExpanded ? "nav-list-expanded" : "nav-list-collapsed")}>
        {items.map((item) => {
          const button = (
            <button
              type="button"
              className={cn(
                "nav-button",
                isExpanded ? "nav-button-expanded" : "nav-button-collapsed",
                item.active && "nav-button-active"
              )}
              aria-current={item.active ? "page" : undefined}
              disabled={!item.active}
            >
              <item.icon className="size-5 shrink-0" />
              {isExpanded && (
                <span className="nav-button-label truncate">{item.expandedLabel}</span>
              )}
              {isExpanded && item.tag && (
                <span className="nav-button-badge">{item.tag}</span>
              )}
              {!isExpanded && <span className="sr-only">{item.label}</span>}
            </button>
          );

          if (isExpanded) {
            return (
              <div key={item.label} className="w-full">
                {button}
              </div>
            );
          }

          return (
            <Tooltip key={item.label}>
              <TooltipTrigger asChild>{button}</TooltipTrigger>
              <TooltipContent side="right">
                {item.label}
                {item.tag ? `（${item.tag}）` : ""}
              </TooltipContent>
            </Tooltip>
          );
        })}
      </nav>

      <div className={cn("nav-footer", isExpanded ? "nav-footer-expanded" : "nav-footer-collapsed")}>
        {!isExpanded && (
          <Tooltip>
            <TooltipTrigger asChild>
              <button
                type="button"
                className="nav-toggle-button"
                onClick={toggleExpanded}
                aria-label="展开导航"
              >
                <PanelLeftOpen className="size-4" />
              </button>
            </TooltipTrigger>
            <TooltipContent side="right">展开导航</TooltipContent>
          </Tooltip>
        )}
        <div className="nav-status-indicator">
          <Tooltip>
            <TooltipTrigger asChild>
              <div className="size-2 rounded-full bg-success shadow-[0_0_0_5px_var(--success-soft)] shrink-0" />
            </TooltipTrigger>
            {!isExpanded && <TooltipContent side="right">服务正常</TooltipContent>}
          </Tooltip>
          {isExpanded && <span className="nav-status-text">服务正常</span>}
        </div>
      </div>
    </aside>
  );
}

function IdentityRow({
  record,
  busy,
  archived,
  onAction,
  onRename,
  onPersona,
  onNetwork,
}: {
  record: IdentityView;
  busy: boolean;
  archived: boolean;
  onAction: (action: "start" | "stop" | "archive" | "restore") => void;
  onRename: () => void;
  onPersona: () => void;
  onNetwork: () => void;
}) {
  const running = record.runtimeState === "running";
  return (
    <tr className="identity-row">
      <td className="border-t border-border px-4 py-3 align-middle text-[13px] font-medium tabular-nums text-muted-foreground">
        {record.displayCode}
      </td>
      <td className="border-t border-border px-3.5 py-3 align-middle">
        <div className="flex min-w-0 items-center gap-2.5">
          <div className="identity-avatar">
            <AppWindow className="size-4" />
          </div>
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold text-foreground">{record.name}</div>
            <div className="mt-0.5 truncate text-xs text-muted-foreground">
              {record.startupUrl ?? "启动后打开空白页"}
            </div>
          </div>
        </div>
      </td>
      <td className="border-t border-border px-3.5 py-3 align-middle">
        <div className="inline-flex items-center gap-2 text-[13px] font-medium">
          <HardDrive className="size-4 text-primary" /> {profileModeLabel(record.profileMode)}
        </div>
      </td>
      <td className="border-t border-border px-3.5 py-3 align-middle">
        {archived ? (
          <span className="inline-flex items-center gap-2 text-[13px] text-muted-foreground">
            <Fingerprint className="size-4" /> {personaStatusLabel(record)}
          </span>
        ) : (
          <button type="button" className="persona-link" onClick={onPersona}>
            <Fingerprint className="size-4" />
            {personaStatusLabel(record)}
          </button>
        )}
      </td>
      <td className="border-t border-border px-3.5 py-3 align-middle">
        {archived ? (
          <span className="inline-flex items-center gap-2 text-[13px] text-muted-foreground">
            <Globe2 className="size-4" /> {egressModeLabel(record.egressMode)}
          </span>
        ) : (
          <button type="button" className="network-link" onClick={onNetwork}>
            <Globe2 className="size-4" />
            {egressModeLabel(record.egressMode)}
          </button>
        )}
      </td>
      <td className="border-t border-border px-3.5 py-3 align-middle">
        <div className="inline-flex items-center gap-2 text-[13px]">
          <Globe className="size-4 text-primary" />
          <span className="max-w-28 truncate">{record.browserVersion ?? "待启动"}</span>
        </div>
      </td>
      <td className="border-t border-border px-3.5 py-3 align-middle text-[13px] tabular-nums text-muted-foreground">
        {relativeTime(record.updatedAtMs)}
      </td>
      <td className="border-t border-border px-4 py-3 align-middle">
        <div className="flex items-center justify-end gap-2">
          {archived ? (
            <Button
              size="sm"
              variant="outline"
              className="min-w-[78px]"
              disabled={busy}
              onClick={() => onAction("restore")}
            >
              <RotateCcw /> {busy ? "处理中" : "恢复"}
            </Button>
          ) : (
            <>
              <Button
                size="sm"
                variant={running ? "subtle" : "default"}
                className="min-w-[78px]"
                disabled={busy}
                onClick={() => onAction(running ? "stop" : "start")}
              >
                {running ? <CircleStop /> : <Play />}
                {busy ? "处理中" : running ? "停止" : "打开"}
              </Button>
              <DropdownMenu>
                <DropdownMenuTrigger asChild>
                  <Button variant="ghost" size="icon" aria-label={`${record.name} 更多操作`}>
                    <MoreHorizontal />
                  </Button>
                </DropdownMenuTrigger>
                <DropdownMenuContent align="end">
                  <DropdownMenuItem onSelect={onPersona}>
                    <Fingerprint /> 指纹设置
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={onNetwork}>
                    <Globe2 /> 代理设置
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={onRename} disabled={running}>
                    <Pencil /> 重命名
                  </DropdownMenuItem>
                  <DropdownMenuSeparator />
                  <DropdownMenuItem
                    className="text-destructive focus:bg-destructive-soft focus:text-destructive"
                    onSelect={() => onAction("archive")}
                    disabled={running}
                  >
                    <Archive /> 移入归档
                  </DropdownMenuItem>
                </DropdownMenuContent>
              </DropdownMenu>
            </>
          )}
        </div>
      </td>
    </tr>
  );
}

function CreateIdentityDialog({
  open,
  system,
  onOpenChange,
  onCreated,
}: {
  open: boolean;
  system: SystemView | null;
  onOpenChange: (open: boolean) => void;
  onCreated: () => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [startupUrl, setStartupUrl] = useState("");
  const [submitting, setSubmitting] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    setSubmitting(true);
    setFieldErrors({});
    try {
      await browserClient.create({
        name,
        startupUrl: startupUrl.trim() || null,
      });
      await onCreated();
      setName("");
      setStartupUrl("");
      onOpenChange(false);
      toast.success("浏览器环境已创建");
    } catch (error) {
      const failure = commandFailure(error);
      setFieldErrors(
        Object.fromEntries((failure.fieldIssues ?? []).map((issue) => [issue.field, issue.message])),
      );
      if (!failure.fieldIssues?.length) toast.error(failure.message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>新建环境</DialogTitle>
          <DialogDescription>创建独立的 RealBrowser 环境。</DialogDescription>
        </DialogHeader>
        <form className="grid gap-5" onSubmit={submit}>
          <div className="grid gap-2">
            <label className="field-label" htmlFor="identity-name">
              环境名称 <span className="text-destructive">*</span>
            </label>
            <Input
              id="identity-name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="例如：欧洲站 · 客服"
              autoFocus
              aria-invalid={Boolean(fieldErrors.name)}
            />
            {fieldErrors.name && <p className="field-error">{fieldErrors.name}</p>}
          </div>
          <div className="grid gap-2">
            <label className="field-label" htmlFor="startup-url">
              启动网址 <span className="font-normal text-muted-foreground">选填</span>
            </label>
            <Input
              id="startup-url"
              value={startupUrl}
              onChange={(event) => setStartupUrl(event.target.value)}
              placeholder="https://seller.example.com"
              aria-invalid={Boolean(fieldErrors.startupUrl)}
            />
            {fieldErrors.startupUrl && <p className="field-error">{fieldErrors.startupUrl}</p>}
          </div>

          <div className="configuration-ledger">
            <div><HardDrive /> <span>Profile</span><strong>{system ? profileModeLabel(system.defaultProfileMode) : "检测中"}</strong></div>
            <div><ShieldCheck /> <span>Persona</span><strong>{system ? personaModeLabel(system.defaultPersonaMode) : "检测中"}</strong></div>
            <div><Globe2 /> <span>网络出口</span><strong>{system ? egressModeLabel(system.defaultEgressMode) : "检测中"}</strong></div>
          </div>
          <div className="flex items-start gap-2.5 rounded-xl bg-muted px-3.5 py-3 text-[13px]">
            <Globe className="mt-0.5 size-4 shrink-0 text-primary" />
            <div>
              <p className="font-semibold text-foreground">
                {system?.browserVersion ? `RealBrowser ${system.browserVersion}` : "未检测到 RealBrowser 内核"}
              </p>
              <p className="mt-1 leading-5 text-muted-foreground">
                Persona {system ? personaModeLabel(system.defaultPersonaMode) : "检测中"} · 网络{system ? egressModeLabel(system.defaultEgressMode) : "检测中"}
              </p>
            </div>
          </div>
          <DialogFooter>
            <DialogClose asChild>
              <Button type="button" variant="ghost">取消</Button>
            </DialogClose>
            <Button type="submit" disabled={submitting}>
              {submitting ? "正在创建…" : "创建环境"}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function RenameIdentityDialog({
  record,
  onOpenChange,
  onRenamed,
}: {
  record: IdentityView | null;
  onOpenChange: (open: boolean) => void;
  onRenamed: () => Promise<void>;
}) {
  const [name, setName] = useState("");
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => setName(record?.name ?? ""), [record]);

  async function submit(event: React.FormEvent) {
    event.preventDefault();
    if (!record) return;
    setSubmitting(true);
    try {
      await browserClient.rename(record.id, record.revision, name);
      await onRenamed();
      onOpenChange(false);
      toast.success("环境名称已更新");
    } catch (error) {
      toast.error(commandFailure(error).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Dialog open={Boolean(record)} onOpenChange={onOpenChange}>
      <DialogContent className="w-[min(92vw,520px)]">
        <DialogHeader>
          <DialogTitle>重命名环境</DialogTitle>
          <DialogDescription>仅修改名称。</DialogDescription>
        </DialogHeader>
        <form className="grid gap-5" onSubmit={submit}>
          <Input value={name} onChange={(event) => setName(event.target.value)} autoFocus />
          <DialogFooter>
            <DialogClose asChild><Button type="button" variant="ghost">取消</Button></DialogClose>
            <Button type="submit" disabled={submitting}>{submitting ? "保存中…" : "保存"}</Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}

function NetworkSheet({
  record,
  onOpenChange,
  onSaved,
}: {
  record: IdentityView | null;
  onOpenChange: (open: boolean) => void;
  onSaved: () => Promise<void>;
}) {
  const [network, setNetwork] = useState<NetworkConfig | null>(null);
  const [submitting, setSubmitting] = useState(false);

  useEffect(() => {
    setNetwork(record ? structuredClone(record.network) : null);
  }, [record]);

  if (!network) return <Sheet open={false} />;

  const running = record?.runtimeState === "running";
  const dirty = Boolean(record && JSON.stringify(record.network) !== JSON.stringify(network));
  const proxy = network.proxy;

  function selectMode(mode: NetworkConfig["mode"]) {
    setNetwork(
      mode === "direct"
        ? defaultNetwork()
        : {
            schemaVersion: 1,
            mode: "proxy",
            proxy: proxy ?? { protocol: "http", host: "", port: 8080 },
          },
    );
  }

  async function save(event: React.FormEvent) {
    event.preventDefault();
    if (!record || !network) return;
    setSubmitting(true);
    try {
      await browserClient.updateNetwork(record.id, record.revision, network);
      await onSaved();
      onOpenChange(false);
      toast.success("网络出口已保存");
    } catch (error) {
      const failure = commandFailure(error);
      toast.error(failure.fieldIssues?.[0]?.message ?? failure.message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Sheet open={Boolean(record)} onOpenChange={onOpenChange}>
      <SheetContent className="w-[min(94vw,520px)]">
        <SheetHeader>
          <SheetTitle>网络出口</SheetTitle>
          <SheetDescription>{record?.name}</SheetDescription>
        </SheetHeader>
        <form className="flex min-h-0 flex-1 flex-col" onSubmit={save}>
          <div className="network-scroll flex-1 overflow-y-auto px-6 py-5">
            <div className="network-mode-grid" role="group" aria-label="网络模式">
              <button
                type="button"
                className={cn("network-mode", network.mode === "direct" && "network-mode-active")}
                aria-pressed={network.mode === "direct"}
                disabled={running}
                onClick={() => selectMode("direct")}
              >
                <Globe2 />
                <span>直连</span>
              </button>
              <button
                type="button"
                className={cn("network-mode", network.mode === "proxy" && "network-mode-active")}
                aria-pressed={network.mode === "proxy"}
                disabled={running}
                onClick={() => selectMode("proxy")}
              >
                <RadioTower />
                <span>代理</span>
              </button>
            </div>

            {network.mode === "proxy" && proxy && (
              <PersonaSection icon={Globe2} title="代理服务器">
                <PersonaField label="协议">
                  <select
                    className="persona-select"
                    aria-label="协议"
                    value={proxy.protocol}
                    disabled={running}
                    onChange={(event) =>
                      setNetwork({
                        ...network,
                        proxy: { ...proxy, protocol: event.target.value as typeof proxy.protocol },
                      })
                    }
                  >
                    <option value="http">HTTP</option>
                    <option value="https">HTTPS</option>
                    <option value="socks5">SOCKS5</option>
                  </select>
                </PersonaField>
                <PersonaField label="主机">
                  <Input
                    aria-label="主机"
                    value={proxy.host}
                    placeholder="127.0.0.1"
                    autoCapitalize="none"
                    autoCorrect="off"
                    disabled={running}
                    required
                    onChange={(event) =>
                      setNetwork({ ...network, proxy: { ...proxy, host: event.target.value } })
                    }
                  />
                </PersonaField>
                <PersonaField label="端口">
                  <Input
                    aria-label="端口"
                    type="number"
                    inputMode="numeric"
                    min={1}
                    max={65535}
                    value={proxy.port || ""}
                    placeholder="8080"
                    disabled={running}
                    required
                    onChange={(event) =>
                      setNetwork({
                        ...network,
                        proxy: { ...proxy, port: Number(event.target.value) },
                      })
                    }
                  />
                </PersonaField>
                <PersonaField label="认证">
                  <BasicValue value="无认证 / IP 白名单" />
                </PersonaField>
                <PersonaField label="WebRTC">
                  <BasicValue
                    value={
                      record?.persona.webrtc === "disable_non_proxied_udp" ? "禁止直连 UDP" : "原生"
                    }
                  />
                </PersonaField>
              </PersonaSection>
            )}
          </div>
          <div className="persona-footer">
            <span>{running ? "运行中只读" : network.mode === "proxy" ? "启动映射" : "系统网络"}</span>
            <div className="flex gap-2">
              <SheetClose asChild><Button type="button" variant="ghost">取消</Button></SheetClose>
              <Button type="submit" disabled={submitting || running || !dirty}>
                {running ? "请先停止" : submitting ? "保存中…" : "保存"}
              </Button>
            </div>
          </div>
        </form>
      </SheetContent>
    </Sheet>
  );
}

type PersonaPanel = "overview" | PersonaGroup;
type CapabilityKind = "native" | "launch" | "profile" | "cdp" | "kernel" | "unavailable";

const personaPanels: Array<{
  id: PersonaPanel;
  label: string;
  icon: typeof Fingerprint;
}> = [
  { id: "overview", label: "基础", icon: Fingerprint },
  { id: "region", label: "地区", icon: Languages },
  { id: "device", label: "设备", icon: Cpu },
  { id: "graphics", label: "图形", icon: Monitor },
  { id: "media", label: "媒体", icon: RadioTower },
  { id: "privacy", label: "隐私", icon: ShieldCheck },
];

const personaFieldLabels: Record<string, string> = {
  "region.language": "语言",
  "region.timezone": "时区",
  "region.geolocation": "地理位置",
  "browser.userAgent": "UA / UA-CH",
  "browser.platform": "操作系统",
  "display.window": "窗口尺寸",
  "display.viewport": "Viewport",
  "display.screen": "屏幕",
  "display.devicePixelRatio": "像素比",
  "hardware.cpu": "CPU",
  "hardware.deviceMemory": "内存",
  "hardware.touch": "触控",
  "browser.plugins": "Plugins",
  "browser.battery": "Battery",
  "graphics.canvas": "Canvas",
  "graphics.webglImage": "WebGL 图像",
  "graphics.webglMetadata": "GPU 信息",
  "graphics.webgpu": "WebGPU",
  "graphics.clientRects": "ClientRects",
  "media.audio": "Audio",
  "media.fonts": "字体",
  "media.mediaDevices": "媒体设备",
  "media.speechVoices": "语音列表",
  "privacy.webrtc": "WebRTC",
  "privacy.permissions": "权限",
  "privacy.doNotTrack": "Do Not Track",
  "privacy.globalPrivacyControl": "GPC",
};

const displayViewportPresets = ["1280x720", "1366x768", "1440x900", "1920x1080"];
const displayScreenPresets = ["1366x768", "1440x900", "1680x1050", "1920x1080", "2560x1440", "3840x2160"];
const displayMetricCapabilityFields = [
  "display.viewport",
  "display.screen",
  "display.devicePixelRatio",
] as const;

function defaultDisplayMetrics(): DisplayMetrics {
  return {
    viewportWidth: 1366,
    viewportHeight: 768,
    screenWidth: 1920,
    screenHeight: 1080,
    deviceScaleFactorPercent: 100,
  };
}

function parseResolution(value: string): [number, number] {
  const [width = "0", height = "0"] = value.split("x");
  return [Number(width), Number(height)];
}

function resolutionLabel(value: string) {
  return value.replace("x", " × ");
}

function formatScalePercent(percent: number) {
  return `${(percent / 100).toFixed(2).replace(/0+$/, "").replace(/\.$/, "")}×`;
}

function PersonaSheet({
  record,
  system,
  onOpenChange,
  onSaved,
}: {
  record: IdentityView | null;
  system: SystemView | null;
  onOpenChange: (open: boolean) => void;
  onSaved: () => Promise<void>;
}) {
  const [persona, setPersona] = useState<PersonaConfig | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [panel, setPanel] = useState<PersonaPanel>("overview");

  useEffect(() => {
    setPersona(record ? structuredClone(record.persona) : null);
    setPanel("overview");
  }, [record]);

  if (!persona) return <Sheet open={false} />;

  const running = record?.runtimeState === "running";
  const capabilities = system?.personaCapabilities ?? [];
  const dirty = Boolean(record && JSON.stringify(record.persona) !== JSON.stringify(persona));
  const editableCount = capabilities.filter((capability) => capability.editable).length;
  const panelDefinition = personaPanels.find((candidate) => candidate.id === panel) ?? personaPanels[0]!;
  const panelCapabilities = capabilities.filter((capability) => capability.group === panel);
  const panelEditableCount = panelCapabilities.filter((capability) => capability.editable).length;
  const editableLabel = panel === "overview"
    ? `全部 ${editableCount} 项可改`
    : `${panelDefinition.label} ${panelEditableCount} 项可改`;

  async function save(event: React.FormEvent) {
    event.preventDefault();
    if (!record || !persona) return;
    setSubmitting(true);
    try {
      await browserClient.updatePersona(record.id, record.revision, persona);
      await onSaved();
      onOpenChange(false);
      toast.success("指纹设置已保存");
    } catch (error) {
      toast.error(commandFailure(error).message);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <Sheet open={Boolean(record)} onOpenChange={onOpenChange}>
      <SheetContent className="w-[min(92vw,560px)]">
        <SheetHeader>
          <SheetTitle>指纹设置</SheetTitle>
          <SheetDescription>{record?.name}</SheetDescription>
        </SheetHeader>
        <form className="flex min-h-0 flex-1 flex-col" onSubmit={save}>
          <nav className="persona-tabs" aria-label="指纹设置分组">
            {personaPanels.map(({ id, label, icon: Icon }) => (
              <button
                key={id}
                type="button"
                className={cn("persona-tab", panel === id && "persona-tab-active")}
                aria-current={panel === id ? "page" : undefined}
                onClick={() => setPanel(id)}
              >
                <Icon />
                {label}
              </button>
            ))}
          </nav>
          <div className="persona-scroll flex-1 overflow-y-auto px-6 py-5">
            {panel === "overview" ? (
              <PersonaOverview persona={persona} record={record} system={system} />
            ) : (
              <PersonaSection icon={panelDefinition.icon} title={panelDefinition.label}>
                <PersonaCapabilityFields
                  capabilities={panelCapabilities}
                  persona={persona}
                  system={system}
                  disabled={Boolean(running)}
                  onChange={setPersona}
                />
              </PersonaSection>
            )}
          </div>
          <div className="persona-footer">
            <span>{running ? "运行中只读" : editableLabel}</span>
            <div className="flex gap-2">
              <Button
                type="button"
                variant="ghost"
                disabled={submitting || running}
                onClick={() => setPersona({ ...defaultPersona(), seed: persona.seed })}
              >
                恢复原生
              </Button>
              <SheetClose asChild><Button type="button" variant="ghost">取消</Button></SheetClose>
              <Button type="submit" disabled={submitting || running || !dirty}>
                {running ? "请先停止" : submitting ? "保存中…" : "保存"}
              </Button>
            </div>
          </div>
        </form>
      </SheetContent>
    </Sheet>
  );
}

function PersonaCapabilityFields({
  capabilities,
  persona,
  system,
  disabled,
  onChange,
}: {
  capabilities: PersonaCapability[];
  persona: PersonaConfig;
  system: SystemView | null;
  disabled: boolean;
  onChange: (persona: PersonaConfig) => void;
}) {
  const displayCapabilities = capabilities.filter((capability) =>
    displayMetricCapabilityFields.includes(
      capability.field as (typeof displayMetricCapabilityFields)[number],
    )
  );

  return capabilities.map((capability) => {
    if (capability.field === "display.viewport" && displayCapabilities.length > 0) {
      return (
        <div className="persona-linked-fields" key="display-metrics">
          <div className="persona-linked-fields-header">
            <span>显示指标</span>
            <span>{persona.displayMetrics ? "CDP 联动" : "原生联动"}</span>
          </div>
          {displayCapabilities.map((displayCapability) => (
            <PersonaCapabilityField
              key={displayCapability.field}
              capability={displayCapability}
              persona={persona}
              system={system}
              disabled={disabled}
              onChange={onChange}
            />
          ))}
        </div>
      );
    }

    if (displayCapabilities.some((displayCapability) => displayCapability.field === capability.field)) {
      return null;
    }

    return (
      <PersonaCapabilityField
        key={capability.field}
        capability={capability}
        persona={persona}
        system={system}
        disabled={disabled}
        onChange={onChange}
      />
    );
  });
}

function PersonaOverview({
  persona,
  record,
  system,
}: {
  persona: PersonaConfig;
  record: IdentityView | null;
  system: SystemView | null;
}) {
  const runtimeFieldCount = 1
    + (persona.timezone === "system" ? 0 : 1)
    + (persona.displayMetrics ? 3 : 0);
  const observationLabel = record?.personaObservation
    ? record.personaObservation.matches
      ? `${record.personaObservation.fields.length} 项一致`
      : "需要检查"
    : "启动后检查";
  return (
    <>
      <div className="persona-runtime-summary">
        <div><ShieldCheck /><span>内核</span><strong>RealBrowser</strong></div>
        <div><Globe /><span>版本</span><strong>{system?.browserVersion ?? "未检测"}</strong></div>
        <div><Fingerprint /><span>模式</span><strong>{personaModeLabel(record?.personaMode ?? "native")}</strong></div>
      </div>
      <PersonaSection icon={Fingerprint} title="配置">
        <PersonaField label="预设"><BasicValue value={personaMode(persona) === "native" ? "系统原生" : "自定义"} /></PersonaField>
        <PersonaField label="Schema"><BasicValue value={`v${persona.schemaVersion}`} /></PersonaField>
        <PersonaField label="稳定种子"><BasicValue value={persona.seed.toString(16).toUpperCase().padStart(8, "0")} /></PersonaField>
        <PersonaField label="Profile"><BasicValue value="独立目录" /></PersonaField>
        <PersonaField label="网络栈"><BasicValue value="RealBrowser 内核原生" /></PersonaField>
        {runtimeFieldCount > 0 && (
          <PersonaField label="运行观测">
            <BasicValue value={observationLabel} />
          </PersonaField>
        )}
      </PersonaSection>
    </>
  );
}

function PersonaCapabilityField({
  capability,
  persona,
  system,
  disabled,
  onChange,
}: {
  capability: PersonaCapability;
  persona: PersonaConfig;
  system: SystemView | null;
  disabled: boolean;
  onChange: (persona: PersonaConfig) => void;
}) {
  const label = personaFieldLabels[capability.field] ?? capability.field;
  if (capability.field === "region.language") {
    return (
      <PersonaField label={label}>
        <EditableSurface capability={persona.locale === "system" ? asNativeCapability(capability) : capability}>
          <select
            aria-label={label}
            className="persona-select"
            value={persona.locale}
            disabled={disabled}
            onChange={(event) =>
              onChange(withCustomRegion(persona, {
                locale: event.target.value as PersonaConfig["locale"],
              }))
            }
          >
            <option value="system">跟随系统</option>
            <option value="zh_cn">简体中文</option>
            <option value="en_us">English (US)</option>
            <option value="ru_ru">Русский</option>
            <option value="de_de">Deutsch</option>
          </select>
        </EditableSurface>
      </PersonaField>
    );
  }
  if (capability.field === "region.timezone") {
    return (
      <PersonaField label={label}>
        <EditableSurface capability={persona.timezone === "system" ? asNativeCapability(capability) : capability}>
          <TimezoneCombobox
            label={label}
            value={persona.timezone}
            disabled={disabled}
            timezoneIds={system?.timezoneIds ?? []}
            onChange={(timezone) =>
              onChange(withCustomRegion(persona, { timezone }))
            }
          />
        </EditableSurface>
      </PersonaField>
    );
  }
  if (capability.field === "display.window") {
    const resolution = `${persona.windowWidth}x${persona.windowHeight}`;
    return (
      <PersonaField label={label}>
        <EditableSurface capability={capability}>
          <select
            aria-label={label}
            className="persona-select"
            value={resolution}
            disabled={disabled}
            onChange={(event) => {
              const [widthText = "1440", heightText = "900"] = event.target.value.split("x");
              onChange({
                ...persona,
                windowWidth: Number(widthText),
                windowHeight: Number(heightText),
              });
            }}
          >
            {!['1440x900', '1680x1050', '1920x1080', '2560x1440'].includes(resolution) && (
              <option value={resolution}>{resolution}</option>
            )}
            <option value="1440x900">1440 × 900</option>
            <option value="1680x1050">1680 × 1050</option>
            <option value="1920x1080">1920 × 1080</option>
            <option value="2560x1440">2560 × 1440</option>
          </select>
        </EditableSurface>
      </PersonaField>
    );
  }
  if (capability.field === "display.viewport") {
    const metrics = persona.displayMetrics;
    const resolution = metrics
      ? `${metrics.viewportWidth}x${metrics.viewportHeight}`
      : "native";
    return (
      <PersonaField label={label}>
        <EditableSurface capability={metrics ? capability : asNativeCapability(capability)}>
          <select
            aria-label={label}
            className="persona-select"
            value={resolution}
            disabled={disabled}
            onChange={(event) => {
              if (event.target.value === "native") {
                onChange({ ...persona, displayMetrics: null });
                return;
              }
              const [viewportWidth, viewportHeight] = parseResolution(event.target.value);
              const next = metrics ?? defaultDisplayMetrics();
              onChange({
                ...persona,
                windowWidth: Math.max(persona.windowWidth, viewportWidth),
                windowHeight: Math.max(persona.windowHeight, viewportHeight),
                displayMetrics: {
                  ...next,
                  viewportWidth,
                  viewportHeight,
                  screenWidth: Math.max(next.screenWidth, viewportWidth),
                  screenHeight: Math.max(next.screenHeight, viewportHeight),
                },
              });
            }}
          >
            <option value="native">跟随窗口</option>
            {!displayViewportPresets.includes(resolution) && resolution !== "native" && (
              <option value={resolution}>{resolutionLabel(resolution)}</option>
            )}
            {displayViewportPresets.map((preset) => (
              <option key={preset} value={preset}>{resolutionLabel(preset)}</option>
            ))}
          </select>
        </EditableSurface>
      </PersonaField>
    );
  }
  if (capability.field === "display.screen") {
    const metrics = persona.displayMetrics;
    if (!metrics) {
      return (
        <PersonaField label={label}>
          <ReadonlySurface value="系统屏幕" capability={asNativeCapability(capability)} />
        </PersonaField>
      );
    }
    const resolution = `${metrics.screenWidth}x${metrics.screenHeight}`;
    const presets = displayScreenPresets.filter((preset) => {
      const [width, height] = parseResolution(preset);
      return width >= metrics.viewportWidth && height >= metrics.viewportHeight;
    });
    return (
      <PersonaField label={label}>
        <EditableSurface capability={capability}>
          <select
            aria-label={label}
            className="persona-select"
            value={resolution}
            disabled={disabled}
            onChange={(event) => {
              const [screenWidth, screenHeight] = parseResolution(event.target.value);
              onChange({
                ...persona,
                displayMetrics: { ...metrics, screenWidth, screenHeight },
              });
            }}
          >
            {!presets.includes(resolution) && (
              <option value={resolution}>{resolutionLabel(resolution)}</option>
            )}
            {presets.map((preset) => (
              <option key={preset} value={preset}>{resolutionLabel(preset)}</option>
            ))}
          </select>
        </EditableSurface>
      </PersonaField>
    );
  }
  if (capability.field === "display.devicePixelRatio") {
    const metrics = persona.displayMetrics;
    if (!metrics) {
      return (
        <PersonaField label={label}>
          <ReadonlySurface value="系统缩放" capability={asNativeCapability(capability)} />
        </PersonaField>
      );
    }
    return (
      <PersonaField label={label}>
        <EditableSurface capability={capability}>
          <select
            aria-label={label}
            className="persona-select"
            value={metrics.deviceScaleFactorPercent}
            disabled={disabled}
            onChange={(event) => onChange({
              ...persona,
              displayMetrics: {
                ...metrics,
                deviceScaleFactorPercent: Number(event.target.value),
              },
            })}
          >
            {[100, 125, 150, 175, 200, 250, 300].map((percent) => (
              <option key={percent} value={percent}>{formatScalePercent(percent)}</option>
            ))}
          </select>
        </EditableSurface>
      </PersonaField>
    );
  }
  if (capability.field === "privacy.webrtc") {
    return (
      <PersonaField label={label}>
        <EditableSurface capability={persona.webrtc === "native" ? asNativeCapability(capability) : capability}>
          <select
            aria-label={label}
            className="persona-select"
            value={persona.webrtc}
            disabled={disabled}
            onChange={(event) =>
              onChange({ ...persona, webrtc: event.target.value as PersonaConfig["webrtc"] })
            }
          >
            <option value="native">RealBrowser 内核原生</option>
            <option value="disable_non_proxied_udp">禁止直连 UDP</option>
          </select>
        </EditableSurface>
      </PersonaField>
    );
  }
  return (
    <PersonaField label={label}>
      <ReadonlySurface
        value={capability.backend === "custom_kernel" ? "种子微扰" : personaFieldValue(capability.field, system)}
        capability={capability}
      />
    </PersonaField>
  );
}

function TimezoneCombobox({
  label,
  value,
  timezoneIds,
  disabled,
  onChange,
}: {
  label: string;
  value: PersonaConfig["timezone"];
  timezoneIds: string[];
  disabled: boolean;
  onChange: (value: PersonaConfig["timezone"]) => void;
}) {
  const [open, setOpen] = useState(false);
  const options = useMemo(() => {
    const values = value === "system" ? timezoneIds : [value, ...timezoneIds];
    return [...new Set(values)];
  }, [timezoneIds, value]);

  return (
    <Popover open={open} onOpenChange={(nextOpen) => !disabled && setOpen(nextOpen)}>
      <PopoverTrigger asChild>
        <button
          type="button"
          role="combobox"
          aria-label={label}
          aria-expanded={open}
          className="persona-select timezone-combobox"
          disabled={disabled}
        >
          <span>{timezoneLabel(value)}</span>
          <ChevronDown aria-hidden="true" />
        </button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-[var(--radix-popover-trigger-width)] min-w-[320px]"
      >
        <Command label="搜索时区">
          <CommandInput placeholder="搜索时区" aria-label="搜索时区" />
          <CommandList>
            <CommandEmpty>没有匹配时区</CommandEmpty>
            <CommandGroup heading="系统">
              <CommandItem
                value="system"
                keywords={["跟随系统"]}
                onSelect={() => {
                  onChange("system");
                  setOpen(false);
                }}
              >
                <Check className={cn("size-4", value === "system" ? "opacity-100" : "opacity-0")} />
                跟随系统
              </CommandItem>
            </CommandGroup>
            <CommandGroup heading="IANA 时区">
              {options.map((timezone) => (
                <CommandItem
                  key={timezone}
                  value={timezone}
                  keywords={[timezoneLabel(timezone)]}
                  onSelect={() => {
                    onChange(timezone);
                    setOpen(false);
                  }}
                >
                  <Check className={cn("size-4", value === timezone ? "opacity-100" : "opacity-0")} />
                  <span className="min-w-0 truncate">{timezoneLabel(timezone)}</span>
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

function timezoneLabel(value: string) {
  return value === "system" ? "跟随系统" : value.replaceAll("_", " ").replaceAll("/", " / ");
}

function PersonaSection({
  icon: Icon,
  title,
  children,
}: {
  icon: typeof Fingerprint;
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="persona-section">
      <h3><Icon />{title}</h3>
      <div>{children}</div>
    </section>
  );
}

function PersonaField({ label, children }: { label: string; children: React.ReactNode }) {
  return <div className="persona-field"><span>{label}</span>{children}</div>;
}

function BasicValue({ value }: { value: string }) {
  return <span className="persona-basic-value">{value}</span>;
}

function ReadonlySurface({
  value,
  capability,
}: {
  value: string;
  capability: PersonaCapability;
}) {
  return <span className="persona-readonly"><span>{value}</span><CapabilityBadge capability={capability} /></span>;
}

function EditableSurface({
  capability,
  children,
}: {
  capability: PersonaCapability;
  children: React.ReactNode;
}) {
  return <span className="persona-editable">{children}<CapabilityBadge capability={capability} /></span>;
}

function CapabilityBadge({ capability }: { capability: PersonaCapability }) {
  const kind = capabilityKind(capability);
  const labels: Record<CapabilityKind, string> = {
    native: "原生",
    launch: "启动映射",
    profile: "Profile",
    cdp: "CDP",
    kernel: "定制内核",
    unavailable: "未接入",
  };
  const detail = capability.confidence === "not_applied"
    ? `需要 ${backendLabel(capability.backend)}`
    : capability.confidence === "mapped_unverified"
      ? "启动映射 · 未观测"
      : labels[kind];
  return (
    <i
      aria-label={detail}
      className="persona-capability"
      data-kind={kind}
      tabIndex={capability.confidence === "mapped_unverified" ? 0 : undefined}
      title={detail}
    >
      {labels[kind]}
    </i>
  );
}

function capabilityKind(capability: PersonaCapability): CapabilityKind {
  if (capability.confidence === "not_applied") return "unavailable";
  if (capability.backend === "profile") return "profile";
  if (capability.backend === "launch_argument") return "launch";
  if (capability.backend === "cdp") return "cdp";
  if (capability.backend === "custom_kernel") return "kernel";
  return "native";
}

function asNativeCapability(capability: PersonaCapability): PersonaCapability {
  return {
    ...capability,
    backend: "native",
    confidence: "native",
    coverage: ["browser"],
  };
}

function backendLabel(backend: PersonaCapability["backend"]) {
  const labels: Record<PersonaCapability["backend"], string> = {
    native: "原生内核",
    profile: "Profile",
    launch_argument: "启动参数",
    cdp: "CDP",
    extension_limited: "扩展",
    custom_kernel: "定制内核",
    unavailable: "运行时",
  };
  return labels[backend];
}

function personaFieldValue(field: string, system: SystemView | null) {
  const values: Record<string, string> = {
    "region.timezone": "跟随系统",
    "region.geolocation": "站点权限",
    "browser.userAgent": "RealBrowser 内核原生",
    "browser.platform": platformLabel(system?.platform),
    "display.viewport": "随窗口",
    "display.screen": "系统屏幕",
    "display.devicePixelRatio": "系统缩放",
    "hardware.cpu": "系统线程",
    "hardware.deviceMemory": "系统内存",
    "hardware.touch": "系统能力",
    "browser.plugins": "RealBrowser 内核原生",
    "browser.battery": "系统原生",
    "graphics.canvas": "RealBrowser 内核原生",
    "graphics.webglImage": "RealBrowser 内核原生",
    "graphics.webglMetadata": "GPU 原生",
    "graphics.webgpu": "RealBrowser 内核原生",
    "graphics.clientRects": "RealBrowser 内核原生",
    "media.audio": "RealBrowser 内核原生",
    "media.fonts": "系统字体",
    "media.mediaDevices": "站点权限",
    "media.speechVoices": "系统语音",
    "privacy.permissions": "独立 Profile",
    "privacy.doNotTrack": "RealBrowser 内核原生",
    "privacy.globalPrivacyControl": "RealBrowser 内核原生",
  };
  return values[field] ?? "RealBrowser 内核原生";
}

function platformLabel(platform: string | undefined) {
  if (platform === "macos") return "macOS 原生";
  if (platform === "windows") return "Windows 原生";
  if (platform === "linux") return "Linux 原生";
  return "系统原生";
}

function withCustomRegion(
  persona: PersonaConfig,
  patch: Pick<Partial<PersonaConfig>, "locale" | "timezone" | "geolocation">,
): PersonaConfig {
  const next = { ...persona, ...patch };
  return {
    ...next,
    regionPreset: next.locale === "system" && next.timezone === "system" && next.geolocation === null
      ? "native"
      : "custom",
  };
}

function EmptyState({
  filtered,
  archived,
  onCreate,
  onReturn,
  onClear,
}: {
  filtered: boolean;
  archived: boolean;
  onCreate: () => void;
  onReturn: () => void;
  onClear: () => void;
}) {
  const title = filtered
    ? "没有匹配的环境"
    : archived
      ? "暂无归档环境"
      : "创建第一个浏览器环境";
  const action = filtered ? onClear : archived ? onReturn : onCreate;
  return (
    <div className="grid h-full min-h-72 place-items-center px-6 text-center">
      <div className="max-w-sm">
        <div className="mx-auto grid size-12 place-items-center rounded-[15px] bg-primary-soft text-primary">
          {filtered ? <Search className="size-5" /> : archived ? <Archive className="size-5" /> : <AppWindow className="size-5" />}
        </div>
        <h2 className="mt-4 text-base font-bold">{title}</h2>
        <p className="mt-1.5 text-[13px] leading-5 text-muted-foreground">
          {filtered
            ? "换个关键词，或清除筛选。"
            : archived
              ? "归档会保留 Profile。"
              : "创建后保留独立登录状态。"}
        </p>
        <Button className="mt-4" onClick={action}>
          {filtered ? "清除筛选" : archived ? "返回使用中" : "新建环境"}
        </Button>
      </div>
    </div>
  );
}

function LoadingTable() {
  return (
    <div className="p-5" aria-label="正在加载浏览器环境">
      <div className="h-10 animate-pulse rounded-xl bg-muted" />
      {[0, 1, 2, 3].map((row) => (
        <div key={row} className="mt-3 h-14 animate-pulse rounded-xl bg-muted/70" />
      ))}
    </div>
  );
}

function StatusBar({ system, count }: { system: SystemView | null; count: number }) {
  return (
    <footer className="flex h-12 shrink-0 items-center gap-3 border-t border-border text-xs text-muted-foreground">
      <span className="inline-flex items-center gap-2 font-medium text-foreground">
        <CheckCircle2 className="size-4 text-success" /> 服务正常
      </span>
      <span aria-hidden>·</span>
      <span className="inline-flex min-w-0 items-center gap-2">
        <Globe className="size-4 text-primary" />
        {system?.browserVersion ? `RealBrowser ${system.browserVersion}` : "RealBrowser 内核未检测"}
      </span>
      <span aria-hidden>·</span>
      <span className="tabular-nums">{count} 个环境</span>
    </footer>
  );
}

function relativeTime(timestamp: number) {
  const minutes = Math.round((timestamp - Date.now()) / 60_000);
  const formatter = new Intl.RelativeTimeFormat("zh-CN", { numeric: "auto" });
  if (Math.abs(minutes) < 60) return formatter.format(minutes, "minute");
  const hours = Math.round(minutes / 60);
  if (Math.abs(hours) < 24) return formatter.format(hours, "hour");
  return formatter.format(Math.round(hours / 24), "day");
}

function personaModeLabel(mode: IdentityView["personaMode"]) {
  if (mode === "managed") return "已配置";
  return "原生";
}

function personaStatusLabel(record: IdentityView) {
  if (record.personaObservation?.matches) return "已观测";
  return personaModeLabel(record.personaMode);
}

function profileModeLabel(mode: IdentityView["profileMode"]) {
  if (mode === "isolated") return "独立";
  return mode satisfies never;
}

function egressModeLabel(mode: IdentityView["egressMode"]) {
  if (mode === "direct") return "直连";
  if (mode === "proxy") return "代理";
  return mode satisfies never;
}
