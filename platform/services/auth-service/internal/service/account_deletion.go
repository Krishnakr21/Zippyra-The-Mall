package service

// AccountDeletionService handles DPDPA-compliant account deletion.
// Publishes account.deletion_requested Kafka event for PII purge cascade.
type AccountDeletionService struct{}
