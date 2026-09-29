BEGIN;

DELETE FROM public.role_permissions
WHERE role_id = 'supply_manager'::public.role_types
	AND permission_code IN (
		'materialReceipt.create',
		'materialReceipt.list',
		'materialReceipt.detail',
		'materialReceipt.update',
		'materialReceipt.delete',
		'materialReceiptItem.create',
		'materialReceiptItem.list',
		'materialReceiptItem.detail',
		'materialReceiptItem.update',
		'materialReceiptItem.delete',
		'materialConsumption.create',
		'materialConsumption.list',
		'materialConsumption.detail',
		'materialConsumption.update',
		'materialConsumption.delete',
		'materialConsumptionItem.create',
		'materialConsumptionItem.list',
		'materialConsumptionItem.detail',
		'materialConsumptionItem.update',
		'materialConsumptionItem.delete',
		'materialTransfer.create',
		'materialTransfer.list',
		'materialTransfer.detail',
		'materialTransfer.update',
		'materialTransfer.delete',
		'materialTransferItem.create',
		'materialTransferItem.list',
		'materialTransferItem.detail',
		'materialTransferItem.update',
		'materialTransferItem.delete',
		'materialActionReport.list',
		'materialBalance.list',
		'inventoryValuation.view'
	);

COMMIT;
