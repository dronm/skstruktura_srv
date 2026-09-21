BEGIN;

	ALTER TABLE public.materials
	ADD COLUMN measure_unit_id integer;

	ALTER TABLE public.materials
	ADD CONSTRAINT materials_measure_unit_id_fkey
	FOREIGN KEY (measure_unit_id)
	REFERENCES public.measure_units(id)
	ON UPDATE CASCADE
	ON DELETE RESTRICT;

	-- Set existing rows to the appropriate unit here.
	UPDATE public.materials
	SET measure_unit_id = 1
	WHERE measure_unit_id IS NULL;

	ALTER TABLE public.materials
	ALTER COLUMN measure_unit_id SET NOT NULL;

COMMIT;
