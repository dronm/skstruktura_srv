BEGIN;

DROP FUNCTION IF EXISTS public.rg_materials_balance(timestamptz, integer[], integer[]);
DROP FUNCTION IF EXISTS public.rg_materials_balance(integer[], integer[]);
DROP FUNCTION IF EXISTS public.rg_materials_rebuild();
DROP FUNCTION IF EXISTS public.ra_materials_remove_acts(text, bigint);
DROP FUNCTION IF EXISTS public.ra_materials_add_act(timestamptz, text, bigint, integer, integer, numeric);

DROP TRIGGER IF EXISTS ra_materials_reject_update_trigger ON public.ra_materials;
DROP FUNCTION IF EXISTS public.ra_materials_reject_update();

DROP TRIGGER IF EXISTS ra_materials_process_trigger ON public.ra_materials;
DROP FUNCTION IF EXISTS public.ra_materials_process();
DROP FUNCTION IF EXISTS public.rg_materials_apply_delta(timestamptz, integer, integer, numeric);

DROP TABLE IF EXISTS public.rg_materials_current;
DROP TABLE IF EXISTS public.rg_materials_period;
DROP TABLE IF EXISTS public.ra_materials;

COMMIT;
