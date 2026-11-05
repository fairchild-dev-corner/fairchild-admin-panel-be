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
| `make migrate-up-dev` / `migrate-down-dev` (`-stage`, `migrate-up-prod`) | migrations (the Docker image also runs `up` itself on every start) |
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
| `GET /admin/members/:id/soa`, `GET /admin/members/:id/soa/:ctrlNo`, `GET /admin/members/:id/standing` (ledger members only; read from that branch ledger's `wkfcsoa`, `sldtl` and `loan`, as fairchild_be does: statements dated in the last 2 months, or else the latest one, and the 12-month coop health score) | admin, staff |
| `GET /admin/portal-members?status=&branches=&q=&page=&pageSize=`, `GET /admin/portal-members/:id` | admin, staff |
| `GET /admin/portal-lookup?memberNo=`, `POST /admin/portal-members` (add a ledger member), `PUT /admin/portal-members/:id` `{email, mobileNumber}`, `DELETE /admin/portal-members/:id` | **admin only** |
| `GET /admin/account-claims?status=&q=&month=&year=&page=&pageSize=` (newest submitted first; `month` YYYY-MM or `year` YYYY filters the submission date), `GET /admin/account-claims/:id` | admin, staff |
| `PUT /admin/account-claims/:id/approve` (puts the requested email/mobile on the matching portal account, then emails and texts the member on them; returns `{claim, notifications}`), `PUT /admin/account-claims/:id/reject` `{reason}` (then emails and texts the member, with the reason, on the details already on their account, never the rejected ones; returns `{claim, notifications}`) | **admin only** |
| `GET /admin/locked-accounts?status=&q=&month=&year=&page=&pageSize=` (login locks; `status` is `locked` or `unlocked`; `month` is `YYYY-MM` or `year` is `YYYY`, otherwise the last 3 months; dates are the lock date on `locked` and the unlock date on `unlocked`) | admin, staff |
| `PUT /admin/locked-accounts/:id/unlock` (then emails and texts the member when the member ID has a portal account; the response says what was sent) | **admin only** |
| `GET /admin/contact-messages?status=unreplied\|replied&q=&page=&pageSize=`, `GET /admin/contact-messages/:id` | admin, staff |
| `POST /admin/contact-messages/:id/replies` `{channel: email\|sms, body}` (emails the sender, or texts the portal account with the same email) | **admin only** |
| `GET /admin/blasts?page=&pageSize=`, `GET /admin/blasts/:id` (with up to 100 undelivered messages) | admin, staff |
| `POST /admin/blasts/preview` `{channel, statuses, branches}`, `POST /admin/blasts/test` `{channel, subject, emailBody, smsBody, email, mobile}`, `POST /admin/blasts` `{channel, statuses, branches, subject, emailBody, smsBody}`, `PUT /admin/blasts/:id/cancel` | **admin only** |
| `GET /admin/voters?status=&q=&page=&pageSize=` (`status`: `all` (default), `eligible` = voting, `ineligible` = non-voting), `GET /admin/voters/master-list` (current list + last upload), `GET /admin/voters/stats` (voting / non-voting per ledger branch, plus `none` for members on no ledger), `GET /admin/election-control` | admin, staff |
| `POST /admin/voters/master-list/check` (multipart `file`: .xlsx or .csv; nothing saved), `PUT /admin/voters/master-list` `{fileName, members: [{memberNo, name}]}` (both only while the election is on; otherwise 409), `POST /admin/voters/master-list/reset` `{password}` (empties the list: everyone non-voting; only while the election is on, otherwise 409), `PUT /admin/election-control` `{enabled, password}` (the admin's own password; wrong = 403 and counts toward the login lockout) | **admin only** |
| `GET /admin/site-status` | admin, staff |
| `PUT /admin/site-status` | **admin only** |
| `GET /admin/health`, `GET /admin/metrics`, `GET /admin/audit-log` | admin, staff |
| `GET /admin/account` (the signed-in user's own profile, active sessions, last 10 sign-ins, last 15 actions) | admin, staff |
| `PUT /admin/account/profile` `{firstName, lastName, email, currentPassword}` (own name and email; email optional, unique among panel accounts), `PUT /admin/account/password` `{currentPassword, newPassword}` (at least 12 characters; signs the user out everywhere). Both confirm with the current password, which counts toward the login lockout when wrong (403), and are audited. | admin, staff |

Application ids are accepted as `LA-01040` / `MA-00560` or the bare number, member ids as `FC20310` or `20310`, claim ids as `CR-00012` or `12`, lock ids as `LK-00012` or `12`, contact message ids as `CM-00012` or `12`, blast ids as `MB-00012` or `12`.

### Voters' master list

One list (`voter_master_list`) of the members who can vote, uploaded on the Generate Master List page; each upload replaces it and is logged in `voter_master_list_uploads`.

- **Election switch** (Election > Controller, `election_control`): the yearly election period is on or off (off by default). While it's off the master list can't be uploaded or reset, and the panel hides Generate Master List and the reset control; the Members List stays available. Turning it on or off needs the admin to re-enter their password, which is checked like a login: a wrong one is recorded in `users_panel_login_audit` and 5 within 15 minutes lock the account (and IP). Every change is audited.

- **Upload:** an Excel `.xlsx` (first worksheet) or CSV with a "Member ID" column (also matched: "Member No.", "ID No.", "Membership No.") and an optional "Name" column, with the header in the first 10 rows. `check` reads it and matches each ID to a live portal account, case-insensitively; an ID Excel stored as a number ("696" for "000696") matches by its digits when that's unambiguous. It returns the matched members, the IDs with no portal account (`notFound`, left out), repeats and blank rows. The admin confirms, and `PUT` matches the IDs again and replaces the whole list. Up to 50,000 rows and 5 MB.
- **Members List** (`GET /admin/voters`) is every live portal account (fairchild_be `users`) joined with the list, plus listed members whose account was removed since: voting or non-voting, and why not (`not_listed`, `account_inactive`, `no_account`).

### SMS/email blasts

One message to many portal members (events, notices), over the same SMTP/Movider setup as contact replies.

- **Audience:** live `users` rows with the chosen statuses (Active by default), optionally only those on chosen branches (matched through the Portal page's ledger index). A member gets one message per channel ("both" = an email and a text); members with no usable address are left out, and an address shared by several accounts (a guardian's number) gets one message. Texts go to the account's mobile, or a Young Saver's guardian's.
- **`{name}`** in the subject, email or text is replaced with the member's first name.
- **Sending:** creating a blast stores every recipient in `message_blast_recipients` and returns at once. A background worker in each server instance claims 50 at a time (`FOR UPDATE SKIP LOCKED`, so instances never double-send) and paces each channel: `BLAST_EMAILS_PER_MINUTE` (default 30) and `BLAST_SMS_PER_MINUTE` (default 60). Each message's outcome and provider error are recorded. A blast left sending when the server stopped is resumed on start; a claim older than 15 minutes is retried.
- **Cancel** skips every message not yet sent.
- **Test mode (any build but prod):** only addresses in `BLAST_TEST_RECIPIENTS` (comma-separated emails and mobile numbers) are actually messaged; every other recipient is recorded as skipped. This keeps dev and staging, whose databases hold real members' contacts, from messaging members. `POST /admin/blasts/test` always sends, to the address the admin types.

### Contact message replies

Replies go out over the same providers fairchild_be uses for OTPs, configured with the same `.env` keys: **SMTP** (`SMTP_HOST`, `SMTP_PORT` (default 587, STARTTLS), `SMTP_USERNAME`, `SMTP_PASSWORD`, `SMTP_FROM`) for email, and **Movider** (`MOVIDER_API_KEY`, `MOVIDER_API_SECRET`, optional `MOVIDER_SENDER_NAME`) for SMS. Each channel is optional: one with missing keys is disabled, logged at startup, and reported to the panel as `channels` in the list response.

- An email goes to the address on the message, using the FCCMPC email layout with the original message quoted.
- A text goes to the mobile number of the portal account (`users`) with the same email, or a Young Saver's guardian's number, normalized to E.164. It is prefixed `FCCMPC:` and capped at 480 characters.
- A reply is recorded only after the provider accepts it. A provider failure answers 424. If a reply was sent but couldn't be recorded, the endpoint answers 409 so the admin doesn't send it twice.

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
| `contact_message_replies` | this service | replies sent to fairchild_be's `contact_messages` (channel, recipient, body, sender); a message with a reply counts as replied |
| `message_blasts`, `message_blast_recipients` | this service | SMS/email blasts and one row per message (recipient, status, provider error), snapshotted when the blast is created |
| `election_control` | this service | single-row election on/off switch (who changed it, when) |
| `voter_master_list`, `voter_master_list_uploads` | this service | the voters' master list (portal account + member number, and the name as written in the uploaded file), replaced on every upload, and a log of uploads (file, count, who, when) |
| `users`, `membership_applications`, `account_claim_requests`, `locked_member_accounts`, `contact_messages` | fairchild_be | read only (`contact_messages` always), except: `membership_applications.status` (reviews); inserting pending rows for requests staff enter manually (`POST /admin/membership-applications`); from the Portal page: adding an account for a ledger member (a `users` row copied from the ledger + a `user_account_cred` row with no password, like migrated accounts), changing `email`/`mobile_number`, or removing an account (`status = 'deleted'`, `deleted_at`, its `auth_sessions` revoked); and from the Claim Accounts page: reviewing a claim (`status`, `rejection_reason`, `reviewed_at`), where approving also sets the matching account's `email`/`mobile_number` in the same transaction. The reviewer is read back from `admin_audit_logs`. From the Locked Accounts page: unlocking a member ID (`is_locked = FALSE`, `unlocked_at` on its active `locked_member_accounts` rows; the rows stay as history). |

Migration history is tracked in **`admin_panel_schema_migrations`**, not `schema_migrations`, so it never collides with fairchild_be's migrations on the same DB.

## Security notes

- **Separate trust domain from members.** Access tokens use their own secret (`ADMIN_JWT_ACCESS_SECRET`) and are checked for issuer `fairchild-admin-panel-be` and audience `fairchild-admin-panel`. A fairchild_be member token never verifies here.
- **Role and status are re-read from `users_panel` on every `/admin` request.** Suspending a user or changing their role takes effect immediately, not when their token expires.
- **Lockout.** 5 failed logins per username or per IP within 15 minutes blocks further attempts until the window rolls. Unknown usernames and wrong passwords get the same response and take the same time.
- **Refresh tokens are rotated on every use and can be redeemed only once,** even under concurrent requests. The cookie is `admin_refresh_token` (HttpOnly, path `/api/v1/auth`). The name differs from fairchild_be's `refresh_token` because cookies aren't port-scoped on localhost.
- **DB sessions are pinned to UTC.** The server runs at +08:00, and without the pin inserted timestamps would land 8 hours off. "This month" and the metric buckets use Asia/Manila.
- **5xx responses return a generic message;** details go to the server log only. Response bodies (tokens, applicant data) are never logged.

## Known gaps


- **Blasts need a server that keeps running.** The blast worker runs in the background, outside any request. On Cloud Run that needs CPU always allocated and at least one instance; otherwise sending stalls between requests.
- **Email blasts are limited by the SMTP account.** A Gmail/Workspace SMTP account sends at most a few hundred to ~2,000 emails a day, far fewer than the ~10k members with an email. Large email blasts need a bulk email provider. SMS cost is per text (per segment for long ones).
- **A blast message can be sent twice in one edge case:** the server stops after the provider accepted a message but before its outcome was saved. That claim is retried after 15 minutes.

- **Nothing writes `loan_applications` yet.** fairchild_be has no member-facing "apply for a loan" endpoint. When one is added, it should insert into this table.
- **`dailyVisitors` in `/admin/metrics` is always `[]`.** There's no analytics source yet.
- **`totalMembers` counts registered portal accounts** (fairchild_be `users`, 12.5k locally), not the Members page's ledger + panel members (13k current members locally).
- **`admin_members` isn't linked to the branch ledgers.** Its `FC` ids come from its own AUTO_INCREMENT (starting at 20001), not ledger client ids. A member added in the panel isn't written to a ledger.
- **Active-loan totals are computed in the background,** at startup and then every 10 minutes (every minute after a failure), so `/admin/dashboard` answers in milliseconds. The scan sums ~5M `sldtl` rows across the ledgers in parallel, ~5 s locally for the largest branch; only a dashboard load in those first seconds after startup waits for it.
- **A portal account's branch comes from a ledger index cached for 10 minutes** (every `OldClientID` per branch, matched to `users.member_id`), so a newly added ledger member shows its branch within 10 minutes. Accounts whose member number is on no ledger show no branch and drop out when a branch filter is picked.
- **Portal accounts added from the panel have no password.** The member sets one with the portal's Forgot Password (email, then an OTP to the mobile number), the same path as the ~3,500 migrated accounts.
- **Ledger member search scans each branch's `client` table** (no full-text index). Fine at ~20k rows; revisit if branches grow a lot.
- **Active-loan totals can be up to 10 minutes old.** If a refresh fails, the previous value keeps being served until one succeeds.
- **No user-management API yet** (create, suspend, reset password). Use `create-panel-user` or SQL for now.
