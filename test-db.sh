#!/bin/bash
# Test DB connections
echo "=== Testing Cloud SQL (35.232.177.102) ==="
PGPASSWORD='Debian23@' psql -h 35.232.177.102 -U postgres -d eva-db -c "SELECT 1 as test" 2>&1 || echo "FAILED with Debian23@"

echo ""
echo "=== Testing without @ ==="
PGPASSWORD='Debian23' psql -h 35.232.177.102 -U postgres -d eva-db -c "SELECT 1 as test" 2>&1 || echo "FAILED with Debian23"

echo ""
echo "=== Testing DATABASE_URL from .env ==="
grep DATABASE_URL /opt/eva-mind/.env
