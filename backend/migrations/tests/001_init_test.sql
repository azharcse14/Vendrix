-- 001_init_test.sql : 001_init.sql er constraint test (M0 er CI te chalao).
-- Chalano: make db-test (scripts/db-test.sh), ba migration er por khali test database e:
--   psql -v ON_ERROR_STOP=1 -f backend/migrations/tests/001_init_test.sql
-- v1.6: migration folder theke tests/ e, karon runner er embed e migration hishebe cholto.
-- Shob kichu ek transaction e, shesh e ROLLBACK: database e kichu thake na.
-- Kono test ashar moto na hole shesh e ERROR, tai CI fail korbe.
-- v1.3: expect 'reject:<constraint>' hole oi constraint er karonei reject hote hobe (onno CHECK e na).
-- v1.4: coupon snapshot, COD confirm token, payment reconcile, commission_bps er test.
-- v1.5: shob reject test e constraint er naam ('reject:<naam>'), tai test er nijer bhul (typo) e pass kore na;
-- order item er ojon/removed, coupon chhara discount, TOTP key version er test.
-- v1.7: je named constraint er test chhilo na (cancelled display, fulfillment total, deleted PII, coupon
-- code case), ar hasanta (juktakkhor) wala Bangla shobdo trigram index diye ILIKE e mile.

BEGIN;
\pset format unaligned
\pset tuples_only on

CREATE TEMP TABLE test_results (label TEXT, ok BOOLEAN, detail TEXT);
CREATE TEMP TABLE ids (name TEXT PRIMARY KEY, id BIGINT);

CREATE FUNCTION pg_temp.t(label TEXT, stmt TEXT, expect TEXT) RETURNS VOID LANGUAGE plpgsql AS $$
DECLARE res TEXT;
BEGIN
  BEGIN
    EXECUTE stmt;
    RAISE EXCEPTION USING ERRCODE = 'P0001', MESSAGE = '__ROLLBACK_OK__';
  EXCEPTION
    WHEN raise_exception THEN
      res := CASE WHEN SQLERRM = '__ROLLBACK_OK__' THEN 'ACCEPTED' ELSE 'REJECTED: ' || SQLERRM END;
    WHEN OTHERS THEN
      res := 'REJECTED: ' || SQLERRM;
  END;
  INSERT INTO test_results VALUES (label,
    (expect = 'reject' AND res LIKE 'REJECTED%')
    OR (expect LIKE 'reject:%' AND res LIKE 'REJECTED%' AND strpos(res, substr(expect, 8)) > 0)
    OR (expect = 'accept' AND res = 'ACCEPTED'), res);
END $$;

CREATE FUNCTION pg_temp.id(n TEXT) RETURNS BIGINT LANGUAGE sql AS $$ SELECT id FROM ids WHERE name = n $$;

