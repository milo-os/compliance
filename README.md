# Compliance

Datum Cloud's compliance service manages the platform's third-party vendor registry and powers the public subprocessor disclosure page.

## What it does

When Datum Cloud works with a vendor that handles personal data — a cloud provider, a payment processor, an analytics tool — that vendor needs to be formally recorded and disclosed. This service automates that process.

Staff add a **Vendor** record with the vendor's name, legal entity, country of incorporation, and a compliance profile describing what personal data they process and why. When the profile is marked **Active**, the service automatically publishes a **Subprocessor** entry — the public disclosure surface that appears on the Datum Cloud website.

## Key concepts

**Vendor** — an internal record representing a third-party company in Datum Cloud's supply chain. Includes the legal entity details, data processing agreement reference, risk tier, and compliance profile.

**Subprocessor** — the public-facing record derived from an active vendor. Contains only information safe for unauthenticated consumption. This is what the marketing site reads from.

**Compliance profile** — the regulatory overlay on a vendor record, capturing what personal data they process, which regions they process it in, the legal transfer mechanism (SCCs, adequacy decision, BCRs), and the current lifecycle phase (Draft → Active).

## Lifecycle

```
Draft → Active
```

Vendors start in `Draft` while the compliance profile is being prepared. Transitioning to `Active` triggers immediate public disclosure. Returning to `Draft` removes the vendor from the public feed.

## License

AGPL-3.0-only
