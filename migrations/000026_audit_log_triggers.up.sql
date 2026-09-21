BEGIN;
-- Add trigger to specific table

-- users
DROP TRIGGER IF EXISTS audit_log_users ON users;
CREATE TRIGGER audit_log_users
AFTER INSERT OR UPDATE OR DELETE ON users
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- construction_sites
DROP TRIGGER IF EXISTS audit_log_construction_sites ON construction_sites;
CREATE TRIGGER audit_log_construction_sites
AFTER INSERT OR UPDATE OR DELETE ON construction_sites
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- suppliers
DROP TRIGGER IF EXISTS audit_log_suppliers ON suppliers;
CREATE TRIGGER audit_log_suppliers
AFTER INSERT OR UPDATE OR DELETE ON suppliers
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- materials
DROP TRIGGER IF EXISTS audit_log_materials ON materials;
CREATE TRIGGER audit_log_materials
AFTER INSERT OR UPDATE OR DELETE ON materials
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- material_receipts
DROP TRIGGER IF EXISTS audit_log_material_receipts ON material_receipts;
CREATE TRIGGER audit_log_material_receipts
AFTER INSERT OR UPDATE OR DELETE ON material_receipts
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- material_receipt_items
DROP TRIGGER IF EXISTS audit_log_material_receipt_items ON material_receipt_items;
CREATE TRIGGER audit_log_material_receipt_items
AFTER INSERT OR UPDATE OR DELETE ON material_receipt_items
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- material_transfers
DROP TRIGGER IF EXISTS audit_log_material_transfers ON material_transfers;
CREATE TRIGGER audit_log_material_transfers
AFTER INSERT OR UPDATE OR DELETE ON material_transfers
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- material_transfer_items
DROP TRIGGER IF EXISTS audit_log_material_transfer_items ON material_transfer_items;
CREATE TRIGGER audit_log_material_transfer_items
AFTER INSERT OR UPDATE OR DELETE ON material_transfer_items
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- material_consumptions
DROP TRIGGER IF EXISTS audit_log_material_consumptions ON material_consumptions;
CREATE TRIGGER audit_log_material_consumptions
AFTER INSERT OR UPDATE OR DELETE ON material_consumptions
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

-- material_consumption_items
DROP TRIGGER IF EXISTS audit_log_material_consumption_items ON material_consumption_items;
CREATE TRIGGER audit_log_material_consumption_items
AFTER INSERT OR UPDATE OR DELETE ON material_consumption_items
FOR EACH ROW EXECUTE FUNCTION audit_log_process();

COMMIT;
