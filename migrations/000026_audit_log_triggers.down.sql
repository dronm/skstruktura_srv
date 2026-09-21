BEGIN;

-- users
DROP TRIGGER IF EXISTS audit_log_users ON users;

-- construction_sites
DROP TRIGGER IF EXISTS audit_log_construction_sites ON construction_sites;

-- suppliers
DROP TRIGGER IF EXISTS audit_log_suppliers ON suppliers;

-- materials
DROP TRIGGER IF EXISTS audit_log_materials ON materials;

-- material_receipts
DROP TRIGGER IF EXISTS audit_log_material_receipts ON material_receipts;

-- material_receipt_items
DROP TRIGGER IF EXISTS audit_log_material_receipt_items ON material_receipt_items;

-- material_transfers
DROP TRIGGER IF EXISTS audit_log_material_transfers ON material_transfers;

-- material_transfer_items
DROP TRIGGER IF EXISTS audit_log_material_transfer_items ON material_transfer_items;

-- material_consumptions
DROP TRIGGER IF EXISTS audit_log_material_consumptions ON material_consumptions;

-- material_consumption_items
DROP TRIGGER IF EXISTS audit_log_material_consumption_items ON material_consumption_items;

COMMIT;

