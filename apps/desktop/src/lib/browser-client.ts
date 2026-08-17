import { invoke } from "@tauri-apps/api/core";

export type RuntimeState = "stopped" | "starting" | "running" | "stopping" | "failed";
export type TerminationKind = "already_exited" | "graceful" | "forced";
export type LocalePolicy = "system" | "zh_cn" | "en_us" | "ru_ru" | "de_de";
export type RegionPreset = "native" | "custom" | "shanghai" | "new_york" | "berlin" | "moscow";
export type TimezonePolicy = string;
export type WebRtcPolicy = "native" | "disable_non_proxied_udp";
export type NativeSurfacePolicy = "native";
export type NetworkMode = "direct" | "proxy";
export type ProxyProtocol = "http" | "https" | "socks5";
export type PersonaGroup = "region" | "device" | "graphics" | "media" | "privacy";
export type PersonaBackend =
  | "native"
  | "profile"
  | "launch_argument"
  | "cdp"
  | "extension_limited"
  | "custom_kernel"
  | "unavailable";
export type PersonaConfidence = "native" | "mapped_unverified" | "observed" | "not_applied";

export interface PersonaSurfaceConfig {
  browser: {
    userAgent: NativeSurfacePolicy;
    platform: NativeSurfacePolicy;
    plugins: NativeSurfacePolicy;
    battery: NativeSurfacePolicy;
  };
  region: { geolocation: NativeSurfacePolicy };
  hardware: {
    cpu: NativeSurfacePolicy;
    deviceMemory: NativeSurfacePolicy;
    touch: NativeSurfacePolicy;
  };
  graphics: {
    canvas: NativeSurfacePolicy;
    webglImage: NativeSurfacePolicy;
    webglMetadata: NativeSurfacePolicy;
    webgpu: NativeSurfacePolicy;
    clientRects: NativeSurfacePolicy;
  };
  media: {
    audio: NativeSurfacePolicy;
    fonts: NativeSurfacePolicy;
    mediaDevices: NativeSurfacePolicy;
    speechVoices: NativeSurfacePolicy;
  };
  privacy: {
    permissions: NativeSurfacePolicy;
    doNotTrack: NativeSurfacePolicy;
    globalPrivacyControl: NativeSurfacePolicy;
  };
}

export interface DisplayMetrics {
  viewportWidth: number;
  viewportHeight: number;
  screenWidth: number;
  screenHeight: number;
  deviceScaleFactorPercent: number;
}

export interface PersonaCapability {
  field: string;
  group: PersonaGroup;
  backend: PersonaBackend;
  confidence: PersonaConfidence;
  coverage: string[];
  editable: boolean;
}

export interface PersonaConfig {
  schemaVersion: number;
  seed: number;
  regionPreset: RegionPreset;
  locale: LocalePolicy;
  timezone: TimezonePolicy;
  geolocation: {
    latitudeE6: number;
    longitudeE6: number;
    accuracyMeters: number;
  } | null;
  windowWidth: number;
  windowHeight: number;
  displayMetrics: DisplayMetrics | null;
  webrtc: WebRtcPolicy;
  surfaces: PersonaSurfaceConfig;
}

export interface PersonaObservation {
  matches: boolean;
  fields: Array<{
    field: string;
    expected: string;
    observed: string;
    matches: boolean;
  }>;
}

export interface NetworkConfig {
  schemaVersion: number;
  mode: NetworkMode;
  proxy: {
    protocol: ProxyProtocol;
    host: string;
    port: number;
  } | null;
}

export interface IdentityView {
  id: string;
  displayCode: string;
  revision: number;
  name: string;
  startupUrl: string | null;
  profileMode: "isolated";
  runtimeState: RuntimeState;
  personaMode: "native" | "managed";
  persona: PersonaConfig;
  personaObservation: PersonaObservation | null;
  network: NetworkConfig;
  egressMode: NetworkMode;
  browserVersion: string | null;
  createdAtMs: number;
  updatedAtMs: number;
}

