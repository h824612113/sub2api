-- Update subscription plan features to advertise current Codex models.

UPDATE subscription_plans
SET
    features = REPLACE(features, '支持 gpt-image-2 出图', '支持最新 GPT-5.6/ 5.5 等 Codex 模型'),
    updated_at = NOW()
WHERE product_name IN ('openai_flagship_monthly', 'openai_starter_monthly')
  AND features LIKE '%支持 gpt-image-2 出图%';
