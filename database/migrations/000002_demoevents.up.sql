
-- Insert some example domain events into the immutable event log:
INSERT INTO system.events (type, payload, time, vcs_revision) VALUES
	-- Register four products
	(
		'register-product',
		'{
			"id": 1,
			"name": "Apple",
			"sku": "DEMO_A1",
			"description": "Fresh red apple"
		}'::jsonb,
		'2025-07-01T08:00:00Z'::timestamptz,
		'demo-migrations'
	),
	(
		'register-product',
		'{
			"id": 2,
			"name": "Orange",
			"sku": "DEMO_O2",
			"description": "Sweet citrus orange"
		}'::jsonb,
		'2025-07-02T08:05:00Z'::timestamptz,
		'demo-migrations'
	),
	(
		'register-product',
		'{
			"id": 3,
			"name": "Watermelon",
			"sku": "DEMO_W3",
			"description": "Juicy summer watermelon"
		}'::jsonb,
		'2025-07-02T08:10:00Z'::timestamptz,
		'demo-migrations'
	),
	(
		'register-product',
		'{
			"id": 4,
			"name": "Banana",
			"sku": "DEMO_B4",
			"description": "Ripe yellow banana"
		}'::jsonb,
		'2025-07-02T08:15:00Z'::timestamptz,
		'demo-migrations'
	),

	-- Set initial stock levels
	(
		'edit-product-stock',
		'{"product_id":1,"stock":20}'::jsonb,
		'2025-07-02T08:20:00Z'::timestamptz,
		'demo-migrations'
	),
	(
		'edit-product-stock',
		'{"product_id":2,"stock":15}'::jsonb,
		'2025-07-02T08:25:00Z'::timestamptz,
		'demo-migrations'
	),

	-- Place two orders
	(
		'place-order',
		'{
			"id":1,
			"user_id":101,
			"items":[{"product_id":1,"amount":3}],
			"delivery_address":"123 Main St"
		}'::jsonb,
		'2025-07-03T09:00:00Z'::timestamptz,
		'demo-migrations'
	),
	(
		'place-order',
		'{
			"id":2,
			"user_id":102,
			"items":[{
				"product_id": 2,
				"amount": 2
			},{
				"product_id": 1,
				"amount": 5
			}],
			"delivery_address":"456 Oak Ave"
		}'::jsonb,
		'2025-07-03T09:05:00Z'::timestamptz,
		'demo-migrations'
	),

	-- Edit the second order’s address
	(
		'edit-order',
		'{"id":2,"delivery_address":"789 Pine Rd"}'::jsonb,
		'2025-07-04T09:10:00Z'::timestamptz,
		'demo-migrations'
	),

	-- Complete order #1 and cancel order #2
	(
		'complete-order',
		'{"id":1}'::jsonb,
		'2025-07-05T09:15:00Z'::timestamptz,
		'demo-migrations'
	),
	(
		'cancel-order',
		'{"id":2}'::jsonb,
		'2025-07-06T09:20:00Z'::timestamptz,
		'demo-migrations'
	)
;