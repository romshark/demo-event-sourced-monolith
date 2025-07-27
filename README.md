# Demo: Event Sourced Monolithic System

This is a tech demo for a modular monolithic backend system that utilizes an
[event sourced](https://learn.microsoft.com/en-us/azure/architecture/patterns/event-sourcing)
architecture.

The immutable event log (table `system.events`) is the single source of truth
of the entire system. Everything else is either a reactor creating side-effects or
a projection (a cached state aggregate of the events).

## Domain Model

This demo tries to model a simple domain of order and product stock management.
It keeps an inventory of products, allows orders to be placed and makes sure an order
can't be placed if not enough stock is available (invariant). When an order is
either placed or canceled, a confirmation email is sent.

## System Design

At the center of the system is the [Conductor](github.com/romshark/conductor) orchestrator
all services are managed by.
The orchestrator has exclusive authority to append events onto the immutable event log.
The `orders` service performs input data validation and enforces invariants.
The `email` service sends email notifications on certain events.
