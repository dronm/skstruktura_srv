BEGIN;

CREATE SCHEMA integration_diadoc;

CREATE TABLE integration_diadoc.state (
	id smallint NOT NULL PRIMARY KEY,
	box_id text,
	access_token text,
	access_token_expires_at timestamp with time zone,
	refresh_token text,
	after_index_key text,
	event_timestamp_from_ticks bigint NOT NULL DEFAULT 0,
	updated_at timestamp with time zone NOT NULL DEFAULT now(),
	CHECK (id = 1),
	CHECK (event_timestamp_from_ticks >= 0)
);

COMMENT ON TABLE integration_diadoc.state IS
	'Singleton authorization and event cursor state for the Diadoc integration.';
COMMENT ON COLUMN integration_diadoc.state.refresh_token IS
	'Sensitive OAuth refresh token. Database access to this schema must be restricted.';
COMMENT ON COLUMN integration_diadoc.state.after_index_key IS
	'Opaque BoxEvent.IndexKey used for incremental GetNewEvents calls.';

INSERT INTO integration_diadoc.state (id)
VALUES (1);

INSERT INTO public.permissions (code, description)
VALUES ('diadoc.manage', 'Управление интеграцией с Диадоком')
ON CONFLICT (code) DO UPDATE
SET description = EXCLUDED.description;

INSERT INTO public.role_permissions (role_id, permission_code)
VALUES ('admin'::public.role_types, 'diadoc.manage')
ON CONFLICT DO NOTHING;

COMMIT;
