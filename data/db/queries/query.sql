-- name: CreateEntity :exec
INSERT INTO entity_state(service, entity_id, image_count, status, max_count)
VALUES ($1, $2, 0, $3, $4);

-- name: DeleteEntity :exec
DELETE FROM entity_state
WHERE service = $1 AND entity_id = $2;

-- name: AddImage :exec
INSERT INTO entity_image_list(service, entity_id, image_path, is_cover)
VALUES ($1, $2, $3, $4);

-- name: IncrementImageCount :exec
UPDATE entity_state
SET image_count = image_count + 1
WHERE service = $1 AND entity_id = $2;

-- name: DeleteImage :exec
DELETE FROM entity_image_list
WHERE image_path = $1;

-- name: DecrementImageCount :exec
UPDATE entity_state
SET image_count = image_count + 1
WHERE service = $1 AND entity_id = $2;

-- name: GetEntityState :one
SELECT *
FROM entity_state
WHERE service = $1 AND entity_id = $2;

-- name: SetStatus :exec
UPDATE entity_state
SET status = $1
WHERE service = $2 AND entity_id = $3;

-- name: GetImageList :many
SELECT *
FROM entity_image_list
WHERE service = $1 AND entity_id = $2;

-- name: GetCoverImage :one
SELECT *
FROM entity_image_list
WHERE service = $1 AND entity_id = $2 AND is_cover = true;