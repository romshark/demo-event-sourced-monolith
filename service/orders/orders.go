package orders

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"log/slog"

	"github.com/romshark/demo-event-sourced-monolith/database"
	"github.com/romshark/demo-event-sourced-monolith/event"
	"github.com/romshark/demo-event-sourced-monolith/pkg/orchestrator"
)

type Service struct {
	log          *slog.Logger
	db           database.Database
	orchestrator *orchestrator.Orchestrator
}

func New(log *slog.Logger, db database.Database) *Service {
	return &Service{log: log, db: db}
}

func (s *Service) ProjectionID() int32 { return 0 }

func (s *Service) Sync(ctx context.Context, e event.Event, tx *database.Tx) error {
	switch e := e.(type) {
	case *event.EventRegisterProduct:
		s.log.Info("applying event RegisterProduct")
		return s.applyRegisterProduct(ctx, e, tx)
	case *event.EventEditProductStock:
		s.log.Info("applying event EditProductStock")
		return s.applyEditProductStock(ctx, e, tx)
	case *event.EventPlaceOrder:
		s.log.Info("applying event PlaceOrder")
		return s.applyPlaceOrder(ctx, e, tx)
	case *event.EventCancelOrder:
		s.log.Info("applying event CancelOrder")
		return s.applyCancelOrder(ctx, e, tx)
	case *event.EventEditOrder:
		s.log.Info("applying event EditOrder")
		return s.applyEditOrder(ctx, e, tx)
	}
	return nil
}

func (s *Service) applyRegisterProduct(
	ctx context.Context, e *event.EventRegisterProduct, tx *database.Tx,
) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO service_orders.products (id, name, stock_amount)
		VALUES ($1, $2, 0)
	`, e.ID, e.Name)
	return err
}

func (s *Service) applyEditProductStock(
	ctx context.Context, e *event.EventEditProductStock, tx *database.Tx,
) error {
	tag, err := tx.Exec(ctx, `
		UPDATE service_orders.products
		SET stock_amount = $2
		WHERE id = $1
	`, e.ProductID, e.Stock)
	if err != nil {
		return fmt.Errorf("update stock: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("product %d not found", e.ProductID)
	}
	return nil
}

var ErrDeliveryAddressEmpty = errors.New("empty delivery address")

func (s *Service) applyPlaceOrder(
	ctx context.Context, e *event.EventPlaceOrder, tx *database.Tx,
) error {
	if strings.TrimSpace(e.DeliveryAddress) == "" {
		return ErrDeliveryAddressEmpty
	}

	productsByID := map[int64]struct{}{}

	// Check each item exists and has enough stock.
	for _, i := range e.Items {
		if _, ok := productsByID[i.ProductID]; ok {
			return fmt.Errorf("repeated product id %d in items", i.ProductID)
		}
		productsByID[i.ProductID] = struct{}{}

		var avail int64
		if err := tx.QueryRow(ctx, `
			SELECT stock_amount
			FROM service_orders.products
			WHERE id = $1
		`, i.ProductID).Scan(&avail); err != nil {
			return fmt.Errorf("lookup product %d: %w", i.ProductID, err)
		}
		if avail < i.Amount {
			return fmt.Errorf("not enough stock for product %d: have %d, need %d",
				i.ProductID, avail, i.Amount)
		}
	}

	// Insert order.
	_, err := tx.Exec(ctx, `
		INSERT INTO service_orders.orders (id, delivery_address, placement_time)
		VALUES ($1, $2, $3)
	`, e.ID, e.DeliveryAddress, e.Time())
	if err != nil {
		return fmt.Errorf("insert order %d: %w", e.ID, err)
	}

	// Insert each order_item and decrement stock.
	for _, i := range e.Items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO service_orders.order_items (order_id, product_id, amount)
			VALUES ($1, $2, $3)
		`, e.ID, i.ProductID, i.Amount); err != nil {
			return fmt.Errorf("insert order_item for order %d, product %d: %w",
				e.ID, i.ProductID, err)
		}
		if _, err := tx.Exec(ctx, `
			UPDATE service_orders.products
			SET stock_amount = stock_amount - $2
			WHERE id = $1
		`, i.ProductID, i.Amount); err != nil {
			return fmt.Errorf("decrement stock for product %d: %w", i.ProductID, err)
		}
	}

	return nil
}

