-- Seed data for local development / manual testing.
-- Not part of the "real" schema - keep actual schema changes in numbered
-- migrations before this one, and re-run this file any time you want to
-- reset test balances.

INSERT INTO institutions (code, name) VALUES
    ('institution-a', 'Institution A'),
    ('institution-b', 'Institution B'),
    ('institution-c', 'Institution C')
ON CONFLICT (code) DO NOTHING;

INSERT INTO liquidity_accounts (institution_id, currency, available)
SELECT id, 'NGN', 1000000.00 FROM institutions WHERE code = 'institution-a'
ON CONFLICT (institution_id, currency) DO NOTHING;

INSERT INTO liquidity_accounts (institution_id, currency, available)
SELECT id, 'NGN', 1000000.00 FROM institutions WHERE code = 'institution-b'
ON CONFLICT (institution_id, currency) DO NOTHING;

INSERT INTO liquidity_accounts (institution_id, currency, available)
SELECT id, 'NGN', 1000000.00 FROM institutions WHERE code = 'institution-c'
ON CONFLICT (institution_id, currency) DO NOTHING;
