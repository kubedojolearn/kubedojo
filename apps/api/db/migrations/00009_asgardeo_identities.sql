-- Sign-in moves from GitHub OAuth to Asgardeo, which brokers Google, GitHub and email/password
-- sign-in (ADR-0008). Asgardeo user IDs are UUIDs. Existing GitHub identities are kept but can no
-- longer sign in: the API only accepts asgardeo subjects.

-- +goose Up
ALTER TABLE oauth_identities DROP CONSTRAINT oauth_identities_provider_check;
ALTER TABLE oauth_identities DROP CONSTRAINT oauth_identities_provider_subject_check;
ALTER TABLE oauth_identities ALTER COLUMN provider_subject TYPE varchar(36);
ALTER TABLE oauth_identities ADD CONSTRAINT oauth_identities_subject_check CHECK (
  (provider = 'asgardeo' AND provider_subject ~ '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$')
  OR (provider = 'github' AND provider_subject ~ '^[1-9][0-9]{0,19}$')
);
COMMENT ON COLUMN oauth_identities.provider_login IS
  'Last seen provider username; informational only, never used for identity, never an email address.';

-- +goose Down
-- Asgardeo identities cannot be represented in the old schema; their users lose their sign-in link.
DELETE FROM oauth_identities WHERE provider = 'asgardeo';
ALTER TABLE oauth_identities DROP CONSTRAINT oauth_identities_subject_check;
ALTER TABLE oauth_identities ALTER COLUMN provider_subject TYPE varchar(32);
ALTER TABLE oauth_identities ADD CONSTRAINT oauth_identities_provider_check CHECK (provider IN ('github'));
ALTER TABLE oauth_identities ADD CONSTRAINT oauth_identities_provider_subject_check
  CHECK (provider_subject ~ '^[1-9][0-9]{0,19}$');
COMMENT ON COLUMN oauth_identities.provider_login IS NULL;
