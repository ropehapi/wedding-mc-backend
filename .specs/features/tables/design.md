# Organização de Mesas — Design

**Spec**: `.specs/features/tables/spec.md`
**Status**: Draft

---

## Visão Geral da Arquitetura

Feature segue o padrão em camadas do projeto: domain → repository → service → handler. Dois novos arquivos por camada (`table.go`), mais extensões nos existentes (`guest.go` repository, `public.go` handler).

```
HTTP Request
    ↓
TableHandler (internal/handler/table.go)
    ↓
TableService (internal/service/table.go)
    ↓
TableRepository (internal/repository/table.go)
    ↓
PostgreSQL (tables + guests com table_id)
```

Para atribuição: TableService coordena TableRepository e GuestRepository (verificar ownership e contar ocupação).

---

## Modelo de Dados

### Tabela `tables`

```sql
CREATE TABLE tables (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    wedding_id UUID        NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    name       VARCHAR(100) NOT NULL,
    capacity   INT         NOT NULL CHECK (capacity > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX ON tables(wedding_id);
```

### Alteração em `guests`

```sql
ALTER TABLE guests
    ADD COLUMN table_id UUID REFERENCES tables(id) ON DELETE SET NULL;
```

`ON DELETE SET NULL` garante que deletar uma mesa não delete convidados — apenas desatribui.

### Struct Go — `domain/table.go`

```go
type Table struct {
    ID        string    `db:"id"         json:"id"`
    WeddingID string    `db:"wedding_id" json:"wedding_id"`
    Name      string    `db:"name"       json:"name"`
    Capacity  int       `db:"capacity"   json:"capacity"`
    CreatedAt time.Time `db:"created_at" json:"created_at"`
    UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
    // Preenchido pelo service (JOIN) — não persistido como coluna
    Guests    []Guest   `db:"-"          json:"guests"`
}
```

### Atualização em `domain/guest.go`

Adicionar campo `TableID *string` ao struct `Guest`:

```go
TableID *string `db:"table_id" json:"table_id,omitempty"`
```

---

## Análise de Reuso de Código

### Componentes Existentes

| Componente | Localização | Como reusar |
|---|---|---|
| Padrão handler (validate + JSON + handleError) | `handler/guest.go` | Copiar estrutura exata — `GuestHandler` como referência |
| `middleware.UserIDFromContext` | `middleware/auth.go` | Extrair userID autenticado em todos os handlers admin |
| `domain.ErrNotFound`, `ErrForbidden`, `ErrValidation` | `domain/errors.go` | Erros padronizados — mesma tabela de mapeamento |
| Padrão ownership check | `service/guest.go:UpdateGuest` | `weddings.FindByUserID` → verificar `weddingID` |
| `publicGuestServicer` interface | `handler/public.go` | Estender para incluir `GetGuestTable` |
| `GuestRepository.FindByID` | `repository/guest.go` | Verificar ownership do convidado antes de atribuir |

### Pontos de Integração

| Sistema | Como conecta |
|---|---|
| `guests` table | Nova coluna `table_id` (nullable FK) |
| `GuestRepository` | Adicionar método `UpdateTableID(ctx, guestID string, tableID *string)` |
| `main.go` | Instanciar `tableRepo`, `tableSvc`, `tableHandler` e registrar rotas |
| `PublicHandler` | Adicionar método `GetGuestTable` e extender interface `publicGuestServicer` |

---

## Componentes

### `domain/table.go`

- **Propósito**: Define struct `Table` e interface `TableRepository`
- **Localização**: `internal/domain/table.go`
- **Interface**:
  - `Create(ctx, *Table) error`
  - `FindAll(ctx, weddingID string) ([]Table, error)` — retorna tabelas sem guests (guests carregados pelo service)
  - `FindByID(ctx, id string) (*Table, error)`
  - `Update(ctx, *Table) error`
  - `Delete(ctx, id string) error`
  - `CountGuests(ctx, tableID string) (int, error)` — para validação de capacidade
- **Deps**: nenhum

---

### `repository/table.go`

- **Propósito**: Implementação SQL das queries de `Table`
- **Localização**: `internal/repository/table.go`
- **Interfaces**:
  - `Create` → INSERT com RETURNING
  - `FindAll` → SELECT por wedding_id ORDER BY name
  - `FindByID` → SELECT com ErrNoRows → ErrNotFound
  - `Update` → UPDATE com RETURNING updated_at
  - `Delete` → DELETE (ON DELETE SET NULL cuida dos guests no DB)
  - `CountGuests` → `SELECT COUNT(*) FROM guests WHERE table_id = $1`
- **Reusa**: padrão `sqlx.GetContext` / `sqlx.SelectContext` — idêntico ao `guestRepo`

---

### `repository/guest.go` (extensão)

- **Propósito**: Adicionar método de atualização de `table_id`
- **Novo método**: `UpdateTableID(ctx, guestID string, tableID *string) error`
  - `UPDATE guests SET table_id = $1, updated_at = NOW() WHERE id = $2 RETURNING updated_at`
- **Na interface** `domain.GuestRepository`: adicionar `UpdateTableID`

---

### `service/table.go`

- **Propósito**: Orquestra lógica de negócio para mesas e atribuição
- **Localização**: `internal/service/table.go`
- **Interface `TableService`**:
  - `CreateTable(ctx, userID, name string, capacity int) (*domain.Table, error)`
  - `ListTables(ctx, userID string) ([]domain.Table, []domain.Guest, error)` — tabelas com guests + lista de unassigned
  - `UpdateTable(ctx, userID, tableID, name string, capacity int) (*domain.Table, error)`
  - `DeleteTable(ctx, userID, tableID string) error`
  - `AssignGuest(ctx, userID, tableID, guestID string) error`
  - `UnassignGuest(ctx, userID, tableID, guestID string) error`
