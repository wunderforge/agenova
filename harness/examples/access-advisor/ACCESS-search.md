---
version: "3"
profiles:
  - id: public-research
    taskType: public-research
    description: Search for and read relevant public sources to answer current or factual questions, with citations. The Agent chooses relevant official sources; the caller need not supply URLs.
    requestedAccess:
      tools: [web.search, web.read]
      resourceScopes: [web:public-pages]
  - id: customer-retention
    taskType: customer-insights
    description: Read the internal synthetic customer-retention dataset to identify usage declines, inactivity and renewal risk.
    requestedAccess:
      tools: [crm.read]
      resourceScopes: [crm:customer-retention]
---
# Researcher access guide

Public questions including weather, dates and product research use public-research.
The Agent searches and reads sources without requiring caller-supplied URLs.
Internal customer questions use customer-retention, which reads synthetic CRM.
Never substitute CRM for missing public access. Mixed internal and public needs
require clarification. These profiles request permissions; they grant nothing.
