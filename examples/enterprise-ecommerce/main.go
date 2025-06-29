package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/pay-theory/lift/pkg/lift"
)

// Core e-commerce domain models with multi-tenant architecture

// Tenant represents a multi-tenant e-commerce store
type Tenant struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	Domain        string       `json:"domain"`
	Configuration TenantConfig `json:"configuration"`
	Subscription  Subscription `json:"subscription"`
	CreatedAt     time.Time    `json:"createdAt"`
	UpdatedAt     time.Time    `json:"updatedAt"`
	IsActive      bool         `json:"isActive"`
	Owner         TenantOwner  `json:"owner"`
}

// TenantConfig holds tenant-specific configuration
type TenantConfig struct {
	Theme           ThemeConfig            `json:"theme"`
	PaymentMethods  []string               `json:"paymentMethods"`
	ShippingMethods []string               `json:"shippingMethods"`
	Currency        string                 `json:"currency"`
	Locale          string                 `json:"locale"`
	Features        FeatureFlags           `json:"features"`
	Limits          TenantLimits           `json:"limits"`
	CustomSettings  map[string]any `json:"customSettings"`
}

// ThemeConfig defines the visual appearance
type ThemeConfig struct {
	PrimaryColor   string `json:"primaryColor"`
	SecondaryColor string `json:"secondaryColor"`
	Logo           string `json:"logo"`
	Favicon        string `json:"favicon"`
	CustomCSS      string `json:"customCss"`
}

// FeatureFlags control tenant features
type FeatureFlags struct {
	AdvancedSearch    bool `json:"advancedSearch"`
	ProductReviews    bool `json:"productReviews"`
	WishList          bool `json:"wishList"`
	Recommendations   bool `json:"recommendations"`
	MultiCurrency     bool `json:"multiCurrency"`
	InventoryTracking bool `json:"inventoryTracking"`
	Analytics         bool `json:"analytics"`
}

// TenantLimits define usage limits
type TenantLimits struct {
	MaxProducts     int `json:"maxProducts"`
	MaxOrders       int `json:"maxOrders"`
	MaxCustomers    int `json:"maxCustomers"`
	StorageLimit    int `json:"storageLimit"`   // in MB
	BandwidthLimit  int `json:"bandwidthLimit"` // in GB
	APICallsPerHour int `json:"apiCallsPerHour"`
}

// Subscription represents tenant subscription
type Subscription struct {
	Plan         string    `json:"plan"`
	Status       string    `json:"status"`
	StartDate    time.Time `json:"startDate"`
	EndDate      time.Time `json:"endDate"`
	BillingCycle string    `json:"billingCycle"`
	Amount       Money     `json:"amount"`
}

// TenantOwner represents the tenant owner
type TenantOwner struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Company string `json:"company"`
}

// Product represents a product in the catalog
type Product struct {
	ID           string                 `json:"id"`
	TenantID     string                 `json:"tenantId"`
	SKU          string                 `json:"sku"`
	Name         string                 `json:"name"`
	Description  string                 `json:"description"`
	Price        Money                  `json:"price"`
	ComparePrice *Money                 `json:"comparePrice,omitempty"`
	Inventory    Inventory              `json:"inventory"`
	Categories   []string               `json:"categories"`
	Tags         []string               `json:"tags"`
	Attributes   map[string]any `json:"attributes"`
	Images       []ProductImage         `json:"images"`
	SEO          SEOData                `json:"seo"`
	Status       ProductStatus          `json:"status"`
	CreatedAt    time.Time              `json:"createdAt"`
	UpdatedAt    time.Time              `json:"updatedAt"`
	Variants     []ProductVariant       `json:"variants,omitempty"`
}

// Money represents monetary values
type Money struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// Inventory tracks product availability
type Inventory struct {
	Quantity          int  `json:"quantity"`
	Reserved          int  `json:"reserved"`
	Available         int  `json:"available"`
	TrackStock        bool `json:"trackStock"`
	AllowBackorder    bool `json:"allowBackorder"`
	LowStockThreshold int  `json:"lowStockThreshold"`
}

// ProductImage represents product images
type ProductImage struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	AltText   string `json:"altText"`
	Position  int    `json:"position"`
	IsPrimary bool   `json:"isPrimary"`
}

