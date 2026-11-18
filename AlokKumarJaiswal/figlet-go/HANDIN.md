# Submission Summary

## Results

| Run / Baseline | Score | Job | Screenshot |
| --- | --- | --- | --- |
| Oracle | 1.0 | `figlet-equivalence-oracle-01` | [screenshots/oracle.png](screenshots/oracle.png) |
| Nop | 0.0 | `figlet-equivalence-nop-01` | [screenshots/nop.png](screenshots/nop.png) |
| Exact-argument visible lookup cheat | 1/6 (0.16666666666666666, below 0.2) | `figlet-equivalence-cheat-01` | [screenshots/cheat.png](screenshots/cheat.png) |

## Hardening Changes

- Root verifier protects grading files; submitted commands run as player.
- Hidden generation rejects normalized argument equivalents shared with visible cases.

## Reflection

1. No smushing defect appeared in the measured first-draft port; the Big-X implementation/comment discrepancy was a notable source finding, and five visible-only omission mutations were detected.
2. No Docker trap was encountered in this assignment; we avoided the Python-containing Debian Go image and explicitly installed bash in Alpine.

## Evidence and Limitations

See [VALIDATION.md](VALIDATION.md) for evidence and limitations.

Normalized-lookup rewards were offline predictions, not Harbor measurements. Universal security and independently measured task difficulty are not claimed.
