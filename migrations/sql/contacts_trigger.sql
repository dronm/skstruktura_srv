-- Trigger: contacts_trigger_before on public.contacts

-- DROP TRIGGER contacts_trigger_before ON public.contacts;


CREATE TRIGGER contacts_trigger_before
  BEFORE INSERT OR UPDATE
  ON public.contacts
  FOR EACH ROW
  EXECUTE PROCEDURE public.contacts_process();


