# fairchild-admin-panel-be

Go API for the Fairchild admin panel (`../fairchild-admin-panel`). It's a separate service from the member backend `fairchild_be`, but it uses the **same central auth DB** (`fccmpc_coop_auth_central`) and the same structure: gin, repo → service → handler, golang-migrate, the same response envelopes.

Panel users are admins and staff stored in their own table, `users_panel`. They sign in with **username + password**. There's no badge number and no OTP.

## Getting started

```bash
cp .env.example .env          # fill MySQL creds + ADMIN_JWT_ACCESS_SECRET (openssl rand -hex 32)
make migrate-up-dev           # creates users_panel & co. in fccmpc_coop_auth_central
make create-panel-user ENV=dev ARGS="-username admin -first-name First -last-name Admin -role admin"
make run-dev                  # http://localhost:8081
```

`create-panel-user` prompts for the password, or reads `PANEL_USER_PASSWORD` when that's set. Passwords must be at least 12 characters. There's no sign-up endpoint: panel accounts are only created this way.

| Command | |
|---|---|
| `make run-dev` / `run-staging` / `run-prod` | build + run |
| `make migrate-up-dev` / `migrate-down-dev` (`-stage`, `migrate-up-prod`) | migrations |
| `make create-panel-user ENV=dev ARGS="..."` | add an admin/staff account |
| `make seed-members ENV=dev` | sample rows for an empty `admin_members` (refuses if it has rows) |
| `make test` | `go test ./...` |
| `go build ./... && go vet ./...` | compile-check |

## Endpoints (`/api/v1`)

