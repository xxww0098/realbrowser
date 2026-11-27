import assert from 'node:assert/strict';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import test from 'node:test';
import {
  HOST_DISPLAY_FILE,
  PROFILE_FILE,
  buildFingerprintArgs,
  emulatedMedia,
  readHostDisplay,
  readProfileIdentity,
  resolveFingerprintIdentity,
  validateFingerprintIdentity,
  validateHostDisplay,
  writeHostDisplay,
} from './fingerprint-identity.mjs';

test('validateFingerprintIdentity accepts camelCase IPC and snake_case .browserkernel.json payloads and fails closed on invalid input', () => {
  const camel = validateFingerprintIdentity({
    version: 1,
    seed: 200002,
    platform: 'macos',
    platformVersion: '15.3.1',
    brand: 'Chrome',
    hardwareConcurrency: 10,
    screen: {width: 1512, height: 982, availTop: 37, availBottom: 70, dpr: 2, windowWidth: 1440, windowHeight: 860, windowX: 36, windowY: 45},
  });
  const snake = validateFingerprintIdentity({
    seed: 200002,
    platform: 'macos',
    platform_version: '15.3.1',
    brand: 'Chrome',
    hardware_concurrency: 10,
    screen: {width: 1512, height: 982, avail_top: 37, avail_bottom: 70, dpr: 2, window_width: 1440, window_height: 860, window_x: 36, window_y: 45},
  });
  assert.deepEqual(snake, camel);

  for (const invalid of [
    null,
    {version: 1, seed: -5, platform: 'macos', brand: 'Chrome', hardwareConcurrency: 8, screen: camel.screen},
    {version: 1, seed: 200002, platform: 'macos', brand: 'Chrome', hardwareConcurrency: 8},
    {version: 1, seed: 200002, platform: 'macos', platformVersion: 'bad-ver', brand: 'Chrome', hardwareConcurrency: 8, screen: camel.screen},
    {version: 1, seed: 200002, platform: 'macos', brand: 'Chrome', hardwareConcurrency: 8, screen: {width: 800, height: 600, availTop: 0, availBottom: 0, dpr: 1, windowWidth: 1440, windowHeight: 900}},
    {version: 1, seed: 200002, platform: 'macos', brand: 'Chrome', hardwareConcurrency: 8, screen: {...camel.screen, windowX: 999}},
    {version: 1, seed: 200002, platform: 'macos', brand: 'Chrome', hardwareConcurrency: 8, screen: {width: 1512, height: 982, availTop: 37, availBottom: 70, dpr: 2, windowWidth: 1440, windowHeight: 860, windowX: 20}},
  ]) {
    assert.throws(() => validateFingerprintIdentity(invalid), /指纹配置不完整/);
  }
});

test('readProfileIdentity and resolveFingerprintIdentity require fingerprintkernel-managed identity and never generate random identities', t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'tulai-fp-id-'));
  t.after(() => fs.rmSync(root, {recursive: true, force: true}));

  const emptyProfile = path.join(root, 'empty-account');
  fs.mkdirSync(emptyProfile, {recursive: true});
  assert.throws(() => readProfileIdentity(emptyProfile), /缺少指纹内核生成的配置文件/);
  assert.throws(() => resolveFingerprintIdentity(emptyProfile, undefined), /缺少指纹内核生成的配置文件/);

  const profile = path.join(root, 'seeded-account');
  fs.mkdirSync(profile, {recursive: true});
  fs.writeFileSync(path.join(profile, PROFILE_FILE), JSON.stringify({
    seed: 200002,
    platform: 'macos',
    platform_version: '15.3.1',
    brand: 'Chrome',
    hardware_concurrency: 10,
    screen: {width: 1512, height: 982, avail_top: 37, avail_bottom: 70, dpr: 2, window_width: 1440, window_height: 860},
  }));
  const fromFile = resolveFingerprintIdentity(profile, undefined);
  assert.equal(fromFile.seed, 200002);
  assert.equal(fromFile.platformVersion, '15.3.1');

  const supplied = {
    version: 1,
    seed: 400005,
    platform: 'windows',
    platformVersion: '15.0.0',
    brand: 'Chrome',
    hardwareConcurrency: 12,
    screen: {width: 1920, height: 1080, availTop: 0, availBottom: 48, dpr: 1, windowWidth: 1440, windowHeight: 900, windowX: 40, windowY: 20},
  };
  assert.deepEqual(resolveFingerprintIdentity(emptyProfile, supplied), supplied);

  const corruptProfile = path.join(root, 'corrupt-account');
  fs.mkdirSync(corruptProfile, {recursive: true});
  fs.writeFileSync(path.join(corruptProfile, PROFILE_FILE), '{broken-json');
  assert.throws(() => readProfileIdentity(corruptProfile), /指纹文件无法读取/);
});

