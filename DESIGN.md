---
name: RealBrowser
description: A quiet, dense desktop console for isolated browser environments.
colors:
  primary: "#2855ff"
  primary-soft: "#edf1ff"
  canvas: "#ffffff"
  ink: "#11182c"
  quiet-surface: "#f7f7f5"
  quiet-text: "#6e768c"
  hairline: "#e8e8e4"
  strong-line: "#d5d5cf"
  success: "#20bd62"
  warning-soft: "#fff6dc"
  destructive: "#dc3b4b"
typography:
  headline:
    fontFamily: "-apple-system, BlinkMacSystemFont, Segoe UI, PingFang SC, Microsoft YaHei, sans-serif"
    fontSize: "28px"
    fontWeight: 750
    lineHeight: 1.2
    letterSpacing: "-0.035em"
  title:
    fontFamily: "-apple-system, BlinkMacSystemFont, Segoe UI, PingFang SC, Microsoft YaHei, sans-serif"
    fontSize: "20px"
    fontWeight: 700
    lineHeight: 1.25
    letterSpacing: "-0.025em"
  body:
    fontFamily: "-apple-system, BlinkMacSystemFont, Segoe UI, PingFang SC, Microsoft YaHei, sans-serif"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "-apple-system, BlinkMacSystemFont, Segoe UI, PingFang SC, Microsoft YaHei, sans-serif"
    fontSize: "12px"
    fontWeight: 600
    lineHeight: 1.25
rounded:
  control-sm: "11px"
  control: "14px"
  field: "15px"
  surface: "19px"
spacing:
  xs: "8px"
  sm: "12px"
  md: "16px"
  lg: "24px"
  xl: "28px"
components:
  button-primary:
    backgroundColor: "{colors.primary}"
    textColor: "{colors.canvas}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "0 16px"
    height: "40px"
  button-outline:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.label}"
    rounded: "{rounded.control}"
    padding: "0 16px"
    height: "40px"
  input:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    typography: "{typography.body}"
    rounded: "{rounded.control}"
    padding: "0 14px"
    height: "40px"
  table-container:
    backgroundColor: "{colors.canvas}"
    textColor: "{colors.ink}"
    rounded: "{rounded.surface}"
  capability-badge:
    backgroundColor: "{colors.quiet-surface}"
    textColor: "{colors.quiet-text}"
    typography: "{typography.label}"
    rounded: "999px"
    padding: "3px 7px"
---

# Design System: RealBrowser

## Overview

**Creative North Star: "The Quiet Operator"**

RealBrowser feels like a calm commercial operations console: dense enough for repeated store work, quiet enough that environment state and the next valid action remain obvious. The visual system borrows the familiar rhythm of browser-profile managers while keeping its own restrained, local-first character.

The main workspace is continuous and table-led. Warm hairlines, white surfaces, deep ink, and a single royal-blue action color create structure without hard boxes or decorative copy. Context appears in dialogs and temporary drawers, never in a persistent second content pane.

**Key Characteristics:**

- One compact icon rail and one continuous work surface.
- Soft corners with fine, warm-gray separators.
- Royal blue reserved for selection and primary actions.
- Short Chinese labels, operational states, and no marketing prose in workflows.

## Colors

The palette is predominantly white and warm-neutral, with one vivid operational blue and small semantic accents.

### Primary

- **Operator Blue:** Used for the active destination, primary actions, focus treatment, and interactive Persona values.
- **Blue Wash:** Used behind subtle selected or mapped states.

### Neutral

- **Clear Canvas:** The application and raised-surface background.
- **Deep Ink:** Primary headings, names, and critical values.
- **Warm Quiet Surface:** Navigation rail, table header, subtle controls, and passive badges.
- **Quiet Text:** Secondary metadata and native values.
- **Warm Hairline / Strong Line:** Low-contrast separators and the stronger tier for inputs and major controls.

### Named Rules

**The One Accent Rule.** Operator Blue is the only chromatic interaction accent; semantic green, amber, and red remain status-only.

## Typography

**Display Font:** System UI sans-serif stack
**Body Font:** System UI sans-serif stack

**Character:** Compact, neutral, and native to the host operating system. Weight and spacing establish hierarchy; decorative typography is absent.

