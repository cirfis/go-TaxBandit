# go-TaxBandit (Unofficial)

[![Go Report Card](https://goreportcard.com/badge/github.com/yourusername/go-TaxBandit)](https://goreportcard.com/report/github.com/yourusername/go-TaxBandit)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL%20v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)

A lightweight, zero-dependency Go implementation for automating corporate tax compliance and e-filing via the [TaxBandits API](https://developer.taxbandits.com/).

This engine is designed for developers, sovereign businesses, and self-hosted environments. It bypasses bloated consumer SaaS subscriptions by compiling down to a single, lightning-fast background binary that securely handles cryptographic JWS authentication, enforces mathematical double-entry invariants, and transmits returns directly to the federal clearinghouse.

> **Disclaimer:** `go-TaxBandit` is an unofficial, community-driven open-source project. It is not affiliated with, endorsed by, or sponsored by SPAN Enterprises or TaxBandits. 

## Features
* **Zero Dependencies:** Built entirely on Go standard libraries (`crypto/hmac`, `net/http`, `net/smtp`). No external package bloat.
* **Configuration-Driven:** Hot-swap wages, periodicity, and tax invariants via a clean JSON config file without recompiling.
* **Double-Entry Enforcement:** Native gRPC integration with [GoDBLedger](https://github.com/darcys22/godbledger) to guarantee internal ledger math is flawless before executing external API requests.
* **Resilient Network Layer:** Built-in exponential backoff and retry logic to survive temporary 503 gateway drops.
* **Native SMTP Auditing:** Automatically dispatches RFC 822 compliant email receipts and error logs after every execution block.

## License & Commercial Use
This software is licensed under the **GNU Affero General Public License v3.0 (AGPLv3)**. 

This ensures the software remains free and open. Any modifications or larger works that incorporate this engine and are interacted with remotely over a computer network (e.g., as a SaaS or web application) must also be open-sourced and distributed under the AGPLv3. See the `LICENSE` file for full details.
