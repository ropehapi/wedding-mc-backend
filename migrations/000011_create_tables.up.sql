CREATE TABLE tables (
    id         UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    wedding_id UUID         NOT NULL REFERENCES weddings(id) ON DELETE CASCADE,
    name       VARCHAR(100) NOT NULL,
    capacity   INT          NOT NULL CHECK (capacity > 0),
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX ON tables(wedding_id);
