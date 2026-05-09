-- Update Pro monthly plan price to 279 without mutating the already-applied 136 migration.

UPDATE subscription_plans
SET
    price = 279.00,
    original_price = 299.00,
    updated_at = NOW()
WHERE product_name = 'openai_flagship_monthly';
