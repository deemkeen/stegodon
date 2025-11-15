# Docker Deployment Guide

This guide explains how to run stegodon using Docker and Docker Compose.

## Quick Start

### Using Docker Compose (Recommended)

1. **Start stegodon:**
   ```bash
   docker-compose up -d
   ```

2. **Connect via SSH:**
   ```bash
   ssh root@localhost -p 23232
   ```

3. **View logs:**
   ```bash
   docker-compose logs -f stegodon
   ```

4. **Stop stegodon:**
   ```bash
   docker-compose down
   ```

### Using Docker CLI

1. **Build the image:**
   ```bash
   docker build -t stegodon .
   ```

2. **Run the container:**
   ```bash
   docker run -d \
     --name stegodon \
     -p 23232:23232 \
     -p 9999:9999 \
     -v stegodon-data:/home/stegodon/.config/stegodon \
     stegodon
   ```

3. **Connect via SSH:**
   ```bash
   ssh root@localhost -p 23232
   ```

## Configuration

### Environment Variables

Configure stegodon by setting environment variables in `docker-compose.yml`:

```yaml
environment:
  - STEGODON_HOST=0.0.0.0
  - STEGODON_SSHPORT=23232
  - STEGODON_HTTPPORT=9999
  - STEGODON_WITH_AP=true
  - STEGODON_SSLDOMAIN=yourdomain.com
  - STEGODON_SINGLE=true
  - STEGODON_CLOSED=true
```

### Data Persistence

All data (database, SSH keys, config) is stored in a Docker volume:

```yaml
volumes:
  - stegodon-data:/home/stegodon/.config/stegodon
```

**Backup data:**
```bash
docker run --rm -v stegodon-data:/data -v $(pwd):/backup alpine tar czf /backup/stegodon-backup.tar.gz -C /data .
```

**Restore data:**
```bash
docker run --rm -v stegodon-data:/data -v $(pwd):/backup alpine tar xzf /backup/stegodon-backup.tar.gz -C /data
```

## ActivityPub Federation with Docker

To enable ActivityPub federation, you need to expose stegodon through a reverse proxy with HTTPS.

### Example with Nginx

1. **Update docker-compose.yml:**
   ```yaml
   services:
     stegodon:
       # ... existing config ...
       environment:
         - STEGODON_WITH_AP=true
         - STEGODON_SSLDOMAIN=yourdomain.com
       networks:
         - stegodon-net

     nginx:
       image: nginx:alpine
       container_name: nginx
       restart: unless-stopped
       ports:
         - "80:80"
         - "443:443"
       volumes:
         - ./nginx.conf:/etc/nginx/nginx.conf:ro
         - certbot-data:/etc/letsencrypt
       networks:
         - stegodon-net

   networks:
     stegodon-net:
       driver: bridge
   ```

2. **Create nginx.conf:**
   ```nginx
   events {
       worker_connections 1024;
   }

   http {
       server {
           listen 443 ssl http2;
           server_name yourdomain.com;

           ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
           ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;

           location / {
               proxy_pass http://stegodon:9999;
               proxy_set_header Host $host;
               proxy_set_header X-Real-IP $remote_addr;
               proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
               proxy_set_header X-Forwarded-Proto $scheme;
           }
       }
   }
   ```

### Example with Caddy (Easier)

1. **Create Caddyfile:**
   ```
   yourdomain.com {
       reverse_proxy stegodon:9999
   }
   ```

2. **Update docker-compose.yml:**
   ```yaml
   services:
     stegodon:
       # ... existing config ...
       environment:
         - STEGODON_WITH_AP=true
         - STEGODON_SSLDOMAIN=yourdomain.com
       networks:
         - stegodon-net

     caddy:
       image: caddy:2-alpine
       container_name: caddy
       restart: unless-stopped
       ports:
         - "80:80"
         - "443:443"
       volumes:
         - ./Caddyfile:/etc/caddy/Caddyfile:ro
         - caddy-data:/data
         - caddy-config:/config
       networks:
         - stegodon-net

   networks:
     stegodon-net:
       driver: bridge

   volumes:
     stegodon-data:
     caddy-data:
     caddy-config:
   ```

## Port Mapping

- **23232** - SSH port for TUI access
- **9999** - HTTP port for RSS feeds and ActivityPub

**Important:** Keep SSH port 23232 exposed for direct SSH access. Only proxy port 9999 through your reverse proxy.

## Resource Limits

The default docker-compose.yml includes resource limits:

```yaml
deploy:
  resources:
    limits:
      cpus: '1'
      memory: 512M
    reservations:
      cpus: '0.25'
      memory: 128M
```

Adjust these based on your needs and user count.

## Security Considerations

1. **SSH Keys:** Users authenticate with their SSH public keys. The container generates a unique SSH host key on first run.

2. **Reverse Proxy:** Always use a reverse proxy with HTTPS for production ActivityPub federation.

3. **Firewall:** Only expose necessary ports:
   - Port 23232: SSH access
   - Port 80/443: HTTPS through reverse proxy

4. **Updates:** Regularly rebuild the image to get security updates:
   ```bash
   docker-compose pull
   docker-compose up -d --build
   ```

## Troubleshooting

### Container won't start

Check logs:
```bash
docker-compose logs stegodon
```

### Can't connect via SSH

1. Check if port 23232 is exposed:
   ```bash
   docker ps
   ```

2. Check if container is running:
   ```bash
   docker-compose ps
   ```

3. Test connection:
   ```bash
   nc -zv localhost 23232
   ```

### Database locked errors

Stop all containers and restart:
```bash
docker-compose down
docker-compose up -d
```

### Reset everything

**Warning:** This deletes all data!

```bash
docker-compose down -v
docker-compose up -d
```

## Health Checks

The Dockerfile includes a health check that pings the RSS feed endpoint every 30 seconds:

```dockerfile
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:9999/feed || exit 1
```

Check health status:
```bash
docker ps
# Look for "healthy" in the STATUS column
```

## Production Deployment

For production deployment:

1. Use a reverse proxy (Caddy recommended for automatic HTTPS)
2. Set up proper DNS records
3. Configure `STEGODON_SSLDOMAIN` to your domain
4. Enable ActivityPub with `STEGODON_WITH_AP=true`
5. Set up regular backups of the data volume
6. Monitor logs and resource usage
7. Consider using Docker Swarm or Kubernetes for high availability

## Example Production Setup

Complete production setup with Caddy:

```yaml
version: '3.8'

services:
  stegodon:
    build: .
    container_name: stegodon
    restart: unless-stopped
    environment:
      - STEGODON_HOST=0.0.0.0
      - STEGODON_WITH_AP=true
      - STEGODON_SSLDOMAIN=stegodon.example.com
      # - STEGODON_SINGLE=true  # Uncomment for personal blog
    volumes:
      - stegodon-data:/home/stegodon/.config/stegodon
    networks:
      - stegodon-net
    deploy:
      resources:
        limits:
          cpus: '2'
          memory: 1G

  caddy:
    image: caddy:2-alpine
    container_name: caddy
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
      - "23232:23232"  # SSH passthrough
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy-data:/data
      - caddy-config:/config
    networks:
      - stegodon-net

networks:
  stegodon-net:
    driver: bridge

volumes:
  stegodon-data:
  caddy-data:
  caddy-config:
```

Caddyfile:
```
stegodon.example.com {
    reverse_proxy stegodon:9999
}

:23232 {
    reverse_proxy stegodon:23232
}
```
