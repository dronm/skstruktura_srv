BEGIN;

DROP FUNCTION public.material_actions_report_documents(timestamptz, timestamptz, integer[], integer[]);
DROP FUNCTION public.material_actions_report_totals(timestamptz, timestamptz, integer[], integer[]);

COMMIT;
