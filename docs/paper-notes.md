# Differential Network Analysis Paper Notes

Reference paper:

- Peng Zhang, Aaron Gember-Jacobson, Yueshang Zuo, Yuhao Huang, Xu Liu, and Hao Li.
  "Differential Network Analysis." NSDI 2022.
- USENIX page: https://www.usenix.org/conference/nsdi22/presentation/zhang-peng

This repository implements a prototype inspired by the paper. The implementation
is intentionally staged:

1. Parse topology and normalized configuration inputs into internal facts.
2. Derive forwarding rules from static and connected routes.
3. Compute full reachability facts between edge ports.
4. Later, compare old/new snapshots and add incremental traversal.

The current MVP focuses on static and connected routes. It does not yet
implement Batfish parsing, OSPF, BGP, ACL/header-space equivalence classes,
waypointing, load balancing, or the paper's incremental change-point traversal.

Do not commit a full `pdftotext` extraction of the paper into this repository.
Use the USENIX open-access page above as the source of record.
