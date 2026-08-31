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
-- how campaign variables are stored and edited. Each entry is {key, label}; the
-- label is what the console shows, the key is what a send resolves against.
ALTER TABLE campaigns ADD COLUMN variants JSONB NOT NULL DEFAULT '[]'::jsonb;

-- A Liquid expression evaluated per recipient when a send does not name a
-- variant outright -- e.g. '{{ user.data.tenant }}'. This is what lets a single
-- broadcast over a mixed-tenant list resolve a different brand per recipient.
ALTER TABLE campaigns ADD COLUMN variant_selector TEXT;

-- A broadcast that targets one client pins its variant here; left NULL, the
-- campaign selector decides per recipient.
ALTER TABLE campaign_broadcasts ADD COLUMN variant VARCHAR(255);