func (s *Service) applyCancelOrder(
	ctx context.Context, e *event.EventCancelOrder, tx *database.Tx,
) error {
	// Ensure order exists.
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM service_orders.orders WHERE id=$1)
	`, e.ID).Scan(&exists); err != nil {
		return fmt.Errorf("check order exists %d: %w", e.ID, err)
	}
	if !exists {
		return fmt.Errorf("order %d not found", e.ID)
	}

	// Restore stock for each item.
	rows, err := tx.Query(ctx, `
		SELECT product_id, amount
		FROM service_orders.order_items
		WHERE order_id = $1
	`, e.ID)
	if err != nil {
		return fmt.Errorf("query items for %d: %w", e.ID, err)
	}
	defer rows.Close()
	var updates []struct {
		pid int64
		amt int64
	}
	for rows.Next() {
		var u struct {
			pid int64
			amt int64
		}
		if err := rows.Scan(&u.pid, &u.amt); err != nil {
			return fmt.Errorf("scan item for %d: %w", e.ID, err)
		}
		updates = append(updates, u)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate items for %d: %w", e.ID, err)
	}

	for _, u := range updates {
		if _, err := tx.Exec(ctx, `
			UPDATE service_orders.products
			SET stock_amount = stock_amount + $2
			WHERE id = $1
		`, u.pid, u.amt); err != nil {
			return fmt.Errorf("restore stock for product %d: %w", u.pid, err)
		}
	}

	// Delete order items.
	if _, err := tx.Exec(ctx, `
		DELETE FROM service_orders.order_items WHERE order_id = $1
	`, e.ID); err != nil {
		return fmt.Errorf("delete items for %d: %w", e.ID, err)
	}

	// Delete order.
	if _, err := tx.Exec(ctx, `
		DELETE FROM service_orders.orders WHERE id = $1
	`, e.ID); err != nil {
		return fmt.Errorf("delete order %d: %w", e.ID, err)
	}
	return nil
}

func (s *Service) applyEditOrder(
	ctx context.Context, e *event.EventEditOrder, tx *database.Tx,
) error {
	// Ensure order exists.
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM service_orders.orders WHERE id=$1)
	`, e.ID).Scan(&exists); err != nil {
		return fmt.Errorf("check order exists %d: %w", e.ID, err)
	}
	if !exists {
		return fmt.Errorf("order %d not found", e.ID)
	}

	// Update delivery address if changed.
	if strings.TrimSpace(e.DeliveryAddress) != "" {
		if _, err := tx.Exec(ctx, `
			UPDATE service_orders.orders
			SET delivery_address = $2
			WHERE id = $1
		`, e.ID, e.DeliveryAddress); err != nil {
			return fmt.Errorf("update delivery address for order %d: %w", e.ID, err)
		}
	}

	return nil
}

type Product struct {
	ID          int64
	Name        string
	StockAmount int64
}

func (s *Service) GetProducts(ctx context.Context) ([]Product, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, name, stock_amount FROM service_orders.products
	`)
	if err != nil {
		return nil, fmt.Errorf("querying products: %w", err)
	}
	defer rows.Close()
	var out []Product
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.StockAmount); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		out = append(out, p)
	}
	return out, nil
}

type Order struct {
	ID            int64
	Items         []OrderItem
	PlacementTime time.Time
}

type OrderItem struct {
	ProductID   int64
	ProductName string
	Amount      int64
}

func (s *Service) GetStandingOrders(ctx context.Context) ([]Order, error) {
	rows, err := s.db.Query(ctx, `
		SELECT
			o.id,
			o.placement_time,
			oi.product_id,
			p.name,
			oi.amount
		FROM service_orders.orders o
		JOIN service_orders.order_items oi ON o.id = oi.id
		JOIN service_orders.products p ON oi.product_id = p.id
		ORDER BY o.placement_time, o.id, oi.id
	`)
	if err != nil {
		return nil, fmt.Errorf("querying standing orders: %w", err)
	}
	defer rows.Close()

	type interim struct {
		id            int64
		placementTime time.Time
		item          OrderItem
	}

	var rowsData []interim
	for rows.Next() {
		var (
			oid           int64
			placementTime time.Time
			pid           int64
			name          string
			amt           int64
		)
		if err := rows.Scan(&oid, &placementTime, &pid, &name, &amt); err != nil {
			return nil, fmt.Errorf("scan order row: %w", err)
		}
		rowsData = append(rowsData, interim{
			id:            oid,
			placementTime: placementTime,
			item: OrderItem{
				ProductID:   pid,
				ProductName: name,
				Amount:      amt,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating orders: %w", err)
	}

	ordersMap := make(map[int64]*Order)
	var ids []int64
	for _, r := range rowsData {
		o, ok := ordersMap[r.id]
		if !ok {
			o = &Order{
				ID:            r.id,
				PlacementTime: r.placementTime,
			}
			ordersMap[r.id] = o
			ids = append(ids, r.id)
		}
		o.Items = append(o.Items, r.item)
	}

	slices.Sort(ids)

	result := make([]Order, 0, len(ids))
	for _, id := range ids {
		result = append(result, *ordersMap[id])
	}
	return result, nil
}

func (s *Service) PlaceOrder(
	ctx context.Context, deliveryAddress string, items []OrderItem,
) (id int64, err error) {
	orderItems := make([]event.OrderItem, len(items))
	for i, x := range items {
		orderItems[i] = event.OrderItem{
			ProductID: x.ProductID,
			Amount:    x.Amount,
		}
	}

	now := time.Now()
	id = now.Unix()
	_, err = s.orchestrator.Publish(ctx, event.New(now, &event.EventPlaceOrder{
		ID:              id,
		UserID:          1,
		Items:           orderItems,
		DeliveryAddress: deliveryAddress,
	}))
	return id, err
}

func (s *Service) CancelOrder(ctx context.Context, id int64) (err error) {
	_, err = s.orchestrator.Publish(ctx, event.New(time.Now(), &event.EventCancelOrder{
		ID: id,
	}))
	return err
}

func (s *Service) EditOrder(
	ctx context.Context, id int64, deliveryAddress string,
) (err error) {
	_, err = s.orchestrator.Publish(ctx, event.New(time.Now(), &event.EventEditOrder{
		ID:              id,
		DeliveryAddress: deliveryAddress,
	}))
	return err
}
