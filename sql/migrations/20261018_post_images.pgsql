BEGIN;

ALTER TABLE post ADD COLUMN image_object TEXT NOT NULL DEFAULT '';
ALTER TABLE post ADD COLUMN image_change TEXT NOT NULL DEFAULT ''
	CHECK (image_change IN ('', 'added', 'updated', 'deleted'));
ALTER TABLE post_revision ADD COLUMN image_change TEXT NOT NULL DEFAULT ''
	CHECK (image_change IN ('', 'added', 'updated', 'deleted'));
ALTER TABLE post ADD CONSTRAINT post_content_required
	CHECK (btrim(post_text) <> '' OR image_object <> '');

CREATE TABLE post_image_cleanup (
	object_name TEXT PRIMARY KEY,
	not_before TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE FUNCTION queue_post_image_cleanup() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
	IF OLD.image_object <> '' THEN
		INSERT INTO post_image_cleanup (object_name) VALUES (OLD.image_object)
			ON CONFLICT (object_name) DO UPDATE SET not_before = now();
	END IF;
	RETURN OLD;
END;
$$;

CREATE TRIGGER post_image_deleted AFTER DELETE ON post
	FOR EACH ROW EXECUTE FUNCTION queue_post_image_cleanup();
CREATE TRIGGER post_image_replaced AFTER UPDATE OF image_object ON post
	FOR EACH ROW WHEN (OLD.image_object IS DISTINCT FROM NEW.image_object)
	EXECUTE FUNCTION queue_post_image_cleanup();

COMMIT;
