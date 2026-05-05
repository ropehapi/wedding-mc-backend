# Organização de Mesas — Tasks

**Spec**: `.specs/features/tables/spec.md`
**Design**: `.specs/features/tables/design.md`
**Status**: Pendente

---

## Ordem de Implementação

As tarefas seguem a ordem natural das dependências na arquitetura em camadas:

```
Migrations → Domain → Repository → Service → Handler → Rotas → Testes
```

---

## Tarefas

### TASK-01 — Migração: criar tabela `tables`

**Arquivos**:
- `migrations/000011_create_tables.up.sql`
- `migrations/000011_create_tables.down.sql`

**Up**:
```sql
CREATE TABLE tables (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    wedding_id UUID         NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    name       VARCHAR(100) NOT NULL,
    capacity   INT          NOT NULL CHECK (capacity > 0),
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX ON tables(wedding_id);
```

**Down**: `DROP TABLE IF EXISTS tables;`

**Verificação**: `make migrate-up` roda sem erro; tabela existe no banco.

---

### TASK-02 — Migração: adicionar `table_id` em `guests`

**Arquivos**:
- `migrations/000012_add_table_id_to_guests.up.sql`
- `migrations/000012_add_table_id_to_guests.down.sql`

**Up**: `ALTER TABLE guests ADD COLUMN table_id UUID REFERENCES tables(id) ON DELETE SET NULL;`

**Down**: `ALTER TABLE guests DROP COLUMN IF EXISTS table_id;`

**Verificação**: migrate roda sem erro; coluna existe; `\d guests` mostra FK correta.

---

### TASK-03 — Domain: `ErrNotAssigned` + `Table` struct + `TableRepository` interface

**Arquivos**: `internal/domain/errors.go` + `internal/domain/table.go`

Em `errors.go`, adicionar:
```go
var ErrNotAssigned = errors.New("guest is not assigned to this table")
```

Criar `table.go` — Struct `Table` com campos: `ID, WeddingID, Name, Capacity, CreatedAt, UpdatedAt, Guests []Guest`.

Interface `TableRepository`:
- `Create(ctx, *Table) error`
- `FindAll(ctx, weddingID string) ([]Table, error)`
- `FindByID(ctx, id string) (*Table, error)`
- `Update(ctx, *Table) error`
- `Delete(ctx, id string) error`
- `CountGuests(ctx, tableID string) (int, error)`

**Verificação**: `go build ./...` compila sem erros.

---

### TASK-04 — Domain: adicionar `TableID` e `UpdateTableID` em `guest.go`

**Arquivo**: `internal/domain/guest.go`

1. Adicionar `TableID *string` ao struct `Guest` com tags `db:"table_id" json:"table_id,omitempty"`
2. Adicionar `UpdateTableID(ctx context.Context, guestID string, tableID *string) error` à interface `GuestRepository`

**Verificação**: `go build ./...` — vai falhar em `repository/guest.go` (interface não satisfeita). Esperado — TASK-05 resolve.

---

### TASK-05 — Repository: implementar `UpdateTableID` + atualizar mock de testes

**Arquivos**: `internal/repository/guest.go` + `internal/service/guest_test.go`

**1.** Implementar o método `UpdateTableID` no `guestRepo`:
```go
func (r *guestRepo) UpdateTableID(ctx context.Context, guestID string, tableID *string) error {
    // UPDATE guests SET table_id = $1, updated_at = NOW() WHERE id = $2
}
```

**2.** Atualizar `mockGuestRepo` em `internal/service/guest_test.go` para satisfazer a interface atualizada. Adicionar ao struct e implementar:
```go
updateTableIDErr error

func (m *mockGuestRepo) UpdateTableID(_ context.Context, _ string, _ *string) error {
    return m.updateTableIDErr
}
```

**Atenção**: verificar se `mockGuestRepo` já implementa `FindByAccessCode` (adicionado na interface em commit recente). Se não implementar, adicionar stub:
```go
func (m *mockGuestRepo) FindByAccessCode(_ context.Context, _, _ string) (*domain.Guest, error) {
    return nil, domain.ErrNotFound
}
```

**Verificação**: `go test ./internal/service/...` compila e todos os testes passam.

---

### TASK-06 — Repository: implementar `TableRepository`

**Arquivo**: `internal/repository/table.go`

Implementar `tableRepo` satisfazendo `domain.TableRepository`:
- `Create`: INSERT com RETURNING id, created_at, updated_at
- `FindAll`: SELECT * FROM tables WHERE wedding_id = $1 ORDER BY name
- `FindByID`: SELECT com ErrNoRows → domain.ErrNotFound
- `Update`: UPDATE com RETURNING updated_at
- `Delete`: DELETE com verificação de rows affected → ErrNotFound se 0
- `CountGuests`: SELECT COUNT(*) FROM guests WHERE table_id = $1

**Verificação**: `go build ./...` compila.

---

### TASK-07 — Service: `TableService` interface + implementação

**Arquivo**: `internal/service/table.go`

Interface `TableService` e struct `tableService{tables TableRepository, guests GuestRepository, weddings WeddingRepository}`.

Implementar todos os métodos (ver design.md para lógica detalhada):

