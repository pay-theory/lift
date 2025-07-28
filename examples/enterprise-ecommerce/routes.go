package main

import (
	"github.com/pay-theory/lift/pkg/lift"
)

// setupRoutes configures all the API routes for the e-commerce platform
func setupRoutes(app *lift.App) error {
	// API versioning
	api := app.Group("/api/v1")

	// Health check endpoint
	if err := api.GET("/health", healthCheck); err != nil {
		return err
	}

	// Tenant management endpoints (admin only)
	tenants := api.Group("/tenants")
	if err := tenants.POST("", createTenantHandler); err != nil {
		return err
	}
	if err := tenants.GET("", listTenantsHandler); err != nil {
		return err
	}
	if err := tenants.GET("/:id", getTenantHandler); err != nil {
		return err
	}

	// Product management endpoints (tenant-scoped)
	products := api.Group("/products")
	if err := products.POST("", createProductHandler); err != nil {
		return err
	}
	if err := products.GET("", listProductsHandler); err != nil {
		return err
	}
	if err := products.GET("/search", searchProductsHandler); err != nil {
		return err
	}
	if err := products.GET("/:id", getProductHandler); err != nil {
		return err
	}
	if err := products.PUT("/:id/inventory", updateInventoryHandler); err != nil {
		return err
	}

	// Customer management endpoints (tenant-scoped)
	customers := api.Group("/customers")
	if err := customers.POST("", createCustomerHandler); err != nil {
		return err
	}
	if err := customers.GET("", listCustomersHandler); err != nil {
		return err
	}
	if err := customers.GET("/:id", getCustomerHandler); err != nil {
		return err
	}
	if err := customers.POST("/auth", authenticateCustomerHandler); err != nil {
		return err
	}
	if err := customers.GET("/:id/orders", getCustomerOrdersHandler); err != nil {
		return err
	}

	// Order management endpoints (tenant-scoped)
	orders := api.Group("/orders")
	if err := orders.POST("", createOrderHandler); err != nil {
		return err
	}
	if err := orders.GET("", listOrdersHandler); err != nil {
		return err
	}
	if err := orders.GET("/:id", getOrderHandler); err != nil {
		return err
	}
	if err := orders.PUT("/:id/status", updateOrderStatusHandler); err != nil {
		return err
	}

	// Shopping cart endpoints (customer-scoped)
	cart := api.Group("/cart")
	if err := cart.GET("", getCartHandler); err != nil {
		return err
	}
	if err := cart.POST("/items", addToCartHandler); err != nil {
		return err
	}
	if err := cart.PUT("/:cartId/items/:itemId", updateCartItemHandler); err != nil {
		return err
	}
	if err := cart.DELETE("/:cartId/items/:itemId", removeFromCartHandler); err != nil {
		return err
	}
	if err := cart.POST("/:cartId/checkout", checkoutHandler); err != nil {
		return err
	}
	
	return nil
}

// Simplified handler functions
func createTenantHandler(ctx *lift.Context) error {
	var req CreateTenantRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request", 400)
	}

	tenant, err := tenantService.CreateTenant(ctx.Context, req)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to create tenant", 500)
	}

	return ctx.Status(201).JSON(tenant)
}

func listTenantsHandler(ctx *lift.Context) error {
	tenants, err := tenantService.ListTenants(ctx.Context, 20, 0)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to list tenants", 500)
	}

	return ctx.JSON(map[string]any{
		"tenants": tenants,
		"count":   len(tenants),
	})
}

func getTenantHandler(ctx *lift.Context) error {
	tenantID := ctx.Param("id")
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}

	tenant, err := tenantService.GetTenant(ctx.Context, tenantID)
	if err != nil {
		return lift.NotFound("Tenant not found")
	}

	return ctx.JSON(tenant)
}

func createProductHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}

	var req CreateProductRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request", 400)
	}

	product, err := productService.CreateProduct(ctx.Context, tenantID, req)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to create product", 500)
	}

	return ctx.Status(201).JSON(product)
}

func listProductsHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}

	filters := ProductFilters{Limit: 20, Offset: 0}
	products, err := productService.ListProducts(ctx.Context, tenantID, filters)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to list products", 500)
	}

	return ctx.JSON(map[string]any{
		"products": products,
		"count":    len(products),
	})
}

func searchProductsHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	query := ctx.Query("q")

	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}
	if query == "" {
		return lift.NewLiftError("BAD_REQUEST", "Search query is required", 400)
	}

	filters := ProductFilters{Limit: 20, Offset: 0}
	products, err := productService.SearchProducts(ctx.Context, tenantID, query, filters)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Product search failed", 500)
	}

	return ctx.JSON(map[string]any{
		"query":    query,
		"products": products,
		"count":    len(products),
	})
}

func getProductHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	productID := ctx.Param("id")

	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}
	if productID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Product ID is required", 400)
	}

	product, err := productService.GetProduct(ctx.Context, tenantID, productID)
	if err != nil {
		return lift.NotFound("Product not found")
	}

	return ctx.JSON(product)
}

func updateInventoryHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	productID := ctx.Param("id")

	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}
	if productID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Product ID is required", 400)
	}

	var req struct {
		Quantity int `json:"quantity"`
	}

	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request", 400)
	}

	err := productService.UpdateInventory(ctx.Context, tenantID, productID, req.Quantity)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to update inventory", 500)
	}

	return ctx.JSON(map[string]any{
		"productId": productID,
		"quantity":  req.Quantity,
		"updated":   true,
	})
}

func createCustomerHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}

	var req CreateCustomerRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request", 400)
	}

	customer, err := customerService.CreateCustomer(ctx.Context, tenantID, req)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to create customer", 500)
	}

	return ctx.Status(201).JSON(customer)
}

func listCustomersHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}

	customers, err := customerService.ListCustomers(ctx.Context, tenantID, 20, 0)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to list customers", 500)
	}

	return ctx.JSON(map[string]any{
		"customers": customers,
		"count":     len(customers),
	})
}

func getCustomerHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	customerID := ctx.Param("id")

	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}
	if customerID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Customer ID is required", 400)
	}

	customer, err := customerService.GetCustomer(ctx.Context, tenantID, customerID)
	if err != nil {
		return lift.NotFound("Customer not found")
	}

	return ctx.JSON(customer)
}

func authenticateCustomerHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request", 400)
	}

	customer, err := customerService.AuthenticateCustomer(ctx.Context, tenantID, req.Email, req.Password)
	if err != nil {
		return lift.Unauthorized("Invalid credentials")
	}

	return ctx.JSON(map[string]any{
		"customer": customer,
		"token":    "jwt_token_here",
	})
}

func createOrderHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}

	var req CreateOrderRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request", 400)
	}

	order, err := orderService.CreateOrder(ctx.Context, tenantID, req)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to create order", 500)
	}

	return ctx.Status(201).JSON(order)
}

func listOrdersHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}

	filters := OrderFilters{Limit: 20, Offset: 0}
	orders, err := orderService.ListOrders(ctx.Context, tenantID, filters)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to list orders", 500)
	}

	return ctx.JSON(map[string]any{
		"orders": orders,
		"count":  len(orders),
	})
}

func getOrderHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	orderID := ctx.Param("id")

	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}
	if orderID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Order ID is required", 400)
	}

	order, err := orderService.GetOrder(ctx.Context, tenantID, orderID)
	if err != nil {
		return lift.NotFound("Order not found")
	}

	return ctx.JSON(order)
}

func updateOrderStatusHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	orderID := ctx.Param("id")

	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}
	if orderID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Order ID is required", 400)
	}

	var req struct {
		Status OrderStatus `json:"status"`
	}

	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request", 400)
	}

	err := orderService.UpdateOrderStatus(ctx.Context, tenantID, orderID, req.Status)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to update order status", 500)
	}

	return ctx.JSON(map[string]any{
		"orderId": orderID,
		"status":  req.Status,
		"updated": true,
	})
}

func getCustomerOrdersHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	customerID := ctx.Param("id")

	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}
	if customerID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Customer ID is required", 400)
	}

	orders, err := orderService.GetCustomerOrders(ctx.Context, tenantID, customerID)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to get customer orders", 500)
	}

	return ctx.JSON(map[string]any{
		"customerId": customerID,
		"orders":     orders,
		"count":      len(orders),
	})
}

func getCartHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	customerID := ctx.Header("X-Customer-ID")

	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}
	if customerID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Customer ID is required", 400)
	}

	cart, err := cartService.GetCart(ctx.Context, tenantID, customerID)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to get cart", 500)
	}

	return ctx.JSON(cart)
}

func addToCartHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	customerID := ctx.Header("X-Customer-ID")

	if tenantID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Tenant ID is required", 400)
	}
	if customerID == "" {
		return lift.NewLiftError("BAD_REQUEST", "Customer ID is required", 400)
	}

	var req AddToCartRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request", 400)
	}

	cart, err := cartService.AddToCart(ctx.Context, tenantID, customerID, req)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to add to cart", 500)
	}

	return ctx.JSON(cart)
}

func updateCartItemHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	cartID := ctx.Param("cartId")
	itemID := ctx.Param("itemId")

	var req UpdateCartItemRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request", 400)
	}

	cart, err := cartService.UpdateCartItem(ctx.Context, tenantID, cartID, itemID, req)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to update cart item", 500)
	}

	return ctx.JSON(cart)
}

func removeFromCartHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	cartID := ctx.Param("cartId")
	itemID := ctx.Param("itemId")

	cart, err := cartService.RemoveFromCart(ctx.Context, tenantID, cartID, itemID)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to remove from cart", 500)
	}

	return ctx.JSON(cart)
}

func checkoutHandler(ctx *lift.Context) error {
	tenantID := getTenantID(ctx)
	cartID := ctx.Param("cartId")

	var req CreateOrderRequest
	if err := ctx.ParseRequest(&req); err != nil {
		return lift.NewLiftError("BAD_REQUEST", "Invalid request", 400)
	}

	order, err := cartService.ConvertCartToOrder(ctx.Context, tenantID, cartID, req)
	if err != nil {
		return lift.NewLiftError("INTERNAL_ERROR", "Failed to checkout", 500)
	}

	return ctx.Status(201).JSON(order)
}
