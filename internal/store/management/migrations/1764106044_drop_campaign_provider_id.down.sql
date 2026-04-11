ALTER TABLE campaigns ADD COLUMN provider_id UUID REFERENCES providers(id);
