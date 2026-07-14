-- Private, admin-only freeform note per user. Kept off the shared user payload;
-- served only via admin-gated /users/{name}/notes endpoints.
ALTER TABLE users ADD COLUMN notes TEXT NOT NULL DEFAULT '';
