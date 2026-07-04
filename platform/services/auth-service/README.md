# auth-service

Authentication, OTP, JWT sessions, account management. Tables: users, auth_events, admin_actions, login_attempts, email_verifications, app_versions.

Kafka produces: auth.otp_sent, auth.login_success, auth.login_failed, auth.account_locked, auth.session_created, auth.session_ended, auth.staff_login, account.deletion_requested.

## Run Locally
```bash
go run ./cmd/server  # Port 8001
```
