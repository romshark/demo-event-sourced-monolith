-- CONDUCTOR ------------------------------------------------------------
CREATE SCHEMA system;

-- system.events is an immutable event log serving as the
-- sole source of truth for the entire system.
-- It must never be mutated and can only be appended to at runtime.
CREATE TABLE system.events (
	-- The highest version is the current version of the system.
	"version" BIGSERIAL PRIMARY KEY,
	"type" TEXT NOT NULL,
	-- The payload must contain a JSON object with arbitrary contents.
	"payload" JSONB NOT NULL,
	"time" TIMESTAMPTZ NOT NULL DEFAULT now(),
	-- version control system revision of the event producer.
	"vcs_revision" TEXT
);

-- system.projection_versions maps the projection and reactor identifier
-- to their current version.
CREATE TABLE system.projection_versions (
	"id" INT PRIMARY KEY,
	"version" BIGINT NOT NULL
);

-- Setup event insertion notifier.
CREATE FUNCTION system.notify_event_insert() RETURNS trigger AS $$
BEGIN
  -- Send payload containing the inserted event version.
  PERFORM pg_notify('event_inserted', NEW.version::text);
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER notify_event_insert
	AFTER INSERT ON system.events
	FOR EACH ROW
	EXECUTE FUNCTION system.notify_event_insert();

-------------------------------------------------------------------------

-- Service: orders

CREATE SCHEMA service_orders;

CREATE TABLE service_orders.products (
	"id" BIGSERIAL PRIMARY KEY,
	"name" TEXT NOT NULL,
	"description" TEXT,
	"sku" TEXT NOT NULL,
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
	"price_currency" TEXT NOT NULL,
	"price_min_units" BIGINT NOT NULL,
	PRIMARY KEY (order_id, product_id)
);