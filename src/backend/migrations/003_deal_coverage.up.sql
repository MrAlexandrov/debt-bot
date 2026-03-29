DROP TABLE IF EXISTS purchase_coverage;

CREATE TABLE IF NOT EXISTS deal_coverage (
    deal_id    UUID NOT NULL REFERENCES deals(id),
    payer_id   UUID NOT NULL REFERENCES users(id),
    covered_id UUID NOT NULL REFERENCES users(id),
    PRIMARY KEY (deal_id, covered_id)
);
