BEGIN;

WITH aliases(table_name, column_name, column_alias) AS (
	VALUES
		('users', 'id', 'Идентификатор'),
		('users', 'name', 'Имя пользователя'),
		('users', 'role_id', 'Роль'),
		('users', 'pwd', 'Пароль'),
		('users', 'create_dt', 'Дата создания'),
		('users', 'banned', 'Доступ запрещён'),
		('measure_units', 'id', 'Идентификатор'),
		('measure_units', 'name', 'Наименование'),
		('measure_units', 'name_full', 'Полное наименование'),
		('measure_units', 'is_active', 'Активен'),
		('construction_sites', 'id', 'Идентификатор'),
		('construction_sites', 'name', 'Наименование'),
		('construction_sites', 'is_active', 'Активен'),
		('materials', 'id', 'Идентификатор'),
		('materials', 'name', 'Наименование'),
		('materials', 'name_full', 'Полное наименование'),
		('materials', 'measure_unit_id', 'Единица измерения'),
		('materials', 'is_active', 'Активен'),
		('material_receipts', 'id', 'Идентификатор'),
		('material_receipts', 'date', 'Дата и время'),
		('material_receipts', 'construction_site_id', 'Объект строительства'),
		('material_receipts', 'supplier_id', 'Поставщик'),
		('material_receipts', 'number', 'Номер'),
		('material_receipts', 'comment', 'Комментарий'),
		('material_receipts', 'version', 'Версия'),
		('material_receipt_items', 'id', 'Идентификатор'),
		('material_receipt_items', 'line_num', 'Номер строки'),
		('material_receipt_items', 'material_receipt_id', 'Поступление материалов'),
		('material_receipt_items', 'material_id', 'Материал'),
		('material_receipt_items', 'measure_unit_id', 'Единица измерения'),
		('material_receipt_items', 'quant', 'Количество'),
		('material_receipt_items', 'price', 'Цена'),
		('material_receipt_items', 'amount', 'Сумма'),
		('material_consumptions', 'id', 'Идентификатор'),
		('material_consumptions', 'date', 'Дата и время'),
		('material_consumptions', 'construction_site_id', 'Объект строительства'),
		('material_consumptions', 'comment', 'Комментарий'),
		('material_consumptions', 'version', 'Версия'),
		('material_consumption_items', 'id', 'Идентификатор'),
		('material_consumption_items', 'line_num', 'Номер строки'),
		('material_consumption_items', 'material_consumption_id', 'Списание материалов'),
		('material_consumption_items', 'material_id', 'Материал'),
		('material_consumption_items', 'measure_unit_id', 'Единица измерения'),
		('material_consumption_items', 'quant', 'Количество'),
		('material_transfers', 'id', 'Идентификатор'),
		('material_transfers', 'date', 'Дата и время'),
		('material_transfers', 'source_construction_site_id', 'Объект-отправитель'),
		('material_transfers', 'destination_construction_site_id', 'Объект-получатель'),
		('material_transfers', 'comment', 'Комментарий'),
		('material_transfers', 'version', 'Версия'),
		('material_transfer_items', 'id', 'Идентификатор'),
		('material_transfer_items', 'line_num', 'Номер строки'),
		('material_transfer_items', 'material_transfer_id', 'Перемещение материалов'),
		('material_transfer_items', 'material_id', 'Материал'),
		('material_transfer_items', 'measure_unit_id', 'Единица измерения'),
		('material_transfer_items', 'quant', 'Количество')
)
DELETE FROM public.audit_column_aliases AS target
USING aliases
WHERE target.table_name = aliases.table_name
	AND target.column_name = aliases.column_name
	AND target.column_alias = aliases.column_alias;

COMMIT;