-- ===================== Seed (duita shop) =====================
DO $$
DECLARE v BIGINT;
BEGIN
  INSERT INTO shops (slug, name, order_prefix) VALUES ('s1','{"bn":"এক"}','SB') RETURNING id INTO v; INSERT INTO ids VALUES ('s1', v);
  INSERT INTO shops (slug, name, order_prefix) VALUES ('s2','{"bn":"দুই"}','SB') RETURNING id INTO v; INSERT INTO ids VALUES ('s2', v);
  INSERT INTO vendors (shop_id, slug, name, is_house) VALUES (pg_temp.id('s1'),'house','{}',true) RETURNING id INTO v; INSERT INTO ids VALUES ('h1', v);
  INSERT INTO vendors (shop_id, slug, name, is_house) VALUES (pg_temp.id('s2'),'house','{}',true) RETURNING id INTO v; INSERT INTO ids VALUES ('h2', v);
  INSERT INTO vendors (shop_id, slug, name, commission_bps) VALUES (pg_temp.id('s1'),'krishok','{}',1000) RETURNING id INTO v; INSERT INTO ids VALUES ('v1b', v);
  INSERT INTO categories (shop_id, slug, name) VALUES (pg_temp.id('s1'),'chal','{}') RETURNING id INTO v; INSERT INTO ids VALUES ('c1', v);
  INSERT INTO categories (shop_id, slug, name) VALUES (pg_temp.id('s2'),'chal','{}') RETURNING id INTO v; INSERT INTO ids VALUES ('c2', v);
  INSERT INTO products (shop_id, vendor_id, category_id, slug, name) VALUES (pg_temp.id('s1'),pg_temp.id('h1'),pg_temp.id('c1'),'red-rice','{"bn":"লাল চাল"}') RETURNING id INTO v; INSERT INTO ids VALUES ('p1', v);
  INSERT INTO products (shop_id, vendor_id, category_id, slug, name) VALUES (pg_temp.id('s2'),pg_temp.id('h2'),pg_temp.id('c2'),'red-rice','{"bn":"লাল চাল"}') RETURNING id INTO v; INSERT INTO ids VALUES ('p2', v);
  INSERT INTO products (shop_id, vendor_id, slug, name) VALUES (pg_temp.id('s1'),pg_temp.id('h1'),'honey','{"bn":"মধু"}') RETURNING id INTO v; INSERT INTO ids VALUES ('p3', v);
  INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price, stock) VALUES (pg_temp.id('s1'),pg_temp.id('p1'),pg_temp.id('h1'),'RR5',75000,10) RETURNING id INTO v; INSERT INTO ids VALUES ('var1', v);
  INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price, stock) VALUES (pg_temp.id('s2'),pg_temp.id('p2'),pg_temp.id('h2'),'RR5',70000,10) RETURNING id INTO v; INSERT INTO ids VALUES ('var2', v);
  INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price, stock) VALUES (pg_temp.id('s1'),pg_temp.id('p3'),pg_temp.id('h1'),'HN1',50000,5) RETURNING id INTO v; INSERT INTO ids VALUES ('var3', v);
  INSERT INTO inventory_batches (shop_id, variant_id, quantity_received, quantity_remaining) VALUES (pg_temp.id('s1'),pg_temp.id('var1'),10,10) RETURNING id INTO v; INSERT INTO ids VALUES ('b1', v);
  INSERT INTO inventory_batches (shop_id, variant_id, quantity_received, quantity_remaining) VALUES (pg_temp.id('s1'),pg_temp.id('var3'),5,5) RETURNING id INTO v; INSERT INTO ids VALUES ('b3', v);
  INSERT INTO delivery_zones (shop_id, name, district, area, charge, same_city) VALUES (pg_temp.id('s1'),'{}','Dhaka',NULL,6000,true);
  INSERT INTO staff (shop_id, email, name, password_hash, role) VALUES (pg_temp.id('s1'),'o@x.com','Owner','x','owner') RETURNING id INTO v; INSERT INTO ids VALUES ('st1', v);
  INSERT INTO users (shop_id, phone) VALUES (pg_temp.id('s1'),'+8801711111111') RETURNING id INTO v; INSERT INTO ids VALUES ('u1', v);
  INSERT INTO coupons (shop_id, code, kind, value) VALUES (pg_temp.id('s1'),'EID','fixed',5000) RETURNING id INTO v; INSERT INTO ids VALUES ('k1', v);
  INSERT INTO coupons (shop_id, code, kind, value) VALUES (pg_temp.id('s1'),'OTHER','fixed',5000) RETURNING id INTO v; INSERT INTO ids VALUES ('k2', v);
  -- oA: online + coupon EID, oB: online, oC: COD
  INSERT INTO orders (shop_id, order_number, public_token, idempotency_key, idempotency_hash, customer_name, customer_phone,
                      recipient_name, recipient_phone, ship_district, ship_area, ship_address, subtotal, discount, total, coupon_id,
                      coupon_code, coupon_kind, coupon_value, coupon_funded_by, payment_method, payment_status)
  VALUES (pg_temp.id('s1'),'SB-100001','tA','kA','\x00','A','+8801711111111','A','+8801711111111','Dhaka','Mirpur','Road 1',75000,5000,70000,pg_temp.id('k1'),
          'EID','fixed',5000,'shop','sslcommerz','pending')
  RETURNING id INTO v; INSERT INTO ids VALUES ('oA', v);
  INSERT INTO orders (shop_id, order_number, public_token, idempotency_key, idempotency_hash, customer_name, customer_phone,
                      recipient_name, recipient_phone, ship_district, ship_area, ship_address, subtotal, total, payment_method, payment_status)
  VALUES (pg_temp.id('s1'),'SB-100002','tB','kB','\x00','B','+8801722222222','B','+8801722222222','Dhaka','Mirpur','Road 2',75000,75000,'sslcommerz','pending')
  RETURNING id INTO v; INSERT INTO ids VALUES ('oB', v);
  INSERT INTO orders (shop_id, order_number, public_token, idempotency_key, idempotency_hash, customer_name, customer_phone,
                      recipient_name, recipient_phone, ship_district, ship_area, ship_address, subtotal, total, payment_method)
  VALUES (pg_temp.id('s1'),'SB-100003','tC','kC','\x00','C','+8801733333333','C','+8801733333333','Dhaka','Mirpur','Road 3',50000,50000,'cod')
  RETURNING id INTO v; INSERT INTO ids VALUES ('oC', v);
  INSERT INTO order_fulfillments (shop_id, order_id, vendor_id, seq, subtotal, discount, total) VALUES (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('h1'),1,75000,5000,70000) RETURNING id INTO v; INSERT INTO ids VALUES ('fA', v);
  INSERT INTO order_fulfillments (shop_id, order_id, vendor_id, seq, subtotal, total) VALUES (pg_temp.id('s1'),pg_temp.id('oB'),pg_temp.id('h1'),1,75000,75000) RETURNING id INTO v; INSERT INTO ids VALUES ('fB', v);
  INSERT INTO order_items (shop_id, order_id, fulfillment_id, vendor_id, variant_id, product_name, sku, unit_price, quantity, line_total, discount_share)
  VALUES (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fA'),pg_temp.id('h1'),pg_temp.id('var1'),'{}','RR5',75000,1,75000,5000) RETURNING id INTO v; INSERT INTO ids VALUES ('iA', v);
  INSERT INTO payments (shop_id, order_id, provider, tran_id, amount) VALUES (pg_temp.id('s1'),pg_temp.id('oA'),'sslcommerz','TA',70000) RETURNING id INTO v; INSERT INTO ids VALUES ('payA', v);
  INSERT INTO payments (shop_id, order_id, provider, tran_id, amount) VALUES (pg_temp.id('s1'),pg_temp.id('oB'),'sslcommerz','TB',75000) RETURNING id INTO v; INSERT INTO ids VALUES ('payB', v);
END $$;

\o /dev/null
-- ===================== Mul dabi (6.4) =====================
SELECT pg_temp.t('6.4: onno shop er variant cart e', $q$
  WITH c AS (INSERT INTO carts (shop_id, session_key) VALUES (pg_temp.id('s1'),'\x01') RETURNING id)
  INSERT INTO cart_items (shop_id, cart_id, variant_id, quantity) SELECT pg_temp.id('s1'), id, pg_temp.id('var2'), 1 FROM c $q$, 'reject:cart_items_shop_id_variant_id_fkey');
SELECT pg_temp.t('6.4: onno shop er category', $q$
  INSERT INTO products (shop_id, vendor_id, category_id, slug, name) VALUES (pg_temp.id('s1'),pg_temp.id('h1'),pg_temp.id('c2'),'x','{}') $q$, 'reject:products_shop_id_category_id_fkey');
SELECT pg_temp.t('6.4: variant er vendor product er vendor na', $q$
  INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, price) VALUES (pg_temp.id('s1'),pg_temp.id('p1'),pg_temp.id('v1b'),'Z',1) $q$, 'reject:product_variants_shop_id_product_id_vendor_id_fkey');
