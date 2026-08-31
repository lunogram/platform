ALTER TABLE campaign_broadcasts DROP COLUMN IF EXISTS variant;
ALTER TABLE campaigns DROP COLUMN IF EXISTS variant_selector;
ALTER TABLE campaigns DROP COLUMN IF EXISTS variants;
DROP INDEX IF EXISTS templates_campaign_variant_locale_idx;
ALTER TABLE templates DROP COLUMN IF EXISTS variant;
