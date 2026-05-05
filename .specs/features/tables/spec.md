# Organização de Mesas — Especificação

## Declaração do Problema

O casal não tem como organizar o layout de assentos do evento dentro da plataforma. Precisam definir quais mesas existem, quantos lugares cada uma comporta e qual convidado senta onde — informação que hoje é gerenciada em planilhas separadas ou de cabeça.

## Objetivos

- [ ] Permitir ao casal criar e gerenciar mesas (nome, capacidade)
- [ ] Permitir atribuir e remover convidados de mesas com validação de capacidade
- [ ] Exibir convidados ainda não alocados junto com a listagem de mesas

## Fora do Escopo

| Feature | Razão |
|---|---|
| Visualização gráfica do salão (drag & drop) | Complexidade de UI — escopo do frontend futuro |
| Múltiplos convidados por assento | Cada assento = 1 pessoa |
| Impressão de cartões de mesa | Frontend — fora do backend |
| Exportação de layout em PDF | Fora do v1 |
| Consulta pública da mesa pelo convidado | Feature é exclusiva do painel admin; convidados não interagem com mesas |

---

## User Stories

### P1: CRUD de Mesas ⭐ MVP

**User Story**: Como casal, quero criar, visualizar, editar e excluir mesas do evento para definir o layout de assentos.

**Por que P1**: Sem mesas não há como fazer nada — é o recurso base da feature.

**Critérios de Aceitação**:

1. WHEN casal envia `POST /v1/tables` com `name` e `capacity` THEN sistema SHALL criar a mesa e retornar 201 com os dados
2. WHEN `capacity` ≤ 0 THEN sistema SHALL rejeitar com 422
3. WHEN `name` está ausente THEN sistema SHALL rejeitar com 422
4. WHEN casal envia `GET /v1/tables` THEN sistema SHALL retornar todas as mesas do casamento com contagem de ocupação, lista de convidados atribuídos e lista separada de convidados ainda não alocados em nenhuma mesa
5. WHEN casal envia `PATCH /v1/tables/{tableID}` THEN sistema SHALL atualizar nome e/ou capacidade
6. WHEN nova capacidade < número de convidados já atribuídos THEN sistema SHALL rejeitar com 422
7. WHEN casal envia `DELETE /v1/tables/{tableID}` THEN sistema SHALL deletar a mesa e desatribuir os convidados (table_id → NULL)
8. WHEN tableID não pertence ao casamento do casal THEN sistema SHALL retornar 404

**Teste Independente**: Criar uma mesa, listá-la, atualizar o nome e capacidade, excluir — tudo sem atribuir convidados.

---

### P1: Atribuição de Convidados a Mesas ⭐ MVP

**User Story**: Como casal, quero atribuir e remover convidados de mesas para montar o layout de assentos completo.

**Por que P1**: A atribuição é o núcleo da feature — sem isso, mesas são entidades vazias sem utilidade.

**Critérios de Aceitação**:

1. WHEN casal envia `PUT /v1/tables/{tableID}/guests/{guestID}` THEN sistema SHALL atribuir o convidado àquela mesa (movendo-o de outra mesa se necessário)
2. WHEN a mesa já está na capacidade máxima THEN sistema SHALL rejeitar com 422 e mensagem "capacity exceeded"
3. WHEN guestID não pertence ao casamento THEN sistema SHALL retornar 404
4. WHEN tableID não pertence ao casamento THEN sistema SHALL retornar 404
5. WHEN casal envia `DELETE /v1/tables/{tableID}/guests/{guestID}` THEN sistema SHALL remover o convidado da mesa (table_id → NULL)
6. WHEN convidado não está na mesa indicada THEN sistema SHALL retornar 409 com code `not_assigned`

**Teste Independente**: Criar mesa com capacity=2, atribuir 2 convidados, tentar adicionar 3º (deve falhar), remover um e adicionar novamente.

---

## Edge Cases

- WHEN convidado já está em mesa X e é atribuído a mesa Y THEN sistema SHALL mover (não duplicar)
- WHEN table é deletada THEN convidados da mesa ficam com `table_id = NULL` (ON DELETE SET NULL)
- WHEN capacity é reduzida abaixo do número atual de convidados THEN sistema SHALL rejeitar com 422
- WHEN mesmo convidado é `PUT` na mesa onde já está THEN sistema SHALL aceitar (idempotente, 200)

---

## Rastreabilidade de Requisitos

| ID | Story | Status |
|---|---|---|
| TABLE-01 | P1: CRUD — criar mesa | Pendente |
| TABLE-02 | P1: CRUD — listar mesas com ocupação | Pendente |
| TABLE-03 | P1: CRUD — atualizar mesa | Pendente |
| TABLE-04 | P1: CRUD — deletar mesa (desatribui convidados) | Pendente |
| TABLE-05 | P1: Atribuição — atribuir convidado (move se necessário) | Pendente |
| TABLE-06 | P1: Atribuição — validar capacidade | Pendente |
| TABLE-07 | P1: Atribuição — remover convidado (ErrNotAssigned se não está na mesa) | Pendente |
| TABLE-08 | P1: Listagem — campo `unassigned` com convidados sem mesa | Pendente |

---

## Critérios de Sucesso

- [ ] Casal consegue criar todas as mesas e atribuir todos os convidados sem erros
- [ ] Sistema impede atribuição acima da capacidade com mensagem clara
- [ ] Deleção de mesa não perde convidados — apenas desatribui
- [ ] Listagem de mesas exibe quem já foi alocado e quem ainda não foi
