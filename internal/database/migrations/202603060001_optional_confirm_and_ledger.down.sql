DROP TABLE IF EXISTS ledger_entries;

ALTER TABLE withdrawals
    DROP COLUMN IF EXISTS confirmed_at;

ALTER TABLE withdrawals DROP CONSTRAINT IF EXISTS withdrawals_status_check;
ALTER TABLE withdrawals
    ADD CONSTRAINT withdrawals_status_check CHECK (status = 'pending');
