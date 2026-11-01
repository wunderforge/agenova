---
version: "1"
profiles:
  - id: public-research
    taskType: public-research
    description: Read public HTTPS pages to summarize or compare public information, with citations to the supplied pages.
    requestedAccess:
      tools: [web.read]
      resourceScopes: [web:public-pages]
  - id: customer-retention
    taskType: customer-analysis
    description: Read the internal synthetic customer-retention dataset to identify usage declines, inactivity and renewal risk.
    requestedAccess:
      tools: [crm.read]
      resourceScopes: [crm:customer-retention]
---
# Researcher access guide

For public sources use the public-research profile. The web reader runs behind
the Tool Gateway; this task does not require direct network access from the Pod.
The caller should supply a public HTTPS URL; this tool reads pages, not search.

For internal customer analysis use customer-retention. Drafting a follow-up
message does not require sending messages, editing CRM records or browsing the web.
Customer records in this example are synthetic; label them accordingly.

Both capabilities belong to the same reusable Agent, but ordinary single-purpose
tasks should request only one profile. If a task needs both internal data and
external enrichment, or its data source is unclear, ask for clarification rather
than choosing one incomplete profile or requesting everything.

This document is author guidance. It does not grant permissions or override
organization Policy, trusted identity, template ceilings or runtime restrictions.
