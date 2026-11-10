-- 001_init.sql : prothom migration (MVP er shob table), design version 1.8
-- Niyom: taka = BIGINT poisha, shomoy = TIMESTAMPTZ (UTC), phone = +8801XXXXXXXXX (domain phone_bd),
-- bhashar lekha = JSONB {"bn": "...", "en": "..."}.
-- Tenant: protiti business table e shop_id. Parent table e UNIQUE (shop_id, id), child er foreign key
-- (shop_id, parent_id), tai ek shop er row kokhono onno shop er row ke point korte pare na.
-- Vendor: catalog, order item, fulfillment, shipment e vendor_id. Single store e shop er ekta house vendor.
-- Runner: ei file ar schema_migrations er insert ek transaction e; v1.6: shurute pg_advisory_xact_lock ar
-- set_config('app.env', $1, true) (6.1).
-- v1.2 theke v1.8 er protiti bodol "v1.2:" ... "v1.8:" comment diye chinhito (v1.6: shudhu runner er comment;
-- v1.7: shesh er role check; v1.8: schema_migrations er onumoti).
-- v1.5: test e je CHECK er naam lage, tar naam deya (auto naam jemon orders_check4 kram bodlale bodlay).

CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- v1.2: database er LC_CTYPE C/POSIX hole Bangla trigram khali ashe, search ranking kaj kore na.
DO $$
BEGIN
  IF cardinality(show_trgm('লাল চাল')) = 0 THEN
    RAISE EXCEPTION 'pg_trgm Bangla trigram banate parche na (LC_CTYPE C/POSIX?). Database C.UTF-8 ba en_US.UTF-8 diye banao.';
  END IF;
END $$;

-- v1.2: Bangladesh mobile, E.164 (Rule 6). Normalize Go te hoy, database shudhu bhul format atkay.
CREATE DOMAIN phone_bd AS TEXT CHECK (VALUE ~ '^\+8801[3-9][0-9]{8}$');

-- ===================== Shop (tenant), vendor, settings =====================

