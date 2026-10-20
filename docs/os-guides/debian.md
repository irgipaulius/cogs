# Debian / Ubuntu

```bash
sudo useradd -r -s /usr/sbin/nologin cogs
sudo mkdir -p /var/lib/cogs /etc/cogs
sudo cp .env /etc/cogs/app.env
cogs service render systemd
sudo cp cogs-app.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now cogs-app
```

Deploy updates with `cogs deploy user@host --target linux/amd64`.