| Route | Auth |
|---|---|
| `POST /auth/login` `{username, password}` | public; IP rate-limited |
| `POST /auth/refresh`, `POST /auth/logout` | `admin_refresh_token` cookie |
| `GET /site/status` | public (fairchild-fe's maintenance banner) |
| `GET /healthz` | public liveness |
| `GET /admin/dashboard` | admin, staff |
| `GET /admin/loan-applications?status=`, `GET /admin/loan-applications/:id`, `PUT /admin/loan-applications/:id/review` | admin, staff |
| `GET /admin/membership-applications?status=&q=`, `POST /admin/membership-applications`, `GET /admin/membership-applications/:id`, `PUT /admin/membership-applications/:id/review` | admin, staff |
| `GET /admin/members?category=&branches=&q=&page=&pageSize=`, `GET /admin/members/:id` | admin, staff |
| `POST /admin/members`, `PUT /admin/members/:id` (panel members only) | admin, staff |
| `DELETE /admin/members/:id` (panel members only; soft delete) | **admin only** |
| `GET /admin/portal-members?status=&branches=&q=&page=&pageSize=`, `GET /admin/portal-members/:id` | admin, staff |
| `GET /admin/portal-lookup?memberNo=`, `POST /admin/portal-members` (add a ledger member), `PUT /admin/portal-members/:id` `{email, mobileNumber}`, `DELETE /admin/portal-members/:id` | **admin only** |
| `GET /admin/site-status` | admin, staff |
| `PUT /admin/site-status` | **admin only** |
| `GET /admin/health`, `GET /admin/metrics`, `GET /admin/audit-log` | admin, staff |

Application ids are accepted as `LA-01040` / `MA-00560` or the bare number, member ids as `FC20310` or `20310`.

### Members

`GET /admin/members` merges two sources into one paged list:

- **Ledger members**, read-only, from each branch ledger's `client` table. `ClientType` 1–4 map to the regular / associate / kiddie / aflatoun tabs, and the branch is the ledger DB (`ClientIDBrCode` 1 main, 2 knowles, 3 tiic). Closed (3) and Disapproved (98) accounts and deceased clients are left out; Dormant and InActive show as `inactive`. Ids are `<branch code>-<client id>` (`01-012345`), and `memberNo` is `OldClientID`. Records with blank `FName`/`LName` take their name from `AccountName` ("LAST, FIRST MIDDLE"). `PUT`/`DELETE` on a ledger id answer 403.
- **Panel members** from `admin_members` (ids `FC20001`…), with full CRUD.

Each page lists panel members first (newest first), then each selected branch's ledger in branch order, by surname. The response also carries `categoryCounts` (over the selected branches) and `branchCounts` (over the selected category), both filtered by `q`.

## Tables (migrations in `cmd/migrate/migrations`)

| Table | Owner | Purpose |
|---|---|---|
| `users_panel` | this service | admin/staff accounts (`role`: admin \| staff; `status`: active \| suspended \| deleted) |
| `users_panel_sessions` | this service | refresh sessions (SHA-256 of the token only) |
| `users_panel_login_audit` | this service | every login attempt; drives lockout |
| `admin_audit_logs` | this service | one row per admin mutation, written in the same transaction |
| `site_status` | this service | single-row maintenance flag |
| `loan_applications` | this service | online loan applications awaiting review |
| `membership_application_reviews` | this service | who/when/why for reviews of fairchild_be's `membership_applications` |
| `admin_members` | this service | members managed from the panel's Members page; deletes set `deleted_at` |
| `users`, `membership_applications` | fairchild_be | read only, except: `membership_applications.status` (reviews); inserting pending rows for requests staff enter manually (`POST /admin/membership-applications`); and from the Portal page: adding an account for a ledger member (a `users` row copied from the ledger + a `user_account_cred` row with no password, like migrated accounts), changing `email`/`mobile_number`, or removing an account (`status = 'deleted'`, `deleted_at`, its `auth_sessions` revoked) |

Migration history is tracked in **`admin_panel_schema_migrations`**, not `schema_migrations`, so it never collides with fairchild_be's migrations on the same DB.

## Security notes

- **Separate trust domain from members.** Access tokens use their own secret (`ADMIN_JWT_ACCESS_SECRET`) and are checked for issuer `fairchild-admin-panel-be` and audience `fairchild-admin-panel`. A fairchild_be member token never verifies here.
- **Role and status are re-read from `users_panel` on every `/admin` request.** Suspending a user or changing their role takes effect immediately, not when their token expires.
- **Lockout.** 5 failed logins per username or per IP within 15 minutes blocks further attempts until the window rolls. Unknown usernames and wrong passwords get the same response and take the same time.
- **Refresh tokens are rotated on every use and can be redeemed only once,** even under concurrent requests. The cookie is `admin_refresh_token` (HttpOnly, path `/api/v1/auth`). The name differs from fairchild_be's `refresh_token` because cookies aren't port-scoped on localhost.
- **DB sessions are pinned to UTC.** The server runs at +08:00, and without the pin inserted timestamps would land 8 hours off. "This month" and the metric buckets use Asia/Manila.
- **5xx responses return a generic message;** details go to the server log only. Response bodies (tokens, applicant data) are never logged.

## Known gaps

- **Nothing writes `loan_applications` yet.** fairchild_be has no member-facing "apply for a loan" endpoint. When one is added, it should insert into this table.
- **`dailyVisitors` in `/admin/metrics` is always `[]`.** There's no analytics source yet.
- **`totalMembers` counts registered portal accounts** (fairchild_be `users`, 12.5k locally), not the Members page's ledger + panel members (13k current members locally).
- **`admin_members` isn't linked to the branch ledgers.** Its `FC` ids come from its own AUTO_INCREMENT (starting at 20001), not ledger client ids. A member added in the panel isn't written to a ledger.
- **A cold `/admin/dashboard` takes ~18 s locally** with all three ledgers connected (it sums ~5M `sldtl` rows); cached responses take milliseconds for the next 10 minutes.
- **A portal account's branch comes from a ledger index cached for 10 minutes** (every `OldClientID` per branch, matched to `users.member_id`), so a newly added ledger member shows its branch within 10 minutes. Accounts whose member number is on no ledger show no branch and drop out when a branch filter is picked.
- **Portal accounts added from the panel have no password.** The member sets one with the portal's Forgot Password (email, then an OTP to the mobile number), the same path as the ~3,500 migrated accounts.
- **Ledger member search scans each branch's `client` table** (no full-text index). Fine at ~20k rows; revisit if branches grow a lot.
- **Active-loan totals scan each branch ledger.** They're cached for 10 minutes. If a refresh fails and an older value exists, the older value is served.
- **No user-management API yet** (create, suspend, reset password). Use `create-panel-user` or SQL for now.
