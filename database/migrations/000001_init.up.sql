CREATE SCHEMA system;

-- system.events is an immutable event log serving as the
-- sole source of truth for the entire system.
-- It must never be mutated and can only be appended to at runtime.
CREATE TABLE system.events (
	-- The highest id is the current version of the system.
	"id" BIGSERIAL PRIMARY KEY,
	-- The event format is:
	-- `{"type": "<TypeName>", "payload": {<...>}}`
	"event" JSONB NOT NULL,
	"time" TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- system.projection_versions maps the projection and handler identifier
-- to their current version.
CREATE TABLE system.projection_versions (
	"id" INT PRIMARY KEY,
	"version" BIGINT NOT NULL
);

-- Service: orders

CREATE SCHEMA service_orders;

CREATE TABLE service_orders.products (
	"id" BIGSERIAL PRIMARY KEY,
	"name" TEXT NOT NULL,
	"stock_amount" INT NOT NULL DEFAULT 0
);

CREATE TABLE service_orders.orders (
	"id" BIGSERIAL PRIMARY KEY,
	"delivery_address" TEXT NOT NULL,
	"placement_time" TIMESTAMPTZ NOT NULL
);

CREATE TABLE service_orders.order_items (
	"order_id" BIGINT NOT NULL REFERENCES service_orders.orders(id),
	"product_id" BIGINT NOT NULL REFERENCES service_orders.products(id),
	"amount" INT NOT NULL,
	PRIMARY KEY (order_id, product_id)
);