#!/bin/bash
set -e

# Fix .env: DigitalOcean -> localhost for local Docker services
sed -i 's/104.248.219.200/localhost/g' /opt/eva-mind/.env

# Fix DATABASE_URL to use Cloud SQL
sed -i 's|DATABASE_URL=.*|DATABASE_URL=postgres://postgres:Debian23%40@35.232.177.102:5432/eva-db?sslmode=disable|' /opt/eva-mind/.env

# Fix NEO4J URI to use localhost (Docker on same VM)
sed -i 's|NEO4J_URI=.*|NEO4J_URI=bolt://localhost:7687|' /opt/eva-mind/.env

echo "=== .env updated ==="
grep -E '(DATABASE_URL|NEO4J|QDRANT|REDIS)' /opt/eva-mind/.env

# Create systemd service
cat > /etc/systemd/system/eva-mind.service << 'EOF'
[Unit]
Description=EVA Mind v2.0
After=network.target docker.service
Wants=docker.service

[Service]
Type=simple
WorkingDirectory=/opt/eva-mind
ExecStart=/opt/eva-mind/eva-mind
Restart=always
RestartSec=5
Environment=PATH=/usr/local/go/bin:/usr/bin:/bin
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
systemctl daemon-reload
systemctl enable eva-mind
systemctl start eva-mind

sleep 3

# Check status
systemctl status eva-mind --no-pager
echo ""
echo "=== Health Check ==="
curl -s http://localhost:8091/api/health || echo "WAITING..."
echo ""
echo "=== DEPLOY COMPLETE ==="