test('buildFingerprintArgs emits desktop window-size, headless screen-info, platformVersion, and proxy WebRTC protection', () => {
  const id = validateFingerprintIdentity({
    version: 1,
    seed: 123457,
    platform: 'macos',
    platformVersion: '15.3.1',
    brand: 'Chrome',
    hardwareConcurrency: 10,
    screen: {width: 1512, height: 982, availTop: 37, availBottom: 70, dpr: 2, windowWidth: 1440, windowHeight: 860, windowX: 36, windowY: 45},
  });

  // Chromium shows its "unsupported command-line flag" bar on the first tab of
  // a headed window that carries any blink-feature switch, and that tab loses
  // 52px of viewport height to it. The kernel already forces navigator.webdriver
  // false, so no such switch belongs on the command line.
  for (const options of [{headless: false}, {headless: true}]) {
    const args = buildFingerprintArgs(id, options);
    assert.deepEqual(args.filter(a => /^--(enable|disable)-blink-features/.test(a)), [], `blink-feature switches in ${args}`);
  }
  assert.deepEqual(
    buildFingerprintArgs(id, {headless: true, directNetwork: true, timezone: 'Asia/Shanghai', lang: 'zh-CN'}),
    [
      '--disable-spoofing=audio,clientrects',
      '--fingerprint=123457',
      '--fingerprint-platform=macos',
      '--fingerprint-platform-version=15.3.1',
      '--fingerprint-brand=Chrome',
      '--fingerprint-hardware-concurrency=10',
      '--window-size=1440,860',
      '--window-position=36,45',
      '--screen-info={3024x1964 workAreaTop=74 workAreaBottom=140 devicePixelRatio=2}',
      '--timezone=Asia/Shanghai',
      '--lang=zh-CN',
      '--no-proxy-server',
    ],
  );
  assert.deepEqual(
    buildFingerprintArgs(id, {headless: false, proxy: 'http://127.0.0.1:7890'}),
    [
      '--disable-spoofing=audio,clientrects',
      '--fingerprint=123457',
      '--fingerprint-platform=macos',
      '--fingerprint-platform-version=15.3.1',
      '--fingerprint-brand=Chrome',
      '--fingerprint-hardware-concurrency=10',
      '--window-size=1440,860',
      '--webrtc-ip-handling-policy=disable_non_proxied_udp',
    ],
  );
  assert.deepEqual(
    buildFingerprintArgs(id, {headless: false, proxy: '   '}),
    [
      '--disable-spoofing=audio,clientrects',
      '--fingerprint=123457',
      '--fingerprint-platform=macos',
      '--fingerprint-platform-version=15.3.1',
      '--fingerprint-brand=Chrome',
      '--fingerprint-hardware-concurrency=10',
      '--window-size=1440,860',
    ],
  );
});

test('host display records round-trip next to the account profiles and reject unusable values', t => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'tulai-fp-host-'));
  t.after(() => fs.rmSync(root, {recursive: true, force: true}));
  const profile = path.join(root, 'Fingerprint', 'account-1');
  fs.mkdirSync(profile, {recursive: true});

  assert.equal(readHostDisplay(profile), null, 'no record yet');
  const display = {
    width: 1728, height: 1117, availLeft: 0, availTop: 37, availRight: 0, availBottom: 70,
    dpr: 2, colorDepth: 30, p3: true, hdr: false, extended: false,
  };
  assert.equal(writeHostDisplay(profile, display), true);
  assert.equal(fs.existsSync(path.join(root, 'Fingerprint', HOST_DISPLAY_FILE)), true);
  assert.deepEqual(readHostDisplay(path.join(root, 'Fingerprint', 'account-2')), display, 'sibling accounts share the record');

  for (const broken of [
    null,
    {...display, width: 100},
    {...display, availTop: display.height},
    {...display, dpr: 0},
    {...display, dpr: '2'},
    {...display, colorDepth: 0},
    {...display, colorDepth: 24.5},
    {...display, availLeft: -1},
  ]) {
    assert.equal(validateHostDisplay(broken), null, `accepted unusable display: ${JSON.stringify(broken)}`);
  }
  assert.deepEqual(validateHostDisplay({...display, p3: 'yes', extended: 1}), {...display, p3: false, extended: false});
  assert.equal(writeHostDisplay(profile, {...display, width: 0}), false, 'an unusable display is never written');

  fs.writeFileSync(path.join(root, 'Fingerprint', HOST_DISPLAY_FILE), '{broken');
  assert.equal(readHostDisplay(profile), null, 'a damaged record falls back to the account preset');
});

