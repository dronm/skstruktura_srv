BEGIN;

	ALTER TABLE materials ADD COLUMN material_type_id int REFERENCES material_types (id);


	UPDATE materials SET material_type_id = 1; -- temp

	ALTER TABLE materials ALTER COLUMN material_type_id SET NOT NULL;

COMMIT;
