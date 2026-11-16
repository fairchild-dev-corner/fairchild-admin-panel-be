# Backend testing

Run these commands from the repository root with the Go version in go.mod.
The runner works in PowerShell and Linux/macOS shells.

```text
go install golang.org/x/vuln/cmd/govulncheck@v1.1.4
go run ./cmd/testsuite
```

Ensure the Go bin directory (go env GOPATH, then bin) is on PATH so the
runner can find govulncheck. All procedures run even if an earlier one fails;
any failed or missing procedure gives a nonzero exit status. The vulnerability
scanner needs internet access to the Go vulnerability database. It is never
silently skipped.

Reports go to artifacts/testing/: separate procedure logs, coverage.out,
coverage.log (per-function coverage), and summary.json (pass/fail and duration).
Use go tool cover -html=artifacts/testing/coverage.out to browse coverage.
CI runs the same command with -race on pushes and pull requests and uploads reports.

## Individual procedures

| Command | Procedure |
| --- | --- |
| go test ./... | Fast local unit tests, including security regressions and fuzz seeds |
| go run ./cmd/testsuite -suite unit | Uncached shuffled unit tests and coverage |
| go run ./cmd/testsuite -suite performance | All module benchmarks with allocations |
| go run ./cmd/testsuite -suite security | go vet, named security regression tests, govulncheck |
| go run ./cmd/testsuite -suite fuzz | Timed JWT and mobile-number fuzzing |
| go run ./cmd/testsuite -race | All procedures plus race detection in unit tests |
| go run ./cmd/testsuite -suite performance -benchtime 1s | Longer benchmark sampling |

Race detection needs CGO and a C compiler; CI supplies these on Linux. On Windows,
use the default runner unless a compatible compiler is installed. The runner
also accepts -out, -fuzztime and -step-timeout. GNU Make aliases are available
as test-all, test-unit, test-performance, test-security and test-fuzz.

## Module inventory and implemented checks

Every service module has unit tests and at least one benchmark. Existing tests
are retained. Coverage is measured across all packages, including handlers and
repositories exercised indirectly. There is no claim of complete branch coverage.

| Module | Unit/security checks | Performance workload |
| --- | --- | --- |
| Auth | Existing login, session rotation, lockout; JWT algorithm/identity rejection; opaque token uniqueness; token fuzzing | JWT parsing and Argon2 password hashing |
| Account | Password policy, missing account, confirmation before profile/password writes | Missing-account lookup |
| Users | Create validation, hierarchy, self-management and escalation denial | Authorized unchanged-role lookup |
| Capabilities | Existing assignments, page prerequisites, role presets and refusal cases | Cached capability authorization |
| Applications | Audited approval, reviewer identity, lookup errors, hostile IDs; existing ID/model tests | Application lookup |
| Members | Existing normalization, guardian validation and merged-source paging | Merged panel and ledger page |
| Portal | Ledger disambiguation, branch cache, ledger failure | Ledger record selection |
| Claims | Repeat review, missing portal account, invalid contact and lookup errors | Claim lookup |
| Locks | Audited unlock, invalid IDs, repository failure | Unlock with audit construction |
| Contact | Empty/unsupported replies, disabled channels, failed send never recorded | Contact lookup |
| Blasts | Deduplication, test-recipient allowlist, content limits | Recipient preparation at 100 and 10,000 accounts |
| Composer | Existing fake OpenAI success/refusal/error cases; Unicode clipping | Unicode output clipping |
| Messaging | Phone normalization, fake Movider request/responses, greetings, all five HTML templates, SMTP header injection/cancellation, phone fuzzing | HTML announcement rendering |
| Voters | Existing CSV categories/tokens/matching; election-off guards and upload size limit | CSV parsing at 100 and 10,000 rows |
| Audit | Existing election action scoping, search and paging | Election audit page |
| Dashboard | Summary aggregation/rounding, Manila month boundary, stale-cache fallback, cancellation | Summary from cached loan totals |
| Metrics | Six zero-filled months across year boundaries, unexpected products, errors | Chart assembly |
| Health | Concurrent probe results, deadlines, HTTP status handling and cancellation | Concurrent local probe |
| Site | Public response hides staff, schedule bounds, audit action and message normalization | Public status projection |
| Shared HTTP/security | All admin handler registrations reject requests without credentials; refresh-cookie flags/deletion; CORS allowlist; role denial; parallel IP limit | Unauthorized HTTP request and parallel limiter |
| Configuration/database | Branch-map validation, production CA requirement, typed driver settings, invalid CA, transaction commit/rollback/errors | Branch-map and driver configuration |

