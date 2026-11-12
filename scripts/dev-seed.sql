-- Shudhu local (M3): `sobujbazar.localhost` shop e category, product, variant ar stock (batch + movement, 8.12).
-- Admin product API M4 er staff session chhara 401, tai SQL. Abar chalale kichu bodlay na (slug/sku thakle bad).
-- Chobi nai: website `tint` placeholder dekhay (10.10 item 3). Chalano: make dev-seed (shop_owner).
\set ON_ERROR_STOP on
BEGIN;

DO $$
DECLARE
  sid BIGINT;
  vid BIGINT;
  cid BIGINT;
  pid BIGINT;
  var_id BIGINT;
  bid BIGINT;
  c JSONB;
  p JSONB;
  v JSONB;
BEGIN
  SELECT shop_id INTO sid FROM shop_domains WHERE domain = 'sobujbazar.localhost';
  IF sid IS NULL THEN
    RAISE EXCEPTION 'sobujbazar.localhost nai: age shop-api shop create (M1 step 7)';
  END IF;
  SELECT id INTO vid FROM vendors WHERE shop_id = sid AND is_house;

  FOR c IN SELECT * FROM jsonb_array_elements('[
    {"slug":"chal","bn":"চাল","en":"Rice","sort":1,"products":[
      {"slug":"red-rice","bn":"অর্গানিক লাল চাল","en":"Organic red rice","unit":"kg",
       "desc_bn":"পাহাড়ি জমির লাল চাল।\n\nরান্নার আগে ৩০ মিনিট ভিজিয়ে রাখুন।","desc_en":"Hill-grown red rice.\n\nSoak for 30 minutes before cooking.",
       "variants":[{"sku":"RICE-RED-1","bn":"১ কেজি","en":"1 kg","price":18000,"compare":20000,"stock":40},
                   {"sku":"RICE-RED-5","bn":"৫ কেজি","en":"5 kg","price":85000,"stock":12}]},
      {"slug":"chinigura","bn":"চিনিগুঁড়া চাল","en":"Chinigura rice","unit":"kg",
       "variants":[{"sku":"RICE-CHG-1","bn":"১ কেজি","en":"1 kg","price":16000,"stock":0}]}]},
    {"slug":"oil","bn":"তেল","en":"Oil","sort":2,"products":[
      {"slug":"mustard-oil","bn":"ঘানি ভাঙা সরিষার তেল","en":"Cold-pressed mustard oil","unit":"litre",
       "variants":[{"sku":"OIL-MUS-1","bn":"১ লিটার","en":"1 litre","price":32000,"stock":25},
                   {"sku":"OIL-MUS-5","bn":"৫ লিটার","en":"5 litre","price":155000,"stock":3}]}]},
    {"slug":"honey","bn":"মধু","en":"Honey","sort":3,"products":[
      {"slug":"sundarban-honey","bn":"সুন্দরবনের খলিশা মধু","en":"Sundarbans khalisha honey","unit":"piece",
       "variants":[{"sku":"HNY-KHL-500","bn":"৫০০ গ্রাম","en":"500 g","price":65000,"stock":18}]},
      {"slug":"mustard-honey","bn":"সরিষা ফুলের মধু","en":"Mustard flower honey","unit":"piece",
       "variants":[{"sku":"HNY-MUS-500","bn":"৫০০ গ্রাম","en":"500 g","price":45000,"compare":50000,"stock":30}]}]}
  ]'::jsonb)
  LOOP
    INSERT INTO categories (shop_id, slug, name, sort_order)
    VALUES (sid, c->>'slug', jsonb_build_object('bn', c->>'bn', 'en', c->>'en'), (c->>'sort')::int)
    ON CONFLICT (shop_id, slug) DO NOTHING;
    SELECT id INTO cid FROM categories WHERE shop_id = sid AND slug = c->>'slug';

    FOR p IN SELECT * FROM jsonb_array_elements(c->'products') LOOP
      INSERT INTO products (shop_id, vendor_id, category_id, slug, name, description, unit, search_keywords)
      VALUES (sid, vid, cid, p->>'slug', jsonb_build_object('bn', p->>'bn', 'en', p->>'en'),
              jsonb_strip_nulls(jsonb_build_object('bn', p->>'desc_bn', 'en', p->>'desc_en')),
              p->>'unit', replace(p->>'slug', '-', ' '))
      ON CONFLICT (shop_id, slug) DO NOTHING
      RETURNING id INTO pid;
      CONTINUE WHEN pid IS NULL;  -- age theke ache

      FOR v IN SELECT * FROM jsonb_array_elements(p->'variants') LOOP
        INSERT INTO product_variants (shop_id, product_id, vendor_id, sku, options, price, compare_at_price, stock, sort_order)
        VALUES (sid, pid, vid, v->>'sku',
                jsonb_build_object('size', jsonb_build_object('bn', v->>'bn', 'en', v->>'en')),
                (v->>'price')::bigint, (v->>'compare')::bigint, (v->>'stock')::int,
                (SELECT count(*) FROM product_variants WHERE product_id = pid))
        RETURNING id INTO var_id;
        -- Stock = batch er baki = movement er jog (8.12)
        IF (v->>'stock')::int > 0 THEN
          INSERT INTO inventory_batches (shop_id, variant_id, quantity_received, quantity_remaining)
          VALUES (sid, var_id, (v->>'stock')::int, (v->>'stock')::int)
          RETURNING id INTO bid;
          INSERT INTO inventory_movements (shop_id, variant_id, batch_id, change, reason, note)
          VALUES (sid, var_id, bid, (v->>'stock')::int, 'purchase', 'dev-seed');
        END IF;
      END LOOP;
    END LOOP;
  END LOOP;
END
$$;

COMMIT;