SELECT pg_temp.t('6.4: product er vendor bodlale variant cascade', $q$
  DO $d$ BEGIN
    UPDATE products SET vendor_id = pg_temp.id('v1b') WHERE id = pg_temp.id('p3');
    IF (SELECT vendor_id FROM product_variants WHERE id = pg_temp.id('var3')) <> pg_temp.id('v1b') THEN RAISE EXCEPTION 'no cascade'; END IF;
  END $d$ $q$, 'accept');
SELECT pg_temp.t('6.4: duita house vendor', $q$
  INSERT INTO vendors (shop_id, slug, name, is_house) VALUES (pg_temp.id('s1'),'h2','{}',true) $q$, 'reject:vendors_one_house');
SELECT pg_temp.t('6.4: ek order e ek vendor er duita fulfillment', $q$
  INSERT INTO order_fulfillments (shop_id, order_id, vendor_id, seq, subtotal, total) VALUES (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('h1'),2,0,0) $q$, 'reject:order_fulfillments_order_id_vendor_id_key');
SELECT pg_temp.t('6.4: item onno vendor er fulfillment e', $q$
  INSERT INTO order_items (shop_id, order_id, fulfillment_id, vendor_id, variant_id, product_name, sku, unit_price, quantity, line_total)
  VALUES (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fA'),pg_temp.id('v1b'),pg_temp.id('var1'),'{}','RR5',1,1,1) $q$, 'reject:order_items_shop_id_order_id_fulfillment_id_vendor_id_fkey');
SELECT pg_temp.t('6.4: vendor_staff er vendor_id nai', $q$
  INSERT INTO staff (shop_id, email, name, password_hash, role) VALUES (pg_temp.id('s1'),'v@x.com','V','x','vendor_staff') $q$, 'reject:staff_vendor_role_check');
SELECT pg_temp.t('6.4: admin er vendor_id ache', $q$
  INSERT INTO staff (shop_id, vendor_id, email, name, password_hash, role) VALUES (pg_temp.id('s1'),pg_temp.id('h1'),'a@x.com','A','x','admin') $q$, 'reject:staff_vendor_role_check');
SELECT pg_temp.t('6.4: total != subtotal - discount + delivery', $q$
  UPDATE orders SET total = total + 1 WHERE id = pg_temp.id('oC') $q$, 'reject:orders_total_sum_check');
SELECT pg_temp.t('6.4: status pending, display confirmed', $q$
  UPDATE orders SET display_status = 'confirmed' WHERE id = pg_temp.id('oC') $q$, 'reject:orders_pending_display_check');
SELECT pg_temp.t('6.4: stock minus', $q$
  UPDATE product_variants SET stock = stock - 11 WHERE id = pg_temp.id('var1') $q$, 'reject:product_variants_stock_check');
SELECT pg_temp.t('6.4: ek district duibar (area NULL)', $q$
  INSERT INTO delivery_zones (shop_id, name, district, area, charge, same_city) VALUES (pg_temp.id('s1'),'{}','Dhaka',NULL,1,true) $q$, 'reject:delivery_zones_shop_id_district_area_key');

-- ===================== v1.2 fix =====================
SELECT pg_temp.t('v1.2: history e onno order er fulfillment', $q$
  INSERT INTO order_status_history (shop_id, order_id, fulfillment_id, to_status, actor_type) VALUES (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fB'),'packed','staff') $q$, 'reject:order_status_history_shop_id_order_id_fulfillment_id_fkey');
SELECT pg_temp.t('v1.2: refund e onno order er payment', $q$
  INSERT INTO refunds (shop_id, order_id, payment_id, amount, method, reason) VALUES (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('payB'),100,'sslcommerz','x') $q$, 'reject:refunds_shop_id_order_id_payment_id_fkey');
SELECT pg_temp.t('v1.2: refund e onno order er fulfillment', $q$
  INSERT INTO refunds (shop_id, order_id, fulfillment_id, amount, method, reason) VALUES (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fB'),100,'sslcommerz','x') $q$, 'reject:refunds_shop_id_order_id_fulfillment_id_fkey');
SELECT pg_temp.t('v1.2: thik refund (nijer order er payment)', $q$
  INSERT INTO refunds (shop_id, order_id, payment_id, fulfillment_id, amount, method, reason) VALUES (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('payA'),pg_temp.id('fA'),100,'sslcommerz','x') $q$, 'accept');
SELECT pg_temp.t('v1.2: refund method ochena', $q$
  INSERT INTO refunds (shop_id, order_id, amount, method, reason) VALUES (pg_temp.id('s1'),pg_temp.id('oA'),100,'whatever','x') $q$, 'reject:refunds_method_check');
SELECT pg_temp.t('v1.2: onno variant er batch item e (recall)', $q$
  INSERT INTO order_item_batches (shop_id, order_item_id, batch_id, variant_id, quantity) VALUES (pg_temp.id('s1'),pg_temp.id('iA'),pg_temp.id('b3'),pg_temp.id('var1'),1) $q$, 'reject:order_item_batches_shop_id_batch_id_variant_id_fkey');
SELECT pg_temp.t('v1.2: thik batch allocation', $q$
  INSERT INTO order_item_batches (shop_id, order_item_id, batch_id, variant_id, quantity) VALUES (pg_temp.id('s1'),pg_temp.id('iA'),pg_temp.id('b1'),pg_temp.id('var1'),1) $q$, 'accept');
SELECT pg_temp.t('v1.2: movement e onno variant er batch', $q$
  INSERT INTO inventory_movements (shop_id, variant_id, batch_id, change, reason) VALUES (pg_temp.id('s1'),pg_temp.id('var1'),pg_temp.id('b3'),-1,'adjust') $q$, 'reject:inventory_movements_shop_id_batch_id_variant_id_fkey');
SELECT pg_temp.t('v1.2: ek fulfillment e duita active shipment', $q$
  INSERT INTO shipments (shop_id, order_id, fulfillment_id, vendor_id, carrier) VALUES
  (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fA'),pg_temp.id('h1'),'steadfast'),
  (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fA'),pg_temp.id('h1'),'pathao') $q$, 'reject:shipments_one_active');
SELECT pg_temp.t('v1.2: return er por abar pathano jay', $q$
  INSERT INTO shipments (shop_id, order_id, fulfillment_id, vendor_id, carrier, status) VALUES
  (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fA'),pg_temp.id('h1'),'steadfast','returned'),
  (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fA'),pg_temp.id('h1'),'steadfast','booked') $q$, 'accept');
SELECT pg_temp.t('v1.2: ekoi carrier e ekoi tracking code', $q$
  INSERT INTO shipments (shop_id, order_id, fulfillment_id, vendor_id, carrier, tracking_code) VALUES
  (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fA'),pg_temp.id('h1'),'steadfast','T1'),
  (pg_temp.id('s1'),pg_temp.id('oB'),pg_temp.id('fB'),pg_temp.id('h1'),'steadfast','T1') $q$, 'reject:shipments_tracking_uniq');
SELECT pg_temp.t('v1.2: shipment cod_amount minus', $q$
  INSERT INTO shipments (shop_id, order_id, fulfillment_id, vendor_id, carrier, cod_amount) VALUES (pg_temp.id('s1'),pg_temp.id('oB'),pg_temp.id('fB'),pg_temp.id('h1'),'other',-5) $q$, 'reject:shipments_cod_amount_check');
SELECT pg_temp.t('v1.2: online order taka chhara confirmed', $q$
  UPDATE orders SET status = 'confirmed', display_status = 'confirmed' WHERE id = pg_temp.id('oB') $q$, 'reject:orders_online_paid_check');
SELECT pg_temp.t('v1.2: online order paid hoye confirmed', $q$
  UPDATE orders SET payment_status = 'paid', status = 'confirmed', display_status = 'confirmed' WHERE id = pg_temp.id('oB') $q$, 'accept');
SELECT pg_temp.t('v1.2: COD order confirm (unpaid) cholbe', $q$
  UPDATE orders SET status = 'confirmed', display_status = 'confirmed' WHERE id = pg_temp.id('oC') $q$, 'accept');
SELECT pg_temp.t('v1.2: redemption er coupon order er coupon na', $q$
  INSERT INTO coupon_redemptions (shop_id, coupon_id, order_id, phone) VALUES (pg_temp.id('s1'),pg_temp.id('k2'),pg_temp.id('oA'),'+8801711111111') $q$, 'reject:coupon_redemptions_shop_id_order_id_coupon_id_fkey');
SELECT pg_temp.t('v1.2: thik redemption', $q$
  INSERT INTO coupon_redemptions (shop_id, coupon_id, order_id, phone) VALUES (pg_temp.id('s1'),pg_temp.id('k1'),pg_temp.id('oA'),'+8801711111111') $q$, 'accept');
SELECT pg_temp.t('v1.2: phone E.164 na (users)', $q$
  INSERT INTO users (shop_id, phone) VALUES (pg_temp.id('s1'),'01711111111') $q$, 'reject:phone_bd_check');
SELECT pg_temp.t('v1.2: phone E.164 na (order update)', $q$
  UPDATE orders SET customer_phone = '01711-111111' WHERE id = pg_temp.id('oC') $q$, 'reject:phone_bd_check');
SELECT pg_temp.t('v1.2: account delete (anonymize) er por ekoi phone e notun account', $q$
  DO $d$ BEGIN
    UPDATE users SET phone = NULL, name = NULL, email = NULL, deleted_at = now() WHERE id = pg_temp.id('u1');
    INSERT INTO users (shop_id, phone) VALUES (pg_temp.id('s1'),'+8801711111111');
  END $d$ $q$, 'accept');
SELECT pg_temp.t('v1.2: deleted user er phone rakha jabe na', $q$
  UPDATE users SET deleted_at = now() WHERE id = pg_temp.id('u1') $q$, 'reject:users_deleted_phone_check');
SELECT pg_temp.t('v1.2: duita default address', $q$
  INSERT INTO addresses (shop_id, user_id, recipient_name, recipient_phone, district, area, address_line, is_default) VALUES
  (pg_temp.id('s1'),pg_temp.id('u1'),'a','+8801711111111','Dhaka','X','Y',true),
  (pg_temp.id('s1'),pg_temp.id('u1'),'b','+8801711111111','Dhaka','X','Z',true) $q$, 'reject:addresses_one_default');
SELECT pg_temp.t('v1.2: category nijer parent', $q$
  UPDATE categories SET parent_id = id WHERE id = pg_temp.id('c1') $q$, 'reject:categories_not_own_parent_check');
SELECT pg_temp.t('v1.2: variable_weight kintu unit piece', $q$
  INSERT INTO products (shop_id, vendor_id, slug, name, unit, variable_weight) VALUES (pg_temp.id('s1'),pg_temp.id('h1'),'fish','{}','piece',true) $q$, 'reject:products_variable_weight_kg_check');
SELECT pg_temp.t('v1.2: compare_at_price dam er cheye kom', $q$
  UPDATE product_variants SET compare_at_price = 1 WHERE id = pg_temp.id('var1') $q$, 'reject:product_variants_compare_at_check');
SELECT pg_temp.t('v1.2: coupon per_phone_limit 0', $q$
  INSERT INTO coupons (shop_id, code, kind, value, per_phone_limit) VALUES (pg_temp.id('s1'),'B1','fixed',100,0) $q$, 'reject:coupons_per_phone_limit_check');
SELECT pg_temp.t('v1.3: coupon usage_limit 0 (nijer CHECK e; -1 onno CHECK eo dhora porto)', $q$
  INSERT INTO coupons (shop_id, code, kind, value, usage_limit) VALUES (pg_temp.id('s1'),'B2','fixed',100,0) $q$, 'reject:coupons_usage_limit_check');
SELECT pg_temp.t('v1.2: coupon min_order minus', $q$
  INSERT INTO coupons (shop_id, code, kind, value, min_order) VALUES (pg_temp.id('s1'),'B3','fixed',100,-500) $q$, 'reject:coupons_min_order_check');
SELECT pg_temp.t('v1.2: coupon shuru-r age shesh', $q$
  INSERT INTO coupons (shop_id, code, kind, value, starts_at, expires_at) VALUES (pg_temp.id('s1'),'B4','fixed',100,now(),now() - interval '1 day') $q$, 'reject:coupons_dates_check');
SELECT pg_temp.t('v1.2: zone min_order minus', $q$
  INSERT INTO delivery_zones (shop_id, name, district, charge, min_order, same_city) VALUES (pg_temp.id('s1'),'{}','Khulna',1,-1,false) $q$, 'reject:delivery_zones_min_order_check');
SELECT pg_temp.t('v1.2: customer session e staff_id', $q$
  INSERT INTO sessions (shop_id, token_hash, subject_type, user_id, staff_id, expires_at) VALUES (pg_temp.id('s1'),'\x10','customer',pg_temp.id('u1'),pg_temp.id('st1'),now()) $q$, 'reject:sessions_staff_subject_check');
SELECT pg_temp.t('v1.2: customer session mfa_pending', $q$
  INSERT INTO sessions (shop_id, token_hash, subject_type, user_id, stage, expires_at) VALUES (pg_temp.id('s1'),'\x11','customer',pg_temp.id('u1'),'mfa_pending',now()) $q$, 'reject:sessions_stage_staff_check');
SELECT pg_temp.t('v1.2: staff temp_token (mfa_pending) cholbe', $q$
  INSERT INTO sessions (shop_id, token_hash, subject_type, staff_id, stage, expires_at) VALUES (pg_temp.id('s1'),'\x12','staff',pg_temp.id('st1'),'mfa_pending',now() + interval '5 minutes') $q$, 'accept');
SELECT pg_temp.t('v1.2: onno shop er staff er session', $q$
  INSERT INTO sessions (shop_id, token_hash, subject_type, staff_id, expires_at) VALUES (pg_temp.id('s2'),'\x13','staff',pg_temp.id('st1'),now()) $q$, 'reject:sessions_shop_id_staff_id_fkey');
SELECT pg_temp.t('v1.2: job attempts max er beshi', $q$
  INSERT INTO jobs (kind, attempts, max_attempts) VALUES ('sms.send',6,5) $q$, 'reject:jobs_attempts_le_max_check');
SELECT pg_temp.t('v1.2: job done ar failed duitai', $q$
  INSERT INTO jobs (kind, done_at, failed_at) VALUES ('sms.send',now(),now()) $q$, 'reject:jobs_done_or_failed_check');
SELECT pg_temp.t('v1.2: job claim (6.4 v1.2 query) max_attempts chhuya job ney na', $q$
  DO $d$ DECLARE n INT; BEGIN
    INSERT INTO jobs (kind, attempts, max_attempts) VALUES ('t.maxed',5,5), ('t.fresh',0,5);
    WITH next AS (
      SELECT id FROM jobs
      WHERE done_at IS NULL AND failed_at IS NULL AND attempts < max_attempts AND run_at <= now()
        AND (locked_at IS NULL OR locked_at < now() - interval '5 minutes')
      ORDER BY run_at FOR UPDATE SKIP LOCKED LIMIT 10)
    UPDATE jobs j SET locked_at = now(), attempts = attempts + 1 FROM next WHERE j.id = next.id;
    SELECT count(*) INTO n FROM jobs WHERE kind = 't.maxed' AND locked_at IS NOT NULL;
    IF n <> 0 THEN RAISE EXCEPTION 'maxed job claimed'; END IF;
  END $d$ $q$, 'accept');

-- ===================== v1.3 fix =====================
SELECT pg_temp.t('v1.3: staff setup_pending session cholbe', $q$
  INSERT INTO sessions (shop_id, token_hash, subject_type, staff_id, stage, expires_at) VALUES (pg_temp.id('s1'),'\x14','staff',pg_temp.id('st1'),'setup_pending',now() + interval '15 minutes') $q$, 'accept');
SELECT pg_temp.t('v1.3: customer session setup_pending na', $q$
  INSERT INTO sessions (shop_id, token_hash, subject_type, user_id, stage, expires_at) VALUES (pg_temp.id('s1'),'\x15','customer',pg_temp.id('u1'),'setup_pending',now()) $q$, 'reject:sessions_stage_staff_check');
SELECT pg_temp.t('v1.3: ek fulfillment e duita cancel refund', $q$
  INSERT INTO refunds (shop_id, order_id, fulfillment_id, payment_id, amount, method, reason, kind) VALUES
  (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fA'),pg_temp.id('payA'),100,'sslcommerz','x','cancel'),
  (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fA'),pg_temp.id('payA'),100,'sslcommerz','x','cancel') $q$, 'reject:refunds_cancel_once');
SELECT pg_temp.t('v1.3: failed cancel refund er por abar cholbe', $q$
  INSERT INTO refunds (shop_id, order_id, fulfillment_id, payment_id, amount, method, reason, kind, status) VALUES
  (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fA'),pg_temp.id('payA'),100,'sslcommerz','x','cancel','failed'),
  (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fA'),pg_temp.id('payA'),100,'sslcommerz','x','cancel','pending') $q$, 'accept');
SELECT pg_temp.t('v1.3: cancel refund fulfillment chhara', $q$
  INSERT INTO refunds (shop_id, order_id, amount, method, reason, kind) VALUES (pg_temp.id('s1'),pg_temp.id('oA'),100,'sslcommerz','x','cancel') $q$, 'reject:refunds_cancel_fulfillment_check');
SELECT pg_temp.t('v1.3: Bangla search NFC (ো ek code point ba ে+া)', $q$
  DO $d$ BEGIN
    INSERT INTO products (shop_id, vendor_id, slug, name) VALUES (pg_temp.id('s1'),pg_temp.id('h1'),'korma',jsonb_build_object('bn', U&'\0995\09CB\09B0\09AE\09BE'));
    IF NOT EXISTS (SELECT 1 FROM products WHERE shop_id = pg_temp.id('s1') AND slug = 'korma'
                   AND search_text ILIKE '%' || normalize(U&'\0995\09C7\09BE\09B0\09AE\09BE', NFC) || '%') THEN
      RAISE EXCEPTION 'NFC search mile na';
    END IF;
  END $d$ $q$, 'accept');
SELECT pg_temp.t('v1.3: complaint due_at = 72 ghonta', $q$
  DO $d$ DECLARE d INTERVAL; BEGIN
    INSERT INTO complaints (shop_id, name, phone, message) VALUES (pg_temp.id('s1'),'a','+8801711111111','x') RETURNING due_at - created_at INTO d;
    IF d <> interval '72 hours' THEN RAISE EXCEPTION 'due_at %', d; END IF;
  END $d$ $q$, 'accept');
SELECT pg_temp.t('v1.3: jobs.sweep: crash e atke thaka job failed, cholche emon job na', $q$
  DO $d$ DECLARE n INT; BEGIN
    INSERT INTO jobs (kind, attempts, max_attempts, locked_at) VALUES
      ('t.stuck', 5, 5, now() - interval '10 minutes'), ('t.running', 5, 5, now() - interval '1 minute');
    UPDATE jobs SET failed_at = now(), locked_at = NULL, last_error = coalesce(last_error, 'worker crash (lock timeout)')
    WHERE done_at IS NULL AND failed_at IS NULL AND attempts >= max_attempts
      AND locked_at < now() - interval '5 minutes';
    SELECT count(*) INTO n FROM jobs WHERE kind = 't.stuck' AND failed_at IS NOT NULL;
    IF n <> 1 THEN RAISE EXCEPTION 'atke thaka job failed hoyni'; END IF;
    SELECT count(*) INTO n FROM jobs WHERE kind = 't.running' AND failed_at IS NOT NULL;
    IF n <> 0 THEN RAISE EXCEPTION 'cholche emon job failed holo'; END IF;
  END $d$ $q$, 'accept');
SELECT pg_temp.t('v1.3: shop_app: audit_logs e shudhu ip UPDATE, orders e DELETE na (role thakle)', $q$
  DO $d$ BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'shop_app') AND NOT (
         has_column_privilege('shop_app', 'audit_logs', 'ip', 'UPDATE')
         AND NOT has_column_privilege('shop_app', 'audit_logs', 'action', 'UPDATE')
         AND NOT has_table_privilege('shop_app', 'orders', 'DELETE')) THEN
      RAISE EXCEPTION 'shop_app er grant bhul';
    END IF;
  END $d$ $q$, 'accept');

-- ===================== v1.4 fix =====================
SELECT pg_temp.t('v1.4: coupon ache kintu tar shorto (snapshot) nai', $q$
  UPDATE orders SET coupon_kind = NULL WHERE id = pg_temp.id('oA') $q$, 'reject:orders_coupon_snapshot_check');
SELECT pg_temp.t('v1.4: coupon chhara coupon er shorto', $q$
  UPDATE orders SET coupon_code = 'X', coupon_kind = 'fixed', coupon_value = 1, coupon_funded_by = 'shop' WHERE id = pg_temp.id('oB') $q$, 'reject:orders_coupon_snapshot_check');
SELECT pg_temp.t('v1.4: snapshot e percent 100 er beshi', $q$
  UPDATE orders SET coupon_kind = 'percent', coupon_value = 150 WHERE id = pg_temp.id('oA') $q$, 'reject:orders_coupon_percent_check');
SELECT pg_temp.t('v1.4: online order e COD confirm token', $q$
  UPDATE orders SET confirm_token_hash = '\x01' WHERE id = pg_temp.id('oB') $q$, 'reject:orders_confirm_token_cod_check');
SELECT pg_temp.t('v1.4: COD order e confirm token cholbe', $q$
  UPDATE orders SET confirm_token_hash = '\x01' WHERE id = pg_temp.id('oC') $q$, 'accept');
SELECT pg_temp.t('v1.4: duita order e ekoi confirm token', $q$
  DO $d$ BEGIN
    UPDATE orders SET confirm_token_hash = '\x02' WHERE id = pg_temp.id('oC');
    INSERT INTO orders (shop_id, order_number, public_token, idempotency_key, idempotency_hash, customer_name, customer_phone,
                        recipient_name, recipient_phone, ship_district, ship_area, ship_address, subtotal, total, payment_method, confirm_token_hash)
    VALUES (pg_temp.id('s1'),'SB-100004','tD','kD','\x00','D','+8801744444444','D','+8801744444444','Dhaka','Mirpur','Road 4',100,100,'cod','\x02');
  END $d$ $q$, 'reject:orders_confirm_token_uniq');
SELECT pg_temp.t('v1.4: succeeded payment abar reconcile e', $q$
  UPDATE payments SET status = 'succeeded', reconcile_after = now() WHERE id = pg_temp.id('payA') $q$, 'reject:payments_reconcile_done_check');
SELECT pg_temp.t('v1.4: local cancelled payment o reconcile e thake', $q$
  UPDATE payments SET status = 'cancelled', reconcile_after = now() WHERE id = pg_temp.id('payA') $q$, 'accept');
SELECT pg_temp.t('v1.4: commission_bps 10000 er beshi', $q$
  UPDATE order_items SET commission_bps = 10001 WHERE id = pg_temp.id('iA') $q$, 'reject:order_items_commission_bps_check');

-- ===================== v1.5 fix =====================
SELECT pg_temp.t('v1.5: ojon-variable item e ordered_weight_g nai', $q$
  INSERT INTO order_items (shop_id, order_id, fulfillment_id, vendor_id, variant_id, product_name, sku, unit_price, quantity, line_total, variable_weight)
  VALUES (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fA'),pg_temp.id('h1'),pg_temp.id('var1'),'{}','RR5',100000,1,100000,true) $q$, 'reject:order_items_weight_check');
SELECT pg_temp.t('v1.5: fixed item e packed_weight_g', $q$
  UPDATE order_items SET packed_weight_g = 500 WHERE id = pg_temp.id('iA') $q$, 'reject:order_items_weight_check');
SELECT pg_temp.t('v1.5: ojon-variable item ojon shoho cholbe', $q$
  INSERT INTO order_items (shop_id, order_id, fulfillment_id, vendor_id, variant_id, product_name, sku, unit_price, quantity, line_total, variable_weight, ordered_weight_g, packed_weight_g)
  VALUES (pg_temp.id('s1'),pg_temp.id('oA'),pg_temp.id('fA'),pg_temp.id('h1'),pg_temp.id('var1'),'{}','RR5',100000,1,105000,true,1000,1050) $q$, 'accept');
SELECT pg_temp.t('v1.5: removed item e line_total baki', $q$
  UPDATE order_items SET status = 'removed' WHERE id = pg_temp.id('iA') $q$, 'reject:order_items_removed_zero_check');
SELECT pg_temp.t('v1.5: removed item, taka 0, quantity ager ta', $q$
  UPDATE order_items SET status = 'removed', line_total = 0, discount_share = 0 WHERE id = pg_temp.id('iA') $q$, 'accept');
SELECT pg_temp.t('v1.5: coupon chhara discount', $q$
  UPDATE orders SET discount = 100, total = total - 100 WHERE id = pg_temp.id('oB') $q$, 'reject:orders_discount_coupon_check');
SELECT pg_temp.t('v1.5: coupon chhara coupon_vendor_id', $q$
  UPDATE orders SET coupon_vendor_id = pg_temp.id('h1') WHERE id = pg_temp.id('oB') $q$, 'reject:orders_coupon_snapshot_check');
SELECT pg_temp.t('v1.5: vendor-funded snapshot e vendor nai', $q$
  UPDATE orders SET coupon_funded_by = 'vendor' WHERE id = pg_temp.id('oA') $q$, 'reject:orders_coupon_vendor_check');
SELECT pg_temp.t('v1.5: TOTP secret ache kintu key_version nai', $q$
  UPDATE staff SET totp_secret_enc = '\x01', totp_nonce = '\x02' WHERE id = pg_temp.id('st1') $q$, 'reject:staff_totp_check');
SELECT pg_temp.t('v1.5: TOTP secret key_version shoho cholbe', $q$
  UPDATE staff SET totp_secret_enc = '\x01', totp_nonce = '\x02', totp_key_version = 1 WHERE id = pg_temp.id('st1') $q$, 'accept');

-- ===================== v1.7 fix =====================
SELECT pg_temp.t('v1.7: status confirmed, display cancelled', $q$
  UPDATE orders SET status = 'confirmed', display_status = 'cancelled' WHERE id = pg_temp.id('oC') $q$, 'reject:orders_cancelled_display_check');
SELECT pg_temp.t('v1.7: fulfillment e total != subtotal - discount + delivery', $q$
  UPDATE order_fulfillments SET total = total + 1 WHERE id = pg_temp.id('fB') $q$, 'reject:order_fulfillments_total_sum_check');
SELECT pg_temp.t('v1.7: delete kora account e naam rakha', $q$
  UPDATE users SET phone = NULL, name = 'X', deleted_at = now() WHERE id = pg_temp.id('u1') $q$, 'reject:users_deleted_pii_check');
SELECT pg_temp.t('v1.7: ekoi coupon code choto hater (EID ar eid)', $q$
  INSERT INTO coupons (shop_id, code, kind, value) VALUES (pg_temp.id('s1'),'eid','fixed',100) $q$, 'reject:coupons_code_uniq');
SELECT pg_temp.t('v1.7: hasanta wala shobdo (অর্গানিক) trigram index diye ILIKE e mile', $q$
  DO $d$ BEGIN
    INSERT INTO products (shop_id, vendor_id, slug, name) VALUES (pg_temp.id('s1'),pg_temp.id('h1'),'organic-chal',jsonb_build_object('bn', 'অর্গানিক চাল'));
    PERFORM set_config('enable_seqscan', 'off', true);   -- products_search_trgm er path; subtransaction e ferot jay
    IF NOT EXISTS (SELECT 1 FROM products WHERE search_text ILIKE '%' || normalize('র্গানি', NFC) || '%') THEN
      RAISE EXCEPTION 'hasanta er moddhe diye search mile na';
    END IF;
  END $d$ $q$, 'accept');

\o
-- ===================== Fol =====================
SELECT (CASE WHEN ok THEN 'ok    ' ELSE 'FAIL  ' END) || label || '  =>  ' || detail FROM test_results;
SELECT count(*) FILTER (WHERE ok) || ' / ' || count(*) || ' test ashar moto' FROM test_results;
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM test_results WHERE NOT ok) THEN
    RAISE EXCEPTION 'constraint test fail: upore FAIL line gulo dekho';
  END IF;
END $$;
ROLLBACK;
