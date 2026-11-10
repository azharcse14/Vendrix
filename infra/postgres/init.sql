-- Local Postgres er role ar database (13.5). docker compose prothom bar chalu holei chole; abar chalaleo kichu
-- bhange na (IF NOT EXISTS). Password shudhu local er; staging/production e secret manager theke.
-- shop_owner: table er malik, migration chalay (MIGRATE_DATABASE_URL). shop_app: API (DATABASE_URL), DDL nai (6.1).
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'shop_owner') THEN
    CREATE ROLE shop_owner LOGIN PASSWORD 'shop_owner';
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'shop_app') THEN
    CREATE ROLE shop_app LOGIN PASSWORD 'shop_app';
  END IF;
END $$;
-- C.UTF-8: C/POSIX e Bangla trigram khali, search kaj kore na (2.1; 001 shuru tei check kore).
SELECT 'CREATE DATABASE shop OWNER shop_owner TEMPLATE template0 ENCODING ''UTF8'' LOCALE ''C.UTF-8'''
WHERE NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = 'shop') \gexec