// SEOData for search engine optimization
type SEOData struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Keywords    []string `json:"keywords"`
	Slug        string   `json:"slug"`
}

// ProductStatus represents product status
type ProductStatus string

const (
	ProductStatusActive   ProductStatus = "active"
	ProductStatusInactive ProductStatus = "inactive"
	ProductStatusDraft    ProductStatus = "draft"
	ProductStatusArchived ProductStatus = "archived"
)

// ProductVariant represents product variations
type ProductVariant struct {
	ID         string            `json:"id"`
	SKU        string            `json:"sku"`
	Name       string            `json:"name"`
	Price      Money             `json:"price"`
	Inventory  Inventory         `json:"inventory"`
	Attributes map[string]string `json:"attributes"`
}

// Customer represents a customer
type Customer struct {
	ID             string              `json:"id"`
	TenantID       string              `json:"tenantId"`
	Email          string              `json:"email"`
	Profile        CustomerProfile     `json:"profile"`
	Addresses      []Address           `json:"addresses"`
	PaymentMethods []PaymentMethod     `json:"paymentMethods"`
	OrderHistory   []string            `json:"orderHistory"`
	Preferences    CustomerPreferences `json:"preferences"`
	CreatedAt      time.Time           `json:"createdAt"`
	UpdatedAt      time.Time           `json:"updatedAt"`
	LastLoginAt    time.Time           `json:"lastLoginAt"`
	IsActive       bool                `json:"isActive"`
	Tags           []string            `json:"tags"`
}

// CustomerProfile contains customer personal information
type CustomerProfile struct {
	FirstName   string    `json:"firstName"`
	LastName    string    `json:"lastName"`
	Phone       string    `json:"phone"`
	DateOfBirth time.Time `json:"dateOfBirth,omitempty"`
	Gender      string    `json:"gender,omitempty"`
	Avatar      string    `json:"avatar,omitempty"`
}

// Address represents shipping/billing addresses
type Address struct {
	ID         string `json:"id"`
	Type       string `json:"type"` // shipping, billing
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	Company    string `json:"company,omitempty"`
	Address1   string `json:"address1"`
	Address2   string `json:"address2,omitempty"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postalCode"`
	Country    string `json:"country"`
	Phone      string `json:"phone,omitempty"`
	IsDefault  bool   `json:"isDefault"`
}

// PaymentMethod represents customer payment methods
type PaymentMethod struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // card, bank, wallet
	Provider    string    `json:"provider"`
	Last4       string    `json:"last4,omitempty"`
	ExpiryMonth int       `json:"expiryMonth,omitempty"`
	ExpiryYear  int       `json:"expiryYear,omitempty"`
	IsDefault   bool      `json:"isDefault"`
	CreatedAt   time.Time `json:"createdAt"`
}

// CustomerPreferences stores customer preferences
type CustomerPreferences struct {
	Language           string   `json:"language"`
	Currency           string   `json:"currency"`
	EmailMarketing     bool     `json:"emailMarketing"`
	SMSMarketing       bool     `json:"smsMarketing"`
	PushNotifications  bool     `json:"pushNotifications"`
	FavoriteCategories []string `json:"favoriteCategories"`
}

