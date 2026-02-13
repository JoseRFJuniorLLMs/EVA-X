#!/bin/bash
# Fix DATABASE_URL to point to Cloud SQL (not localhost)
sed -i 's|DATABASE_URL=.*|DATABASE_URL=postgres://postgres:Debian23%40@35.232.177.102:5432/eva-db?sslmode=disable|' /opt/eva-mind/.env

# Verify
echo "=== DB Config ==="
grep DATABASE_URL /opt/eva-mind/.env

# Restart service
systemctl restart eva-mind
sleep 4

# Check
journalctl -u eva-mind --no-pager -n 5
echo ""
curl -s http://localhost:8091/api/health
echo ""
echo "=== DONE ==="
