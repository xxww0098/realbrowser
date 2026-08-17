# Define the Local Data and Secret Lifecycle

Type: grilling
Status: open
Blocked by: 05, 06

## Question

Which product metadata, proxy credentials, browser state, imported/exported Cookies, audit events, deletion tombstones, and recovery material exist locally; who owns each item; and how are they encrypted, backed up, restored, expired, and destroyed? Decide an explicit per-Profile Cookie portability flow with validation, preview, audit, temporary-data cleanup, and no cloud upload by default. The application must not become a website password or 2FA vault, logs must exclude page content and secrets, and recovery must never restore session material into the wrong Browser Identity.
