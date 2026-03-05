ALTER TABLE withdrawals DROP CONSTRAINT IF EXISTS withdrawals_status_check;
ALTER TABLE withdrawals
    ADD CONSTRAINT withdrawals_status_check CHECK (status IN ('pending', 'confirmed'));

ALTER TABLE withdrawals
    ADD COLUMN IF NOT EXISTS confirmed_at TIMESTAMPTZ NULL;

CREATE TABLE IF NOT EXISTS ledger_entries (
    id BIGSERIAL PRIMARY KEY,
    withdrawal_id BIGINT NOT NULL REFERENCES withdrawals(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    entry_type VARCHAR(32) NOT NULL CHECK (entry_type IN ('withdrawal_create', 'withdrawal_confirm')),
    amount_delta BIGINT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_withdrawal_id ON ledger_entries (withdrawal_id);
