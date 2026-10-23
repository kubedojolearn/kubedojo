# KubeDojo documentation

| Area | Document | What it answers |
| --- | --- | --- |
| Plan | [master-plan.md](master-plan.md) | The V1 implementation plan, split into 20 parts |
| Plan | [plan-review.md](plan-review.md) | Gaps found in the plan and where Part 01 resolves them |
| Product | [product/PRD.md](product/PRD.md) | Who KubeDojo is for, what V1 does, how success is measured |
| Product | [product/course-outline.md](product/course-outline.md) | The 30 V1 lessons, their slugs and how each one is graded |
| Architecture | [architecture/overview.md](architecture/overview.md) | Components, request flows, lab lifecycle, data ownership |
| Architecture | [architecture/simulator-compatibility.md](architecture/simulator-compatibility.md) | Exactly which kubectl commands, resources and behaviors are simulated |
| Architecture | [architecture/requirements.md](architecture/requirements.md) | Accessibility, performance, security, testing, privacy and retention requirements; launch criteria |
| Architecture | [architecture/data-model.md](architecture/data-model.md) | ER diagram, invariants the database enforces, write paths |
| Architecture | [architecture/risk-register.md](architecture/risk-register.md) | Delivery and product risks with owners and mitigations |
| Architecture | [architecture/review-checklist.md](architecture/review-checklist.md) | Unresolved tradeoffs and the checklist every part's PR is reviewed against |
| Decisions | [adr/](adr/README.md) | Architecture decision records |
| Security | [security/threat-model.md](security/threat-model.md) | Threats, controls and the parts that implement them |
| API | [api/openapi.yaml](api/openapi.yaml) | OpenAPI 3.1 contract for the Go API |
| API | [api/conventions.md](api/conventions.md) | Request lifecycle, authentication, errors, pagination, rate limits, network restriction |
| API | [../packages/api-client/](../packages/api-client/src/index.ts) | Typed TypeScript client generated from the OpenAPI contract |
| API | [api/websocket.md](api/websocket.md) | WebSocket protocol v1 (schema: [api/ws-protocol.schema.json](api/ws-protocol.schema.json)) |
| Design | [design/screens.md](design/screens.md) | Every V1 screen: layout, states and the part that builds it |
| Design | [design/tokens.md](design/tokens.md), [brand.md](design/brand.md) | Colors, type, spacing, terminal and graph styles; logo and voice |
| Design | [design/baseline.md](design/baseline.md) | Lighthouse and accessibility baseline |
| Operations | [operations/keys.md](operations/keys.md) | Secrets, key generation and rotation |
| Operations | [operations/auth.md](operations/auth.md) | Sign-in flow, BFF rules, GitHub OAuth setup, test sign-in, secret rotation |
| Operations | [operations/database.md](operations/database.md) | Migrations, seeds, initial admin, retention and account deletion |
| Content | [../packages/content-schema/](../packages/content-schema/README.md) | JSON Schemas for courses, challenges and fixtures |

## Where truth lives (one-paragraph version)

PostgreSQL is the system of record for everything: users, progress, lab state, XP. Redis holds only short-lived coordination data (WebSocket tickets, rate-limit counters, pub/sub notifications) and can be flushed without losing anything a learner cares about. Lesson content lives in Git under `content/` and is imported into PostgreSQL as immutable versions. The browser holds nothing authoritative: it renders what the API returns and refetches a snapshot whenever it is unsure.
