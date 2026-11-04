-- +goose Up
-- Notifications (docs/domain.md#notifications): who follows an issue, one
-- inbox row per user and issue, bumped by each event that notifies, and
-- each user's email settings. The notify worker writes them.

CREATE TABLE subscriptions (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    issue_id     uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    reason       text NOT NULL,
    created_at   timestamptz NOT NULL,
    PRIMARY KEY (issue_id, user_id),
    CONSTRAINT subscriptions_reason CHECK (reason IN ('creator', 'assignee', 'mention', 'commenter', 'manual'))
);

CREATE INDEX subscriptions_user ON subscriptions (user_id);

-- An inbox row stands for an issue: a new event bumps it (unread again,
-- out of the archive and of a snooze). first_event_id is the oldest event
-- not yet read; reason the strongest since then.
CREATE TABLE inbox_items (
    workspace_id     uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    id               uuid PRIMARY KEY,
    user_id          uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    issue_id         uuid NOT NULL REFERENCES issues (id) ON DELETE CASCADE,
    reason           text NOT NULL,
    event_id         uuid NOT NULL, -- the latest event; events may be scrubbed, so no key
    first_event_id   uuid NOT NULL,
    unread_count     integer NOT NULL DEFAULT 1,
    updated_at       timestamptz NOT NULL,
    read_at          timestamptz,
    snoozed_until    timestamptz,
    archived_at      timestamptz,
    emailed_event_id uuid,          -- the latest event an email told of
    UNIQUE (user_id, issue_id),
    CONSTRAINT inbox_items_reason CHECK (reason IN ('assigned', 'mentioned', 'subscribed'))
);

CREATE INDEX inbox_items_user ON inbox_items (user_id, workspace_id, updated_at DESC, id DESC);
CREATE INDEX inbox_items_unemailed ON inbox_items (updated_at)
    WHERE read_at IS NULL AND archived_at IS NULL AND emailed_event_id IS DISTINCT FROM event_id;

-- No row: the defaults (email for unread items, no digest).
CREATE TABLE notification_settings (
    user_id        uuid PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    email          text NOT NULL DEFAULT 'unread',
    digest         boolean NOT NULL DEFAULT false,
    digest_sent_at timestamptz,
    updated_at     timestamptz NOT NULL,
    CONSTRAINT notification_settings_email CHECK (email IN ('unread', 'off'))
);

-- +goose Down
DROP TABLE notification_settings;
DROP TABLE inbox_items;
DROP TABLE subscriptions;
