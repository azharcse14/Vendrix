# Vendrix

**One commerce engine. Multiple business models. Unlimited possibilities.**

Vendrix is a **modular, extensible e-commerce platform in development**, designed to support **single-vendor stores, multi-vendor marketplaces, and multi-tenant commerce** through a unified architecture.

**Project Status:** 🚧 Under Active Development. Current milestone, next steps and open questions: [`docs/STATUS.md`](docs/STATUS.md).

## Overview

Vendrix aims to provide a flexible commerce foundation that can be configured for standalone online stores, marketplaces with multiple sellers, or SaaS platforms hosting independent businesses.

## Commerce Models

| Model | Description |
|---|---|
| **Single Vendor** | One business manages its products, customers, inventory, and orders. |
| **Multi Vendor** | Multiple sellers operate within a unified marketplace. |
| **Multi Tenant** | Independent businesses operate isolated stores with their own configuration and data. |

A tenant can operate either a single-vendor store or its own multi-vendor marketplace.

## Planned Features

- **Multi-Tenancy:** Tenant isolation, store configuration, and custom domains.
- **Vendor Management:** Vendor onboarding, dashboards, commissions, and payouts.
- **Product Management:** Categories, variants, attributes, images, and inventory.
- **Order Management:** Shopping carts, checkout, fulfillment, and order tracking.
- **Payment Integration:** Extensible payment gateway architecture.
- **User Management:** Authentication and role-based access control.
- **Storefront Customization:** Branding and store-specific configurations.
- **Promotions:** Coupons, discounts, and marketing campaigns.
- **Analytics:** Sales reporting and operational dashboards.
- **API-First Architecture:** Modular, maintainable services for future integrations.

## Architecture Principles

1. **Modular Design** — Separate business domains with clear boundaries.
2. **Tenant Isolation** — Securely separate tenant-specific data and operations.
3. **Flexible Commerce Models** — Support both independent stores and marketplaces.
4. **Scalability** — Design for growth without unnecessary infrastructure complexity.
5. **Security** — Enforce authentication, authorization, and data validation.

## Core Modules

- Identity and Authentication
- Tenant Management
- Vendor Management
- Product Catalog
- Inventory Management
- Cart and Checkout
- Order Management
- Payments and Billing
- Shipping and Fulfillment
- Reporting and Analytics

## Tech Stack

| Part | Technology |
|---|---|
| Backend API | Go 1.27 (`net/http` + standard library, `pgx`) |
| Database | PostgreSQL 16 (`C.UTF-8`, `pg_trgm`) |
| Customer website | Jaspr (Dart, server-side rendering) + Tailwind CSS |
| Admin panel | Flutter web |
| Customer app | Flutter (Phase 2) |
| Payments | Cash on Delivery, SSLCommerz (bKash/Nagad in Phase 2) |
| Infrastructure | Caddy, systemd, Cloudflare, Cloudflare R2 |

Full system design and build plan: [`docs/design/`](docs/design/README.md).

## Repository Structure

```
backend/          Go API: cmd/api (binary shop-api), internal/*, migrations/
packages/shared/  Shared pure-Dart package (milestone M2)
website/          Jaspr storefront (M3)
admin/            Flutter web admin (M7)
app/              Flutter customer app (Phase 2)
infra/            Local PostgreSQL (Docker Compose), Caddy, systemd
docs/design/      System design and build plan (16 sections)
docs/features/    How each feature works today: code map, rules and their tests
docs/milestones/  Milestone plans and progress
docs/STATUS.md    Current state, next steps, open questions, session log
scripts/          Checks, local run, Claude Code hooks
.claude/          Claude Code settings, rules, skills, agents
.github/          CI workflow and pull request template
```

## Development Roadmap

- [ ] Project foundation and authentication
- [ ] Multi-tenant architecture and data isolation
- [ ] Product catalog and inventory
- [ ] Cart and checkout
- [ ] Single-vendor workflows
- [ ] Multi-vendor marketplace functionality
- [ ] Tenant-specific storefront configuration
- [ ] Payment and shipping integrations
- [ ] Analytics and reporting
- [ ] Testing, documentation, and deployment

Detailed milestones (M0–M12, marketplace, multi-tenant) with acceptance criteria: [`docs/design/15-milestones.md`](docs/design/15-milestones.md).

## Getting Started

Prerequisites: Go 1.27.3, Docker (local PostgreSQL), `make`, `bash`, `git`; for `make db-test` also the PostgreSQL client tools (`psql`, `createdb`, `dropdb`).

```bash
make db-up                            # PostgreSQL 16 with the shop_owner/shop_app roles and the shop database
cp backend/env.example backend/.env   # local settings (never commit .env)
make migrate                          # apply database migrations
make run                              # API on http://127.0.0.1:8080 (GET /healthz)
make quick-check                      # format, vet, tests, doc checks, rule checks
make check                            # plus race tests and database constraint tests
```

The PostgreSQL driver (pgx) is added in milestone M0, step 1; until then `make migrate` and `make run` stop with a message pointing to [`docs/milestones/M0.md`](docs/milestones/M0.md).

## Working with Claude Code

The repository is set up so [Claude Code](https://docs.claude.com/en/docs/claude-code/overview) can carry the project forward without repeated instructions:

1. Open a terminal in the repository root and start `claude`.
2. Say **"next"** (or "shuru koro"). Claude reads [`docs/STATUS.md`](docs/STATUS.md), continues the current milestone, and asks only the open questions listed there.
3. For each milestone Claude writes a plan in `docs/milestones/`, waits for your approval, writes tests first, implements, updates the feature docs and `docs/STATUS.md`, and commits. You push, merge and deploy.

Other commands: `/milestone M1`, `/feature-doc checkout`, `/fix` with a bug description.

Guardrails: hooks show the status at session start, block edits to protected files (deployed migrations, hook settings, secrets), format Go files, and stop Claude from finishing while checks fail or `docs/STATUS.md` is out of date. Details: [`CLAUDE.md`](CLAUDE.md), [`AGENTS.md`](AGENTS.md).

## Documentation

| What | Where |
|---|---|
| Design and build plan | [`docs/design/README.md`](docs/design/README.md) |
| How each feature works | [`docs/features/README.md`](docs/features/README.md) |
| Current status and next steps | [`docs/STATUS.md`](docs/STATUS.md) |
| Milestone plans | [`docs/milestones/`](docs/milestones/README.md) |
| Rules for AI agents | [`AGENTS.md`](AGENTS.md), [`CLAUDE.md`](CLAUDE.md) |

Design and feature docs are written in Banglish (Bengali in Latin script); code identifiers are in English.

## Contributing

Contributions, suggestions, and feature discussions are welcome. Please open a GitHub issue before proposing significant architectural changes.

## License

A license has not yet been selected.

---

**Vendrix — Build a store. Launch a marketplace. Power a commerce platform.**
