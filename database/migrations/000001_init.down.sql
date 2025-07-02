-- service: Orders
DROP TABLE IF EXISTS service_orders.orders;
DROP TABLE IF EXISTS service_orders.products;
DROP SCHEMA IF EXISTS service_orders;

-- system
DROP TABLE IF EXISTS system.projection_versions;
DROP TABLE IF EXISTS system.events;
DROP SCHEMA IF EXISTS system;