Auth, capabilities, members, audit, composer and voter parsing had tests before
this work. New test files add coverage to the remaining service modules.

## Interpretation and remaining integration coverage

These tests use in-memory repositories, local HTTP servers and fake transports.
They need no MySQL credentials, SMTP/Movider/OpenAI keys, production server or
running frontend. They do not run migrations or send real notifications.
The admin route sweep mounts the real handler registrations on a guarded test
group; it does not test the production APIServer.Run wiring.

Benchmarks report time/op and bytes/allocations per operation. They measure
local computation and stubbed repository calls, not database latency, provider
throughput or end-to-end capacity. Several fast-path benchmarks deliberately
measure rejection or cache/lookup paths; use the workload column when comparing.
There are no arbitrary wall-clock pass/fail thresholds. Save a stable baseline
from the same Go version and machine before evaluating regressions.

The following modules still need a disposable, seeded MySQL integration
environment and realistic API fixtures before their full behavior can be certified:

- Every repository: SQL filtering/pagination, transaction isolation, atomic audit
  writes, rollback, unique constraints and concurrent writes.
- Auth/account/users/portal: persisted revocation, refresh replay races, suspension,
  uniqueness, complete successful profile/password changes.
- Claims/locks/contact/blasts: successful notification workflows, retry/cancel and
  provider timeouts; verify no duplicate delivery using fake providers.
- Applications/members/voters: complete mutation round trips, cross-branch data,
  XLSX limits, token claim races and import transactions.
- Site/dashboard/metrics/health: actual MySQL time-zone behavior, scheduled
  maintenance session revocation and outages across multiple branch databases.
- HTTP API: authenticated success/denial matrix for every endpoint, complete
  request validation and response contracts, and production route wiring.
- Migrations: apply every up migration to an empty isolated schema and verify
  supported rollback on synthetic data.

Run API load and dynamic security scans only against a disposable test deployment,
with fake messaging providers and synthetic accounts. Include concurrent readers,
auth failures, filtered pages, imports and mutation/audit contention. Record p50,
p95, p99 latency, error rate and database pool saturation; establish environment
specific budgets before making these CI gates. This repository does not yet
contain such a deployment, so the automated default suite does not claim these
integration or load results.

## Initial validation snapshot (2026-11-12)

The first local run passed unit tests, all service benchmarks, security
regression tests and go vet. Repository-wide statement coverage was 23.3%.
Race detection is configured in Linux CI; this Windows environment has
CGO_ENABLED=0 and did not execute the race detector.

govulncheck v1.1.4 reported 31 reachable vulnerability entries:
20 in the Go 1.26.5 standard library, 10 in github.com/xuri/excelize/v2 v2.9.0,
and one in github.com/quic-go/quic-go v0.59.0. It also reported additional
imported/required vulnerabilities without detected calls. See
artifacts/testing/vulnerabilities.log for IDs, fixed versions and call traces.
This is a static reachability report, not confirmation that every finding
is exploitable in this deployment.

The vulnerability procedure and the combined suite intentionally return failure
while these findings remain. Dependency/toolchain remediation is separate from
these test additions; go.mod and go.sum were not changed by this work.
Re-run the scanner after remediation. Counts change with the vulnerability
database, toolchain and dependency versions.
