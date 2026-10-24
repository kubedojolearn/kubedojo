-- Admin read views (Part 18 adds publishing from the browser).

-- name: ListAdminContent :many
-- Every lesson with every imported version, in curriculum order (lessons without versions too).
SELECT c.slug AS course_slug, l.slug AS lesson_slug, l.title, l.current_challenge_version_id,
       v.id AS version_id, v.version, v.content_sha256, v.status, v.imported_at, v.published_at
FROM lessons l
JOIN courses c ON c.id = l.course_id
LEFT JOIN challenge_versions v ON v.lesson_id = l.id
ORDER BY c.display_order, l.display_order, v.version DESC;