- `AssignGuest`: idempotência se guest já está na mesa, validação de capacidade, suporte a mover de outra mesa
- `UnassignGuest`: verificar `guest.TableID == nil || *guest.TableID != tableID` → `domain.ErrNotAssigned`
- `UpdateTable`: rejeitar se nova capacity < `CountGuests(tableID)` → `domain.ErrValidation`
- `ListTables`: duas queries (FindAll tables + FindAll guests por weddingID), particionar no service — guests com `table_id != nil` agrupados por mesa, guests com `table_id == nil` → lista unassigned. Retornar `([]domain.Table, []domain.Guest, error)`

**Verificação**: `go build ./...` compila.

---

### TASK-08 — Handler: `TableHandler` com CRUD e atribuição

**Arquivo**: `internal/handler/table.go`

Handlers: `Create`, `List`, `Update`, `Delete`, `AssignGuest`, `UnassignGuest`.

Request types: `createTableRequest{Name, Capacity}`, `updateTableRequest{Name *string, Capacity *int}`.

Response types: `tableResponse{ID, Name, Capacity, Occupied, Guests []tableGuestResponse, CreatedAt, UpdatedAt}` e `listTablesResponse{Tables []tableResponse, Unassigned []tableGuestResponse}`.

Seguir exato padrão de `GuestHandler`: `validator.New()`, `handleError`, `toTableResponse`.

`handleError` mapeia: ErrNotFound→404, ErrValidation→422, ErrForbidden→403, ErrNotAssigned→409, default→500.

**Verificação**: `go build ./...` compila.

---

### TASK-09 — Rotas e DI: registrar tudo em `main.go`

**Arquivo**: `cmd/api/main.go`

1. Instanciar: `tableRepo := repository.NewTableRepository(db)`
2. Instanciar: `tableSvc := service.NewTableService(tableRepo, guestRepo, weddingRepo)`
3. Instanciar: `tableHandler := handler.NewTableHandler(tableSvc)`
4. Registrar rotas:

```go
r.Route("/tables", func(r chi.Router) {
    r.Use(middleware.Auth(cfg.JWTSecret))
    r.Post("/", tableHandler.Create)
    r.Get("/", tableHandler.List)
    r.Patch("/{tableID}", tableHandler.Update)
    r.Delete("/{tableID}", tableHandler.Delete)
    r.Put("/{tableID}/guests/{guestID}", tableHandler.AssignGuest)
    r.Delete("/{tableID}/guests/{guestID}", tableHandler.UnassignGuest)
})
```

**Verificação**: `go build ./...` compila; `go run ./cmd/api` sobe sem erros.

---

### TASK-10 — Smoke test manual dos endpoints

---

Com o servidor rodando (`make run` ou `go run ./cmd/api`):

1. `POST /v1/tables` — criar "Mesa 1" capacity=2
2. `GET /v1/tables` — listar: Mesa 1 com occupied=0, todos os convidados em `unassigned`
3. `PUT /v1/tables/{id}/guests/{guestID}` — atribuir convidado
4. `GET /v1/tables` — ver occupied=1, guest saiu de `unassigned` e está em `tables[0].guests`
5. Atribuir 2º convidado (capacity=2, ok). Tentar 3º → deve retornar 422
6. `DELETE /v1/tables/{id}/guests/{guestID}` — remover convidado → volta para `unassigned`
7. Tentar remover convidado que não está nessa mesa → deve retornar 409
8. `PATCH /v1/tables/{id}` — tentar reduzir capacity abaixo do ocupado → 422; depois renomear
9. `DELETE /v1/tables/{id}` — deletar; verificar via `GET /v1/guests` que convidados continuam existindo

**Verificação**: todos os fluxos respondem conforme spec.

---

### TASK-11 — Atualizar Swagger

Rodar `make swagger` (ou `swag init`) para regenerar `docs/`.

**Verificação**: `/swagger/` mostra os novos endpoints documentados.

---

## Dependências entre Tasks

```
TASK-01 → TASK-02           (migrations em ordem)
TASK-03 → TASK-04           (ErrNotAssigned + domain table antes de estender guest)
TASK-04 → TASK-05           (interface antes de implementação + mock)
TASK-03 → TASK-06           (interface antes de implementação)
TASK-05, TASK-06 → TASK-07  (repositories antes de service)
TASK-07 → TASK-08           (service antes de handler)
TASK-08 → TASK-09           (handler antes de registrar rotas)
TASK-09 → TASK-10           (rotas antes de smoke test)
TASK-10 → TASK-11           (código estável antes de gerar swagger)
```

## Rastreabilidade Spec → Tasks

| Req ID | Tasks |
|---|---|
| TABLE-01 (criar mesa) | TASK-01, TASK-03, TASK-06, TASK-07, TASK-08, TASK-09 |
| TABLE-02 (listar mesas) | TASK-06, TASK-07, TASK-08, TASK-09 |
| TABLE-03 (atualizar mesa) | TASK-06, TASK-07, TASK-08, TASK-09 |
| TABLE-04 (deletar mesa) | TASK-02, TASK-06, TASK-07, TASK-08, TASK-09 |
| TABLE-05 (atribuir guest) | TASK-04, TASK-05, TASK-07, TASK-08, TASK-09 |
| TABLE-06 (validar capacidade) | TASK-06, TASK-07 |
| TABLE-07 (remover guest + ErrNotAssigned) | TASK-03, TASK-04, TASK-05, TASK-07, TASK-08, TASK-09 |
| TABLE-08 (unassigned na listagem) | TASK-07, TASK-08 |