CREATE TABLE shops (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  slug         TEXT NOT NULL UNIQUE,
  name         JSONB NOT NULL,
  mode         TEXT NOT NULL DEFAULT 'single' CHECK (mode IN ('single','marketplace')),
  order_prefix TEXT NOT NULL CHECK (order_prefix ~ '^[A-Z]{2,5}$'),   -- 'SB' → "SB-100001"
  timezone     TEXT NOT NULL DEFAULT 'Asia/Dhaka',                   -- dekhano, "ajker", cutoff
  status       TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','closed')),
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE shop_domains (
  domain     TEXT PRIMARY KEY CHECK (domain = lower(domain)),        -- 'sobujbazar.com'; v1.3: www na (Caddy 301, 3.2)
  shop_id    BIGINT NOT NULL REFERENCES shops(id),
  is_primary BOOLEAN NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX shop_domains_primary_uniq ON shop_domains (shop_id) WHERE is_primary;

CREATE TABLE vendors (
  id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id        BIGINT NOT NULL REFERENCES shops(id),
  slug           TEXT NOT NULL,
  name           JSONB NOT NULL,
  is_house       BOOLEAN NOT NULL DEFAULT false,      -- shop nijei; single store e ekmatro vendor
  status         TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('pending','active','suspended','closed')),
  commission_bps INT NOT NULL DEFAULT 0 CHECK (commission_bps BETWEEN 0 AND 10000),  -- 1000 = 10%
  profile        JSONB NOT NULL DEFAULT '{}',         -- logo_key, description, pickup thikana
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (NOT is_house OR commission_bps = 0),
  UNIQUE (shop_id, slug),
  UNIQUE (shop_id, id)
);
CREATE UNIQUE INDEX vendors_one_house ON vendors (shop_id) WHERE is_house;

CREATE TABLE shop_settings (
  shop_id    BIGINT PRIMARY KEY REFERENCES shops(id),
  draft      JSONB NOT NULL,                -- admin ekhon edit korche
  published  JSONB NOT NULL,                -- website/app ei ta dekhay
  version    INT NOT NULL DEFAULT 1,
  updated_by BIGINT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE settings_history (
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id    BIGINT NOT NULL REFERENCES shops(id),
  version    INT NOT NULL,
  data       JSONB NOT NULL,
  created_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (shop_id, version)
);

CREATE TABLE shop_credentials (
  shop_id     BIGINT NOT NULL REFERENCES shops(id),
  provider    TEXT NOT NULL CHECK (provider IN ('sslcommerz','bkash','nagad','sms','steadfast','pathao','redx')),
  secret_enc  BYTEA NOT NULL,               -- AES-GCM; v1.3: AAD = shop_id:provider:key_version (2.2)
  nonce       BYTEA NOT NULL,
  key_version SMALLINT NOT NULL DEFAULT 1,  -- kon CREDENTIALS_KEY diye encrypt (rotation er jonno)
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (shop_id, provider)
);
-- v1.2: 'r2' provider bad. R2 platform er jinish (Rule 15), credential env e.

CREATE TABLE shop_counters (
  shop_id BIGINT NOT NULL REFERENCES shops(id),
  name    TEXT NOT NULL,                    -- 'order_number' (shop khola-r shomoy 0 diye seed)
  value   BIGINT NOT NULL DEFAULT 0,
  PRIMARY KEY (shop_id, name)
);

CREATE TABLE translations (
  shop_id BIGINT NOT NULL REFERENCES shops(id),
  locale  TEXT NOT NULL CHECK (locale IN ('bn','en')),
  key     TEXT NOT NULL,                    -- 'cart.add'
  value   TEXT NOT NULL,
  PRIMARY KEY (shop_id, locale, key)
);

-- ===================== Customer, staff, login =====================

CREATE TABLE users (                        -- customer; protiti shop er nijer customer
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id    BIGINT NOT NULL REFERENCES shops(id),
  phone      phone_bd,                      -- v1.2: account delete hole NULL (anonymize)
  name       TEXT,
  email      TEXT,
  locale     TEXT NOT NULL DEFAULT 'bn' CHECK (locale IN ('bn','en')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  -- v1.2: delete kora account e personal data thakbe na, ar ekoi phone e notun account khola jabe
  CONSTRAINT users_deleted_phone_check CHECK ((deleted_at IS NULL) = (phone IS NOT NULL)),
  CONSTRAINT users_deleted_pii_check CHECK (deleted_at IS NULL OR (name IS NULL AND email IS NULL)),
  UNIQUE (shop_id, phone),                  -- NULL alada gona hoy
  UNIQUE (shop_id, id)
);

CREATE TABLE staff (
  id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id              BIGINT NOT NULL REFERENCES shops(id),
  vendor_id            BIGINT,              -- NULL = shop er staff (shob vendor); thakle shudhu oi vendor er
  email                TEXT NOT NULL,
  name                 TEXT NOT NULL,
  password_hash        TEXT NOT NULL,       -- pbkdf2-sha256$600000$<salt>$<hash>
  must_change_password BOOLEAN NOT NULL DEFAULT true,  -- v1.2: temporary password, prothom login e bodlabe (3.8)
  password_changed_at  TIMESTAMPTZ,
  role                 TEXT NOT NULL CHECK (role IN ('owner','admin','staff','vendor_owner','vendor_staff')),
  totp_secret_enc      BYTEA,               -- v1.5: AES-GCM, TOTP_KEY, AAD = shop_id:staff_id:key_version (8.3)
  totp_nonce           BYTEA,
  totp_key_version     SMALLINT,            -- v1.5: kon TOTP_KEY diye encrypt (rotation, 13.5)
  totp_last_step       BIGINT,              -- v1.2: shesh use kora TOTP time-step; ekoi code abar cholbe na
  failed_logins        INT NOT NULL DEFAULT 0 CHECK (failed_logins >= 0),  -- v1.2: restart e-o lock thake
  failed_mfa           INT NOT NULL DEFAULT 0 CHECK (failed_mfa >= 0),        -- v1.3: TOTP/recovery bhul, temp_token jure; 10 e lock (8.3)
  locked_until         TIMESTAMPTZ,
  is_active            BOOLEAN NOT NULL DEFAULT true,
  last_login_at        TIMESTAMPTZ,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT staff_vendor_role_check CHECK ((vendor_id IS NULL) = (role IN ('owner','admin','staff'))),
  CONSTRAINT staff_totp_check CHECK ((totp_secret_enc IS NULL) = (totp_nonce IS NULL)
    AND (totp_secret_enc IS NULL) = (totp_key_version IS NULL)),   -- v1.5: key_version o
  UNIQUE (shop_id, id),
  FOREIGN KEY (shop_id, vendor_id) REFERENCES vendors (shop_id, id)
);
CREATE UNIQUE INDEX staff_email_uniq ON staff (shop_id, lower(email));

CREATE TABLE staff_recovery_codes (         -- v1.2: TOTP er phone harale; 10 ta code, protiti ekbar
  shop_id   BIGINT NOT NULL REFERENCES shops(id),
  staff_id  BIGINT NOT NULL,
  code_hash BYTEA NOT NULL,                 -- v1.5: HMAC-SHA256(RECOVERY_SECRET, staff_id:code) (8.3)
  used_at   TIMESTAMPTZ,
  PRIMARY KEY (staff_id, code_hash),
  FOREIGN KEY (shop_id, staff_id) REFERENCES staff (shop_id, id)
);

CREATE TABLE sessions (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id      BIGINT NOT NULL REFERENCES shops(id),
  token_hash   BYTEA NOT NULL UNIQUE,       -- sha256(token); asol token kokhono save na
  subject_type TEXT NOT NULL CHECK (subject_type IN ('customer','staff')),
  -- v1.2: subject_id er bodole duita column + foreign key. Customer er token kokhono staff id hote parbe na.
  user_id      BIGINT,
  staff_id     BIGINT,
  -- v1.2: password er por TOTP er opekkhay thaka temp_token ekhanei (stage 'mfa_pending', 5 minute)
  -- v1.3: 'setup_pending' = prothom login e password bodlano ar TOTP setup (15 minute, shudhu setup endpoint, 8.3)
  stage        TEXT NOT NULL DEFAULT 'active' CHECK (stage IN ('active','mfa_pending','setup_pending')),
  mfa_attempts INT NOT NULL DEFAULT 0 CHECK (mfa_attempts BETWEEN 0 AND 5),
  user_agent   TEXT,
  ip           TEXT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at   TIMESTAMPTZ NOT NULL,
  revoked_at   TIMESTAMPTZ,
  CONSTRAINT sessions_customer_subject_check CHECK ((subject_type = 'customer') = (user_id IS NOT NULL)),
  CONSTRAINT sessions_staff_subject_check CHECK ((subject_type = 'staff') = (staff_id IS NOT NULL)),
  CONSTRAINT sessions_stage_staff_check CHECK (stage = 'active' OR subject_type = 'staff'),
  FOREIGN KEY (shop_id, user_id)  REFERENCES users (shop_id, id),
  FOREIGN KEY (shop_id, staff_id) REFERENCES staff (shop_id, id)
);
CREATE INDEX sessions_user_idx   ON sessions (user_id)  WHERE user_id IS NOT NULL;
CREATE INDEX sessions_staff_idx  ON sessions (staff_id) WHERE staff_id IS NOT NULL;
CREATE INDEX sessions_expiry_idx ON sessions (expires_at);

CREATE TABLE otp_codes (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id     BIGINT NOT NULL REFERENCES shops(id),
  phone       phone_bd NOT NULL,
  purpose     TEXT NOT NULL CHECK (purpose IN ('login','checkout')),
  code_hash   BYTEA NOT NULL,               -- HMAC-SHA256(OTP_SECRET, phone:purpose:code) (8.2)
  attempts    INT NOT NULL DEFAULT 0 CHECK (attempts BETWEEN 0 AND 5),
  expires_at  TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX otp_phone_idx ON otp_codes (shop_id, phone, created_at DESC);

CREATE TABLE addresses (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id         BIGINT NOT NULL REFERENCES shops(id),
  user_id         BIGINT NOT NULL,
  label           TEXT,
  recipient_name  TEXT NOT NULL,
  recipient_phone phone_bd NOT NULL,
  district        TEXT NOT NULL,
  area            TEXT NOT NULL,
  address_line    TEXT NOT NULL,
  is_default      BOOLEAN NOT NULL DEFAULT false,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (shop_id, user_id) REFERENCES users (shop_id, id)
);
CREATE INDEX addresses_user_idx ON addresses (user_id);
CREATE UNIQUE INDEX addresses_one_default ON addresses (user_id) WHERE is_default;  -- v1.2

CREATE TABLE blocked_phones (
  shop_id    BIGINT NOT NULL REFERENCES shops(id),
  phone      phone_bd NOT NULL,
  reason     TEXT,
  staff_id   BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (shop_id, phone),
  FOREIGN KEY (shop_id, staff_id) REFERENCES staff (shop_id, id)
);

-- ===================== Delivery area =====================

CREATE TABLE delivery_zones (
  id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id            BIGINT NOT NULL REFERENCES shops(id),
  name               JSONB NOT NULL,
  district           TEXT NOT NULL,
  area               TEXT,                  -- NULL = puro district
  charge             BIGINT NOT NULL CHECK (charge >= 0),
  min_order          BIGINT NOT NULL DEFAULT 0 CHECK (min_order >= 0),                            -- v1.2
  free_delivery_over BIGINT CHECK (free_delivery_over IS NULL OR free_delivery_over > 0),  -- v1.2; NULL = nai
  same_city          BOOLEAN NOT NULL,      -- deadline: true = 5 din, false = 10 din
  same_day_cutoff    TIME,                  -- shop er timezone e; NULL = same-day delivery nai
  own_rider          BOOLEAN NOT NULL DEFAULT false,
  is_active          BOOLEAN NOT NULL DEFAULT true,
  UNIQUE NULLS NOT DISTINCT (shop_id, district, area),
  UNIQUE (shop_id, id)
);

-- ===================== Catalog =====================

CREATE TABLE categories (                   -- shop er (marketplace e shob vendor ekoi category tree)
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id    BIGINT NOT NULL REFERENCES shops(id),
  parent_id  BIGINT,
  slug       TEXT NOT NULL,
  name       JSONB NOT NULL,
  image_key  TEXT,
  sort_order INT NOT NULL DEFAULT 0,
  is_active  BOOLEAN NOT NULL DEFAULT true,
  CONSTRAINT categories_not_own_parent_check CHECK (parent_id IS NULL OR parent_id <> id),   -- v1.2 (lomba cycle app e atkabe)
  UNIQUE (shop_id, slug),
  UNIQUE (shop_id, id),
  FOREIGN KEY (shop_id, parent_id) REFERENCES categories (shop_id, id)
);

CREATE TABLE products (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id         BIGINT NOT NULL REFERENCES shops(id),
  vendor_id       BIGINT NOT NULL,                 -- single store e house vendor
  category_id     BIGINT,
  slug            TEXT NOT NULL,
  name            JSONB NOT NULL,
  description     JSONB NOT NULL DEFAULT '{}',
  unit            TEXT NOT NULL DEFAULT 'piece'
                  CHECK (unit IN ('piece','kg','gram','litre','ml','dozen','pack')),
  variable_weight BOOLEAN NOT NULL DEFAULT false,
  attributes      JSONB NOT NULL DEFAULT '{}',   -- origin, bfsa_number, certification, warranty_months ...
  search_keywords TEXT NOT NULL DEFAULT '',      -- "lal chal red rice লাল চাল"
  seo             JSONB NOT NULL DEFAULT '{}',   -- {"bn":{"title":"","description":""},"en":{...}}
  rating_sum      INT NOT NULL DEFAULT 0 CHECK (rating_sum >= 0),
  review_count    INT NOT NULL DEFAULT 0 CHECK (review_count >= 0),
  is_active       BOOLEAN NOT NULL DEFAULT true,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- v1.3: NFC, tai "ো" ek code point ba "ে"+"া" duirokom lekha-i mile (query o normalize kore, 6.4)
  search_text     TEXT GENERATED ALWAYS AS (normalize(
                    coalesce(name->>'bn','') || ' ' || coalesce(name->>'en','') || ' ' || search_keywords, NFC)
                  ) STORED,
  CONSTRAINT products_variable_weight_kg_check CHECK (NOT variable_weight OR unit = 'kg'),    -- v1.2: price = 1 kg er dam (8.5)
  UNIQUE (shop_id, slug),
  UNIQUE (shop_id, id),
  UNIQUE (shop_id, id, vendor_id),               -- variant er vendor product er sathe milbe
  FOREIGN KEY (shop_id, vendor_id)   REFERENCES vendors (shop_id, id),
  FOREIGN KEY (shop_id, category_id) REFERENCES categories (shop_id, id)
);
CREATE INDEX products_search_trgm ON products USING gin (search_text gin_trgm_ops);
CREATE INDEX products_category_idx ON products (shop_id, category_id) WHERE is_active;
CREATE INDEX products_vendor_idx ON products (shop_id, vendor_id) WHERE is_active;

CREATE TABLE product_variants (
  id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id          BIGINT NOT NULL REFERENCES shops(id),
  product_id       BIGINT NOT NULL,
  vendor_id        BIGINT NOT NULL,                     -- product er vendor (FK diye mile, product bodlale cascade)
  sku              TEXT NOT NULL,
  options          JSONB NOT NULL DEFAULT '{}',         -- {"weight":{"bn":"৫ কেজি","en":"5 kg"}}
  price            BIGINT NOT NULL CHECK (price >= 0),  -- variable_weight hole 1 kg er dam
  compare_at_price BIGINT CONSTRAINT product_variants_compare_at_check
                   CHECK (compare_at_price IS NULL OR compare_at_price > price),  -- v1.2
  cost_price       BIGINT CHECK (cost_price >= 0),
  nominal_weight_g INT CHECK (nominal_weight_g > 0),    -- variable_weight hole ek unit er ojon
  stock            INT NOT NULL DEFAULT 0 CHECK (stock >= 0),
  low_stock_at     INT NOT NULL DEFAULT 5 CHECK (low_stock_at >= 0),
  is_active        BOOLEAN NOT NULL DEFAULT true,
  sort_order       INT NOT NULL DEFAULT 0,
  UNIQUE (shop_id, vendor_id, sku),
  UNIQUE (shop_id, id),
  FOREIGN KEY (shop_id, product_id, vendor_id) REFERENCES products (shop_id, id, vendor_id) ON UPDATE CASCADE
);
CREATE INDEX variants_product_idx ON product_variants (product_id);

CREATE TABLE product_images (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id     BIGINT NOT NULL REFERENCES shops(id),
  product_id  BIGINT NOT NULL,
  storage_key TEXT NOT NULL,                -- "shops/{shop_id}/..." prefix
  alt         JSONB NOT NULL DEFAULT '{}',
  width       INT NOT NULL CHECK (width > 0),
  height      INT NOT NULL CHECK (height > 0),
  sort_order  INT NOT NULL DEFAULT 0,
  FOREIGN KEY (shop_id, product_id) REFERENCES products (shop_id, id)
);
CREATE INDEX product_images_product_idx ON product_images (product_id, sort_order);

-- ===================== Inventory =====================

CREATE TABLE suppliers (                    -- Phase 2 te puro use, table ekhon-i
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id    BIGINT NOT NULL REFERENCES shops(id),
  vendor_id  BIGINT NOT NULL,               -- kon vendor er supplier/krishok
  name       TEXT NOT NULL,
  phone      phone_bd,
  address    TEXT,
  note       TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (shop_id, id),
  FOREIGN KEY (shop_id, vendor_id) REFERENCES vendors (shop_id, id)
);

CREATE TABLE inventory_batches (            -- vendor ashe variant theke
  id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id            BIGINT NOT NULL REFERENCES shops(id),
  variant_id         BIGINT NOT NULL,
  supplier_id        BIGINT,
  batch_code         TEXT,
  quantity_received  INT NOT NULL CHECK (quantity_received > 0),
  quantity_remaining INT NOT NULL CHECK (quantity_remaining >= 0),
  cost_price         BIGINT CHECK (cost_price >= 0),
  harvest_date       DATE,
  expiry_date        DATE,
  received_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (quantity_remaining <= quantity_received),
  CHECK (expiry_date IS NULL OR harvest_date IS NULL OR expiry_date >= harvest_date),  -- v1.2
  UNIQUE (shop_id, id),
  UNIQUE (shop_id, id, variant_id),         -- v1.2: order_item_batches ar movement er variant mile
  FOREIGN KEY (shop_id, variant_id)  REFERENCES product_variants (shop_id, id),
  FOREIGN KEY (shop_id, supplier_id) REFERENCES suppliers (shop_id, id)
);
CREATE INDEX batches_fifo_idx ON inventory_batches (variant_id, expiry_date NULLS LAST, received_at)
  WHERE quantity_remaining > 0;

CREATE TABLE inventory_movements (
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id    BIGINT NOT NULL REFERENCES shops(id),
  variant_id BIGINT NOT NULL,
  batch_id   BIGINT,
  change     INT NOT NULL CHECK (change <> 0),   -- + jog, - komano
  -- v1.2: 'expired' = protidin er job expire hoya batch er qty stock theke komay (8.12)
  reason     TEXT NOT NULL CHECK (reason IN ('purchase','order','cancel','return','adjust','wastage','expired')),
  order_id   BIGINT,
  staff_id   BIGINT,
  note       TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (shop_id, variant_id) REFERENCES product_variants (shop_id, id),
  FOREIGN KEY (shop_id, batch_id, variant_id) REFERENCES inventory_batches (shop_id, id, variant_id),  -- v1.2
  FOREIGN KEY (shop_id, staff_id)   REFERENCES staff (shop_id, id)
);
CREATE INDEX movements_variant_idx ON inventory_movements (variant_id, created_at DESC);

-- ===================== Cart, wishlist =====================

CREATE TABLE wishlists (
  shop_id    BIGINT NOT NULL REFERENCES shops(id),
  user_id    BIGINT NOT NULL,
  product_id BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (shop_id, user_id, product_id),
  FOREIGN KEY (shop_id, user_id)    REFERENCES users (shop_id, id),
  FOREIGN KEY (shop_id, product_id) REFERENCES products (shop_id, id)
);

CREATE TABLE carts (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id     BIGINT NOT NULL REFERENCES shops(id),
  user_id     BIGINT,
  session_key BYTEA,                         -- guest cart cookie er sha256
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (user_id IS NOT NULL OR session_key IS NOT NULL),
  UNIQUE (shop_id, id),
  FOREIGN KEY (shop_id, user_id) REFERENCES users (shop_id, id)
);
CREATE UNIQUE INDEX carts_user_uniq ON carts (shop_id, user_id) WHERE user_id IS NOT NULL;
CREATE UNIQUE INDEX carts_session_uniq ON carts (shop_id, session_key) WHERE session_key IS NOT NULL;
CREATE INDEX carts_updated_idx ON carts (updated_at) WHERE user_id IS NULL;  -- v1.2: cart.cleanup job

CREATE TABLE cart_items (
  shop_id    BIGINT NOT NULL REFERENCES shops(id),
  cart_id    BIGINT NOT NULL,
  variant_id BIGINT NOT NULL,
  quantity   INT NOT NULL CHECK (quantity > 0),
  added_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (cart_id, variant_id),
  FOREIGN KEY (shop_id, cart_id)    REFERENCES carts (shop_id, id) ON DELETE CASCADE,
  FOREIGN KEY (shop_id, variant_id) REFERENCES product_variants (shop_id, id)
);

-- ===================== Coupon =====================

CREATE TABLE coupons (
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id         BIGINT NOT NULL REFERENCES shops(id),
  vendor_id       BIGINT,                             -- NULL = shob vendor er line e; thakle shudhu oi vendor er
  funded_by       TEXT NOT NULL DEFAULT 'shop' CHECK (funded_by IN ('shop','vendor')),
  code            TEXT NOT NULL,
  kind            TEXT NOT NULL CHECK (kind IN ('percent','fixed')),
  value           BIGINT NOT NULL CHECK (value > 0),  -- percent: 1-100, fixed: poisha
  min_order       BIGINT NOT NULL DEFAULT 0 CHECK (min_order >= 0),                     -- v1.2
  max_discount    BIGINT CHECK (max_discount IS NULL OR max_discount > 0),             -- v1.2
  usage_limit     INT CHECK (usage_limit IS NULL OR usage_limit > 0),                  -- v1.2; NULL = unlimited
  used_count      INT NOT NULL DEFAULT 0 CHECK (used_count >= 0),
  per_phone_limit INT NOT NULL DEFAULT 1 CHECK (per_phone_limit >= 1),                 -- v1.2
  starts_at       TIMESTAMPTZ,
  expires_at      TIMESTAMPTZ,
  is_active       BOOLEAN NOT NULL DEFAULT true,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT coupons_percent_value_check CHECK (kind <> 'percent' OR value <= 100),
  CONSTRAINT coupons_vendor_funded_check CHECK (funded_by = 'shop' OR vendor_id IS NOT NULL),
  CONSTRAINT coupons_dates_check CHECK (starts_at IS NULL OR expires_at IS NULL OR starts_at < expires_at),  -- v1.2
  CONSTRAINT coupons_used_le_limit_check CHECK (usage_limit IS NULL OR used_count <= usage_limit),         -- v1.2
  UNIQUE (shop_id, id),
  FOREIGN KEY (shop_id, vendor_id) REFERENCES vendors (shop_id, id)
);
CREATE UNIQUE INDEX coupons_code_uniq ON coupons (shop_id, upper(code));

-- ===================== Order, fulfillment =====================

CREATE TABLE orders (                       -- customer er kena ar payment er unit
  id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id           BIGINT NOT NULL REFERENCES shops(id),
  order_number      TEXT NOT NULL,                 -- order_prefix + "-" + number: "SB-100245"
  public_token      TEXT NOT NULL UNIQUE,          -- crypto/rand.Text(), link er jonno
  confirm_token_hash BYTEA,                        -- v1.4: sha256(confirm_token), shudhu COD confirmation e; link shudhu SMS e (8.10)
  idempotency_key   TEXT NOT NULL,
  idempotency_hash  BYTEA NOT NULL,                -- v1.2: request body er sha256; ekoi key e alada body → 422
  user_id           BIGINT,                        -- NULL = guest
  customer_name     TEXT NOT NULL,
  customer_phone    phone_bd NOT NULL,
  customer_email    TEXT,
  recipient_name    TEXT NOT NULL,
  recipient_phone   phone_bd NOT NULL,
  ship_district     TEXT NOT NULL,
  ship_area         TEXT NOT NULL,
  ship_address      TEXT NOT NULL,
  delivery_zone_id  BIGINT,
  delivery_slot     TSTZRANGE,                     -- Phase 2
  note              TEXT,
  locale            TEXT NOT NULL DEFAULT 'bn' CHECK (locale IN ('bn','en')),
  subtotal          BIGINT NOT NULL CHECK (subtotal >= 0),
  discount          BIGINT NOT NULL DEFAULT 0 CHECK (discount >= 0),
  delivery_charge   BIGINT NOT NULL DEFAULT 0 CHECK (delivery_charge >= 0),
  total             BIGINT NOT NULL CHECK (total >= 0),
  coupon_id         BIGINT,
  -- v1.4: coupon er shorto snapshot (Rule 8). Pore coupon edit ba bondho holeo Reprice egulo-i nay (8.5)
  coupon_code         TEXT,
  coupon_kind         TEXT CHECK (coupon_kind IN ('percent','fixed')),
  coupon_value        BIGINT CHECK (coupon_value > 0),
  coupon_max_discount BIGINT CHECK (coupon_max_discount IS NULL OR coupon_max_discount > 0),
  coupon_vendor_id    BIGINT,
  coupon_funded_by    TEXT CHECK (coupon_funded_by IN ('shop','vendor')),
  payment_method    TEXT NOT NULL CHECK (payment_method IN ('cod','sslcommerz','bkash','nagad')),
  payment_status    TEXT NOT NULL DEFAULT 'unpaid'
                    CHECK (payment_status IN ('unpaid','pending','paid','partially_refunded','refunded','failed')),
  status            TEXT NOT NULL DEFAULT 'pending'           -- order er nijer status (8.8)
                    CHECK (status IN ('pending','confirmed','cancelled')),
  display_status    TEXT NOT NULL DEFAULT 'pending'           -- order.Summary(status, fulfillment gulo)
                    CHECK (display_status IN ('pending','confirmed','packed','shipped','delivered',
                                              'returned','cancelled','partial')),
  risk_flags        JSONB NOT NULL DEFAULT '[]',
  otp_verified      BOOLEAN NOT NULL DEFAULT false,
  ip                TEXT,
  delivery_deadline TIMESTAMPTZ,
  placed_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  confirmed_at      TIMESTAMPTZ,
  cancelled_at      TIMESTAMPTZ,
  cancel_reason     TEXT,
  CONSTRAINT orders_total_sum_check CHECK (total = subtotal - discount + delivery_charge),
  CONSTRAINT orders_discount_le_subtotal_check CHECK (discount <= subtotal),
  CONSTRAINT orders_pending_display_check CHECK ((status = 'pending') = (display_status = 'pending')),
  CONSTRAINT orders_cancelled_display_check CHECK ((status = 'cancelled') = (display_status = 'cancelled')),
  -- v1.2: online payment er order taka chhara confirmed hote pare na. Ek statement e
  -- payment_status age (ba ekshathe) bodlao, tarpor status.
  CONSTRAINT orders_online_paid_check CHECK (payment_method = 'cod' OR status <> 'confirmed'
         OR payment_status IN ('paid','partially_refunded','refunded')),
  -- v1.4: coupon thakle tar shorto-o thakbe, na thakle kichu na (v1.5: max_discount ar vendor o)
  CONSTRAINT orders_coupon_snapshot_check CHECK ((coupon_id IS NULL) = (coupon_code IS NULL)
    AND (coupon_id IS NULL) = (coupon_kind IS NULL) AND (coupon_id IS NULL) = (coupon_value IS NULL)
    AND (coupon_id IS NULL) = (coupon_funded_by IS NULL)
    AND (coupon_id IS NOT NULL OR (coupon_max_discount IS NULL AND coupon_vendor_id IS NULL))),
  CONSTRAINT orders_discount_coupon_check CHECK (discount = 0 OR coupon_id IS NOT NULL),  -- v1.5: discount shudhu coupon theke (8.5)
  CONSTRAINT orders_coupon_percent_check CHECK (coupon_kind IS DISTINCT FROM 'percent' OR coupon_value <= 100),
  CONSTRAINT orders_coupon_vendor_check CHECK (coupon_funded_by IS DISTINCT FROM 'vendor' OR coupon_vendor_id IS NOT NULL),
  CONSTRAINT orders_confirm_token_cod_check CHECK (confirm_token_hash IS NULL OR payment_method = 'cod'),  -- v1.4 (8.10)
  UNIQUE (shop_id, order_number),
  UNIQUE (shop_id, idempotency_key),
  UNIQUE (shop_id, id),
  UNIQUE (shop_id, id, coupon_id),          -- v1.2: coupon_redemptions er coupon = order er coupon
  FOREIGN KEY (shop_id, user_id)          REFERENCES users (shop_id, id),
  FOREIGN KEY (shop_id, delivery_zone_id) REFERENCES delivery_zones (shop_id, id),
  FOREIGN KEY (shop_id, coupon_id)        REFERENCES coupons (shop_id, id),
  FOREIGN KEY (shop_id, coupon_vendor_id) REFERENCES vendors (shop_id, id)   -- v1.4
);
CREATE UNIQUE INDEX orders_confirm_token_uniq ON orders (confirm_token_hash) WHERE confirm_token_hash IS NOT NULL;  -- v1.4 (8.10)
CREATE INDEX orders_status_idx ON orders (shop_id, display_status, placed_at DESC);
CREATE INDEX orders_placed_idx ON orders (shop_id, placed_at DESC);   -- v1.2: dashboard "ajker"
CREATE INDEX orders_phone_idx ON orders (shop_id, customer_phone, placed_at DESC);
CREATE INDEX orders_user_idx ON orders (user_id, placed_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX orders_deadline_idx ON orders (delivery_deadline)
  WHERE display_status IN ('confirmed','packed','shipped','partial');

CREATE TABLE order_fulfillments (           -- ek order e ek vendor er ongsho: pack, ship, deliver
  id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id         BIGINT NOT NULL REFERENCES shops(id),
  order_id        BIGINT NOT NULL,
  vendor_id       BIGINT NOT NULL,
  seq             SMALLINT NOT NULL CHECK (seq > 0),       -- "SB-100245-1"
  status          TEXT NOT NULL DEFAULT 'open'
                  CHECK (status IN ('open','packed','shipped','delivered','cancelled','returned')),
  subtotal        BIGINT NOT NULL CHECK (subtotal >= 0),   -- pricing er vendor bhag (8.5)
  discount        BIGINT NOT NULL DEFAULT 0 CHECK (discount >= 0),
  delivery_charge BIGINT NOT NULL DEFAULT 0 CHECK (delivery_charge >= 0),
  total           BIGINT NOT NULL CHECK (total >= 0),      -- COD hole courier eta tole
  packed_at       TIMESTAMPTZ,
  shipped_at      TIMESTAMPTZ,
  delivered_at    TIMESTAMPTZ,
  cancelled_at    TIMESTAMPTZ,
  returned_at     TIMESTAMPTZ,
  CONSTRAINT order_fulfillments_total_sum_check CHECK (total = subtotal - discount + delivery_charge),
  CONSTRAINT order_fulfillments_discount_le_subtotal_check CHECK (discount <= subtotal),
  UNIQUE (order_id, vendor_id),
  UNIQUE (order_id, seq),
  UNIQUE (shop_id, id),
  UNIQUE (shop_id, order_id, id),           -- v1.2: history, refund er fulfillment oi order er-i
  UNIQUE (shop_id, order_id, id, vendor_id),
  FOREIGN KEY (shop_id, order_id)  REFERENCES orders (shop_id, id),
  FOREIGN KEY (shop_id, vendor_id) REFERENCES vendors (shop_id, id)
);
CREATE INDEX fulfillments_vendor_idx ON order_fulfillments (shop_id, vendor_id, status);

CREATE TABLE order_items (
  id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id          BIGINT NOT NULL REFERENCES shops(id),
  order_id         BIGINT NOT NULL,
  fulfillment_id   BIGINT NOT NULL,
  vendor_id        BIGINT NOT NULL,         -- snapshot: order er shomoy product er vendor
  variant_id       BIGINT NOT NULL,
  product_name     JSONB NOT NULL,          -- snapshot
  variant_label    JSONB NOT NULL DEFAULT '{}',
  sku              TEXT NOT NULL,
  unit_price       BIGINT NOT NULL CHECK (unit_price >= 0),  -- snapshot; variable_weight hole 1 kg er dam
  cost_price       BIGINT,                  -- snapshot
  quantity         INT NOT NULL CHECK (quantity > 0),     -- removed hole-o ager quantity (hishab e 0, 8.5 LineFromItem)
  variable_weight  BOOLEAN NOT NULL DEFAULT false,        -- v1.5: snapshot; product pore bodlaleo order er hishab ek
  ordered_weight_g INT CHECK (ordered_weight_g IS NULL OR ordered_weight_g > 0),  -- puro line er ojon
  packed_weight_g  INT CHECK (packed_weight_g IS NULL OR packed_weight_g > 0),    -- packing e asol mot ojon
  line_total       BIGINT NOT NULL CHECK (line_total >= 0),
  discount_share   BIGINT NOT NULL DEFAULT 0 CHECK (discount_share >= 0),  -- order discount er ei line er bhag
  commission       BIGINT NOT NULL DEFAULT 0 CHECK (commission >= 0),      -- marketplace; single store e 0
  commission_bps   INT NOT NULL DEFAULT 0 CHECK (commission_bps BETWEEN 0 AND 10000),  -- v1.4: order er shomoyer vendor rate (snapshot, 8.5)
  status           TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','removed')),
  CHECK (discount_share <= line_total),
  -- v1.5: ojon shudhu ojon-variable line e (fixed line e WeightG 0, 8.5)
  CONSTRAINT order_items_weight_check CHECK (CASE WHEN variable_weight THEN ordered_weight_g IS NOT NULL
                                             ELSE ordered_weight_g IS NULL AND packed_weight_g IS NULL END),
  -- v1.5: removed item er taka 0 (Reprice er Qty 0 line er moto)
  CONSTRAINT order_items_removed_zero_check CHECK (status = 'active'
    OR (line_total = 0 AND discount_share = 0 AND commission = 0)),
  UNIQUE (shop_id, id),
  UNIQUE (shop_id, id, variant_id),         -- v1.2: order_item_batches er variant mile
  FOREIGN KEY (shop_id, order_id, fulfillment_id, vendor_id)
    REFERENCES order_fulfillments (shop_id, order_id, id, vendor_id),
  FOREIGN KEY (shop_id, variant_id) REFERENCES product_variants (shop_id, id)
);
CREATE INDEX order_items_order_idx ON order_items (order_id);
CREATE INDEX order_items_fulfillment_idx ON order_items (fulfillment_id);

CREATE TABLE order_item_batches (           -- kon batch kon customer er kache gelo (recall er jonno)
  shop_id       BIGINT NOT NULL REFERENCES shops(id),
  order_item_id BIGINT NOT NULL,
  batch_id      BIGINT NOT NULL,
  variant_id    BIGINT NOT NULL,            -- v1.2: item ar batch ekoi variant er (bhul recall atkay)
  quantity      INT NOT NULL CHECK (quantity > 0),
  PRIMARY KEY (order_item_id, batch_id),
  FOREIGN KEY (shop_id, order_item_id, variant_id) REFERENCES order_items (shop_id, id, variant_id),
  FOREIGN KEY (shop_id, batch_id, variant_id)      REFERENCES inventory_batches (shop_id, id, variant_id)
);
CREATE INDEX order_item_batches_batch_idx ON order_item_batches (batch_id);

CREATE TABLE order_status_history (
  id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id        BIGINT NOT NULL REFERENCES shops(id),
  order_id       BIGINT NOT NULL,
  fulfillment_id BIGINT,                    -- NULL = order er status; thakle oi fulfillment er
  from_status    TEXT,
  to_status      TEXT NOT NULL,
  actor_type     TEXT NOT NULL CHECK (actor_type IN ('system','staff','vendor','customer')),
  actor_id       BIGINT,
  note           TEXT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (shop_id, order_id)                 REFERENCES orders (shop_id, id),
  FOREIGN KEY (shop_id, order_id, fulfillment_id) REFERENCES order_fulfillments (shop_id, order_id, id)  -- v1.2
);
CREATE INDEX order_history_order_idx ON order_status_history (order_id, created_at);

CREATE TABLE order_notes (                  -- v1.2: staff er note, phone call (7.7 POST /orders/{id}/notes)
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id    BIGINT NOT NULL REFERENCES shops(id),
  order_id   BIGINT NOT NULL,
  staff_id   BIGINT NOT NULL,
  kind       TEXT NOT NULL DEFAULT 'note' CHECK (kind IN ('note','call')),
  note       TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (shop_id, order_id) REFERENCES orders (shop_id, id),
  FOREIGN KEY (shop_id, staff_id) REFERENCES staff (shop_id, id)
);
CREATE INDEX order_notes_order_idx ON order_notes (order_id, created_at);

CREATE TABLE coupon_redemptions (
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id    BIGINT NOT NULL REFERENCES shops(id),
  coupon_id  BIGINT NOT NULL,
  order_id   BIGINT NOT NULL UNIQUE,
  phone      phone_bd NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (shop_id, coupon_id)           REFERENCES coupons (shop_id, id),
  FOREIGN KEY (shop_id, order_id, coupon_id) REFERENCES orders (shop_id, id, coupon_id)  -- v1.2
);
CREATE INDEX coupon_redemptions_phone_idx ON coupon_redemptions (coupon_id, phone);

-- ===================== Payment, refund =====================

CREATE TABLE payments (                     -- order level; marketplace e-o ekta payment (shop er gateway)
  id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id        BIGINT NOT NULL REFERENCES shops(id),
  order_id       BIGINT NOT NULL,
  provider       TEXT NOT NULL CHECK (provider IN ('sslcommerz','bkash','nagad')),
  tran_id        TEXT NOT NULL UNIQUE,      -- random (rand.Text()); IPN e eta diye shop chena hoy
  amount         BIGINT NOT NULL CHECK (amount > 0),
  status         TEXT NOT NULL DEFAULT 'initiated'
                 CHECK (status IN ('initiated','succeeded','failed','cancelled')),
  val_id         TEXT,                      -- v1.2: provider_ref bhenge duita column
  bank_tran_id   TEXT,                      --       (refund API te bank_tran_id lage)
  risk_level     SMALLINT,                  -- v1.2: SSLCommerz validation er risk_level (1 = review)
  store_amount   BIGINT CHECK (store_amount >= 0),  -- v1.3: gateway fee kete pawa taka (settlement mela, 8.9)
  raw            JSONB NOT NULL DEFAULT '{}',
  settled_at     DATE,                      -- bank e taka ashar din (settlement mela)
  settlement_ref TEXT,
  reconcile_after TIMESTAMPTZ,              -- v1.4: porer Transaction Query kokhon (8.9); NULL = ar lagbe na
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT payments_reconcile_done_check CHECK (status <> 'succeeded' OR reconcile_after IS NULL),  -- v1.4
  UNIQUE (shop_id, id),
  UNIQUE (shop_id, order_id, id),           -- v1.2: refund er payment oi order er-i
  FOREIGN KEY (shop_id, order_id) REFERENCES orders (shop_id, id)
);
CREATE INDEX payments_order_idx ON payments (order_id);
CREATE UNIQUE INDEX payments_val_uniq ON payments (provider, val_id) WHERE val_id IS NOT NULL;  -- v1.2
CREATE INDEX payments_initiated_idx ON payments (created_at) WHERE status = 'initiated';      -- v1.2: monitoring (13.8)
CREATE INDEX payments_reconcile_idx ON payments (reconcile_after) WHERE reconcile_after IS NOT NULL;  -- v1.4: reconcile job (8.9)

CREATE TABLE refunds (
  id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id        BIGINT NOT NULL REFERENCES shops(id),
  order_id       BIGINT NOT NULL,
  fulfillment_id BIGINT,                    -- kon vendor er ongsho (NULL = puro order)
  payment_id     BIGINT,
  amount         BIGINT NOT NULL CHECK (amount > 0),
  -- v1.2: je madhyome payment, shei madhyome (DCOG 2021). Mot refund ≤ paid: app + test (cross-row).
  method         TEXT NOT NULL CHECK (method IN ('sslcommerz','bkash','nagad','cash','bank_transfer')),
  -- v1.3: kon karone refund (8.8, 8.9); 'cancel' ek fulfillment e ekbar-i (refunds_cancel_once)
  kind           TEXT NOT NULL DEFAULT 'manual'
                 CHECK (kind IN ('cancel','reprice','return','duplicate','late_payment','manual'))
                 CONSTRAINT refunds_cancel_fulfillment_check CHECK (kind <> 'cancel' OR fulfillment_id IS NOT NULL),
  status         TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','done','failed')),
  reason         TEXT NOT NULL,
  provider_ref   TEXT,                      -- v1.2: gateway er refund_ref_id
  staff_id       BIGINT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  done_at        TIMESTAMPTZ,
  FOREIGN KEY (shop_id, order_id)                 REFERENCES orders (shop_id, id),
  FOREIGN KEY (shop_id, order_id, fulfillment_id) REFERENCES order_fulfillments (shop_id, order_id, id),  -- v1.2
  FOREIGN KEY (shop_id, order_id, payment_id)     REFERENCES payments (shop_id, order_id, id),            -- v1.2
  FOREIGN KEY (shop_id, staff_id)                 REFERENCES staff (shop_id, id)
);
CREATE INDEX refunds_order_idx ON refunds (order_id);
CREATE UNIQUE INDEX refunds_cancel_once ON refunds (fulfillment_id) WHERE kind = 'cancel' AND status <> 'failed';  -- v1.3 (8.8)

-- ===================== Delivery, COD hishab =====================

CREATE TABLE shipments (                    -- fulfillment er parcel
  id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id        BIGINT NOT NULL REFERENCES shops(id),
  order_id       BIGINT NOT NULL,
  fulfillment_id BIGINT NOT NULL,
  vendor_id      BIGINT NOT NULL,
  carrier        TEXT NOT NULL CHECK (carrier IN ('own_rider','steadfast','pathao','redx','other')),
  rider_staff_id BIGINT,
  tracking_code  TEXT,
  consignment_id TEXT,
  status         TEXT NOT NULL DEFAULT 'booked'
                 CHECK (status IN ('booked','picked','in_transit','delivered','returned','cancelled')),
  cod_amount     BIGINT NOT NULL DEFAULT 0 CHECK (cod_amount >= 0),  -- COD hole fulfillment er total
  carrier_fee    BIGINT CHECK (carrier_fee >= 0),
  raw            JSONB NOT NULL DEFAULT '{}',
  booked_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  delivered_at   TIMESTAMPTZ,
  UNIQUE (shop_id, id),
  FOREIGN KEY (shop_id, order_id, fulfillment_id, vendor_id)
    REFERENCES order_fulfillments (shop_id, order_id, id, vendor_id),
  FOREIGN KEY (shop_id, rider_staff_id) REFERENCES staff (shop_id, id)
);
CREATE INDEX shipments_fulfillment_idx ON shipments (fulfillment_id);
-- v1.2: ek fulfillment e ek shomoy ekta-i active parcel (COD duibar tola atkay); return er por abar pathano jay
CREATE UNIQUE INDEX shipments_one_active ON shipments (fulfillment_id) WHERE status NOT IN ('cancelled','returned');
CREATE UNIQUE INDEX shipments_tracking_uniq ON shipments (shop_id, carrier, tracking_code) WHERE tracking_code IS NOT NULL;

CREATE TABLE cod_settlements (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id     BIGINT NOT NULL REFERENCES shops(id),
  carrier     TEXT NOT NULL CHECK (carrier IN ('own_rider','steadfast','pathao','redx','other')),  -- v1.2
  reference   TEXT,
  amount      BIGINT NOT NULL CHECK (amount >= 0),  -- hate pawa taka
  fee         BIGINT NOT NULL DEFAULT 0 CHECK (fee >= 0),
  received_at DATE NOT NULL,
  note        TEXT,
  created_by  BIGINT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (shop_id, id),
  FOREIGN KEY (shop_id, created_by) REFERENCES staff (shop_id, id)
);

CREATE TABLE cod_settlement_items (
  shop_id       BIGINT NOT NULL REFERENCES shops(id),
  settlement_id BIGINT NOT NULL,
  shipment_id   BIGINT NOT NULL UNIQUE,
  collected     BIGINT NOT NULL CHECK (collected >= 0),
  fee           BIGINT NOT NULL DEFAULT 0 CHECK (fee >= 0),
  PRIMARY KEY (settlement_id, shipment_id),
  FOREIGN KEY (shop_id, settlement_id) REFERENCES cod_settlements (shop_id, id),
  FOREIGN KEY (shop_id, shipment_id)   REFERENCES shipments (shop_id, id)
);

-- ===================== Review, support, privacy =====================

CREATE TABLE reviews (
  id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id       BIGINT NOT NULL REFERENCES shops(id),
  product_id    BIGINT NOT NULL,
  vendor_id     BIGINT NOT NULL,            -- order item theke (pore vendor rating)
  order_id      BIGINT NOT NULL,
  user_id       BIGINT,
  reviewer_name TEXT NOT NULL,              -- account delete hole 'Deleted user' (privacy flow)
  rating        SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
  comment       TEXT,
  is_hidden     BOOLEAN NOT NULL DEFAULT false,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (order_id, product_id),
  FOREIGN KEY (shop_id, product_id) REFERENCES products (shop_id, id),
  FOREIGN KEY (shop_id, vendor_id)  REFERENCES vendors (shop_id, id),
  FOREIGN KEY (shop_id, order_id)   REFERENCES orders (shop_id, id),
  FOREIGN KEY (shop_id, user_id)    REFERENCES users (shop_id, id)
);
CREATE INDEX reviews_product_idx ON reviews (product_id, created_at DESC) WHERE NOT is_hidden;

CREATE TABLE complaints (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id     BIGINT NOT NULL REFERENCES shops(id),
  order_id    BIGINT,
  vendor_id   BIGINT,                       -- marketplace e kon vendor niye
  name        TEXT NOT NULL,
  phone       phone_bd NOT NULL,
  message     TEXT NOT NULL,
  status      TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open','in_progress','resolved')),
  resolution  TEXT,
  due_at      TIMESTAMPTZ NOT NULL DEFAULT now() + interval '72 hours',  -- v1.3: DCOG 2021, 72 ghontay shomadhan (12.1)
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  resolved_at TIMESTAMPTZ,
  FOREIGN KEY (shop_id, order_id)  REFERENCES orders (shop_id, id),
  FOREIGN KEY (shop_id, vendor_id) REFERENCES vendors (shop_id, id)
);
CREATE INDEX complaints_status_idx ON complaints (shop_id, status, created_at DESC);
CREATE INDEX complaints_due_idx ON complaints (due_at) WHERE status <> 'resolved';  -- v1.3: alert.complaint_due (8.17)

CREATE TABLE privacy_requests (
  id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id    BIGINT NOT NULL REFERENCES shops(id),
  user_id    BIGINT,
  phone      phone_bd NOT NULL,
  kind       TEXT NOT NULL CHECK (kind IN ('export','delete')),
  status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','done','rejected')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  done_at    TIMESTAMPTZ,
  FOREIGN KEY (shop_id, user_id) REFERENCES users (shop_id, id)
);

-- ===================== System =====================

CREATE TABLE jobs (
  id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id      BIGINT REFERENCES shops(id),  -- NULL = platform er kaj
  kind         TEXT NOT NULL,               -- 'sms.send', 'alert.low_stock' ...
  payload      JSONB NOT NULL DEFAULT '{}',
  dedupe_key   TEXT UNIQUE,                 -- shop shoho: 'alert.inventory:1:2026-10-28'
  run_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  attempts     INT NOT NULL DEFAULT 0,
  max_attempts INT NOT NULL DEFAULT 5 CHECK (max_attempts > 0),
  last_error   TEXT,
  locked_at    TIMESTAMPTZ,
  done_at      TIMESTAMPTZ,
  failed_at    TIMESTAMPTZ,                 -- v1.2: max_attempts er por thame, admin alert
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT jobs_attempts_le_max_check CHECK (attempts BETWEEN 0 AND max_attempts),   -- v1.2
  CONSTRAINT jobs_done_or_failed_check CHECK (done_at IS NULL OR failed_at IS NULL)     -- v1.2
);
CREATE INDEX jobs_due_idx ON jobs (run_at) WHERE done_at IS NULL AND failed_at IS NULL;

CREATE TABLE audit_logs (
  id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  shop_id     BIGINT NOT NULL REFERENCES shops(id),
  staff_id    BIGINT,
  action      TEXT NOT NULL,                -- 'product.update', 'settings.publish' ...
  entity_type TEXT NOT NULL,
  entity_id   BIGINT,
  before      JSONB,
  after       JSONB,
  ip          TEXT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (shop_id, staff_id) REFERENCES staff (shop_id, id)
);
CREATE INDEX audit_shop_idx ON audit_logs (shop_id, created_at DESC);

-- Pore toiri hoyeche emon table er foreign key
ALTER TABLE shop_settings       ADD FOREIGN KEY (shop_id, updated_by) REFERENCES staff (shop_id, id);
ALTER TABLE settings_history    ADD FOREIGN KEY (shop_id, created_by) REFERENCES staff (shop_id, id);
ALTER TABLE inventory_movements ADD FOREIGN KEY (shop_id, order_id)   REFERENCES orders (shop_id, id);

-- v1.2: DB role (12.2 "app user DROP korte pare na", 6.1 "hard delete na").
-- Migration chalay table er malik (MIGRATE_DATABASE_URL); API chalay shop_app (DATABASE_URL), DDL chhara.
-- Role duita infra theke ekbar banao; local e shop_app na thakle ei block kichu kore na.
-- v1.3: staging/production e (runner er app.env; v1.6: set_config, 6.1) role na thakle fail kore, chupchap skip na.
-- Notun migration er notun table default privilege e DELETE pay: dorkar hole oi file e REVOKE (6.1).
-- v1.7: fail-closed. app.env na thakle ba khali holeo (jemon psql diye hate chalano) role chhara fail;
-- role chhara shudhu 'local' e cholbe. Ager version e app.env chhara production eo chupchap skip hoto.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'shop_app') THEN
    GRANT USAGE ON SCHEMA public TO shop_app;
    GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO shop_app;
    ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO shop_app;
    REVOKE DELETE ON orders, order_fulfillments, order_items, order_status_history, payments, refunds,
                     reviews, shipments, cod_settlements, cod_settlement_items, inventory_movements,
                     audit_logs, settings_history FROM shop_app;
    REVOKE UPDATE ON order_status_history, inventory_movements, audit_logs, settings_history FROM shop_app;
    GRANT UPDATE (ip) ON audit_logs TO shop_app;   -- v1.3: data.retention job (8.17) shudhu ip muche
    -- v1.8: runner er schema_migrations (6.1) app shudhu porte pare, migration er itihash bodlate pare na.
    IF to_regclass('public.schema_migrations') IS NOT NULL THEN
      REVOKE INSERT, UPDATE, DELETE, TRUNCATE ON schema_migrations FROM shop_app;
    END IF;
  ELSIF coalesce(current_setting('app.env', true), '') <> 'local' THEN
    RAISE EXCEPTION 'shop_app role nai (app.env = %): migration er age role banao (13.5); local e set_config(''app.env'', ''local'', true)',
      coalesce(nullif(current_setting('app.env', true), ''), '<nai>');
  END IF;
END $$;