export interface SystemView {
  platform: string;
  browserVersion: string | null;
  defaultProfileMode: "isolated";
  defaultPersonaMode: "native";
  defaultEgressMode: "direct";
  personaRuntimeAvailable: boolean;
  personaCapabilities: PersonaCapability[];
  timezoneIds: string[];
}

export interface CreateIdentityInput {
  name: string;
  startupUrl: string | null;
}

export interface StopIdentityResult {
  identity: IdentityView;
  termination: TerminationKind;
}

export interface CommandFailure {
  code: string;
  message: string;
  fieldIssues?: Array<{ field: string; code: string; message: string }>;
}

export interface BrowserClient {
  readonly isPreview: boolean;
  list(): Promise<IdentityView[]>;
  listArchived(): Promise<IdentityView[]>;
  system(): Promise<SystemView>;
  create(input: CreateIdentityInput): Promise<IdentityView>;
  start(id: string): Promise<IdentityView>;
  stop(id: string): Promise<StopIdentityResult>;
  rename(id: string, revision: number, name: string): Promise<IdentityView>;
  updatePersona(id: string, revision: number, persona: PersonaConfig): Promise<IdentityView>;
  updateNetwork(id: string, revision: number, network: NetworkConfig): Promise<IdentityView>;
  archive(id: string, revision: number): Promise<void>;
  restore(id: string, revision: number): Promise<IdentityView>;
}

function isTauriRuntime() {
  return "__TAURI_INTERNALS__" in window;
}

const nativeClient: BrowserClient = {
  isPreview: false,
  list: () => invoke("list_identities"),
  listArchived: () => invoke("list_archived_identities"),
  system: () => invoke("system_status"),
  create: (input) => invoke("create_identity", { input }),
  start: (id) => invoke("start_identity", { id }),
  stop: (id) => invoke("stop_identity", { id }),
  rename: (id, revision, name) => invoke("rename_identity", { id, revision, name }),
  updatePersona: (id, revision, persona) =>
    invoke("update_persona", { id, revision, persona }),
  updateNetwork: (id, revision, network) =>
    invoke("update_network", { id, revision, network }),
  archive: (id, revision) => invoke("archive_identity", { id, revision }),
  restore: (id, revision) => invoke("restore_identity", { id, revision }),
};

let previewRecords: IdentityView[] = [
  previewIdentity("a1000001", "东南亚店铺 · 主账号", "https://seller.example", "running", 1),
  previewIdentity("a1000002", "欧洲站 · 客服", "https://merchant.example", "stopped", 2),
  previewIdentity("a1000003", "北美站 · 售后", null, "stopped", 3),
];
let previewArchivedRecords: IdentityView[] = [];