- **Deps**: `TableRepository`, `GuestRepository`, `WeddingRepository`

**Lógica chave — `UnassignGuest`**:
1. `weddings.FindByUserID` → weddingID
2. `tables.FindByID` → verifica ownership
3. `guests.FindByID` → verifica ownership
4. Se `guest.TableID == nil` OU `*guest.TableID != tableID` → `domain.ErrNotAssigned`
5. `guests.UpdateTableID(guestID, nil)`

**Lógica chave — `AssignGuest`**:
1. `weddings.FindByUserID` → obtém weddingID
2. `tables.FindByID` → verifica ownership (table.WeddingID == weddingID), senão ErrNotFound
3. `guests.FindByID` → verifica ownership (guest.WeddingID == weddingID), senão ErrNotFound
4. Se `guest.TableID == tableID` → retorna nil (idempotente)
5. `tables.CountGuests(tableID)` → se count >= capacity → ErrValidation "capacity exceeded"
6. `guests.UpdateTableID(guestID, &tableID)`

**Lógica chave — `UpdateTable`**:
- Se nova capacity < CountGuests(tableID) → ErrValidation "capacity below current occupancy"

**Lógica chave — `ListTables`**:
1. `tables.FindAll(weddingID)` → lista de mesas
2. `guests.FindAll(ctx, weddingID, nil)` → todos os convidados do casamento
3. Particionar no service: guests com `table_id != nil` → agrupados por mesa; guests com `table_id == nil` → lista `unassigned`
4. Retornar `([]Table com Guests preenchido, []Guest unassigned, error)`

**Decisão**: duas queries separadas + particionamento no service — simples de manter; volume esperado (dezenas de mesas, centenas de convidados) não justifica query complexa.

---

### `handler/table.go`

- **Propósito**: HTTP handlers para CRUD de mesas e atribuição de convidados
- **Localização**: `internal/handler/table.go`
- **Endpoints**:
  - `POST /v1/tables` → `Create`
  - `GET /v1/tables` → `List`
  - `PATCH /v1/tables/{tableID}` → `Update`
  - `DELETE /v1/tables/{tableID}` → `Delete`
  - `PUT /v1/tables/{tableID}/guests/{guestID}` → `AssignGuest`
  - `DELETE /v1/tables/{tableID}/guests/{guestID}` → `UnassignGuest`
- **Reusa**: exato mesmo padrão de `GuestHandler` — validate, handleError, toResponse

**Request/Response types**:
```go
type createTableRequest struct {
    Name     string `json:"name"     validate:"required"`
    Capacity int    `json:"capacity" validate:"required,min=1"`
}

type updateTableRequest struct {
    Name     *string `json:"name"`
    Capacity *int    `json:"capacity" validate:"omitempty,min=1"`
}

type tableGuestResponse struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    Status string `json:"status"`
}

type tableResponse struct {
    ID        string               `json:"id"`
    Name      string               `json:"name"`
    Capacity  int                  `json:"capacity"`
    Occupied  int                  `json:"occupied"`
    Guests    []tableGuestResponse `json:"guests"`
    CreatedAt string               `json:"created_at"`
    UpdatedAt string               `json:"updated_at"`
}

// listTablesResponse é o envelope do GET /v1/tables
type listTablesResponse struct {
    Tables     []tableResponse      `json:"tables"`
    Unassigned []tableGuestResponse `json:"unassigned"`
}
```

---

## Migrações

| Arquivo | Ação |
|---|---|
| `000011_create_tables.up.sql` | Criar tabela `tables` |
| `000011_create_tables.down.sql` | `DROP TABLE tables` |
| `000012_add_table_id_to_guests.up.sql` | `ALTER TABLE guests ADD COLUMN table_id UUID REFERENCES tables(id) ON DELETE SET NULL` |
| `000012_add_table_id_to_guests.down.sql` | `ALTER TABLE guests DROP COLUMN table_id` |

---

## Tratamento de Erros

| Cenário | Handling | HTTP |
|---|---|---|
| tableID não pertence ao casal | `ErrNotFound` | 404 |
| guestID não pertence ao casal | `ErrNotFound` | 404 |
| Capacidade excedida | `ErrValidation` | 422 |
| Nova capacidade < ocupação atual | `ErrValidation` | 422 |
| `UnassignGuest` e guest não está na mesa | `ErrNotAssigned` | 409 |

---

## Decisões Técnicas

| Decisão | Escolha | Razão |
|---|---|---|
| Armazenar atribuição | `table_id` FK em `guests` | Guest tem 1 mesa; join table seria over-engineering |
| Deleção de mesa | ON DELETE SET NULL | Não perde convidados — garante integridade sem cascade delete |
| N+1 no ListTables | Duas queries separadas | Simples de manter; volume pequeno não justifica query complexa |
| Endpoint público | Novo endpoint separado | Mais explícito que sobrescrever validate-code; fácil de documentar |
| Atribuição idempotente | PUT em vez de POST | Semanticamente correto — "mesa do guest = X" é uma substituição |
| Registro de `UpdateTableID` na interface | Sim | Testabilidade e consistência com o padrão de interfaces do domínio |
| `ErrNotAssigned` como erro de domínio | Sim | Semântica precisa — diferente de "recurso não existe"; mapeia para 409 |
| Feature exclusivamente admin | Sim | Convidados não interagem com mesas; sem endpoint público |
