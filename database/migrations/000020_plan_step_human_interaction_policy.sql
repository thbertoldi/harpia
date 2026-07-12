-- Declarative human-interaction policy belongs to the catalog PlanStep contract.
-- It is nullable for existing catalog steps that do not declare elicitation or review.
ALTER TABLE plan_template_steps
    ADD COLUMN human_interaction_policy JSONB NOT NULL DEFAULT '{}'::jsonb;