### Hierarchy

- **Headline** (750, 32px, 1.2): Reserved only when explicitly required; workspace/detail pages omit top-level headings and lead directly into operational controls.
- **Title** (700, 24px, 1.25): Dialog and drawer titles.
- **Body** (400, 14px, 1.5): Metadata and field values.
- **Label** (600, 13px, 1.25): Buttons, table headers, badges, and field labels.

### Named Rules

**The Short Label Rule.** Operational controls use the shortest unambiguous Chinese label; helper prose appears only when an error or empty state needs it.

## Layout

The desktop shell uses an 88px icon rail and one fluid content column. Workspace pages omit top-level titles, allowing single-line top controls (scope, search, status, and actions) to lead directly before a table fills the remaining viewport. At the supported 960px minimum width, the table scrolls horizontally instead of collapsing columns into ambiguous cards. Creation and renaming use centered dialogs; Persona settings use a temporary right drawer no wider than 560px.

Spacing follows an 8px-based rhythm, with 24–28px reserved for workspace and drawer padding. Table rows remain airy enough for two lines of identity metadata while preserving scanning density.

## Elevation & Depth

The system is flat by default. Fine borders and slight tonal changes provide most separation; soft shadows appear only on primary actions, floating popovers, dialogs, and the temporary drawer.

### Shadow Vocabulary

- **Primary lift** (`0 8px 18px -9px rgba(40,85,255,.78)`): Blue primary actions.
- **Surface ambient** (`0 14px 42px -36px rgba(42,42,38,.34)`): Major table container.
- **Drawer separation** (`-24px 0 70px -38px rgba(42,42,38,.34)`): Temporary right drawer only.

### Named Rules

**The Earned Elevation Rule.** A shadow means an element is floating or immediately actionable; ordinary containers stay border-defined.

## Shapes

Controls use softened 11–15px corners, while major containers use 16–19px corners. Pills are reserved for compact status or capability badges. Borders are one pixel and warm gray; sharp rectangles and oversized capsule containers do not belong in this system.

## Components

### Buttons

- **Shape:** Soft control corners (11–14px) with 32–44px height by density.
- **Primary:** Operator Blue with white text and a restrained downward shadow.
- **Hover / Focus:** Small tonal change and a translucent blue focus ring; active state moves down one pixel.
- **Outline / Ghost:** White or transparent with the same corner language and no resting shadow.

### Chips

- **Style:** Compact rounded capability badges with a warm-neutral fill and hairline border.
- **State:** Launch-mapped fields use Blue Wash and Operator Blue; native and Profile-owned fields remain neutral.

### Cards / Containers

- **Corner Style:** Generous soft surface corners (16–19px).
- **Background:** Clear Canvas or Warm Quiet Surface.
- **Shadow Strategy:** Flat at rest; see Earned Elevation Rule.
- **Border:** Warm Hairline, with Strong Line only for structural controls.
- **Internal Padding:** 16–28px depending on density.

### Inputs / Fields

- **Style:** White fill, Strong Line, 14–15px corners, and compact system type.
- **Focus:** Operator Blue border plus a low-opacity three-pixel ring.
- **Error / Disabled:** Semantic color is paired with text; disabled controls retain visible structure.

### Navigation

The narrow icon rail uses quiet gray icons on a warm-neutral surface. The active destination is a rounded Operator Blue square with a soft shadow. Unavailable destinations remain visible but disabled so the product map does not shift.

### Persona Capability Row

Each row pairs a short field name with either a real control or a read-only value and capability badge. “启动映射” means the Rust execution plan emits a launch argument; “原生” and “Profile” never imply fingerprint spoofing.

## Do's and Don'ts

### Do:

- **Do** keep environment management table-first and action-forward.
- **Do** use dialogs and temporary drawers for editing without changing the workspace composition.
- **Do** show backend capability beside every Persona field.
- **Do** keep borders fine, warm, and consistent across nested surfaces.

### Don't:

- **Don't** introduce a persistent left-right content split.
- **Don't** add verbose onboarding or promotional paragraphs to operational screens.
- **Don't** expose controls for fingerprint behavior the runtime cannot execute and observe.
- **Don't** use sharp corners, heavy outlines, gradients, or indiscriminate pill containers.
