
-- Add trigger to specific table

-- users
/*
CREATE TRIGGER audit_log_users
AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- construction_sites
CREATE TRIGGER audit_log_construction_sites
AFTER INSERT OR UPDATE OR DELETE ON construction_sites
FOR EACH ROW EXECUTE FUNCTION audit_log_process();
	*/

-- suppliers
CREATE TRIGGER audit_log_suppliers
AFTER INSERT OR UPDATE OR DELETE ON suppliers
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- materials
CREATE TRIGGER audit_log_materials
AFTER INSERT OR UPDATE OR DELETE ON materials
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- material_receipts
CREATE TRIGGER audit_log_material_receipts
AFTER INSERT OR UPDATE OR DELETE ON material_receipts
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- material_receipt_items
CREATE TRIGGER audit_log_material_receipt_items
AFTER INSERT OR UPDATE OR DELETE ON material_receipt_items
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- material_transfers
CREATE TRIGGER audit_log_material_transfers
AFTER INSERT OR UPDATE OR DELETE ON material_transfers
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- material_transfer_items
CREATE TRIGGER audit_log_material_transfer_items
AFTER INSERT OR UPDATE OR DELETE ON material_transfer_items
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- material_consumptions
CREATE TRIGGER audit_log_material_consumptions
AFTER INSERT OR UPDATE OR DELETE ON material_consumptions
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- material_consumption_items
CREATE TRIGGER audit_log_material_consumption_items
AFTER INSERT OR UPDATE OR DELETE ON material_consumption_items
FOR EACH ROW EXECUTE FUNCTION audit_log_process();
