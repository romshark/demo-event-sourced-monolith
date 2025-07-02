# Demo: Event Sourced Monolithic System

This is a tech demo for a modular monolithic backend system that utilizes an
[event sourced](https://learn.microsoft.com/en-us/azure/architecture/patterns/event-sourcing)
architecture.

The immutable event log (table `system.events`) is the sole single source of truth
of the entire system. Everything else is just a projection
(a cached state aggregate of the events).

# Domain Model

This demo tries to model a simple domain or order and product stock management.
It keeps an inventory of products, allows orders to be placed and makes sure an order
can't be placed if not enough stock is available (invariant). When an order is
either placed or canceled, a confirmation email is sent.

# System Design

At the center of the system is the orchestrator all services are managed by.
The orchestrator has exclusive authority to append events onto the immutable event log.
The services it manages are categorized into 2 distinct types:

- Projections (`ProjectionSynchronizer`) are strongly consistent with the event log.
- Handlers (`Handler`) are invoked with an at-least-once delivery guarantee
  after an event is written.

When an event is queued for publishing, the orchestrator tries to update all projections
first and if all updates succeed it irreversibly appends a new event onto the event log
and commits the transaction. If any projection fails to apply an event
(for example it may encounter an invariant) the whole transaction is rolled back and
the event is never appended. Each service is a projection of the event log.

# Design Trade-Offs

This implementation focuses on a monolithic single-process modular system design.
It trades off scalability for strong consistency and simplicity.
Multi-instance support is theoretically relatively simple to implement through
[OCC (Optimistic Concurrency Control)](https://en.wikipedia.org/wiki/Optimistic_concurrency_control)
when every instance of the system tries to optimistically push events while checking
whether their version of the system is up to date, and if not, resync and retry.
