# Define the Browser Identity Lifecycle

Type: grilling
Status: open

## Question

What are the authoritative states and allowed transitions for creating, starting, stopping, renaming, archiving, restoring, and recoverably deleting a Browser Identity? Decide ownership and failure behavior for duplicate launches, stale locks, abrupt Rust-process exit, Chrome crash, machine reboot, partial creation, and deletion while running. Define Identity Templates and configuration duplication separately from any session-bearing Profile copy, and decide whether encrypted backup or cross-machine migration belongs to a later product tier.
