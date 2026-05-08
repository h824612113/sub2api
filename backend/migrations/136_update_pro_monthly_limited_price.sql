-- Update the runtime monthly plan prices and labels.

UPDATE subscription_plans
SET
    price = 279.00,
    original_price = 299.00,
    description = 'Pro 月度套餐限时优惠，每日 100 美元使用额度，未用完次日重置。',
    features = E'每日 $100 使用额度\n每周 $600 使用额度\n支持 gpt-image-2 出图\nPro 优先级通道\n限时优惠',
    updated_at = NOW()
WHERE product_name = 'openai_flagship_monthly';

UPDATE subscription_plans
SET
    original_price = 199.00,
    updated_at = NOW()
WHERE product_name = 'openai_advanced_monthly';

UPDATE subscription_plans
SET
    name = 'Basic 月度套餐',
    description = 'Basic 月度套餐，每日 20 美元使用额度，未用完次日重置。',
    original_price = 89.00,
    features = E'每日 $20 使用额度\n每周 $120 使用额度\n支持 gpt-image-2 出图\nBasic 优先级通道',
    updated_at = NOW()
WHERE product_name = 'openai_starter_monthly';
