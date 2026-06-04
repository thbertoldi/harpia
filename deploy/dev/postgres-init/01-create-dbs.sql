SELECT 'CREATE DATABASE zitadel'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'zitadel')\gexec

GRANT ALL PRIVILEGES ON DATABASE zitadel TO harpia;
