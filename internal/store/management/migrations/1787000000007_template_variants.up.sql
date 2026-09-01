-- A campaign's templates are keyed by locale today. White-labelling adds a
-- second, orthogonal dimension: the same campaign carries one template per
-- (locale, variant) pair, where the empty variant is the house brand every
-- existing row already belongs to. Defaulting to '' rather than NULL keeps the
-- lookup a plain equality match instead of forcing every query to spell out an
-- IS NULL branch.
ALTER TABLE templates ADD COLUMN variant VARCHAR(255) NOT NULL DEFAULT '';

-- Send-time selection narrows by campaign, then variant, then locale, which is
-- exactly this column order. No unique index yet: (campaign_id, locale) has
-- never been constrained, so existing projects may already hold duplicate rows
-- that a unique index over the new triple would refuse to build.
CREATE INDEX templates_campaign_variant_locale_idx ON templates(campaign_id, variant, locale);

-- Variants are declared per campaign and edited on the campaign page, mirroring
-- how campaign variables are stored and edited. The declared set and the rule
-- that picks between them are one concept, so they live in one object:
--
--   {"selector": {"type": "expression", "expression": "{{ user.data.tenant }}"},
--    "options":  [{"key": "acme", "label": "Acme Corp"}]}
--
-- selector is optional; without one, a send that names no variant of its own
-- gets the default variant.
ALTER TABLE campaigns ADD COLUMN variants JSONB NOT NULL DEFAULT '{}'::jsonb;

-- A broadcast may override the campaign's rule, either pinning one variant
-- ({"type": "static", "key": "acme"}) or carrying its own expression for a list
-- that spans several clients. NULL defers to the campaign.
ALTER TABLE campaign_broadcasts ADD COLUMN variant JSONB;