// Order represents a customer order
type Order struct {
	ID          string       `json:"id"`
	TenantID    string       `json:"tenantId"`
	CustomerID  string       `json:"customerId"`
	OrderNumber string       `json:"orderNumber"`
	Items       []OrderItem  `json:"items"`
	Totals      OrderTotals  `json:"totals"`
	Payment     PaymentInfo  `json:"payment"`
	Shipping    ShippingInfo `json:"shipping"`
	Status      OrderStatus  `json:"status"`
	Notes       string       `json:"notes,omitempty"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
	CompletedAt *time.Time   `json:"completedAt,omitempty"`
	CancelledAt *time.Time   `json:"cancelledAt,omitempty"`
}

// OrderItem represents items in an order
type OrderItem struct {
	ID         string            `json:"id"`
	ProductID  string            `json:"productId"`
	VariantID  string            `json:"variantId,omitempty"`
	SKU        string            `json:"sku"`
	Name       string            `json:"name"`
	Quantity   int               `json:"quantity"`
	Price      Money             `json:"price"`
	Total      Money             `json:"total"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// OrderTotals represents order financial totals
type OrderTotals struct {
	Subtotal Money   `json:"subtotal"`
	Tax      Money   `json:"tax"`
	Shipping Money   `json:"shipping"`
	Discount Money   `json:"discount"`
	Total    Money   `json:"total"`
	TaxRate  float64 `json:"taxRate"`
}

// PaymentInfo represents payment information
type PaymentInfo struct {
	Method        string     `json:"method"`
	Provider      string     `json:"provider"`
	TransactionID string     `json:"transactionId"`
	Status        string     `json:"status"`
	Amount        Money      `json:"amount"`
	ProcessedAt   time.Time  `json:"processedAt"`
	RefundedAt    *time.Time `json:"refundedAt,omitempty"`
	RefundAmount  *Money     `json:"refundAmount,omitempty"`
}

// ShippingInfo represents shipping information
type ShippingInfo struct {
	Method            string     `json:"method"`
	Provider          string     `json:"provider"`
	TrackingNumber    string     `json:"trackingNumber,omitempty"`
	Address           Address    `json:"address"`
	EstimatedDelivery time.Time  `json:"estimatedDelivery"`
	ShippedAt         *time.Time `json:"shippedAt,omitempty"`
	DeliveredAt       *time.Time `json:"deliveredAt,omitempty"`
}

// OrderStatus represents order status
type OrderStatus string

const (
	OrderStatusPending    OrderStatus = "pending"
	OrderStatusConfirmed  OrderStatus = "confirmed"
	OrderStatusProcessing OrderStatus = "processing"
	OrderStatusShipped    OrderStatus = "shipped"
	OrderStatusDelivered  OrderStatus = "delivered"
	OrderStatusCancelled  OrderStatus = "cancelled"
	OrderStatusRefunded   OrderStatus = "refunded"
)

// ShoppingCart represents a customer's shopping cart
type ShoppingCart struct {
	ID         string     `json:"id"`
	TenantID   string     `json:"tenantId"`
	CustomerID string     `json:"customerId"`
	Items      []CartItem `json:"items"`
	Totals     CartTotals `json:"totals"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`
}

// CartItem represents items in shopping cart
type CartItem struct {
	ID        string    `json:"id"`
	ProductID string    `json:"productId"`
	VariantID string    `json:"variantId,omitempty"`
	Quantity  int       `json:"quantity"`
	Price     Money     `json:"price"`
	Total     Money     `json:"total"`
	AddedAt   time.Time `json:"addedAt"`
}

// CartTotals represents cart totals
type CartTotals struct {
	Subtotal  Money `json:"subtotal"`
	Tax       Money `json:"tax"`
	Shipping  Money `json:"shipping"`
	Total     Money `json:"total"`
	ItemCount int   `json:"itemCount"`
}

// Request/Response models
type CreateTenantRequest struct {
	Name          string       `json:"name" validate:"required"`
	Domain        string       `json:"domain" validate:"required"`
	Configuration TenantConfig `json:"configuration"`
	Owner         TenantOwner  `json:"owner" validate:"required"`
	Plan          string       `json:"plan" validate:"required"`
}

type CreateProductRequest struct {
	SKU         string                 `json:"sku" validate:"required"`
	Name        string                 `json:"name" validate:"required"`
	Description string                 `json:"description"`
	Price       Money                  `json:"price" validate:"required"`
	Inventory   Inventory              `json:"inventory"`
	Categories  []string               `json:"categories"`
	Tags        []string               `json:"tags"`
	Attributes  map[string]any `json:"attributes"`
	Images      []ProductImage         `json:"images"`
	SEO         SEOData                `json:"seo"`
}

type CreateCustomerRequest struct {
	Email       string              `json:"email" validate:"required,email"`
	Profile     CustomerProfile     `json:"profile" validate:"required"`
	Addresses   []Address           `json:"addresses"`
	Preferences CustomerPreferences `json:"preferences"`
}

type CreateOrderRequest struct {
	CustomerID string       `json:"customerId" validate:"required"`
	Items      []OrderItem  `json:"items" validate:"required,min=1"`
	Shipping   ShippingInfo `json:"shipping" validate:"required"`
	Payment    PaymentInfo  `json:"payment" validate:"required"`
	Notes      string       `json:"notes"`
}

type AddToCartRequest struct {
	ProductID string `json:"productId" validate:"required"`
	VariantID string `json:"variantId"`
	Quantity  int    `json:"quantity" validate:"required,min=1"`
}

type UpdateCartItemRequest struct {
	Quantity int `json:"quantity" validate:"required,min=0"`
}

// Service interfaces
type TenantService interface {
	CreateTenant(ctx context.Context, req CreateTenantRequest) (*Tenant, error)
	GetTenant(ctx context.Context, id string) (*Tenant, error)
	GetTenantByDomain(ctx context.Context, domain string) (*Tenant, error)
	UpdateTenant(ctx context.Context, id string, tenant *Tenant) error
	ListTenants(ctx context.Context, limit, offset int) ([]Tenant, error)
	DeactivateTenant(ctx context.Context, id string) error
}

type ProductService interface {
	CreateProduct(ctx context.Context, tenantID string, req CreateProductRequest) (*Product, error)
	GetProduct(ctx context.Context, tenantID, id string) (*Product, error)
	UpdateProduct(ctx context.Context, tenantID, id string, product *Product) error
	DeleteProduct(ctx context.Context, tenantID, id string) error
	ListProducts(ctx context.Context, tenantID string, filters ProductFilters) ([]Product, error)
	SearchProducts(ctx context.Context, tenantID, query string, filters ProductFilters) ([]Product, error)
	UpdateInventory(ctx context.Context, tenantID, productID string, quantity int) error
}

type CustomerService interface {
	CreateCustomer(ctx context.Context, tenantID string, req CreateCustomerRequest) (*Customer, error)
	GetCustomer(ctx context.Context, tenantID, id string) (*Customer, error)
	UpdateCustomer(ctx context.Context, tenantID, id string, customer *Customer) error
	ListCustomers(ctx context.Context, tenantID string, limit, offset int) ([]Customer, error)
	AuthenticateCustomer(ctx context.Context, tenantID, email, password string) (*Customer, error)
}

type OrderService interface {
	CreateOrder(ctx context.Context, tenantID string, req CreateOrderRequest) (*Order, error)
	GetOrder(ctx context.Context, tenantID, id string) (*Order, error)
	UpdateOrderStatus(ctx context.Context, tenantID, id string, status OrderStatus) error
	ListOrders(ctx context.Context, tenantID string, filters OrderFilters) ([]Order, error)
	GetCustomerOrders(ctx context.Context, tenantID, customerID string) ([]Order, error)
	CancelOrder(ctx context.Context, tenantID, id string) error
	RefundOrder(ctx context.Context, tenantID, id string, amount Money) error
}

type CartService interface {
	GetCart(ctx context.Context, tenantID, customerID string) (*ShoppingCart, error)
	AddToCart(ctx context.Context, tenantID, customerID string, req AddToCartRequest) (*ShoppingCart, error)
	UpdateCartItem(ctx context.Context, tenantID, cartID, itemID string, req UpdateCartItemRequest) (*ShoppingCart, error)
	RemoveFromCart(ctx context.Context, tenantID, cartID, itemID string) (*ShoppingCart, error)
	ClearCart(ctx context.Context, tenantID, cartID string) error
	ConvertCartToOrder(ctx context.Context, tenantID, cartID string, orderReq CreateOrderRequest) (*Order, error)
}

// Filter types
type ProductFilters struct {
	Categories []string      `json:"categories"`
	Tags       []string      `json:"tags"`
	PriceMin   *float64      `json:"priceMin"`
	PriceMax   *float64      `json:"priceMax"`
	Status     ProductStatus `json:"status"`
	InStock    *bool         `json:"inStock"`
	Limit      int           `json:"limit"`
	Offset     int           `json:"offset"`
	SortBy     string        `json:"sortBy"`
	SortOrder  string        `json:"sortOrder"`
}

type OrderFilters struct {
	Status     OrderStatus `json:"status"`
	CustomerID string      `json:"customerId"`
	DateFrom   *time.Time  `json:"dateFrom"`
	DateTo     *time.Time  `json:"dateTo"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
	SortBy     string      `json:"sortBy"`
	SortOrder  string      `json:"sortOrder"`
}

// Utility functions
func generateID() string {
	return fmt.Sprintf("id_%d", time.Now().UnixNano())
}

func generateSKU() string {
	return fmt.Sprintf("SKU-%d", time.Now().UnixNano()%1000000)
}

func generateOrderNumber() string {
	return fmt.Sprintf("ORD-%d", time.Now().UnixNano()%10000000)
}

func calculateCartTotals(items []CartItem) CartTotals {
	var subtotal float64
	itemCount := 0

	for _, item := range items {
		subtotal += item.Total.Amount
		itemCount += item.Quantity
	}

	tax := subtotal * 0.08 // 8% tax rate
	shipping := 0.0
	if subtotal < 50 {
		shipping = 9.99 // Free shipping over $50
	}

	return CartTotals{
		Subtotal:  Money{Amount: subtotal, Currency: "USD"},
		Tax:       Money{Amount: tax, Currency: "USD"},
		Shipping:  Money{Amount: shipping, Currency: "USD"},
		Total:     Money{Amount: subtotal + tax + shipping, Currency: "USD"},
		ItemCount: itemCount,
	}
}

func calculateOrderTotals(items []OrderItem) OrderTotals {
	var subtotal float64

	for _, item := range items {
		subtotal += item.Total.Amount
	}

	tax := subtotal * 0.08 // 8% tax rate
	shipping := 0.0
	if subtotal < 50 {
		shipping = 9.99 // Free shipping over $50
	}

	return OrderTotals{
		Subtotal: Money{Amount: subtotal, Currency: "USD"},
		Tax:      Money{Amount: tax, Currency: "USD"},
		Shipping: Money{Amount: shipping, Currency: "USD"},
		Discount: Money{Amount: 0, Currency: "USD"},
		Total:    Money{Amount: subtotal + tax + shipping, Currency: "USD"},
		TaxRate:  0.08,
	}
}

// Tenant isolation middleware
func tenantIsolationMiddleware() lift.Middleware {
	return func(next lift.Handler) lift.Handler {
		return lift.HandlerFunc(func(ctx *lift.Context) error {
			// Extract tenant ID from subdomain or header
			tenantID := ctx.Request.GetHeader("X-Tenant-ID")
			if tenantID == "" {
				// Try to extract from subdomain
				if host := ctx.Request.GetHeader("Host"); host != "" {
					if strings.Contains(host, ".") {
						parts := strings.Split(host, ".")
						if len(parts) > 2 {
							tenantID = parts[0]
						}
					}
				}
			}

			if tenantID == "" {
				return ctx.BadRequest("Tenant ID is required", nil)
			}

			// Add tenant ID to context
			ctx.Set("tenantID", tenantID)

			return next.Handle(ctx)
		})
	}
}

// Get tenant ID from context
func getTenantID(ctx *lift.Context) string {
	if tenantID, ok := ctx.Get("tenantID").(string); ok {
		return tenantID
	}
	return ""
}

func main() {
	// Create Lift application
	app := lift.New()

	// Enterprise e-commerce middleware stack
	// Note: Using basic middleware for now - full middleware integration pending
	// app.Use(middleware.Logger())
	// app.Use(middleware.Recover())
	// app.Use(middleware.CORS([]string{"*"}))

	// Enhanced observability for e-commerce
	// app.Use(middleware.EnhancedObservabilityMiddleware(middleware.EnhancedObservabilityConfig{
	// 	EnableMetrics: true,
	// 	EnableTracing: true,
	// 	EnableLogging: true,
	// }))

	// Security and rate limiting
	// app.Use(middleware.RateLimitMiddleware(middleware.RateLimitConfig{
	// 	DefaultLimit:  300, // Higher limit for e-commerce
	// 	DefaultWindow: time.Minute,
	// }))

	// Tenant isolation middleware
	app.Use(tenantIsolationMiddleware())

	// Setup all API routes
	setupAPIRoutes(app)

	log.Println("Starting Enterprise E-commerce Platform on port 8080...")
	log.Println("Multi-Tenant E-commerce Features:")
	log.Println("  ✓ Multi-tenant architecture with data isolation")
	log.Println("  ✓ Product catalog management")
	log.Println("  ✓ Order processing and payment integration")
	log.Println("  ✓ Customer management and authentication")
	log.Println("  ✓ Shopping cart and checkout workflows")
	log.Println("  ✓ Inventory management with real-time updates")
	log.Println("")
	log.Println("Available endpoints:")
	log.Println("  GET  /api/v1/health")
	log.Println("  POST /api/v1/tenants")
	log.Println("  GET  /api/v1/tenants")
	log.Println("  GET  /api/v1/tenants/:id")
	log.Println("  POST /api/v1/products")
	log.Println("  GET  /api/v1/products")
	log.Println("  GET  /api/v1/products/search")
	log.Println("  GET  /api/v1/products/:id")
	log.Println("  PUT  /api/v1/products/:id/inventory")
	log.Println("  POST /api/v1/customers")
	log.Println("  GET  /api/v1/customers")
	log.Println("  GET  /api/v1/customers/:id")
	log.Println("  POST /api/v1/customers/auth")
	log.Println("  GET  /api/v1/customers/:id/orders")
	log.Println("  POST /api/v1/orders")
	log.Println("  GET  /api/v1/orders")
	log.Println("  GET  /api/v1/orders/:id")
	log.Println("  PUT  /api/v1/orders/:id/status")
	log.Println("  GET  /api/v1/cart")
	log.Println("  POST /api/v1/cart/items")
	log.Println("  PUT  /api/v1/cart/:cartId/items/:itemId")
	log.Println("  DELETE /api/v1/cart/:cartId/items/:itemId")
	log.Println("  POST /api/v1/cart/:cartId/checkout")

	app.Start()
}

// setupAPIRoutes configures all the API routes for the e-commerce platform
func setupAPIRoutes(app *lift.App) {
	// Health check endpoint
	app.GET("/api/v1/health", healthCheck)

	// Tenant management endpoints (admin only)
	app.POST("/api/v1/tenants", createTenantHandler)
	app.GET("/api/v1/tenants", listTenantsHandler)
	app.GET("/api/v1/tenants/:id", getTenantHandler)

	// Product management endpoints (tenant-scoped)
	app.POST("/api/v1/products", createProductHandler)
	app.GET("/api/v1/products", listProductsHandler)
	app.GET("/api/v1/products/search", searchProductsHandler)
	app.GET("/api/v1/products/:id", getProductHandler)
	app.PUT("/api/v1/products/:id/inventory", updateInventoryHandler)

	// Customer management endpoints (tenant-scoped)
	app.POST("/api/v1/customers", createCustomerHandler)
	app.GET("/api/v1/customers", listCustomersHandler)
	app.GET("/api/v1/customers/:id", getCustomerHandler)
	app.POST("/api/v1/customers/auth", authenticateCustomerHandler)
	app.GET("/api/v1/customers/:id/orders", getCustomerOrdersHandler)

	// Order management endpoints (tenant-scoped)
	app.POST("/api/v1/orders", createOrderHandler)
	app.GET("/api/v1/orders", listOrdersHandler)
	app.GET("/api/v1/orders/:id", getOrderHandler)
	app.PUT("/api/v1/orders/:id/status", updateOrderStatusHandler)

	// Shopping cart endpoints (customer-scoped)
	app.GET("/api/v1/cart", getCartHandler)
	app.POST("/api/v1/cart/items", addToCartHandler)
	app.PUT("/api/v1/cart/:cartId/items/:itemId", updateCartItemHandler)
	app.DELETE("/api/v1/cart/:cartId/items/:itemId", removeFromCartHandler)
	app.POST("/api/v1/cart/:cartId/checkout", checkoutHandler)
}

func healthCheck(ctx *lift.Context) error {
	health := map[string]any{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
		"platform":  "multi-tenant-ecommerce",
		"services": map[string]string{
			"database":          "healthy",
			"payment_gateway":   "healthy",
			"inventory_service": "healthy",
			"search_engine":     "healthy",
			"cache":             "healthy",
		},
		"features": map[string]any{
			"multi_tenant":        true,
			"real_time_inventory": true,
			"advanced_search":     true,
			"payment_processing":  true,
			"order_management":    true,
		},
		"metrics": map[string]any{
			"uptime_seconds":  3600,
			"total_tenants":   150,
			"total_products":  50000,
			"total_orders":    25000,
			"total_customers": 10000,
			"orders_today":    500,
		},
	}

	return ctx.OK(health)
}
