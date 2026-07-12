# Proposal: agent-team-deferred-followups

## Summary

Hold the three scope items deferred out of `agent-team-model` so that change
can archive with a clean delivery record. Each item is already partly built
in `agent-team-model`; this change completes the user-facing surfaces.

## Motivation

`agent-team-model` shipped the tiered/multi-capable agent model, the
opt-in/opt-out capability mechanism, the team-recommendation algorithm, and
the single-publish re-authoring. Three items were intentionally deferred
because they are UI/depth work, not correctness work, and blocking the model
change on them would delay the working opt-out + Sênior tier:

1. Team-recommendation UI (the `RecommendTeam` algorithm exists; no surface).
2. Composable `LinkedInPost` artifact (text + carousel + images in one post).
3. "Choose your team" surface (accept/swap the recommended team).

## Scope

- Render the existing `RecommendTeam` result as a conversational "recommended
  team" card with accept / swap-tier / swap-agent actions.
- Introduce a composable `LinkedInPost` artifact that embeds text + optional
  carousel + optional images, and a single publish step over it.
- These are content/UI surfaces; they must follow Harpia's i18n (en + pt-BR)
  and motion-safety rules.

## Non-goals

- Re-opening the opt-out capability mechanism or the canonical
  `planrules.StepWillRun` predicate (those are done in `agent-team-model`).
- Jr/Pleno seniority tiers (separate follow-up).

## References

- Parent change: `openspec/changes/agent-team-model/`
- Constitution ubiquitous language (Áreas, PlanStep, ExecutorInstallation).
