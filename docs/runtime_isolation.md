# Runtime Isolation for Riptide

An evaluation of sandboxing options for Riptide's Go + chromedp + headless Chrome workload, across two concerns: **safety** (providing the isolated execution environment Google's computer use best practices require) and **scalability** (running many concurrent Riptide sessions without interference).

Four options are evaluated: Apple Container, Docker, GKE Agent Sandbox, and Cloud Run.

---

## Executive Summary

| | Apple Container | Docker | GKE Agent Sandbox | Cloud Run Jobs |
|---|---|---|---|---|
| Isolation mechanism | VM per container (Virtualization.framework) | Linux namespaces + cgroups | gVisor userspace kernel | microVM (Gen 2) or gVisor (Gen 1, N/A for Jobs) |
| Chrome headless | ✅ Full — no compromises | ✅ Works with `--shm-size` + `--no-sandbox` | ⚠️ Works with `--no-sandbox --disable-dev-shm-usage`; perf overhead | ✅ Gen 2: full Linux compat; Google officially supports headless Chrome |
| CDP (localhost) | ✅ Native | ✅ Native | ✅ Via gVisor netstack (some overhead) | ✅ Native (within container) |
| Vertex AI egress | ✅ vmnet NAT | ✅ bridge NAT | ✅ Explicit egress rules required | ✅ Default internet access; Workload Identity supported |
| `--no-sandbox` needed | **No** (VM provides boundary) | Yes (standard for containers) | Yes (gVisor incompatible with Chrome's internal sandbox) | Yes (container boundary); or use Chrome seccomp profile |
| Developer platform | macOS 26, Apple Silicon only | Any (macOS via Docker Desktop) | GKE (cloud only) | GCP (cloud only) |
| Production path | Mac-only; no cloud | Docker → GKE Standard | GKE Agent Sandbox (natural GCP fit) | Cloud Run Jobs (ideal for batch/benchmark runs) |
| Parallel sessions | 1 VM per session; RAM-bound | 1 container per session; host-bound | Horizontal autoscaling + warm pools | Native task parallelism; scale to zero |
| Maturity | 1.0.0 (June 2026) | Battle-tested | GA on GKE ≥1.35.2 | GA |
| Session output | Volume mounts | Volume mounts | PVC / emptyDir | GCS FUSE volume mount |
| Task timeout | N/A | N/A | No limit (pod lifetime) | Up to 168 hours |
| Pricing model | Machine cost | Machine cost | Per-node-hour (always on) | Per-vCPU-second + per-GiB-second (pay per use) |

**Recommended path:** Docker for immediate safety improvement and CI; Apple Container for local dev on macOS 26; GKE Agent Sandbox for production at scale on GCP; Cloud Run Jobs for benchmark/evaluation batch runs (cost-efficient, zero-ops).

---

## Context: The Current Gap

Riptide currently uses `chromedp.NoSandbox`, which disables Chrome's internal process sandbox. This is flagged in `docs/safety_best_practices_analysis.md` as an unresolved tension with Google's published best practices recommendation to run in a sandboxed environment. The three options below each close this gap in different ways.

---

## Option 1: Apple Container

**What it is:** Apple's native container runtime, open-sourced at WWDC 2025. Uses `container` CLI (not `docker`). Each container is its own lightweight Linux VM backed by Apple Virtualization.framework. Version 1.0.0 released June 2026. OCI-compatible.

### Architecture

```
macOS (Apple Silicon)
└── container-apiserver (Launchd service)
    └── Per-container Linux VM (Virtualization.framework)
        └── riptide binary + Chrome subprocess
```

Unlike Docker on macOS (all containers share one Linux VM), Apple Container gives each session a fully isolated hypervisor boundary.

### Safety Properties

- **VM-level isolation**: a Chrome exploit must escape Apple Virtualization.framework to reach the host — not just Linux namespaces.
- **Chrome's internal sandbox works natively**: the VM provides the Linux kernel primitives (`clone(CLONE_NEWUSER)`, etc.) that Chrome's sandbox requires. `chromedp.NoSandbox` can be **removed** inside Apple Container — this is the only option where that's straightforwardly true.
- **No shared kernel**: contrast with Docker's shared-host-kernel model.

### Networking

- `vmnet.framework` NAT: outbound internet works (Vertex AI HTTPS ✅).
- Port forwarding: `--publish 127.0.0.1:8083:8083` standard syntax.
- Container DNS: `<name>.test` hostname.
- CDP runs on `localhost:9222` entirely inside the VM — no cross-VM complexity.

### Developer Experience

```bash
# Install (download signed pkg from GitHub releases)
# Requires macOS 26, Apple Silicon

container system start

container build -t riptide:latest .

container run \
  --cpus 4 --memory 4g \
  --volume $(pwd)/sessions:/sessions \
  riptide:latest \
  --prompt "Navigate to google.com" --sessions-dir /sessions
```

**Limits:** macOS 26 (Tahoe) required — not available on macOS 15 or earlier, or Intel Macs. No cloud deployment path — this is a developer-only option.

### Memory Considerations

Default: 1 GiB per container VM. Chrome + renderer processes need ~2-3 GB for moderate workloads. Set `--memory 4g` per session. Memory freed inside the VM is not immediately returned to macOS host (no balloon compressor yet); long-running sessions accumulate host memory until container exits.

### Parallel Sessions

One VM per session. On a 16 GB Mac: ~3 concurrent Riptide sessions. Resource-intensive but fully isolated.

---

## Option 2: Docker

**What it is:** The standard container runtime. Uses Linux namespaces + cgroups. On macOS, Docker Desktop runs a shared Linux VM (Apple Virtualization.framework); containers share that VM's kernel. Universally available and well-established for browser automation workloads.

### Architecture

```
macOS (Docker Desktop)
└── Linux VM (shared, all containers)
    └── dockerd
        └── Container (PID/net/mnt namespaces)
            └── riptide + Chrome
```

### Safety Properties

- Namespace isolation: each container has its own PID, network, mount, and IPC namespaces.
- Chrome requires `--no-sandbox` in standard Docker — the container boundary substitutes for Chrome's internal sandbox.
- Defense-in-depth: seccomp profiles (default blocks ~300 dangerous syscalls), AppArmor, capability dropping, non-root user.
- Best practice: use a Chrome-specific seccomp profile that allows `CLONE_NEWUSER` while blocking other dangerous syscalls.

### Chrome-Specific Requirements

Two flags are non-negotiable in Docker:

| Flag | Why |
|---|---|
| `--shm-size 2g` | Chrome uses `/dev/shm` for IPC; Docker default is 64 MB — Chrome will crash |
| `--no-sandbox` (Chrome flag) | Docker's seccomp profile prevents Chrome's internal sandbox syscalls |

Alternative to `--no-sandbox`: use a custom seccomp profile permitting `CLONE_NEWUSER`. This preserves Chrome's internal sandbox while Docker provides the outer boundary.

### Dockerfile Skeleton

```dockerfile
FROM debian:bookworm-slim AS builder
# ... build riptide binary ...

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y \
    chromium fonts-liberation libgbm1 ca-certificates \
    --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

RUN useradd -m -u 1000 riptide
USER riptide
WORKDIR /home/riptide
VOLUME ["/sessions"]

COPY --from=builder /go/bin/riptide /usr/local/bin/riptide
ENTRYPOINT ["riptide"]
```

```bash
docker run \
  --shm-size 2g \
  --cpus 2 --memory 3g \
  --security-opt seccomp=chrome-seccomp.json \
  --read-only --tmpfs /tmp \
  -v $(pwd)/sessions:/sessions \
  riptide:latest \
  --prompt "..." --sessions-dir /sessions
```

### Networking

- Bridge network (default): NAT to host; Vertex AI HTTPS ✅.
- `--network none`: complete isolation — breaks Vertex AI; useful for offline testserver runs.
- CDP on `localhost:9222` is within the container's network namespace — no cross-container routing.

### Recommended Security Posture

```
--security-opt seccomp=chrome-seccomp.json  # Chrome-tuned seccomp profile
--cap-drop ALL                               # Drop all capabilities
--read-only                                  # Read-only root filesystem
--tmpfs /tmp --tmpfs /run                    # Writable ephemeral dirs
USER riptide (non-root)                      # Non-root inside container
--shm-size 2g                                # Required for Chrome
```

### Docker on macOS Overhead

Docker Desktop uses Apple Virtualization.framework on Apple Silicon. All containers share a single Linux VM (configurable size in Docker Desktop preferences). Performance characteristics:

- Network: one extra NAT hop; ~0.1-0.5 ms per call (negligible vs Vertex AI latency).
- File I/O: bind mounts from macOS cross the VM via virtiofs — can be slow for write-heavy session directories. Use a Docker volume inside the VM for screenshot output.
- CPU: low overhead on Apple Silicon (native VM).

In production on GKE (native Linux): no VM overhead.

### Production Path

Docker images are directly deployable to GKE, Cloud Run, or any Kubernetes cluster. The same `riptide:latest` image built for local Docker runs on GKE without modification.

---

## Option 3: GKE Agent Sandbox

**What it is:** A managed Kubernetes add-on (GA on GKE ≥1.35.2) that provides declarative sandboxed execution for AI agent workloads. Built on gVisor — a Go implementation of the Linux kernel API that intercepts every syscall from the application, preventing direct host kernel access.

Adds orchestration on top of raw gVisor: `SandboxTemplate` (define the environment), `SandboxWarmPool` (pre-warmed pods for sub-second startup), `SandboxClaim` (claim a warm pod for a session), `SandboxRouter` (network routing).

Based on the open-source [agent-sandbox](https://github.com/kubernetes-sigs/agent-sandbox) project.

### Architecture

```
GKE Cluster
├── Agent Sandbox Controller (managed add-on)
├── SandboxWarmPool → pre-warmed riptide pods
│
└── Sandbox Pod (one per Riptide session)
    ├── runtimeClassName: gvisor
    └── Container
        ├── gVisor Sentry (userspace Linux kernel)
        └── riptide + Chrome
            └── All syscalls → Sentry → minimal host syscalls
```

### Isolation Guarantees

gVisor's **Sentry** is a userspace implementation of the Linux kernel:

- Application syscalls (`read`, `write`, `clone`, `mmap`, etc.) go to the Sentry — not the host kernel.
- The Sentry makes a minimal set (~30) of host syscalls.
- A host kernel vulnerability cannot be exploited from inside the sandbox because the application has no direct kernel access.
- Stronger than Docker namespaces (which still expose the host kernel surface); slightly weaker than a full VM (Apple Container).

### Chrome Under gVisor: Critical Details

Chrome can run, but requires specific flags:

```go
// Required for gVisor environments
chromedp.Flag("no-sandbox", true),            // Chrome sandbox conflicts with gVisor seccomp virtualization
chromedp.Flag("disable-dev-shm-usage", true), // Use /tmp instead of /dev/shm
chromedp.Flag("disable-gpu", true),           // No GPU in gVisor
```

gVisor intercepts every Chrome syscall — Chrome is extremely syscall-intensive (IPC, futex, timers). Expect meaningful performance overhead:

| Operation | Native Docker | GKE Agent Sandbox (gVisor) |
|---|---|---|
| CDP round-trip (local socket) | ~1 ms | ~3-8 ms |
| Screenshot capture | ~50 ms | ~80-150 ms |
| Vertex AI HTTPS call | ~200 ms | ~200 ms (network, not gVisor) |
| Session startup (warm pool) | ~2-3 s | **<1 s** (warm pool) |

The performance overhead is real but bounded. For Riptide's workload (screenshot every turn, ~10-30 turns per session), gVisor overhead adds seconds to a session that already takes minutes. The warm pool advantage (sub-second session claim) more than compensates for startup time.

### Networking: Default Deny + Explicit Egress

```yaml
# SandboxTemplate with Vertex AI egress
spec:
  podTemplate:
    spec:
      runtimeClassName: gvisor
      containers:
      - name: riptide
        image: gcr.io/PROJECT/riptide:latest
        resources:
          limits:
            cpu: "2"
            memory: "4Gi"
        securityContext:
          capabilities:
            drop: ["ALL"]
      automountServiceAccountToken: false
      securityContext:
        runAsNonRoot: true
  networkPolicy:
    egress:
    - ports:
      - port: 443
        protocol: TCP
      to:
      - ipBlock:
          cidr: 0.0.0.0/0  # Restrict to aiplatform.googleapis.com CIDRs for tighter control
```

CDP (`localhost:9222`) is within the pod's network namespace — no egress rules needed for it.

### Warm Pools for Sub-Second Session Startup

```yaml
apiVersion: extensions.agents.x-k8s.io/v1alpha1
kind: SandboxWarmPool
metadata:
  name: riptide-warmpool
spec:
  replicas: 5
  sandboxTemplateRef:
    name: riptide-template
```

New sessions claim a pre-warmed pod (Chrome already started, creds cached) in `<1 second`. With Pod Snapshots (GKE ≥1.35.2): snapshot a fully initialized sandbox and restore it for each new session.

### Workload Identity for Vertex AI

No API keys in containers:

```bash
gcloud iam service-accounts add-iam-policy-binding \
    riptide-sa@PROJECT.iam.gserviceaccount.com \
    --role roles/iam.workloadIdentityUser \
    --member "serviceAccount:PROJECT.svc.id.goog[default/riptide-k8s-sa]"

gcloud projects add-iam-policy-binding PROJECT \
    --role roles/aiplatform.user \
    --member serviceAccount:riptide-sa@PROJECT.iam.gserviceaccount.com
```

Riptide inside the sandbox authenticates to Vertex AI transparently via the pod's service account.

### Known Limitations for Riptide

| Limitation | Impact | Workaround |
|---|---|---|
| `--no-sandbox` required | Chrome internal sandbox disabled | gVisor Sentry is the isolation layer |
| `kubectl port-forward` incompatible | Can't use for session viewer access | Use Sandbox Router or Service |
| HostPath volumes incompatible | Can't bind-mount host directories | Use PVC or `emptyDir` for sessions |
| gVisor syscall overhead | ~2-3x slower screenshot ops | Warm pools offset startup; per-turn overhead is acceptable |
| Chrome `--disable-dev-shm-usage` | Chrome uses `/tmp` for shm | No functional impact; minor memory layout difference |
| N2/e2 node types recommended | Constrains node pool choice | Use `e2-standard-4` minimum per session |

### GKE Agent Sandbox vs Raw GKE Sandbox (gVisor)

Raw GKE Sandbox (`--sandbox type=gvisor` on node pool) provides gVisor isolation but lacks session lifecycle management. GKE Agent Sandbox adds:

- `SandboxWarmPool`: pre-warmed pods ready to claim
- `SandboxClaim`: named session claim model
- `SandboxRouter`: network routing to sandbox pods
- Pod Snapshots: restore from snapshot for instant init
- Python SDK (`k8s-agent-sandbox`) for orchestration

For Riptide, use **GKE Agent Sandbox** (not raw GKE Sandbox).

---

## Option 4: Cloud Run Jobs

**What it is:** Google Cloud's fully managed serverless compute platform. For Riptide's workload — start, run an agent loop to completion, write session output, exit — **Cloud Run Jobs** is the correct product (not Cloud Run Services). Jobs are batch/task execution containers: no HTTP server required, task-level parallelism built in, timeout configurable up to 168 hours.

Cloud Run Jobs always use the **second generation execution environment** (cannot be changed). Gen 2 is a microVM — full Linux kernel, all syscalls, namespaces, and cgroups. This is the key difference from Cloud Run Services Gen 1 (which uses gVisor like GKE Agent Sandbox).

### Execution Environments: Gen 1 vs Gen 2

| | Gen 1 (Services only) | Gen 2 (Services + Jobs) |
|---|---|---|
| Sandboxing | gVisor userspace kernel | microVM (Firecracker-family) |
| Syscall support | Most, not all — emulated | Full Linux compatibility |
| Chrome sandbox | `--no-sandbox` required (same as GKE gVisor) | Standard container rules apply; `--no-sandbox` standard practice |
| `/dev/shm` | Emulated; limited (use `--disable-dev-shm-usage`) | Available; set via in-memory volume mount |
| Cold start | Faster | Slower (microVM boot), but acceptable |
| Min memory | 128 MiB | **512 MiB minimum** |
| Namespace/cgroup support | Partial | Full |
| Jobs | Not applicable — Jobs always Gen 2 | ✅ |

**For Riptide: Jobs always run Gen 2. Chrome works without gVisor's syscall overhead.**

### Cloud Run Jobs vs Cloud Run Services for Riptide

Riptide's execution model: one invocation = one complete agent session. No persistent HTTP server. This is Jobs, not Services.

| | Cloud Run Services | Cloud Run Jobs |
|---|---|---|
| Trigger | HTTP request | Manual, scheduled, or programmatic |
| Model | Request/response; containers scale to zero between requests | Task runs to completion; exits |
| Timeout | Max 3600 seconds (60 min) per request | Up to 168 hours per task |
| Concurrency | Multiple requests per container supported | 1 task per container instance |
| Session output | Ephemeral FS only (must upload mid-request) | Write to ephemeral FS; flush to GCS on exit |
| Fit for Riptide | ❌ Wrong shape — no long-running HTTP handler | ✅ Perfect — run agent loop, write output, exit |

**Use Cloud Run Jobs.** A Riptide benchmark run of N tasks × M seeds maps directly to a job execution with `--tasks N*M` and `--parallelism P`.

### Resource Limits

| Resource | Max (Gen 2) |
|---|---|
| vCPU | 8 vCPU |
| Memory | 32 GiB |
| Ephemeral disk | Configurable (billed separately) |
| `/dev/shm` | Via in-memory volume mount (counts against memory limit) |

**Recommended config for Riptide:** 2 vCPU, 4 GiB. Chrome + renderer processes need ~2-3 GB; Go binary + overhead ~256 MB. This maps to `--cpu 2 --memory 4Gi`.

There is no `/dev/shm` size setting equivalent to `--shm-size`. Instead, mount an in-memory volume at `/dev/shm`:

```yaml
# In Cloud Run Job YAML (gen2 supports this)
volumes:
- name: shm
  emptyDir:
    medium: Memory
    sizeLimit: 2Gi
containers:
- volumeMounts:
  - name: shm
    mountPath: /dev/shm
```

Alternatively, use `--disable-dev-shm-usage` Chrome flag to route IPC through `/tmp` instead.

### Headless Chrome on Cloud Run — Official Support

Google explicitly documents browser automation on Cloud Run:

> "To let your AI agent navigate the web, install Chromium in your Cloud Run container... Cloud Run provides built-in streaming support for streaming browser data back to the agent."

The Cloud Run docs list headless Chrome via CDP as a supported pattern for:
- Web scraping and data extraction
- UI testing
- Screenshots

This is a documented, supported use case. **Cloud Run Gen 2 (used by all Jobs) has no gVisor compatibility issues with Chrome.** Required Chrome flags in a Cloud Run Job container are the same as standard Docker:

```go
chromedp.Flag("no-sandbox", true),            // Standard container practice (no root Chrome)
chromedp.Flag("disable-dev-shm-usage", true), // Unless in-memory volume mounted at /dev/shm
chromedp.Flag("disable-gpu", true),           // No GPU available
```

`--no-sandbox` note: Chrome's `--no-sandbox` disables Chrome's internal process sandbox. In Cloud Run Gen 2 (microVM), the container boundary itself provides isolation — same logic as Docker. This is an accepted trade-off for containerized Chrome workloads.

### Networking and Vertex AI Access

**Default egress:** Cloud Run containers have outbound internet access by default. Vertex AI HTTPS calls work without configuration.

**Workload Identity:** Cloud Run Jobs support Workload Identity Federation. Attach a service account to the job; the container authenticates to Vertex AI transparently:

```bash
gcloud run jobs create riptide-benchmark \
  --image gcr.io/PROJECT/riptide:latest \
  --service-account riptide-sa@PROJECT.iam.gserviceaccount.com \
  --region us-central1
```

The service account needs `roles/aiplatform.user`. No API keys in the container.

**Egress restriction:** For tighter security, use Direct VPC egress with VPC Service Controls to restrict which Google APIs the container can reach. For Riptide, Vertex AI endpoints and arbitrary web URLs both need egress — the same challenge as GKE Agent Sandbox.

**CDP:** The Chrome DevTools Protocol runs on `localhost:9222` entirely within the container's network namespace. No egress rules needed.

### Session Output / Storage

Cloud Run containers have an ephemeral filesystem (writable, lost on container exit). Options for Riptide's session output (screenshots + logs):

| Approach | Mechanism | Pros | Cons |
|---|---|---|---|
| **GCS FUSE volume** | Mount GCS bucket as filesystem | Write files normally; persistent | FUSE overhead; requires bucket setup |
| **GCS upload on exit** | Sync `sessions/` to GCS after run | Simple; batch upload | Files lost if container crashes |
| **Ephemeral disk** | Larger local disk (billed separately) | Fast local I/O | Still ephemeral; lost on exit |
| **In-memory volume** | `emptyDir: medium: Memory` | Fast; good for `/dev/shm` | Counts against memory limit |

**Recommended:** Mount a GCS bucket via FUSE at `/sessions`. Cloud Run Jobs support Cloud Storage volume mounts natively:

```yaml
volumes:
- name: sessions-output
  csi:
    driver: gcsfuse.run.googleapis.com
    volumeAttributes:
      bucketName: riptide-sessions-PROJECTID
containers:
- volumeMounts:
  - name: sessions-output
    mountPath: /sessions
```

Riptide writes `sessions/SESSION_ID/screenshots/` and logs to `/sessions` exactly as in local runs. Output is immediately durable in GCS.

### Cold Start and Warm Instances

Cloud Run Jobs cold start = container pull + microVM boot + process startup.

| Phase | Estimated time |
|---|---|
| Container image pull (first run per region) | 10-30 s (image cached after first pull per instance) |
| microVM boot | ~1-3 s |
| Go binary init | <1 s |
| Chrome startup | ~3-5 s |
| **Total cold start** | **~15-40 s** |

After first pull, images are cached on the underlying infrastructure. Subsequent starts: ~5-10 s.

**Comparison to GKE Agent Sandbox warm pools:** GKE Agent Sandbox warm pools pre-warm pods for `<1 second` session claim. Cloud Run Jobs have no equivalent warm pool — each task starts cold. For benchmark runs (N tasks launched together), cold start is amortized — tasks start in parallel and the additional startup time (~10-30 s) is negligible compared to a 5-20 minute agent session.

Cold start matters more for interactive use cases. For batch benchmark runs, it is not a material concern.

**Min instances:** Cloud Run Services support min instances (always-warm, billed at idle rate). Cloud Run Jobs have no equivalent — each execution starts fresh. This is correct for Riptide's batch model.

### Pricing Comparison

Cloud Run Jobs pricing (us-central1, as of June 2026):

| Resource | Price |
|---|---|
| vCPU | $0.000018/vCPU-second |
| Memory | $0.000002/GiB-second |
| Free tier | 240,000 vCPU-seconds + 450,000 GiB-seconds/month |

**Per-session cost estimate (2 vCPU, 4 GiB, 15-minute session):**

```
vCPU: 2 × 900s × $0.000018  = $0.0324
Mem:  4 × 900s × $0.000002  = $0.0072
Total per session:             $0.0396
```

**GKE Agent Sandbox comparison (e2-standard-4, 4 vCPU, 16 GB, ~$0.134/hour):**

| Scenario | GKE cost/session | Cloud Run cost/session |
|---|---|---|
| 2 concurrent sessions (100% util.) | $0.134/hour / 2 × 0.25h = **$0.017** | **$0.040** |
| 1 session, node idle 50% | $0.134/hour × 0.25h = **$0.034** | **$0.040** |
| 1 session, node idle 90% | $0.134/hour × 0.25h = **$0.034** | **$0.040** |
| 10 sessions in burst, then idle | $0.134/hr × duration = **variable** | **$0.040 × 10 = $0.40** |

**Analysis:**
- At sustained high utilization (multiple sessions always running), GKE with committed use discounts can be cheaper.
- For sporadic/bursty runs (benchmarks, CI evaluations), Cloud Run Jobs are significantly cheaper because there is no always-on node cost.
- Cloud Run free tier covers ~13,000 session-minutes/month (240,000 vCPU-s / 2 vCPU / 60s × 2 = ~2000 sessions of 1 minute each; for 15-min sessions, ~888 free sessions per month across the project).
- GCS FUSE adds ~$0.02/GiB/month for session storage (negligible).

**Breakeven:** Cloud Run is cost-competitive or cheaper unless you have a dedicated GKE cluster running at >70% utilization continuously. For benchmark workloads that run N×M sessions over hours and then stop, Cloud Run Jobs are the economical choice.

### Parallelism: N Tasks × M Seeds

Cloud Run Jobs support native task parallelism:

```bash
# Run 50 tasks (10 prompts × 5 seeds), up to 10 in parallel
gcloud run jobs create riptide-benchmark \
  --image gcr.io/PROJECT/riptide:latest \
  --tasks 50 \
  --parallelism 10 \
  --task-timeout 30m \
  --cpu 2 --memory 4Gi \
  --service-account riptide-sa@PROJECT.iam.gserviceaccount.com \
  --region us-central1
```

Each task gets `CLOUD_RUN_TASK_INDEX` (0-49) and `CLOUD_RUN_TASK_COUNT` (50) env vars. Riptide can use these to select which prompt/seed combination to run:

```go
taskIdx := os.Getenv("CLOUD_RUN_TASK_INDEX")
// Use taskIdx to index into a benchmark matrix loaded from GCS
```

This replaces the need for a custom parallelism orchestrator. The Cloud Run Jobs API handles scheduling, retries, and task tracking natively.

### Task Timeout

Cloud Run Jobs default task timeout: **10 minutes**. Maximum: **168 hours (7 days)**.

For Riptide sessions (10-30 turns, estimated 5-20 minutes): set `--task-timeout 30m` to be safe. Tasks exceeding the timeout are stopped and can be retried if `--max-retries` is configured.

Note: **maintenance events** can interrupt tasks. During a maintenance event, the task is migrated to another machine (brief pause). Outbound VPC connections may reset. Riptide's Vertex AI client library handles connection resets automatically via gRPC retry logic.

### Security Model

**Cloud Run Gen 2 (microVM) isolation:**
- Each container instance runs in a separate microVM
- Container-to-container and container-to-host isolation via hardware virtualization boundary
- Comparable to Docker on a VM: stronger than Linux namespace isolation alone, weaker than dedicated VM per user
- No gVisor syscall overhead or compatibility issues

**Comparison to other options:**

| | Isolation boundary | Chrome `--no-sandbox` | Overhead |
|---|---|---|---|
| Cloud Run Gen 1 (Services only) | gVisor userspace kernel | Required | Syscall overhead (like GKE sandbox) |
| Cloud Run Gen 2 / Jobs | microVM | Required (container standard) | Near-native |
| GKE Agent Sandbox | gVisor userspace kernel | Required | Syscall overhead |
| Docker | Linux namespaces + seccomp | Required (or custom seccomp) | Near-native |
| Apple Container | Virtualization.framework VM | **Not required** | Near-native |

Cloud Run Gen 2 is **comparable to Docker on a VM**: stronger isolation than raw namespaces, no gVisor overhead. For Riptide's threat model (running an AI agent that browses arbitrary websites), the microVM boundary provides meaningful defense-in-depth.

**Service account scoping:** The job's service account should be granted only `roles/aiplatform.user` (Vertex AI) and `roles/storage.objectAdmin` on the sessions bucket. No other permissions needed.

**No persistent attack surface:** Unlike GKE (persistent nodes) or Cloud Run Services (long-lived instances), Cloud Run Job containers exit after the task completes. Each session starts from a clean container image with no state from prior sessions.

### Known Limitations for Riptide

| Limitation | Impact | Workaround |
|---|---|---|
| `--no-sandbox` required | Chrome internal sandbox disabled | microVM boundary provides isolation |
| No warm pool | Cold start adds ~10-30s per task | Acceptable for batch runs; not suitable for interactive <1s session starts |
| Ephemeral filesystem | Session output lost if container crashes | GCS FUSE volume mount (write-through to GCS) |
| No Docker `--shm-size` equivalent | Chrome IPC may be limited | Mount in-memory volume at `/dev/shm`, or use `--disable-dev-shm-usage` |
| No persistent background processes | Each task starts Chrome fresh | Acceptable; cold start included in task timeout |
| Maintenance events can interrupt tasks | Outbound connections reset briefly | Vertex AI client libraries handle connection resets |
| Task-level retries only | If Chrome crashes mid-session, whole task retries | Implement checkpointing via GCS if sessions are long |

### Deployment Sketch

```bash
# Build and push image (same Dockerfile as Docker option)
docker build -t gcr.io/PROJECT/riptide:latest .
docker push gcr.io/PROJECT/riptide:latest

# Create GCS bucket for session output
gsutil mb -l us-central1 gs://riptide-sessions-PROJECT

# Grant service account access
gcloud iam service-accounts create riptide-sa
gcloud projects add-iam-policy-binding PROJECT \
  --role roles/aiplatform.user \
  --member serviceAccount:riptide-sa@PROJECT.iam.gserviceaccount.com
gsutil iam ch serviceAccount:riptide-sa@PROJECT.iam.gserviceaccount.com:objectAdmin \
  gs://riptide-sessions-PROJECT

# Create the job
gcloud run jobs create riptide-benchmark \
  --image gcr.io/PROJECT/riptide:latest \
  --tasks 1 \
  --task-timeout 30m \
  --cpu 2 --memory 4Gi \
  --execution-environment gen2 \
  --service-account riptide-sa@PROJECT.iam.gserviceaccount.com \
  --set-volumes name=sessions,type=cloud-storage,bucket=riptide-sessions-PROJECT \
  --set-volume-mounts volume=sessions,mount-path=/sessions \
  --region us-central1

# Execute with overrides for benchmark matrix
gcloud run jobs execute riptide-benchmark \
  --tasks 50 \
  --parallelism 10 \
  --update-env-vars BENCHMARK_MODE=true,SEED_COUNT=5 \
  --region us-central1 \
  --wait
```

---

## Cross-Cutting Analysis

### The `--no-sandbox` Question

| Scenario | `--no-sandbox` needed? | What provides isolation? |
|---|---|---|
| Bare host (current) | Yes | Nothing — this is the gap |
| Apple Container | **No** | Virtualization.framework VM |
| Docker + default seccomp | Yes (or custom seccomp profile) | Linux namespaces + cgroups + seccomp |
| Docker + Chrome seccomp profile | **No** | Linux namespaces + Chrome internal sandbox |
| GKE Agent Sandbox (gVisor) | Yes | gVisor Sentry |
| Cloud Run Jobs (Gen 2 microVM) | Yes | microVM boundary |

Apple Container is the only option that restores Chrome's internal sandbox without additional seccomp tuning. A well-tuned Docker seccomp profile can also restore it on Linux. Cloud Run Gen 2 and Docker share the same trade-off: `--no-sandbox` in exchange for container boundary isolation.

### Safety vs Scalability Priority

| Option | Safety priority | Scalability priority |
|---|---|---|
| Apple Container | ✅ High (VM isolation, no `--no-sandbox`) | ❌ Mac-only, RAM-bound parallelism |
| Docker | ✅ Good (namespace + seccomp) | ✅ Path to GKE horizontal scaling |
| GKE Agent Sandbox | ✅ High (gVisor + default deny network) | ✅ Best for continuous prod — autoscaling + warm pools |
| Cloud Run Jobs | ✅ Good (microVM + ephemeral; no persistent state) | ✅ Best for batch/benchmark — native task parallelism, scale to zero |

### Developer → Production Path

```
Recommended:

Local dev (any macOS):
  Docker Desktop → docker build + docker run

Local dev (macOS 26 Apple Silicon):
  Apple Container → container build + container run
  (Use to validate Chrome without --no-sandbox)

CI:
  docker build → push to Artifact Registry

Benchmark / evaluation runs (GCP, batch):
  Same OCI image → Cloud Run Jobs
  gcloud run jobs create + execute --tasks N --parallelism P
  Output → GCS bucket via FUSE volume mount

Production (GCP, continuous):
  Same OCI image → GKE Agent Sandbox node pool
  (Validate Chrome under gVisor in staging first)
  Use for interactive or high-throughput production workloads
  where warm pool sub-second startup matters
```

### What Riptide Actually Needs for Egress

Riptide requires outbound HTTPS access to:
- `us-central1-aiplatform.googleapis.com` (or your Vertex AI region) — model API calls
- Any target URLs the agent visits (e.g., `google.com`, `flights.google.com`) — browser navigation

In GKE Agent Sandbox, these both require explicit egress rules. The Vertex AI endpoint has a known CIDR range; web navigation egress is arbitrary by design. A practical policy: allow `443/TCP` to all destinations; block `25/TCP` (email), `22/TCP` (SSH), and other exfiltration-prone ports.

---

## Issues to File

Based on this research, the following issues are proposed under a new runtime isolation epic:

| Priority | Issue |
|---|---|
| P1 | Dockerfile: package riptide + Chromium into OCI image with `--shm-size` and seccomp |
| P1 | Apple Container local dev guide: Containerfile, invocation, volume mounts |
| P1 | Investigate re-enabling Chrome internal sandbox (custom seccomp profile for Docker; Apple Container) |
| P2 | Validate Chrome under gVisor: benchmark CDP latency, screenshot ops, required flags |
| P2 | GKE Agent Sandbox deployment manifests: SandboxTemplate, SandboxWarmPool, WLI config |
| P2 | Workload Identity for Vertex AI in sandboxed deployments |
| P2 | Ensure `--sessions-dir` works correctly with volume mounts and PVCs |
| P3 | NetworkPolicy design: Default Deny + explicit egress for Vertex AI + web navigation |
| P2 | Cloud Run Jobs: create job definition, GCS FUSE volume mount for session output, Workload Identity setup |
| P2 | Cloud Run Jobs: implement `CLOUD_RUN_TASK_INDEX`-based benchmark matrix dispatch |
| P2 | Cloud Run Jobs: validate headless Chrome in Gen 2 microVM; document required flags and `/dev/shm` strategy |
| P3 | Cloud Run Jobs: cost model validation — run benchmark suite and compare actual vCPU-second cost vs GKE estimate |

---

## Relationship to Riptide's Safety & Scalability Roadmap

The concerns this doc addresses map directly onto the existing epic structure:

- **Safety**: runtime isolation closes the `--no-sandbox` / execution environment gap in `riptide-69z` (Safety Best Practices epic). Specifically, it provides the "secure execution environment" that `riptide-69z.2` begins to document.
- **Scalability**: GKE Agent Sandbox with warm pools and horizontal autoscaling is the production architecture for running many concurrent Riptide sessions — relevant to the benchmark epic (`riptide-ipo`) where running N tasks × M seeds requires parallel execution, and to any future multi-user or API-served deployment of Riptide.
- **Benchmarking**: Cloud Run Jobs adds a new path for the benchmark epic (`riptide-ipo`): N tasks × M seeds can be dispatched as a Cloud Run Job execution with `--tasks N*M --parallelism P`, with output written to GCS. This is operationally simpler than managing a GKE cluster for periodic evaluation runs, and significantly cheaper for bursty/batch workloads.

The two concerns reinforce each other: a well-isolated runtime is both safer and easier to scale, because session isolation prevents one compromised or runaway session from affecting others. Cloud Run Jobs adds a third dimension — operational simplicity — by eliminating cluster management for batch benchmark workloads entirely.
