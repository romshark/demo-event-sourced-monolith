package server

import (
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/romshark/demo-event-sourced-monolith/service/orders"
)

type Server struct {
	log    *slog.Logger
	mux    *http.ServeMux
	orders *orders.Service
}

func New(log *slog.Logger, orders *orders.Service) *Server {
	s := &Server{
		mux:    http.NewServeMux(),
		orders: orders,
		log:    log,
	}

	s.mux.HandleFunc("GET /orders", s.handleGetOrders)
	s.mux.HandleFunc("POST /orders", s.handlePostOrder)
	s.mux.HandleFunc("DELETE /orders/{id}", s.handleDeleteOrders)
	s.mux.HandleFunc("GET /products", s.handleGetProducts)

	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) handleGetOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := s.orders.GetStandingOrders(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}

	type OrderItem struct {
		ProductID   int64  `json:"product_id"`
		ProductName string `json:"product_name"`
		Amount      int64  `json:"amount"`
	}
	type Order struct {
		ID            int64       `json:"id"`
		Items         []OrderItem `json:"items"`
		PlacementTime time.Time   `json:"placement_time"`
	}

	result := make([]Order, len(orders))
	for i, o := range orders {
		items := make([]OrderItem, len(o.Items))
		for i, t := range o.Items {
			items[i] = OrderItem{
				ProductID:   t.ProductID,
				ProductName: t.ProductName,
				Amount:      t.Amount,
			}
		}
		result[i] = Order{
			ID:            o.ID,
			Items:         items,
			PlacementTime: o.PlacementTime,
		}
	}

	if err := json.NewEncoder(w).Encode(result); err != nil {
		s.log.Error("encoding response JSON", slog.Any("err", err))
	}
}

func (s *Server) handleDeleteOrders(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid ID", 400)
		return
	}

	if err := s.orders.CancelOrder(r.Context(), id); err != nil {
		s.internalError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePostOrder(w http.ResponseWriter, r *http.Request) {
	type reqItem struct {
		ProductID int64 `json:"product_id"`
		Amount    int64 `json:"amount"`
	}
	var req struct {
		DeliveryAddress string    `json:"deliveryAddress"`
		Items           []reqItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if req.DeliveryAddress == "" {
		http.Error(w, "deliveryAddress is required", http.StatusBadRequest)
		return
	}
	if len(req.Items) == 0 {
		http.Error(w, "items list cannot be empty", http.StatusBadRequest)
		return
	}

	// Convert to service.OrderItem
	svcItems := make([]orders.OrderItem, len(req.Items))
	for i, it := range req.Items {
		svcItems[i] = orders.OrderItem{
			ProductID: it.ProductID,
			Amount:    it.Amount,
		}
	}

	id, err := s.orders.PlaceOrder(r.Context(), req.DeliveryAddress, svcItems)
	if err != nil {
		http.Error(w, "failed to place order: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]int64{"id": id}); err != nil {
		s.log.Error("encoding response JSON", slog.Any("err", err))
	}
}

func (s *Server) handleGetProducts(w http.ResponseWriter, r *http.Request) {
	type productResponse struct {
		ID          int64  `json:"id"`
		Name        string `json:"name"`
		StockAmount int64  `json:"stock_amount"`
	}

	prods, err := s.orders.GetProducts(r.Context())
	if err != nil {
		http.Error(w, "failed to get products: "+err.Error(), http.StatusInternalServerError)
		return
	}

	resp := make([]productResponse, len(prods))
	for i, p := range prods {
		resp[i] = productResponse{
			ID:          p.ID,
			Name:        p.Name,
			StockAmount: p.StockAmount,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.log.Error("encoding response JSON", slog.Any("err", err))
	}
}

func (s *Server) internalError(w http.ResponseWriter, err error) {
	log.Printf("ERR: %v", err)
	const code = http.StatusInternalServerError
	http.Error(w, http.StatusText(code), code)
}