const previewClient: BrowserClient = {
  isPreview: true,
  async list() {
    return structuredClone(previewRecords);
  },
  async listArchived() {
    return structuredClone(previewArchivedRecords);
  },
  async system() {
    const canvasObserved = previewRecords.some((record) =>
      record.personaObservation?.fields.some((field) => field.field === "graphics.canvas" && field.matches),
    );
    return {
      platform: "macos",
      browserVersion: "151.0.0.0",
      defaultProfileMode: "isolated",
      defaultPersonaMode: "native",
      defaultEgressMode: "direct",
      personaRuntimeAvailable: true,
      personaCapabilities: createPreviewPersonaCapabilities(canvasObserved),
      timezoneIds: previewTimezoneIds,
    };
  },
  async create(input) {
    const record = previewIdentity(
      crypto.randomUUID(),
      input.name.trim(),
      input.startupUrl,
      "stopped",
      previewRecords.length + 1,
    );
    previewRecords = [record, ...previewRecords];
    return structuredClone(record);
  },
  async start(id) {
    const record = previewRecords.find((candidate) => candidate.id === id);
    const timezone = record?.persona.timezone;
    const observed = timezone && timezone !== "system" ? timezoneIanaValue(timezone) : null;
    const metrics = record?.persona.displayMetrics;
    const fields: PersonaObservation["fields"] = [];
    fields.push({
      field: "graphics.canvas",
      expected: "seeded_idempotent_copy_all_contexts_native",
      observed: "seeded_idempotent_copy_all_contexts_native",
      matches: true,
    });
    if (observed) {
      fields.push({
        field: "region.timezone",
        expected: observed,
        observed,
        matches: true,
      });
    }
    if (metrics) {
      const viewport = `${metrics.viewportWidth}x${metrics.viewportHeight}`;
      const screen = `${metrics.screenWidth}x${metrics.screenHeight}`;
      const scale = formatScaleFactor(metrics.deviceScaleFactorPercent);
      fields.push(
        { field: "display.viewport", expected: viewport, observed: viewport, matches: true },
        { field: "display.screen", expected: screen, observed: screen, matches: true },
        { field: "display.devicePixelRatio", expected: scale, observed: scale, matches: true },
      );
    }
    return updatePreview(id, {
      runtimeState: "running",
      personaObservation: { matches: true, fields },
    });
  },
  async stop(id) {
    const identity = await updatePreview(id, {
      runtimeState: "stopped",
      personaObservation: null,
    });
    return { identity, termination: "graceful" };
  },
  async rename(id, revision, name) {
    return updatePreview(id, { name: name.trim(), revision: revision + 1 });
  },
  async updatePersona(id, revision, persona) {
    return updatePreview(id, {
      persona: structuredClone(persona),
      personaMode: personaMode(persona),
      revision: revision + 1,
    });
  },
  async updateNetwork(id, revision, network) {
    return updatePreview(id, {
      network: structuredClone(network),
      egressMode: network.mode,
      revision: revision + 1,
    });
  },
  async archive(id, revision) {
    const record = previewRecords.find((candidate) => candidate.id === id);
    if (!record) {
      return Promise.reject({ code: "not_found", message: "找不到这个浏览器环境" });
    }
    previewRecords = previewRecords.filter((candidate) => candidate.id !== id);
    previewArchivedRecords = [
      { ...record, revision: revision + 1, updatedAtMs: Date.now() },
      ...previewArchivedRecords,
    ];
  },
  async restore(id, revision) {
    const record = previewArchivedRecords.find((candidate) => candidate.id === id);
    if (!record) {
      return Promise.reject({ code: "not_found", message: "找不到这个浏览器环境" });
    }
    const restored = { ...record, revision: revision + 1, updatedAtMs: Date.now() };
    previewArchivedRecords = previewArchivedRecords.filter((candidate) => candidate.id !== id);
    previewRecords = [restored, ...previewRecords];
    return structuredClone(restored);
  },
};

function previewIdentity(
  id: string,
  name: string,
  startupUrl: string | null,
  runtimeState: RuntimeState,
  order: number,
): IdentityView {
  const now = Date.now() - order * 47 * 60_000;
  return {
    id,
    displayCode: `RB-${String(order).padStart(4, "0")}`,
    revision: 1,
    name,
    startupUrl,
    profileMode: "isolated",
    runtimeState,
    personaMode: "native",
    persona: { ...defaultPersona(), seed: order },
    personaObservation: null,
    network: defaultNetwork(),
    egressMode: "direct",
    browserVersion: runtimeState === "running" ? "151.0.0.0" : null,
    createdAtMs: now - 86_400_000,
    updatedAtMs: now,
  };
}

export function defaultPersona(): PersonaConfig {
  return {
    schemaVersion: 5,
    seed: 0,
    regionPreset: "native",
    locale: "system",
    timezone: "system",
    geolocation: null,
    windowWidth: 1440,
    windowHeight: 900,
    displayMetrics: null,
    webrtc: "native",
    surfaces: {
      browser: { userAgent: "native", platform: "native", plugins: "native", battery: "native" },
      region: { geolocation: "native" },
      hardware: { cpu: "native", deviceMemory: "native", touch: "native" },
      graphics: {
        canvas: "native",
        webglImage: "native",
        webglMetadata: "native",
        webgpu: "native",
        clientRects: "native",
      },
      media: {
        audio: "native",
        fonts: "native",
        mediaDevices: "native",
        speechVoices: "native",
      },
      privacy: { permissions: "native", doNotTrack: "native", globalPrivacyControl: "native" },
    },
  };
}

