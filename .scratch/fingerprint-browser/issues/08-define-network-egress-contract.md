# Define the Network Egress Fail-Closed Contract

Type: grilling
Status: open
Blocked by: 04, 05

## Question

What exact guarantees must hold when a Browser Identity uses the proxy library or is bound to a user-supplied HTTP(S), SOCKS5, SSH-tunnel, static, or provider-backed dynamic route? Decide import, validation, authentication, geographic metadata, pre-launch verification, DNS/WebRTC/QUIC treatment, permitted rotation semantics, behavior when the route fails before or during a session, direct-connection identities, egress evidence shown to the operator, and whether any condition may ever fall back to another route.
