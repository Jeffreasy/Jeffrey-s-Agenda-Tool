# Deployment Guide

This guide covers deploying the Agenda Automator backend to production environments.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Environment Setup](#environment-setup)
- [Database Setup](#database-setup)
- [Application Deployment](#application-deployment)
- [Container Deployment](#container-deployment)
- [Monitoring](#monitoring)
- [Security](#security)
- [Scaling](#scaling)
- [Backup and Recovery](#backup-and-recovery)

## Prerequisites

### Infrastructure Requirements

- **Server**: Linux-based (Ubuntu 20.04+, CentOS 7+)
- **CPU**: 2+ cores
- **RAM**: 4GB+ minimum, 8GB+ recommended
- **Storage**: 20GB+ SSD
- **Network**: Stable internet connection

### Software Requirements

- Docker and Docker Compose (for containerized deployment)
- PostgreSQL 13+ (managed or self-hosted)
- Reverse proxy (nginx, Caddy, or cloud load balancer)
- SSL certificate (Let's Encrypt or commercial)

### Cloud Providers

Supported deployment platforms:
- **AWS**: EC2, ECS, EKS
- **Google Cloud**: Compute Engine, Cloud Run, GKE
- **Azure**: VMs, AKS, App Service
- **DigitalOcean**: Droplets, App Platform
- **Heroku**: Buildpacks or containers

## Environment Setup

### Production Environment Variables

Create a production `.env` file with secure values:

```env
#---------------------------------------------------
# 1. APPLICATION CONFIGURATION
#---------------------------------------------------
APP_ENV=production
API_PORT=8080

#---------------------------------------------------
# 2. DATABASE (POSTGRES)
#---------------------------------------------------
POSTGRES_USER=agenda_prod_user
POSTGRES_PASSWORD=SECURE_RANDOM_PASSWORD_32_CHARS
POSTGRES_DB=agenda_automator_prod
POSTGRES_HOST=your-prod-db-host.com
POSTGRES_PORT=5432

DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}?sslmode=require"

#---------------------------------------------------
# 3. SECURITY & ENCRYPTION
#---------------------------------------------------
# Generate a new 32-character key for production
ENCRYPTION_KEY="YOUR_NEW_PRODUCTION_ENCRYPTION_KEY_32"

#---------------------------------------------------
# 5. OAUTH CLIENTS (Google)
#---------------------------------------------------
CLIENT_BASE_URL="https://yourdomain.com"
OAUTH_REDIRECT_URL="https://yourdomain.com/api/v1/auth/google/callback"

GOOGLE_OAUTH_CLIENT_ID="PROD-CLIENT-ID.apps.googleusercontent.com"
GOOGLE_OAUTH_CLIENT_SECRET="PROD-CLIENT-SECRET"
```

### Security Best Practices

- Use strong, unique passwords
- Store secrets in environment variables or secret managers
- Never commit `.env` files to version control
- Rotate encryption keys periodically
- Use different credentials for each environment

## Database Setup

### Managed PostgreSQL

For cloud-managed databases:

#### AWS RDS
```bash
# Example CloudFormation or Terraform configuration
# PostgreSQL 13+, 2 vCPUs, 8GB RAM minimum
```

#### Google Cloud SQL
```bash
# Create PostgreSQL instance
gcloud sql instances create agenda-prod \
  --database-version=POSTGRES_13 \
  --cpu=2 \
  --memory=8GB \
  --region=us-central1
```

#### Azure Database
```bash
# Create PostgreSQL server
az postgres server create \
  --resource-group myResourceGroup \
  --name agenda-prod \
  --location eastus \
  --admin-user agendaadmin \
  --admin-password SECURE_PASSWORD \
  --sku-name GP_Gen5_2 \
  --version 13
```

### Self-Hosted PostgreSQL

```bash
# Install PostgreSQL
sudo apt update
sudo apt install postgresql postgresql-contrib

# Create database and user
sudo -u postgres psql
CREATE DATABASE agenda_automator_prod;
CREATE USER agenda_prod_user WITH ENCRYPTED PASSWORD 'SECURE_PASSWORD';
GRANT ALL PRIVILEGES ON DATABASE agenda_automator_prod TO agenda_prod_user;
\q
```

### Run Migrations

```bash
# Install golang-migrate on production server
# Then run migrations
migrate -database "${DATABASE_URL}" -path db/migrations up
```

## Application Deployment

### Build the Application

```bash
# Build for production
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o agenda-automator cmd/server/main.go

# Or build with optimizations
go build -ldflags="-w -s" -o agenda-automator cmd/server/main.go
```

### Systemd Service

Create `/etc/systemd/system/agenda-automator.service`:

```ini
[Unit]
Description=Agenda Automator Backend
After=network.target

[Service]
Type=simple
User=agenda
Group=agenda
WorkingDirectory=/opt/agenda-automator
ExecStart=/opt/agenda-automator/agenda-automator
Restart=always
RestartSec=5
EnvironmentFile=/opt/agenda-automator/.env

# Security settings
NoNewPrivileges=yes
PrivateTmp=yes
ProtectSystem=strict
ReadWritePaths=/opt/agenda-automator
ProtectHome=yes

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable agenda-automator
sudo systemctl start agenda-automator
sudo systemctl status agenda-automator
```

## Container Deployment

### Docker Build

Create `Dockerfile`:

```dockerfile
# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o main cmd/server/main.go

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/main .

EXPOSE 8080

CMD ["./main"]
```

### Docker Compose (Production)

```yaml
version: '3.8'

services:
  agenda-automator:
    build: .
    ports:
      - "8080:8080"
    environment:
      - APP_ENV=production
      - DATABASE_URL=${DATABASE_URL}
      - ENCRYPTION_KEY=${ENCRYPTION_KEY}
      - GOOGLE_OAUTH_CLIENT_ID=${GOOGLE_OAUTH_CLIENT_ID}
      - GOOGLE_OAUTH_CLIENT_SECRET=${GOOGLE_OAUTH_CLIENT_SECRET}
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 10s
      retries: 3
```

### Kubernetes Deployment

Create `k8s/deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: agenda-automator
spec:
  replicas: 3
  selector:
    matchLabels:
      app: agenda-automator
  template:
    metadata:
      labels:
        app: agenda-automator
    spec:
      containers:
      - name: agenda-automator
        image: your-registry/agenda-automator:latest
        ports:
        - containerPort: 8080
        env:
        - name: APP_ENV
          value: "production"
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: agenda-secrets
              key: database-url
        - name: ENCRYPTION_KEY
          valueFrom:
            secretKeyRef:
              name: agenda-secrets
              key: encryption-key
        - name: GOOGLE_OAUTH_CLIENT_ID
          valueFrom:
            secretKeyRef:
              name: agenda-secrets
              key: google-oauth-client-id
        - name: GOOGLE_OAUTH_CLIENT_SECRET
          valueFrom:
            secretKeyRef:
              name: agenda-secrets
              key: google-oauth-client-secret
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /api/v1/health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

## Reverse Proxy Setup

### nginx Configuration

```nginx
server {
    listen 80;
    server_name yourdomain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Timeout settings
        proxy_connect_timeout 30s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
    }

    # Security headers
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains";
}
```

### Caddy Configuration

```caddyfile
yourdomain.com {
    reverse_proxy localhost:8080

    # Automatic HTTPS
    tls your-email@example.com

    # Security headers
    header {
        X-Frame-Options DENY
        X-Content-Type-Options nosniff
        X-XSS-Protection "1; mode=block"
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
    }
}
```

## Monitoring

### Health Checks

The application provides a health endpoint at `/api/v1/health`.

### Logging

- Application logs to stdout/stderr
- Use log aggregation services (ELK, Loki, CloudWatch)
- Monitor for errors and performance issues

### Metrics

Consider adding:
- Response times
- Error rates
- Database connection pool status
- Worker execution metrics

### Alerts

Set up alerts for:
- Application crashes
- High error rates
- Database connection issues
- High memory/CPU usage

## Security

### Network Security

- Use firewalls to restrict access
- Implement rate limiting
- Use HTTPS everywhere
- Regular security updates

### Application Security

- Keep dependencies updated
- Regular security audits
- Input validation and sanitization
- Secure session management

### Data Protection

- Encrypt sensitive data at rest
- Use secure communication channels
- Regular backups with encryption
- Data retention policies

## Scaling

### Horizontal Scaling

- Deploy multiple instances behind a load balancer
- Use session affinity if needed
- Ensure database connection pooling

### Vertical Scaling

- Increase CPU/memory based on load
- Monitor resource usage
- Optimize database queries

### Database Scaling

- Read replicas for read-heavy workloads
- Connection pooling
- Query optimization and indexing

## Backup and Recovery

### Database Backups

```bash
# Automated backup script
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME > backup_$DATE.sql

# Compress and store
gzip backup_$DATE.sql
aws s3 cp backup_$DATE.sql.gz s3://your-backup-bucket/
```

### Application Backups

- Backup configuration files
- Backup SSL certificates
- Document recovery procedures

### Disaster Recovery

- Multi-region deployment
- Automated failover
- Regular recovery testing
- Backup validation

## Maintenance

### Updates

1. Test updates in staging environment
2. Create backup before updates
3. Update application and dependencies
4. Monitor for issues post-update
5. Rollback plan if needed

### Monitoring Maintenance

- Regular log review
- Performance monitoring
- Security scanning
- Dependency updates

## Troubleshooting

### Common Issues

#### Application Won't Start
- Check environment variables
- Verify database connectivity
- Check file permissions
- Review application logs

#### High Memory Usage
- Check for memory leaks
- Optimize database queries
- Implement connection pooling
- Consider garbage collection tuning

#### Slow Responses
- Database query optimization
- Add caching layers
- Implement rate limiting
- Check network latency

### Logs and Debugging

```bash
# View application logs
sudo journalctl -u agenda-automator -f

# Check database connections
psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT count(*) FROM pg_stat_activity;"

# Monitor system resources
htop
df -h
free -h
```

## Support

For deployment issues:
1. Check application logs
2. Verify configuration
3. Test connectivity
4. Review system resources
5. Check network configuration

Remember to test your deployment thoroughly before going live!