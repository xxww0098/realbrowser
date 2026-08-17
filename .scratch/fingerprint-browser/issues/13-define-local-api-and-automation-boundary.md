# Define the Local API and Automation Boundary

Type: grilling
Status: open
Blocked by: 06, 09, 12

## Question

Which standard local API and automation capabilities belong to the product: Browser Identity CRUD, import/export, start/stop, window operations, navigation, Selenium/Playwright integration, and a brokered CDP subset? Decide transport, caller authentication, origin and process restrictions, per-Identity capability grants, concurrency semantics, operator confirmation, audit events, versioning, error behavior, and MCP adaptation. Never expose raw CDP endpoints, website passwords, 2FA secrets, or unrestricted Cookie access; arbitrary page JavaScript requires an explicit high-risk decision rather than arriving implicitly through an automation framework.
