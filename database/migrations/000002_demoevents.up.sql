INSERT INTO system.events (event, time) VALUES
	-- Register three products
	('{"type":"RegisterProduct","payload":{"id":1,"name":"Apple"}}', '2025-07-01T08:00:00Z'::timestamptz),
	('{"type":"RegisterProduct","payload":{"id":2,"name":"Orange"}}', '2025-07-02T08:05:00Z'::timestamptz),
	('{"type":"RegisterProduct","payload":{"id":3,"name":"Watermelon"}}', '2025-07-02T08:10:00Z'::timestamptz),
	('{"type":"RegisterProduct","payload":{"id":4,"name":"Banana"}}', '2025-07-02T08:15:00Z'::timestamptz),

	-- Set initial stock levels
	('{"type":"EditProductStock","payload":{"product_id":1,"stock":20}}', '2025-07-02T08:15:00Z'::timestamptz),
	('{"type":"EditProductStock","payload":{"product_id":2,"stock":15}}', '2025-07-02T08:20:00Z'::timestamptz),

	-- Place two orders
	('{"type":"PlaceOrder","payload":{"id":1,"user_id":101,"items":[{"product_id":1,"amount":3}],"delivery_address":"123 Main St"}}', '2025-07-03T09:00:00Z'::timestamptz),
	('{"type":"PlaceOrder","payload":{"id":2,"user_id":102,"items":[{"product_id":2,"amount":2},{"product_id":1,"amount":5}],"delivery_address":"456 Oak Ave"}}', '2025-07-03T09:05:00Z'::timestamptz),

	-- Edit one order
	('{"type":"EditOrder","payload":{"id":2,"delivery_address":"789 Pine Rd"}}', '2025-07-04T09:10:00Z'::timestamptz),

	-- Complete the first order and cancel the second
	('{"type":"CompleteOrder","payload":{"id":1}}', '2025-07-05T09:15:00Z'::timestamptz),
	('{"type":"CancelOrder","payload":{"id":2}}', '2025-07-06T09:20:00Z'::timestamptz);
