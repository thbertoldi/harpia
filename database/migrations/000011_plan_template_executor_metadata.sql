-- Populate executor metadata for weekly-newsletter-linkedin template steps (ADR-012).
-- Without default_executor_sku_key and executor_requirement, slot binding cannot match installations.

UPDATE plan_template_steps
SET default_executor_sku_key = 'rss-news-feed',
    executor_requirement = '{"executor_kind": 2, "connection_type": "rss_feed"}'::jsonb
WHERE plan_template_id = 'a1000000-0000-4000-8000-000000000001'
  AND key = 'fetch-news';

UPDATE plan_template_steps
SET default_executor_sku_key = 'newsletter-writer-senior',
    executor_requirement = '{"executor_kind": 1}'::jsonb
WHERE plan_template_id = 'a1000000-0000-4000-8000-000000000001'
  AND key = 'write-draft';

UPDATE plan_template_steps
SET default_executor_sku_key = 'linkedin-voice-senior',
    executor_requirement = '{"executor_kind": 1}'::jsonb
WHERE plan_template_id = 'a1000000-0000-4000-8000-000000000001'
  AND key = 'adapt-for-linkedin';

UPDATE plan_template_steps
SET default_executor_sku_key = 'linkedin-publish',
    executor_requirement = '{"executor_kind": 2, "connection_type": "oauth_linkedin"}'::jsonb
WHERE plan_template_id = 'a1000000-0000-4000-8000-000000000001'
  AND key = 'publish-linkedin';
