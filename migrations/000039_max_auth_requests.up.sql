BEGIN;

CREATE TABLE public.max_auth_requests (
	request_id varchar(64) PRIMARY KEY,
	session_id varchar(128) NOT NULL,
	phone varchar(11) NOT NULL,
	contact_id integer REFERENCES public.contacts(id) ON DELETE SET NULL ON UPDATE CASCADE,
	max_user_id bigint REFERENCES public.max_users(max_user_id) ON DELETE SET NULL ON UPDATE CASCADE,
	user_id integer REFERENCES public.users(id) ON DELETE SET NULL ON UPDATE CASCADE,
	status text NOT NULL DEFAULT 'pending',
	created_at timestamptz NOT NULL DEFAULT now(),
	expires_at timestamptz NOT NULL,
	decided_at timestamptz,
	consumed_at timestamptz,
	CONSTRAINT max_auth_requests_request_id_not_empty CHECK (btrim(request_id) <> ''),
	CONSTRAINT max_auth_requests_session_id_not_empty CHECK (btrim(session_id) <> ''),
	CONSTRAINT max_auth_requests_phone_check CHECK (phone ~ '^7[0-9]{10}$'),
	CONSTRAINT max_auth_requests_status_check CHECK (
		status IN ('pending', 'approved', 'declined', 'expired', 'consumed')
	),
	CONSTRAINT max_auth_requests_expires_after_create CHECK (expires_at > created_at)
);

COMMENT ON TABLE public.max_auth_requests IS
	'Short-lived MAX authentication requests bound to the browser session that initiated them.';
COMMENT ON COLUMN public.max_auth_requests.request_id IS
	'Cryptographically random opaque token also embedded in MAX callback button payloads.';
COMMENT ON COLUMN public.max_auth_requests.session_id IS
	'Anonymous web session that initiated this authentication request.';
COMMENT ON COLUMN public.max_auth_requests.phone IS
	'Normalized Russian phone submitted by the browser.';
COMMENT ON COLUMN public.max_auth_requests.status IS
	'Authentication state: pending, approved, declined, expired, or consumed.';

CREATE INDEX max_auth_requests_session_idx
	ON public.max_auth_requests (session_id, created_at DESC);
CREATE UNIQUE INDEX max_auth_requests_session_active_idx
	ON public.max_auth_requests (session_id)
	WHERE status IN ('pending', 'approved');
CREATE INDEX max_auth_requests_phone_idx
	ON public.max_auth_requests (phone, created_at DESC);
CREATE INDEX max_auth_requests_pending_idx
	ON public.max_auth_requests (expires_at, created_at)
	WHERE status = 'pending';
CREATE INDEX max_auth_requests_max_user_idx
	ON public.max_auth_requests (max_user_id, created_at DESC)
	WHERE max_user_id IS NOT NULL;

COMMIT;
