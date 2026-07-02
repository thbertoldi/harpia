---
stepsCompleted: [1]
inputDocuments: []
session_topic: 'M7 LinkedIn content-generation agent demo'
session_goals: 'Deliver a demoable end-to-end LinkedIn plan by Monday, June 29, 2026: a real agent run that generates content through the plan path. Pricing is out of scope; provider/model work is in scope only if needed for content generation.'
selected_approach: 'Demo-First True E2E for Monday, with future Smart Recommendation Flow from /new'
techniques_used: []
ideas_generated:
  - 'Demo boundary: RSS/input to generated LinkedIn-ready draft, with approval/review step; no actual LinkedIn publish required for Monday.'
  - 'Demo input must be real RSS, not seeded demo content.'
  - 'Plan configuration should allow multiple RSS feeds, ideally through an agent/question prompt that suggests known feed options and accepts custom feed URLs.'
  - 'Candidate Monday demo feeds verified with HTTP 200 on 2026-06-26: https://hnrss.org/frontpage and https://feeds.folha.uol.com.br/esporte/rss091.xml.'
  - 'Suggestion UX should align with M6 Binding Matrix: capture or infer a content topic, then suggest a plan configuration instead of reopening a step-by-step wizard.'
  - 'Eventually, a customer intent like "newsletter about sports" should recommend the LinkedIn plan and preselect sports-oriented feeds; for Monday, this can be a deterministic suggestion path with editable defaults.'
  - 'For Monday, it is acceptable to manually choose the LinkedIn plan first; topic-aware suggestions can happen inside the plan configuration screen.'
  - 'The demo should visibly produce both the intermediate TextDraft/newsletter-style draft and the final LinkedInPostDraft, so the content-generation chain is explicit.'
  - 'Platform Engineer must be able to configure a tenant DeepSeek BYOK credential for the Monday demo.'
  - 'DeepSeek should be treated as an M7 content-generation enabler, not as a pricing milestone.'
  - 'Official DeepSeek API docs checked on 2026-06-26: OpenAI-compatible base URL is https://api.deepseek.com; current model names include deepseek-v4-flash and deepseek-v4-pro, while deepseek-chat/deepseek-reasoner are marked for deprecation on 2026-07-24.'
  - 'DeepSeek should be added as a first-class provider card for M7, not as a generic OpenAI-compatible custom provider.'
  - 'Monday demo must use the full app path: configure the LinkedIn plan in UI, click Run now, let control-plane/Temporal execute RSS plus agents, and show the pending approval in the UI.'
  - 'Selected M7 approach: Demo-First True E2E for Monday. Future direction: smart recommendation flow from /new that can infer "sports newsletter for LinkedIn" and prefill the plan.'
  - 'Integration UI must stop assuming a single installation per integration SKU. News feeds should support several configured RSS feed groups, displayed under an RSS integration type group.'
  - 'Each RSS feed group can contain multiple feed URLs; the LinkedIn plan binds to one RSS feed group for the fetch-news step.'
  - 'Confirmed feed model: each selectable RSS installation is a named feed group containing one or more feed URLs, not one installation per URL.'
  - 'Backend executor_installations already allows multiple installations per tenant/SKU; M7 can fix this at the UI/helper layer without a schema migration.'
context_file: ''
---

# Brainstorming Session Results

**Facilitator:** Codex
**Date:** 2026-06-25 23:23:22 America/Sao_Paulo

## Session Overview

**Topic:** M7 LinkedIn content-generation agent demo
**Goals:** Deliver a demoable end-to-end LinkedIn plan by Monday, June 29, 2026: a real agent run that generates content through the plan path. Pricing is out of scope; provider/model work is in scope only if needed for content generation.

### Context Guidance

M7 should pivot from broad provider/pricing work to the narrowest reliable demonstration of the LinkedIn plan producing real generated content. Existing context shows newsletter and LinkedIn voice agents, RSS and LinkedIn integration code paths, and M6 UI realignment already in place.

### Session Setup

The session will focus on the operational demo boundary first: what must be real for Monday, what can be simulated safely, and which content-generation artifacts need to be visible enough to prove the agent ran end to end.

### Decisions So Far

- The Monday demo should stop at an approval-ready LinkedIn draft. It must show a real agent/content-generation run and review state, but it does not need to publish to LinkedIn.
- The demo should ingest a real RSS feed so the run proves the input integration as well as content generation.
- The plan should support more than one RSS feed in configuration. The setup flow can ask which feeds to consume, show suggested feed options, and accept custom URLs.
- M7 should introduce suggested configuration without breaking the M6 decision to collapse configuration into the Binding Matrix. A "Suggest" affordance or assistant prompt can fill topic-aligned defaults, while the matrix remains the editable source of truth.
- For the Monday demo, `/new` does not need free-text plan recommendation. The user may manually choose the LinkedIn plan, then use topic-aware configuration suggestions inside that plan.
- The run should expose both generated artifacts: `TextDraft` from the newsletter writer and `LinkedInPostDraft` from the LinkedIn voice agent.
- Platform Engineer configuration is in scope for M7: the settings surface must allow storing a tenant DeepSeek API key and selecting the DeepSeek model used by the content agents.
- M7 should add DeepSeek explicitly to the provider catalogue and agent model routing. A generic custom provider endpoint UX remains out of scope.
- The demo path must be full product execution, not CLI or backend-only. The run starts from the app and ends in the app with an approval-ready LinkedIn draft.
- The selected Monday slice is Demo-First True E2E. The later product direction is Smart Recommendation Flow, where `/new` can infer the plan and configuration from natural language.
- Integration setup must be multi-instance. The current UI collapses one installation per SKU, but M7 needs an inventory grouped by integration type, for example an "RSS News Feeds" section containing several named feed groups.
- The chosen RSS unit is a named feed group. Individual URLs are entries inside that group; plan binding selects the group installation.