test('buildFingerprintArgs replays the recorded host display and keeps the window on its work area', () => {
  const id = validateFingerprintIdentity({
    version: 1,
    seed: 123458,
    platform: 'macos',
    platformVersion: '15.3.1',
    brand: 'Chrome',
    hardwareConcurrency: 10,
    screen: {width: 2560, height: 1440, availTop: 25, availBottom: 70, dpr: 2, windowWidth: 1680, windowHeight: 1020, windowX: 700, windowY: 300},
  });
  const display = {
    width: 1440, height: 900, availLeft: 0, availTop: 25, availRight: 0, availBottom: 60,
    dpr: 2, colorDepth: 30, p3: true, hdr: true, extended: true,
  };
  // The host display is smaller than the account preset: the window shrinks to fit.
  const headless = buildFingerprintArgs(id, {headless: true, display});
  assert.ok(headless.includes('--window-size=1440,815'), `window not clamped to the work area: ${headless}`);
  assert.ok(headless.includes('--window-position=0,25'), `window position not clamped: ${headless}`);
  assert.ok(headless.includes('--screen-info={2880x1800 workAreaTop=50 workAreaBottom=120 devicePixelRatio=2 colorDepth=30}{1920x1080 isInternal=false}'), `missing host display: ${headless}`);
  assert.ok(headless.includes('--force-color-profile=hdr10'), `missing HDR profile: ${headless}`);

  const headed = buildFingerprintArgs(id, {headless: false, display});
  assert.ok(headed.includes('--window-size=1440,815'), `headed window not clamped: ${headed}`);
  assert.ok(headed.includes('--window-position=0,25'), `headed window position not clamped: ${headed}`);
  assert.equal(headed.some(a => a.startsWith('--screen-info')), false);
  assert.equal(headed.some(a => a.startsWith('--force-color-profile')), false);

  // Without a record the account preset is replayed with the defaults of a plain panel.
  const preset = buildFingerprintArgs(id, {headless: true});
  assert.ok(preset.includes('--window-size=1680,1020'), `preset window: ${preset}`);
  assert.ok(preset.includes('--window-position=700,300'), `preset position: ${preset}`);
  assert.ok(preset.includes('--screen-info={5120x2880 workAreaTop=50 workAreaBottom=140 devicePixelRatio=2}'), `preset display: ${preset}`);
  assert.equal(preset.some(a => a.startsWith('--force-color-profile')), false);

  // An unusable record is ignored rather than trusted.
  assert.deepEqual(buildFingerprintArgs(id, {headless: true, display: {width: -1}}), preset);
  // Only an HDR display gets a forced profile: the P3 one recolors screenshots.
  const p3Only = buildFingerprintArgs(id, {headless: true, display: {...display, hdr: false}}).filter(a => !a.includes('--screen-info'));
  const plain = buildFingerprintArgs(id, {headless: true, display: {...display, hdr: false, p3: false}}).filter(a => !a.includes('--screen-info'));
  assert.equal(p3Only.some(a => a.startsWith('--force-color-profile')), false);
  assert.deepEqual(p3Only, plain);
});

test('emulatedMedia answers the P3 query for a wide-gamut host that is not HDR, in headless pages only', () => {
  const hdr = {width: 1512, height: 982, availLeft: 0, availTop: 37, availRight: 0, availBottom: 70, dpr: 2, colorDepth: 30, p3: true, hdr: true};
  const p3 = {...hdr, hdr: false};
  const srgb = {...hdr, p3: false};
  // An HDR host is covered by its hdr10 profile; a plain display needs nothing.
  assert.deepEqual(emulatedMedia(p3, true), [{name: 'color-gamut', value: 'p3'}]);
  assert.deepEqual(emulatedMedia(hdr, true), []);
  assert.deepEqual(emulatedMedia(srgb, true), []);
  // A headed window shows the real display, and an unusable record is no record.
  assert.deepEqual(emulatedMedia(p3, false), []);
  assert.deepEqual(emulatedMedia(null, true), []);
  assert.deepEqual(emulatedMedia({p3: true}, true), []);
});
