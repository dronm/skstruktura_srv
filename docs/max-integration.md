# MAX backend integration

The MAX bot runs as a separate process from the main `skstruktura` HTTP application:

```bash
make run-max
```

It uses the same PostgreSQL database and the `max_users`, `max_in_messages`, and
`max_out_messages` tables.

## Configuration

The incoming update transport is selected with `max.update_mode`:

- `webhook` for production;
- `long_polling` for local development and testing.

The outgoing `max_out_messages` sender runs in both modes. Only the way incoming
MAX updates reach the shared processor changes.

### Local development with Long Polling

```json
{
	"max": {
		"http_addr": "127.0.0.1:59001",
		"update_mode": "long_polling",
		"bot_token": "...",
		"webhook_url": "",
		"webhook_secret": "",
		"auth_request_ttl": "5m",
		"long_polling_timeout": "30s",
		"long_polling_limit": 100,
		"sender_poll_interval": "2s",
		"sender_notify_reconnect_interval": "5s",
		"sender_lock_timeout": "2m",
		"sender_retry_base_delay": "5s",
		"sender_retry_max_delay": "5m",
		"sender_max_attempts": 10
	}
}
```

Long Polling calls `GET /updates` with the configured timeout/limit and requests
only the update types used by this application. The returned `marker` is kept in
memory and is advanced only after the complete returned batch has been processed
successfully. On restart the initial request intentionally has no marker, as
required by the MAX API.

Before polling starts, `cmd/max` lists all active webhook subscriptions for the bot
and removes them. This is necessary because MAX does not deliver Long Polling
updates while a webhook subscription is active. This cleanup is idempotent.

`long_polling_timeout` must be a whole number of seconds greater than zero and no
more than 90 seconds. `long_polling_limit` must be between 1 and 1000.

The local HTTP webhook server is not started in `long_polling` mode, so
`http_addr`, `webhook_url`, and `webhook_secret` are not needed for incoming events.
They may remain configured, however, which makes switching back to production
webhook mode a one-field change.

### Production with Webhook

```json
{
	"max": {
		"http_addr": "127.0.0.1:59001",
		"update_mode": "webhook",
		"bot_token": "...",
		"webhook_url": "https://example.org/max/webhook",
		"webhook_secret": "...",
		"auth_request_ttl": "5m",
		"long_polling_timeout": "30s",
		"long_polling_limit": 100,
		"sender_poll_interval": "2s",
		"sender_notify_reconnect_interval": "5s",
		"sender_lock_timeout": "2m",
		"sender_retry_base_delay": "5s",
		"sender_retry_max_delay": "5m",
		"sender_max_attempts": 10
	}
}
```

In webhook mode, `cmd/max` configures the MAX webhook subscription and starts the
local HTTP server exposing `POST /webhook`. Normally nginx proxies the public
`max.webhook_url` to this endpoint. The webhook subscription requests:

- `bot_started`
- `bot_stopped`
- `message_created`
- `message_callback`

Both Webhook and Long Polling pass the raw `Update` JSON into the same
`maxbot.Processor`, so registration, phone sharing, authentication callbacks, and
`max_in_messages` persistence are transport-independent.

## MAX user registration

On `bot_started` the complete incoming update is first written to
`max_in_messages`. The MAX user is then inserted/upserted by `max_user_id`.
The upsert refreshes MAX-owned profile data and sets `is_active=true`, but does
not overwrite an existing `contact_id` assigned by an administrator.

If the MAX user has no `contact_id`, a message with a native
`request_contact` button is queued in `max_out_messages`. If a contact is
already linked, a normal welcome message is queued instead.

On `bot_stopped`, the existing MAX user is marked inactive.

## Phone sharing and contact binding

A contact is accepted only from a `message_created` update containing a
`contact` attachment with `vcf_info` and `hash`.

The bot verifies the attachment with HMAC-SHA256 using the bot token as the
key and the normalized vCard text as the message. Unverified manually shared
contacts are not used for identity binding.

