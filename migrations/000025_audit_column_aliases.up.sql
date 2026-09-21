BEGIN;

WITH aliases(table_name, column_name, column_alias) AS (
	VALUES
		-- Users.
		('users', 'id', 'Идентификатор'),
		('users', 'name', 'Имя пользователя'),
		('users', 'role_id', 'Роль'),
		('users', 'pwd', 'Пароль'),
		('users', 'create_dt', 'Дата создания'),
		('users', 'banned', 'Доступ запрещён'),

		-- Measurement units.
		('measure_units', 'id', 'Идентификатор'),
		('measure_units', 'name', 'Наименование'),
		('measure_units', 'name_full', 'Полное наименование'),
		('measure_units', 'is_active', 'Активен'),

		-- Construction sites.
		('construction_sites', 'id', 'Идентификатор'),
		('construction_sites', 'name', 'Наименование'),
		('construction_sites', 'is_active', 'Активен'),

		-- Materials.
		('materials', 'id', 'Идентификатор'),
		('materials', 'name', 'Наименование'),
		('materials', 'name_full', 'Полное наименование'),
		('materials', 'measure_unit_id', 'Единица измерения'),
		('materials', 'is_active', 'Активен'),

		-- Material receipt headers.
		('material_receipts', 'id', 'Идентификатор'),
		('material_receipts', 'date', 'Дата и время'),
		('material_receipts', 'construction_site_id', 'Объект строительства'),
		('material_receipts', 'supplier_id', 'Поставщик'),
		('material_receipts', 'number', 'Номер'),
		('material_receipts', 'comment', 'Комментарий'),
		('material_receipts', 'version', 'Версия'),

		-- Material receipt items.
		('material_receipt_items', 'id', 'Идентификатор'),
		('material_receipt_items', 'line_num', 'Номер строки'),
		('material_receipt_items', 'material_receipt_id', 'Поступление материалов'),
		('material_receipt_items', 'material_id', 'Материал'),
		('material_receipt_items', 'measure_unit_id', 'Единица измерения'),
		('material_receipt_items', 'quant', 'Количество'),
		('material_receipt_items', 'price', 'Цена'),
		('material_receipt_items', 'amount', 'Сумма'),

		-- Material consumption headers.
		('material_consumptions', 'id', 'Идентификатор'),
		('material_consumptions', 'date', 'Дата и время'),
		('material_consumptions', 'construction_site_id', 'Объект строительства'),
		('material_consumptions', 'comment', 'Комментарий'),
		('material_consumptions', 'version', 'Версия'),

		-- Material consumption items.
		('material_consumption_items', 'id', 'Идентификатор'),
		('material_consumption_items', 'line_num', 'Номер строки'),
		('material_consumption_items', 'material_consumption_id', 'Списание материалов'),
		('material_consumption_items', 'material_id', 'Материал'),
		('material_consumption_items', 'measure_unit_id', 'Единица измерения'),
		('material_consumption_items', 'quant', 'Количество'),

		-- Material transfer headers.
		('material_transfers', 'id', 'Идентификатор'),
		('material_transfers', 'date', 'Дата и время'),
		('material_transfers', 'source_construction_site_id', 'Объект-отправитель'),
		('material_transfers', 'destination_construction_site_id', 'Объект-получатель'),
		('material_transfers', 'comment', 'Комментарий'),
		('material_transfers', 'version', 'Версия'),

		-- Material transfer items.
		('material_transfer_items', 'id', 'Идентификатор'),
		('material_transfer_items', 'line_num', 'Номер строки'),
		('material_transfer_items', 'material_transfer_id', 'Перемещение материалов'),
		('material_transfer_items', 'material_id', 'Материал'),
		('material_transfer_items', 'measure_unit_id', 'Единица измерения'),
		('material_transfer_items', 'quant', 'Количество')
)
INSERT INTO public.audit_column_aliases (
	table_name,
	column_name,
	column_alias,
	is_active
)
SELECT
	aliases.table_name,
	aliases.column_name,
	aliases.column_alias,
	true
FROM aliases
ON CONFLICT (table_name, column_name) DO UPDATE
SET
	column_alias = EXCLUDED.column_alias,
	is_active = true;

COMMIT;
