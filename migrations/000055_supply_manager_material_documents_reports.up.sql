BEGIN;

WITH required(permission_code) AS (
	VALUES
		('materialReceipt.create'),
		('materialReceipt.list'),
		('materialReceipt.detail'),
		('materialReceipt.update'),
		('materialReceipt.delete'),
		('materialReceiptItem.create'),
		('materialReceiptItem.list'),
		('materialReceiptItem.detail'),
		('materialReceiptItem.update'),
		('materialReceiptItem.delete'),
		('materialConsumption.create'),
		('materialConsumption.list'),
		('materialConsumption.detail'),
		('materialConsumption.update'),
		('materialConsumption.delete'),
		('materialConsumptionItem.create'),
		('materialConsumptionItem.list'),
		('materialConsumptionItem.detail'),
		('materialConsumptionItem.update'),
		('materialConsumptionItem.delete'),
		('materialTransfer.create'),
		('materialTransfer.list'),
		('materialTransfer.detail'),
		('materialTransfer.update'),
		('materialTransfer.delete'),
		('materialTransferItem.create'),
		('materialTransferItem.list'),
		('materialTransferItem.detail'),
		('materialTransferItem.update'),
		('materialTransferItem.delete'),
		('materialActionReport.list'),
		('materialBalance.list'),
		('inventoryValuation.view')
)
INSERT INTO public.role_permissions (role_id, permission_code)
SELECT 'supply_manager'::public.role_types, required.permission_code
FROM required
ON CONFLICT DO NOTHING;

COMMIT;
