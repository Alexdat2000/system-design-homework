```bash
python3 load_testing/seed_long_lived_orders.py \
  --orders 600 \
  --concurrency 80 \
  --min-duration 10 \
  --max-duration 20 \
  --gets-per-order 5 \
  --finish-ratio 0.9 \
  --start-window 570 \
  --start-pattern waves \
  --wave-period 90 \
  --wave-width 8 \
  --wave-strength 0.9 \
  --scooter-id-mode load
```

```bash
docker compose down && docker compose build && docker compose up -d
```

```bash
docker compose down -v --remove-orphans
```

```bash
http://51.250.101.214:8088/login/?next=http%3A%2F%2F51.250.101.214%3A8088%2Fhome
```
