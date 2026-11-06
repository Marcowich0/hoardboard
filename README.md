# hoardboard

Self-hosted disk usage dashboard. Track the size of your folders over time, grouped into custom categories, with low-impact scheduled scans.

Built for Unraid: it reads each array disk and the cache pool directly (metadata only, never file contents) at idle I/O priority, and skips Scans while a parity check, rebuild or the mover runs.

## Run

```sh
docker run -d --name hoardboard \
  --cap-drop ALL --cap-add DAC_READ_SEARCH --security-opt no-new-privileges:true \
  -p 8098:8098 \
  -v /mnt:/mnt:ro \
  -v /var/local/emhttp:/var/local/emhttp:ro \
  -v /mnt/user/appdata/hoardboard:/data \
  -e TZ=Europe/Copenhagen \
  ghcr.io/marcowich0/hoardboard:latest
```

Settings and development commands are in [AGENTS.md](AGENTS.md).
