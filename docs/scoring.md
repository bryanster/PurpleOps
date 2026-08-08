# Scoring vocabulary

The scoring vocabulary is the ATT&CK Evaluations model: a detection category
ordinal, descriptive modifiers, and a derived outcome. Every term here is
normative for the UI, the report builder (M6), and the analytics rollups (M5).

## Detection categories

Ordinal: `none` (0) < `telemetry` (1) < `general` (2) < `tactic` (3) <
`technique` (4). Higher is better.

| Label | Ordinal | Meaning |
|---|---|---|
| `none` | 0 | No detection capability. |
| `telemetry` | 1 | Minimal data collected — process creation, network flow. |
| `general` | 2 | Broad-spectrum alerting (AV, EDR default rule). |
| `tactic` | 3 | Detection logic aligned to a tactic (e.g. "credential access"). |
| `technique` | 4 | Technique-specific detection (e.g. T1003.001). |

## Protection levels

| Label | Meaning |
|---|---|
| `blocked` | The attack was fully prevented. |
| `partial` | Some parts were prevented; others executed. |
| `not_blocked` | The attack ran uninhibited. |
| `n/a` | Blue did not report prevention status. |

## Modifiers

Flags that qualify a detection without altering its ordinal. Multi-select;
empty set is valid.

- `alert` — generated an alert.
- `correlated` — correlated with other events.
- `delayed` — detected after a lag (batch, analyst review).
- `config_change` — detected via a configuration change (new rule, tuning).
- `residual_artifact` — detected via post-execution artifacts (logs, forensic).

## Derived outcome

Computed from category × protection. **Never persisted** — queried as a derived
column or computed in the API response.

| | `none` | `telemetry` | `general` | `tactic` | `technique` |
|---|---|---|---|---|---|
| `blocked` | prevented | prevented | prevented | prevented | prevented |
| `partial` | prevented | prevented | prevented | prevented | prevented |
| `not_blocked` | not_detected | detected | detected | detected | detected |
| `n/a` | not_applicable | not_applicable | not_applicable | not_applicable | not_applicable |

### Outcome labels

| Label | Meaning |
|---|---|
| `prevented` | Attack was blocked or partially blocked. Detection category is irrelevant — prevention is the strongest signal. |
| `detected` | Detected but not prevented. The category ordinal describes detection quality. |
| `not_detected` | No detection reported (category is `none`, protection is `not_blocked`). |
| `not_applicable` | Blue did not report (protection is `n/a`). |

## MTTD

Mean Time To Detect = `detected_at − started_at`. Computed only when both
timestamps are set. The HTTP layer enforces `detected_at ≥ started_at` on
write, so inverted values indicate a programming error and are rejected by
the domain function.
