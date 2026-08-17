# Fingerprint Browser

A general-purpose desktop product for managing authorized website accounts through separated, persistent browser identities, configurable coherent personas, and controlled network egress. E-commerce is a primary use case, not a platform boundary or an anti-detection guarantee.

## Language

**Fingerprint Browser**:
A product that manages multiple persistent Browser Identities with configurable Browser Personas and Network Egress.
_Avoid_: Ozon browser, undetectable browser

**Store Account**:
An e-commerce platform account owned by the operator or explicitly entrusted to them.
_Avoid_: Browser account, farmed account

**Seller Platform**:
An e-commerce marketplace or merchant administration service on which Store Accounts are operated.
_Avoid_: Target website, Ozon-specific core

**Platform Acceptance Journey**:
A real seller workflow used to prove compatibility with one Seller Platform without making that platform part of the product core.
_Avoid_: Product scope, generic smoke test

**Browser Identity**:
The persistent operator-facing unit that binds one Browser Profile, one Browser Persona, and one Network Egress.
_Avoid_: Account, instance, fingerprint

**Browser Profile**:
An isolated container for one Browser Identity's browsing state, including cookies, site storage, and preferences.
_Avoid_: Account, window, tab

**Identity Template**:
A reusable, non-secret configuration for creating Browser Identities without copying live website sessions or credentials.
_Avoid_: Profile clone, Cookie copy

**Browser Persona**:
The stable and internally coherent set of browser characteristics observable by websites for one Browser Identity.
_Avoid_: Random fingerprint, noise profile

**Persona Runtime**:
The versioned browser-side implementation required to apply managed Canvas, WebGL, WebGPU, Audio, Navigator, font, media-device, and hardware surfaces consistently across supported execution contexts.
_Avoid_: Fingerprint plugin, stealth patch

**Persona Capability**:
The explicit runtime status of one Persona field: native in the RealBrowser kernel, mapped by the launch plan/CDP runtime, or observed through `CustomKernel`. Unsupported or unobserved fields must never silently fall back or be labelled applied.
_Avoid_: Enabled toggle, best-effort spoof

**Network Egress**:
The declared network route associated with one Browser Identity, either direct or through a specific proxy.
_Avoid_: IP identity, rotating IP

**Profile Isolation**:
The product property that browsing state, secrets, persona, and network configuration do not leak between Browser Identities.
_Avoid_: Incognito mode, tab separation
