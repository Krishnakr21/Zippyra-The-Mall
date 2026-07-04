# exit-validation-service

RFID gate validation, conveyor belt scanning (Phase 3), exit token generation, staff override. Tables: exit_tokens, override_log. Hypertables: rfid_scan_events, conveyor_rfid_events. MQTT: RFID reader integration.

## Run Locally
```bash
go run ./cmd/server  # Port 8009
```