export function defaultNetwork(): NetworkConfig {
  return {
    schemaVersion: 1,
    mode: "direct",
    proxy: null,
  };
}

function createPreviewPersonaCapabilities(canvasObserved: boolean): PersonaCapability[] {
  return [
    capability("region.language", "region", "launch_argument", "mapped_unverified", true),
    capability("region.timezone", "region", "cdp", "observed", true),
    capability("region.geolocation", "region", "cdp", "not_applied"),
    capability("browser.userAgent", "device"),
    capability("browser.platform", "device"),
    capability("display.window", "device", "launch_argument", "mapped_unverified", true),
    capability("display.viewport", "device", "cdp", "observed", true),
    capability("display.screen", "device", "cdp", "observed", true),
    capability("display.devicePixelRatio", "device", "cdp", "observed", true),
    capability("hardware.cpu", "device"),
    capability("hardware.deviceMemory", "device"),
    capability("hardware.touch", "device"),
    capability("browser.plugins", "device"),
    capability("browser.battery", "device"),
    canvasObserved
      ? { field: "graphics.canvas", group: "graphics", backend: "custom_kernel", confidence: "observed", coverage: ["all_frames", "dedicated_worker"], editable: false }
      : capability("graphics.canvas", "graphics"),
    capability("graphics.webglImage", "graphics"),
    capability("graphics.webglMetadata", "graphics"),
    capability("graphics.webgpu", "graphics"),
    capability("graphics.clientRects", "graphics"),
    capability("media.audio", "media"),
    capability("media.fonts", "media"),
    capability("media.mediaDevices", "media"),
    capability("media.speechVoices", "media"),
    capability("privacy.webrtc", "privacy", "launch_argument", "mapped_unverified", true),
    capability("privacy.permissions", "privacy", "profile", "native"),
    capability("privacy.doNotTrack", "privacy"),
    capability("privacy.globalPrivacyControl", "privacy"),
  ];
}

function capability(
  field: string,
  group: PersonaGroup,
  backend: PersonaBackend = "native",
  confidence: PersonaConfidence = "native",
  editable = false,
): PersonaCapability {
  const coverage = backend === "profile"
    ? ["profile"]
    : backend === "launch_argument"
      ? ["process"]
      : backend === "cdp"
        ? ["top_frame"]
        : backend === "native"
          ? ["browser"]
          : [];
  return { field, group, backend, confidence, coverage, editable };
}

export function personaMode(persona: PersonaConfig): IdentityView["personaMode"] {
  const native = { ...defaultPersona(), seed: persona.seed, schemaVersion: persona.schemaVersion };
  return JSON.stringify(persona) === JSON.stringify(native) ? "native" : "managed";
}

function timezoneIanaValue(timezone: Exclude<TimezonePolicy, "system">) {
  return timezone;
}

function formatScaleFactor(percent: number) {
  return (percent / 100).toFixed(2).replace(/0+$/, "").replace(/\.$/, "");
}

const previewTimezoneIds = [
  "America/Chicago",
  "America/Los_Angeles",
  "America/New_York",
  "Asia/Dubai",
  "Asia/Hong_Kong",
  "Asia/Shanghai",
  "Asia/Singapore",
  "Asia/Tokyo",
  "Europe/Berlin",
  "Europe/London",
  "Europe/Moscow",
  "UTC",
];

function updatePreview(id: string, patch: Partial<IdentityView>) {
  const index = previewRecords.findIndex((record) => record.id === id);
  const existing = previewRecords[index];
  if (!existing) {
    return Promise.reject({ code: "not_found", message: "找不到这个浏览器环境" });
  }
  const updated = { ...existing, ...patch, updatedAtMs: Date.now() };
  previewRecords = previewRecords.map((record) => (record.id === id ? updated : record));
  return Promise.resolve(structuredClone(updated));
}

export const browserClient = isTauriRuntime() ? nativeClient : previewClient;

export function commandFailure(error: unknown): CommandFailure {
  if (typeof error === "object" && error !== null && "message" in error) {
    return error as CommandFailure;
  }
  return {
    code: "unknown",
    message: typeof error === "string" ? error : "操作失败，请重试",
  };
}