The phone number is normalized to the database representation:

```text
+7(922)-269-52-51 -> 79222695251
8 922 269 52 51   -> 79222695251
9222695251        -> 79222695251
```

The contact is inserted idempotently using the unique `contacts.phone` key:

```sql
INSERT INTO public.contacts (...)
VALUES (...)
ON CONFLICT (phone) DO UPDATE
SET phone = EXCLUDED.phone
RETURNING id;
```

The no-op conflict update deliberately preserves the existing contact's name,
post, email, and active state while still returning its ID. A newly created
MAX contact has `employee_post_id = NULL` and `email = NULL`.

The resulting contact ID is assigned to `max_users.contact_id`. Existing MAX
user contact bindings are never overwritten by MAX update data; administrator
bindings remain authoritative.

## Outgoing messages

All bot replies are inserted into `max_out_messages`. The sender inherited
from the meatshop MAX integration uses `LISTEN/NOTIFY`, polling fallback,
row locking, retries, stale-lock recovery, and per-dialog rate limiting. The
business application should use this queue for later notification and MAX
authentication messages as well.

## MAX authentication

Migration `000039_max_auth_requests` adds short-lived browser-to-MAX authentication
requests. The request is bound to the anonymous web session that started it; the
web session identifier is never placed in a MAX callback payload.

Configure the request lifetime with:

```json
{
	"max": {
		"auth_request_ttl": "5m"
	}
}
```

### Start authentication

The public endpoint is:

```http
POST /api/users/login/max/request
Content-Type: application/json

{
	"phone": "+7(922)-269-52-51"
}
```

The phone is normalized to `79222695251`. The service resolves exactly one active
chain:

```text
contacts.phone
  -> max_users.contact_id (active)
  -> entity_contacts.contact_id (entity_type = users, active)
  -> users.id (not banned)
```

If zero or multiple candidate chains exist, no MAX message is sent. The HTTP
response deliberately has the same shape so the anonymous caller cannot use the
endpoint to enumerate registered phones.

A successful request always returns an opaque request ID and the expiry time:

```json
{
	"request_id": "...",
	"status": "pending",
	"expires_at": "..."
}
```

When exactly one candidate is resolved, an outgoing MAX message is queued with
callback buttons `Войти` and `Отклонить`. Their payload contains only the random
request ID and the requested decision; it contains no phone, application user ID,
or web session ID.

Repeating an already resolved phone request from the same browser session reuses its
still-valid pending or approved request instead of sending another MAX message. Starting a request
for another phone expires older pending or approved-but-not-consumed requests for that
browser session.

### MAX callback

`message_callback` updates are persisted in `max_in_messages` like every other
incoming update. The callback is accepted only when its MAX actor equals the
`max_user_id` stored on the authentication request.

Approval also rechecks that:

- the MAX user is still active and still has the same `contact_id`;
- that contact is still actively bound to the same application user through
  `entity_contacts`;
- the application user is not banned;
- the request is still pending and has not expired.

The request becomes `approved` or `declined`. The bot then calls MAX `POST /answers`
to replace the callback message with the result text, removing the active approval
buttons from the dialog.

### Complete authentication in the browser

The frontend polls using:

```http
POST /api/users/login/max/complete
Content-Type: application/json

{
	"request_id": "..."
}
```

Possible non-authenticated states are `pending`, `declined`, `expired`, and
`consumed`.

When the request is `approved`, the service rechecks the current MAX/contact/user
relationship and calls the same internal `loginUser()` function used by password
login. Therefore the normal session data, login audit row, banned-user checks,
device-ban checks, public key, and session lifetime remain shared between password
and MAX authentication.

A successful completion returns:

```json
{
	"status": "authenticated",
	"user": { "...": "normal UserLogin fields" },
	"auth": { "...": "normal webapp auth fields" }
}
```

The database request is then marked `consumed`.
