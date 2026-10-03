# ðŸ›ï¸ Architecture & Coding Standards

This project follows a **Clean, Decoupled 6-Layer Architecture** optimized for high performance, maintainability, and rapid developer velocity.

---

## ðŸ”„ Request Lifecycle & Data Flow

```
[ Client Request ]
       â”‚
       â–¼
[ Middlewares ]  â”€â”€â–¶ (CORS, Request Logger, Rate Limiter, Auth JWT)
       â”‚
       â–¼
[ Routes ]       â”€â”€â–¶ Chi Router maps URL + Method to Controller method
       â”‚
       â–¼
[ Controller ]   â”€â”€â–¶ Binds JSON body into DTO, validates inputs using validator
       â”‚
       â–¼
[ Service ]      â”€â”€â–¶ Contains core business rules, coordinates repositories
       â”‚
       â–¼
[ Repository ]   â”€â”€â–¶ Executes parameterized SQL queries via pgxpool
       â”‚
       â–¼
[ PostgreSQL ]   â”€â”€â–¶ Data Layer
```

---

## ðŸ“ General Utility Helpers

### 1. Database Transactions (`internal/utils/tx.go`)
Wrap multiple database operations in an atomic transaction:
```go
err := utils.WithTx(ctx, db, func(tx pgx.Tx) error {
    // Both operations must succeed or both roll back
    if err := repo.DeductBalanceTx(ctx, tx, userID, amount); err != nil {
        return err
    }
    return repo.CreateOrderTx(ctx, tx, order)
})
```

### 2. Pagination & Sorting (`internal/reuse/pagination.go`)
Standardized pagination query parser and JSON response:
```go
// In Controller:
pagination := reuse.GetPagination(r) // parses ?page=1&limit=10&order=desc&search=foo
items, total, err := c.service.List(r.Context(), pagination)
if err != nil {
    reuse.Error(w, http.StatusInternalServerError, err.Error())
    return
}
reuse.PaginatedResponse(w, items, total, pagination)
```

### 3. In-Memory Cache (`internal/reuse/cache.go`)
High-performance thread-safe in-memory cache with TTL:
```go
cache := reuse.GetCache()
cache.Set("otp:user@example.com", "123456", 5*time.Minute)

if val, found := cache.Get("otp:user@example.com"); found {
    otp := val.(string)
}
```

### 4. Interactive Swagger Docs (`docs/` & `/swagger/index.html`)
Live OpenAPI testing UI mounted at `/swagger/index.html`. Add Go doc comments to controllers to auto-document new routes.

---

## ðŸš€ Domain Generation Command

To scaffold any entity in 2 seconds:
```powershell
.\scripts\generate-domain.bat <EntityName>
```
Creates:
- `model/<entity>.go`
- `dto/<entity>_dto.go`
- `repository/<entity>_repository.go`
- `services/<entity>_service.go`
- `controller/<entity>_controller.go`
- Up & down SQL migrations!