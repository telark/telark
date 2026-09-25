# Local AI Analyzer — research: lighter, faster, air-gap/connected

Date: 2026-09-24. Scope: `services/analyzer` (Python agent loop over Ollama 0.17.7, CPU-only, 2× m7i-flex.large = 2 vCPU / 8 GiB each). No code was changed. Live numbers below are from the dev cluster and are indicative only (node shared with the whole product).

## TL;DR

1. **The 120 s timeouts are mostly not the model's fault.** Two runtime defects dominate: (a) Ollama runs **4 ggml threads on a 2-vCPU node** (runner log `NumThreads:4`, the chart's `limits.cpu: 4`); ggml busy-waits at barriers, so oversubscription collapses throughput (a 93-token prompt + 80-token answer on `gemma3:270m` timed out at 150 s with default threads and took **2.6 s with `num_thread: 2`**); (b) Ollama's free-memory check is `memory.max − memory.current`, and `memory.current` includes page cache, so any pulled/loaded model's file pages make later loads fail ("requires 3.8 GiB, available 3.5 GiB") until the pod restarts.
2. **Even fixed, a 1.5–4B model cannot run the current 8-step tool loop in seconds on 2 vCPU**: measured prompt eval on `qwen2.5:1.5b` is ~22 tok/s and generation ~10 tok/s (2 threads); one 650-token tool step = ~33 s; the loop re-sends the whole history each step and 0.17.x on CPU re-evaluates it (`/api/chat` KV reuse bug #14780). 8 steps + EMIT ≈ 5–10 min. Also `qwen2.5:1.5b` answered the tool step **in prose, no tool call**.
3. **Target architecture: deterministic first, LLM last and small.** Telark already has the evidence (health, change log, events, workload status). Compute the insight (kind, subject, severity, evidence refs) in code with rules (k8sgpt-style analyzers keyed on `CrashLoopBackOff`/`OOMKilled`/`ImagePullBackOff`/probe/`FailedScheduling`/rollout stuck/config-change-regression = newest incident generation), then make **one** schema-constrained LLM call (no tools, ~700-token pre-compressed facts, `format` JSON, `think:false`, `num_predict ≤ 200`) to write title+summary, or none at all when confidence is high. Rules: sub-second. LLM narration on 2 vCPU: `granite4:350m`/`gemma3:270m`-class ≈ 3–5 s, ~1B class ≈ 10–20 s, 1.5B ≈ 40–80 s (measured 84 s for a full 3-insight EMIT on `qwen2.5:1.5b`).
4. **Default model: `qwen3:1.7b` (Apache-2.0, `tools`+`thinking` tags, 1.4 GB) as the "connected/bigger CPU" default, `granite4:350m`(-h) or `qwen3.5:0.8b` (Apache-2.0, tools) as the tiny default when narration only.** Drop `qwen3:4b` as default (3.8 GiB RAM, minutes on 2 vCPU). Keep tool calling only as an opt-in "deep investigation" mode for ≥4 vCPU or GPU.
5. **Runtime: keep Ollama but bump to the current line (0.34.x, chart otwld 1.83.0)**: it now runs llama-server (prompt cache reuse, `sapphirerapids` AMX variant, `prompt_eval_cached_count`), and set `num_thread` explicitly, `OLLAMA_KV_CACHE_TYPE=q8_0`, `num_ctx 4096`, `keep_alive -1`. llama.cpp `llama-server` directly is the leaner alternative (same GGUFs, `--cache-reuse`, `--json-schema`, `--threads`, no memory-check bug); OpenVINO/vLLM-CPU are not worth it at this size.
6. **Modes (no commercial providers, no keys, ever):** `air-gapped` = no egress, model pre-seeded (PVC/baked image/OCI image volume), `autoPull=false`; `connected` = egress only to `registry.ollama.ai`/Hugging Face to pull open-weight models and to an optional **self-hosted OSS endpoint** the customer runs (their own Ollama/llama.cpp/vLLM on a GPU box, `OLLAMA_HOST`-style URL, no key). Tools/prompts/schema/insight document are identical in both modes.

Tiers: **tiny CPU (default, 2 vCPU/≤3 GiB)** rules + optional 0.3–0.8B narration · **bigger CPU (4+ vCPU/6 GiB)** rules + `qwen3:1.7b` narration, optional tool loop · **GPU** `qwen3:4b`/`qwen3.5:4b`/`gemma4:e4b` tool loop · **self-hosted endpoint** (customer's OSS runtime, connected mode only).

---

## 1. What we measured on the dev cluster (2026-09-24)

Setup: `kubectl port-forward svc/telark-ollama`, Ollama 0.17.7, pod limits cpu 4 / mem 6 GiB (requests cpu 500m / 5 GiB), `OLLAMA_NUM_PARALLEL=1`, ctx 8192, KV f16. Node: m7i-flex.large (2 vCPU; `/proc/cpuinfo` shows `amx_tile amx_int8 avx512_vnni avx512_bf16`). Ollama loaded `libggml-cpu-icelake.so`; the 0.17.7 image ships **no** `sapphirerapids` variant, so AMX is unused. Requests use the analyzer's real `SYSTEM_PROMPT`, `tools.SPECS` and `EMIT_SCHEMA`; the one-shot case feeds ~760 tokens of pre-computed facts and asks for the insights JSON via `format`.

| Model (Q4/Q8 as shipped) | threads | case | prompt tok | prompt eval tok/s | gen tok | gen tok/s | wall | result |
|---|---|---|---|---|---|---|---|---|
| gemma3:270m | auto (4) | narrate 93→80 tok | — | — | — | — | **>150 s timeout** | thread oversubscription |
| gemma3:270m | 2 | narrate | 93 | 347 | 62 | 34.8 | 2.6 s | ok |
| gemma3:270m | 2 | one-shot, full schema (maxLength/maxItems) | 762 | 282 | 308 | 32.6 | 14.9 s | valid JSON (grammar cost ≈ 0) |
| gemma3:270m | 2 | one-shot, schema without maxLength/maxItems | 762 | 292 | 512 | 32.2 | 22.9 s | ran to num_predict, invalid JSON |
| qwen2.5:1.5b | 2 | load | 30 | 329 | 1 | — | 0.5 s (warm) | — |
| qwen2.5:1.5b | 2 | agent step 1 (system+tools) | 647 | **21.9** | 32 | 10.1 | 33 s | **prose, no tool call** |
| qwen2.5:1.5b | 2 | one-shot, full schema | 758 | 21.7 | 397 | 9.5 | 84.6 s | valid JSON, 3 insights |
| qwen3:4b | — | (from the user's run) | model load 11 s, then one `/api/chat` > 120 s | | | | | timeout, 0 steps |

The remaining suite (`qwen3.5:0.8b`, `qwen3:1.7b`, `granite4:350m`, `qwen3:4b`, all with `num_thread 2`) was still running when this report was written; results append to `scratchpad/run_all.log` / `bench2.jsonl` and pulled models are deleted by the script.

Readings:
- **Prompt processing is the bottleneck on 2 vCPU**: ~22 tok/s for a 1.5B model, ~290 tok/s for 270M. The current loop's ~700-token growth per step (system 1.2 KB + tools 1.5 KB + results ≤2 KB each) costs 30 s+ per step at 1.5B before any generation, and 0.17.x re-evaluates the whole history on CPU (`ollama/ollama#14780`).
- **JSON-schema grammar is free** (32.6 vs 32.2 tok/s), and `maxItems`/`maxLength` are what stop a small model from rambling to `num_predict`. Keep them.
- **Thread oversubscription** is catastrophic, consistent with ggml's spin barriers (`ggml-org/llama.cpp#29258`, PR proposing yield after 2048 spins; text-embeddings-inference `#170` measured 6× slower with 2 CPU quota vs cpuset) and Ollama ignoring CFS quota (`ollama/ollama#12396` open, `#17134`: `OLLAMA_NUM_THREAD` env ignored).
- **Page-cache accounting**: `discover/cpu_linux.go` on main still does `FreeMemory = memory.max − memory.current` (fetched 2026-09-24); `memory.current` counts file pages (in-pod: `file 1.05 GB` with no model loaded). Open issues `#15704`, `#15650`, `#11497`, `#10256`; no fix in 0.34.x release notes.

## 2. Models: small, tool-capable, commercially usable

Only Apache-2.0/MIT models qualify (hard rule R1). Non-starters: LFM2/LFM2.5 (LFM Open License: free only under $10 M revenue), xLAM-2 and Hammer 2.1 (CC-BY-NC-4.0), Llama 3.2 (custom licence + BFCL removed the 1B/3B FC entries; 1B scores 0 on ToLeaP), Gemma 3/3n (Gemma Terms, no native tools), qwen2.5:3b (Qwen Research). ToolACE is 8B and Llama-licensed.

| Model | Params / Ollama size | Licence | Tools in Ollama | Tool-calling evidence | RAM at Q4 + 4k KV (est.) | CPU speed evidence |
|---|---|---|---|---|---|---|
| **qwen3:1.7b** | 1.7B / 1.4 GB | Apache-2.0 | yes (`tools thinking`) | BFCL-v3 55.7 overall, 80.2 non-live AST (ToolRM paper, arXiv 2509.11963); Non-live parallel 85 % FP16 / 77.5 % Q4_K_M no-think (arXiv 2608.22472); LFM2.5 card: BFCLv3 46.3 | ~2.0 GB | 181 pp / 40 tg tok/s on a phone CPU Q4_0 (LiquidAI card); expect ~2× qwen2.5:1.5b cost here |
| qwen3:0.6b | 0.6B / 0.5 GB | Apache-2.0 | yes | BFCL non-live parallel **18.5 % at Q4_K_M no-think** (65 % FP16) — breaks when quantised without thinking | ~1 GB | very fast; not usable for tools at Q4 |
| **qwen3.5:0.8b / 2b / 4b** | 0.8B 1.0 GB, 2B 2.7 GB, 4B 3.4 GB | Apache-2.0 | yes (`vision tools thinking`, Ollama 2026-03) | no public BFCL for small sizes; 0.8B "prone to thinking loops" (Artificial Analysis) | 0.8B ~1.6 GB | no CPU numbers published; Unsloth fixed a tool-template bug in GGUFs |
| **granite4:350m(-h) / 1b(-h) / micro(-h)** | 350M 0.7 GB; "1b" is ~1.5–2B, 3.3 GB | Apache-2.0 (ISO 42001) | yes (`tools`) | Granite-4.0-1B **BFCLv3 54.8** (IBM), best sub-2B; LFM2.5 card: 52.4 | 350M ~1 GB | hybrid Mamba-2 `-h` = 70 % less KV; Raspberry Pi 5 demo |
| gemma4:e2b / e4b | Ollama tags 7.2 / 9.6 GB (bf16); Q4 GGUF ~1.5–3 GB | **Apache-2.0** (2026-04) | yes, native FC tokens | E4B "mid-high 80s" BFCL-v4 composite (Ertas, illustrative); E2B T1-Bench tool-call F1 72.8 ≈ E4B 73.6 | E2B Q4 ~3 GB | 133 pp tok/s on Pi 5 (2-bit) |
| phi4-mini 3.8B | 2.5 GB | MIT | yes | trails Qwen3-4B/Gemma4-E4B on BFCL-v4 (Ertas) | ~3.5 GB | 12 tok/s on a desktop CPU |
| SmolLM3-3B | 1.9 GB | Apache-2.0 | community GGUF only | BFCL numbers conflict (32 % vs 92 %) | ~3 GB | — |
| qwen3:4b (current default) | 2.5 GB | Apache-2.0 | yes | BFCL-v3 62.5 overall; top sub-7B (Ertas) | 3.8 GiB (Ollama's own estimate at 8k) | too slow on 2 vCPU (measured) |
| functiongemma (Gemma-3-270M FC finetune) | 0.3 GB | Gemma Terms | yes | fine-tune for FC | — | rejected: licence |

Sources: Ollama library pages (`ollama.com/library/{qwen3,qwen3.5,granite4,gemma4,functiongemma}`), Qwen3 blog (Apache-2.0 for all dense sizes), IBM Granite 4.0 Nano blog (2025-10-28), LiquidAI LFM2.5-1.2B card table, ToolRM (arXiv 2509.11963), "Small Reasoning Models are Instruction Followers in Function Calling" (arXiv 2608.22472), Ertas "On-device tool calling 2026", LFM licence page, xLAM-2/Hammer HF cards.

Takeaways: at ≤1B parameters, native tool calling at Q4 is unreliable (Qwen3-0.6B collapses; even `qwen2.5:1.5b` skipped the call here); structured-output (`format` = grammar) is reliable at every size because the grammar forces the shape. So the small tier must not depend on tool calls.

## 3. Runtime options

| Runtime | Fit for telark | Pros | Cons / evidence |
|---|---|---|---|
| **Ollama 0.34.x (chart otwld 1.83.0, 2026-09-19)** | keep, bump | llama-server backend since 0.30 (2026-05): prompt-cache reuse, `prompt_eval_cached_count` (0.30.2); ships `sapphirerapids` AMX variant; `/v1` OpenAI-compat; `format` JSON schema; `think:false`; pull/registry UX; existing subchart | memory check still page-cache-blind (main); no global thread setting (`num_thread` per request/Modelfile); `sapphirerapids` segfault on m7i reported (#17205, 2026-07; workaround disable variant); prompt cache RAM outside accounting (#18264, `LLAMA_ARG_CACHE_RAM`) |
| **llama.cpp `ghcr.io/ggml-org/llama.cpp:server`** | strong alternative (esp. air-gapped) | one GGUF file on a PVC/image volume; `--threads`, `-np 1`, `--cache-reuse N`, `--json-schema`/grammar, `--jinja` tool parsing, prefix cache on by default; no cgroup memory heuristic; smaller image | no pull UX (telark would fetch GGUFs from HF in connected mode); tool-template quirks (`--jinja` injects a JSON system line); no Helm chart |
| OpenVINO (GenAI / OVMS 2026.3, llama.cpp OpenVINO backend preview) | not now | AMX bf16/int8 kernels; OVMS has OpenAI API + tool parsers (Qwen3/3.5, LFM2.5); EAGLE-3 spec decoding | needs IR conversion per model; llama.cpp OV backend validated only on AI PCs, no dynamic shapes (pp numbers not reproducible in server use); no published Xeon small-model numbers |
| vLLM CPU | no | high concurrency | 2.9× slower than llama.cpp on Ice Lake CPU (Red Hat, 2026-06); heavy image |
| ONNX Runtime GenAI | no | Phi-4-mini int4 CPU builds | only Phi/Llama models, no tools story, few numbers |
| llamafile / MLC | no | single binary | no operational advantage over llama-server |

### Ollama tuning that matters (ordered)
1. **`num_thread` = min(pod CPU limit, node vCPU)** on every request (Ollama keys runners by options, so always send it) — the single largest win measured (>50×). Also set `limits.cpu` ≤ node cores; use Guaranteed QoS with integer CPUs where possible.
2. **Fix the page-cache memory check**: any one of (a) drop `resources.limits.memory` (then `memory.max` = "max" → parse fails → Ollama falls back to host `/proc/meminfo` MemAvailable, which is cache-aware) and keep the request; (b) `keep_alive: -1` so the model is never unloaded and re-checked; (c) size the limit ≥ 2× weights + KV + 0.8 GiB; (d) llama-server (no such check). Note `DELETE /api/delete` frees the cached pages of that model.
3. `num_ctx 4096` (halves KV and prompt-shift risk), `OLLAMA_KV_CACHE_TYPE=q8_0` (needs flash attention; halves KV), `OLLAMA_FLASH_ATTENTION` now auto since 2025-10.
4. Keep the prefix stable (system + tools first, changing facts last) — only pays off on ≥0.30 on CPU.
5. `format` schema with `maxItems`/`maxLength` (measured free) instead of tools for the final answer; `think:false` (format+thinking = empty/invalid output, #10929/#10976/#14645).
6. `num_predict` small (narration 80–200) — generation is 10 tok/s at 1.5B.
7. Speculative decoding: 1.5–2× on CPU for predictable text (llama.cpp docs) — only via llama-server, later.

## 4. Architecture patterns for seconds-level results

| Pattern | Latency on 2 vCPU | Accuracy | Who ships it |
|---|---|---|---|
| **Deterministic analyzers, LLM optional** | ms | high for the 8 insight kinds telark defines | k8sgpt (23 analyzers, `--explain` optional, Apache-2.0); Dynatrace Davis (causal/deterministic, LLM only for narration); MorrisLaw crashloop analyzer |
| **Pre-computed compressed context + one structured call** | 3–80 s depending on model | good; grammar guarantees shape | Datadog Bits (hypotheses from pre-gathered context); Holmes users: "runbooks mattered more than the model" (CNCF 2026-04) |
| Multi-step tool loop (current) | minutes | best with ≥4B; unreliable ≤1.5B | HolmesGPT (Ollama "experimental, tool calling limited"), Komodor Klaudia (Bedrock, 50+ agents), kagent, kubectl-ai, Lens Prism — all assume a capable model or GPU |
| Small classifier/embedding for the long tail | ms | needs labelled data | aws-samples/slemify (CPU triage classifier + encoder head), MetaKube (fine-tuned Qwen3-8B) — not needed for V1 |

Recommended pipeline for `run_analysis`:
1. Gather (already implemented as tools): overview, change history (newest incident generation), workload status, warning events — call them directly in code, no model in the loop (≤1 s, 4 GETs).
2. Rules → candidate insights: `crashloop` (Waiting.reason CrashLoopBackOff / BackOff events; lastTermination OOMKilled → `oom`), `image_pull` (ErrImagePull/ImagePullBackOff), `probe_failure` (Unhealthy events), `scheduling` (FailedScheduling/Pending), `rollout_stuck` (Progressing=False/ProgressDeadlineExceeded, updated < desired), `resource_pressure` (Evicted/node pressure), `config_change_regression` (incident generation within N minutes of a `config` change touching env/image/probes/resources), `other`. Each rule yields kind, subject, severity, confidence, evidence refs from the same ref grammar the schema already uses. This mirrors k8sgpt's `pod.go`/`deployment.go` logic and is deterministic and testable.
3. Optional narration: one `format`-constrained call: `{"title","summary"}` per insight (or the whole `EmitOutput` with kinds/refs pre-filled and only text fields free), `num_predict` ≤ 200, `think:false`, `num_thread` set. Skip when the model runtime is absent/slow; the rule-derived title/summary template is the fallback, so insights never depend on the model (matches the "zero impact when off" rule).
4. Deep mode (opt-in, per cluster): today's tool loop, only when `analyzer.mode=deep` and the runtime tier is ≥4 vCPU or GPU.

Expected wall on 2 vCPU: rules-only ≈ 1 s; + `granite4:350m`/`qwen3.5:0.8b` narration ≈ 5–15 s; + `qwen3:1.7b` ≈ 30–60 s. UI already streams SSE, so "Analyzing…" with rule results shown immediately and prose filled in later is feasible.

## 5. Modes: air-gapped vs connected (no commercial providers, no keys)

Design axis added on request. There are exactly two modes; nothing ever calls a paid API.

| | air-gapped | connected |
|---|---|---|
| Egress | none (NetworkPolicy: DNS + analyzer only) | Ollama pod: 443 to model registries only (`registry.ollama.ai`, `huggingface.co`) — already what `app.ollama.autoPull` toggles |
| Model source | pre-seeded: `ollama.persistentVolume.existingClaim`, baked image (`OLLAMA_MODELS=/models`), or Kubernetes **image volume** (OCI artifact, GA in 1.36; `pullPolicy: Never`) | `ollama pull` of open-weight tags from the catalog; catalog updates |
| Runtime endpoint | in-cluster Ollama/llama-server only | in-cluster **or** a customer-run OSS endpoint (their own Ollama/llama.cpp/vLLM on a GPU host): URL only, no key; the analyzer already speaks Ollama's native API and Ollama/llama-server/vLLM all expose `/v1/chat/completions` |
| Identical across modes | tools, prompts, `EMIT_SCHEMA`, insight document, cooldowns, Settings validation (`/api/show` capabilities) |
| Where the switch lives | `GlobalConfig.spec.ai.mode: air-gapped|connected` (CRD yaml first — CRDs prune unknown fields) + `ai.runtimeUrl` (optional, connected only); Settings UI: radio + URL field, disabled when the chart has `app.ollama.autoPull=false` (chart egress is the hard gate; the CR cannot open egress) |
| Secrets | none; a self-hosted endpoint with basic auth is out of scope (would need a Secret ref) |
| Privacy | air-gapped: nothing leaves; connected: only pull traffic (model name) leaves; analysis data never leaves the cluster in either mode unless the customer points `runtimeUrl` at their own host |

Re-introducing providers: not needed. One OpenAI-compatible client is unnecessary because Ollama's native `/api/chat` is what the analyzer uses; if a customer runs llama-server/vLLM instead of Ollama, the minimal change is a `runtimeKind: ollama|openai-compatible` flag with a thin `/v1/chat/completions` mapping (same messages/tools/JSON-schema `response_format`), ~100 lines in `providers/`. Competitors expose the choice as `--backend`/`baseurl` (k8sgpt), `modelList`+`OPENAI_API_BASE` (HolmesGPT), `ModelConfig` CR (kagent), `ollamaLlmModel` (SUSE Rancher Liz, Ollama-first, air-gapped supported).

## 6. Concrete change list (impact/effort ordered)

| # | Change | Files | Gain | Risk |
|---|---|---|---|---|
| 1 | Send `options.num_thread` = env `ANALYZER_NUM_THREAD` (chart sets it from `ollama.resources.limits.cpu`, default 2) on every chat; set `ollama.resources.limits.cpu: "2"` for the CPU profile | `providers/ollama.py:chat_payload`, `constants.py`/`config.py`, `charts/telark/values.yaml` (`services.analyzer.env`, `ollama.resources`), README env table | >50× on oversubscribed nodes; turns 120 s timeouts into seconds | none; document "must not exceed node vCPU" |
| 2 | Page-cache fix: `keep_alive: -1` (constant `OLLAMA_KEEP_ALIVE`) + `OLLAMA_KEEP_ALIVE=-1` env; remove `ollama.resources.limits.memory` (keep request) or set limit ≥ 2×weights+KV+0.8 GiB; add README note that `DELETE /api/delete` frees cache | `constants.py`, `values.yaml` `ollama.extraEnv`/`resources`, README "Analyzer runtime" | no more "requires more memory" after pulls/reloads | no limit → Burstable QoS; state it |
| 3 | Rules-first analysis: `rules.py` (domain package, thin handler), reuse the existing tool handlers directly; LLM narration one `format` call; keep tool loop behind `ANALYZER_MODE=deep` | `analyzer.py`, new `rules.py`, `prompts/analyzer_prompt.py` (NARRATE prompt + text-only schema), `constants.py`, tests under `tests/` | seconds instead of minutes; deterministic evidence refs; works with any model | rule coverage; keep `other` + low confidence |
| 4 | Catalog/defaults: default `qwen3:1.7b`; add `qwen3.5:0.8b`, `qwen3.5:2b`, `granite4:350m-h`, `granite4:micro-h`, `gemma4:e2b` (Q4 tag) with licences; drop `qwen2.5:3b`; seed GlobalConfig default | `constants.py` LICENSES/DEFAULT_ANALYZER_MODEL, `internal/data/resources/globalconfig` seed, CRD description, README model table | 2–3× less RAM/time than 4b | validate `tools` capability per tag on the pinned Ollama |
| 5 | Ollama pin: chart dep 1.50.0→1.83.0 (0.34.2); test m7i `sapphirerapids` segfault (#17205) and prompt-cache RAM (`LLAMA_ARG_CACHE_RAM=512`) | `charts/telark/Chart.yaml`, `values.yaml` extraEnv | CPU prompt-cache reuse; AMX kernels | version-bump only when asked (GHA bumps); run smoke |
| 6 | Context/KV: `OLLAMA_CONTEXT_LENGTH`/`ANALYZER_CONTEXT_TOKENS` 4096, `OLLAMA_KV_CACHE_TYPE=q8_0`, timeouts `LOOP 60`, `EMIT 60`, `WALL 120` for the default mode | `values.yaml`, README env table | −0.5 GiB RAM; fail fast | 8k needed only in deep mode (make it mode-dependent) |
| 7 | Profiles rewrite in README: tiny CPU (350m–0.8b, req 1 CPU/2 GiB), CPU (1.7b, 2 CPU/3 GiB), GPU (4b), self-hosted endpoint | `charts/telark/README.md`, `INSTALL.md`, `VALUES.md` | honest sizing | docs must ship in the same change |
| 8 | Mode switch: `ai.mode`, `ai.runtimeUrl` in CRD + exporter + Settings; NetworkPolicy already keyed on `autoPull`; egress allow-list to registry hosts (FQDN policies need Cilium/Calico — otherwise keep port-443 rule) | `charts/telark-crds/.../globalconfig.yaml`, exporter `globalconfig` def/builtin, `main.py` config poll, dashboard Settings | explicit air-gap contract | CRD before Go field |
| 9 | Later: llama-server runtime option (`runtimeKind`), speculative decoding, image-volume model delivery (K8s 1.36) | new subchart values | leaner air-gap path | only after 1–4 land |

## 7. Vendors (what they actually ship)

- **k8sgpt** (CNCF, Apache-2.0): 23 rule analyzers, no AI by default; `--explain` with `ollama`/`localai`/custom REST; `--anonymize` masks names (self-admitted gaps). docs.k8sgpt.ai.
- **HolmesGPT/Robusta** (CNCF sandbox): agentic tool loop; Ollama "experimental… tool-calling limited"; CNCF case study 2026-04: runbooks > model. holmesgpt.dev.
- **Komodor Klaudia**: SaaS, Bedrock/Claude, 50+ expert agents + knowledge graph; self-hosted on-prem claimed, no documented local-model/air-gap path. komodor.com.
- **Datadog Bits Investigation**: hypothesis tree over pre-gathered telemetry, benchmarked on labelled incidents; SaaS only.
- **Dynatrace Davis**: causal/deterministic fault tree first, GenAI (CoPilot) only for language; markets "deterministic AI, unlike LLMs".
- **SUSE Rancher "Liz"**: Ollama-first, air-gapped supported, default `gpt-oss:20b` (GPU-class).
- **OpenShift Lightspeed**: BYO vLLM on OpenShift AI (GPU); **kagent/kubectl-ai/Lens Prism**: OpenAI-compatible URL incl. in-cluster Ollama.
- Nobody ships a tool-loop agent sized for 2 vCPU; the ones that work on small CPU are rules-first (k8sgpt) or narration-only.

## 8. Open questions

1. Accept "rules-first, LLM narrates" as the default product behaviour (insights exist even with no model)? This changes the "model proposes, code checks" story in the design doc.
2. Default tiny model: `qwen3:1.7b` (best quality, ~30–60 s narration on 2 vCPU) vs `qwen3.5:0.8b`/`granite4:350m-h` (5–15 s, weaker text)? Suite numbers in `bench2.jsonl` when finished.
3. Ollama bump to 0.34.x (chart 1.83.0) — allowed now, given the m7i `sapphirerapids` segfault report? Or add llama-server as the CPU runtime instead?
4. Drop `limits.memory` on the Ollama pod (Burstable) vs oversize it — which is acceptable for SaaS-MVP sizing?
5. Mode switch: should `ai.mode` be settable from the UI at all, or derived purely from chart values (`app.ollama.autoPull`, `app.ollama.runtimeUrl`) so air-gap is a deploy-time contract?
6. Egress allow-list by FQDN requires a CNI with L7/FQDN policies (Cilium/Calico); is port-443-only acceptable in connected mode?
