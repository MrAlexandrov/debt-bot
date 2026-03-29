CREATE TABLE IF NOT EXISTS purchase_coverage (
    purchase_id UUID NOT NULL REFERENCES purchases(id),
    payer_id    UUID NOT NULL REFERENCES users(id),
    covered_id  UUID NOT NULL REFERENCES users(id),
    PRIMARY KEY (purchase_id, covered_id)
);
