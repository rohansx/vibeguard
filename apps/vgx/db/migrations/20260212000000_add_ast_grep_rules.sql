-- migrate:up

-- Update compliance rules with ast-grep rule patterns so the scanner can produce real findings.
-- These patterns detect common compliance violations in Python, TypeScript, and Go codebases.

-- GDPR-ENC-001: Encryption of personal data at rest
-- Detects database connections without encryption/SSL parameters
UPDATE compliance_rules SET technical_checks = '{
  "checks": ["db_column_encryption", "file_encryption", "backup_encryption"],
  "ast_grep_rules": [
    {
      "id": "gdpr-enc-001-py-unencrypted-db",
      "language": "Python",
      "rule": {"pattern": "create_engine($URL)"},
      "message": "GDPR Art. 32: Database connection may lack encryption. Ensure SSL/TLS is configured.",
      "severity": "warning"
    },
    {
      "id": "gdpr-enc-001-ts-plaintext-storage",
      "language": "TypeScript",
      "rule": {"pattern": "localStorage.setItem($KEY, $VALUE)"},
      "message": "GDPR Art. 32: Storing data in localStorage without encryption. Personal data must be encrypted at rest.",
      "severity": "warning"
    }
  ]
}'::jsonb WHERE id = 'GDPR-ENC-001';

-- GDPR-ENC-002: Encryption of personal data in transit
-- Detects HTTP URLs (should be HTTPS) and disabled TLS verification
UPDATE compliance_rules SET technical_checks = '{
  "checks": ["tls_version_check", "certificate_validation", "hsts_header"],
  "ast_grep_rules": [
    {
      "id": "gdpr-enc-002-py-http-url",
      "language": "Python",
      "rule": {"pattern": "requests.get(\"http://$$$\")"},
      "message": "GDPR Art. 32: Using unencrypted HTTP. All data transmission must use TLS 1.2+.",
      "severity": "error"
    },
    {
      "id": "gdpr-enc-002-py-no-verify",
      "language": "Python",
      "rule": {"pattern": "verify=False"},
      "message": "GDPR Art. 32: TLS certificate verification disabled. This allows MITM attacks.",
      "severity": "error"
    },
    {
      "id": "gdpr-enc-002-ts-http-fetch",
      "language": "TypeScript",
      "rule": {"pattern": "fetch(\"http://$$$\")"},
      "message": "GDPR Art. 32: Using unencrypted HTTP fetch. Use HTTPS for all data transmission.",
      "severity": "error"
    }
  ]
}'::jsonb WHERE id = 'GDPR-ENC-002';

-- GDPR-CNS-001: Consent management
-- Detects cookie/tracking without consent checks
UPDATE compliance_rules SET technical_checks = '{
  "checks": ["consent_banner_present", "consent_record_stored", "consent_withdrawal_mechanism"],
  "ast_grep_rules": [
    {
      "id": "gdpr-cns-001-ts-cookie-no-consent",
      "language": "TypeScript",
      "rule": {"pattern": "document.cookie = $VALUE"},
      "message": "GDPR Art. 7: Setting cookies without consent check. Implement consent management before storing cookies.",
      "severity": "warning"
    },
    {
      "id": "gdpr-cns-001-ts-tracking-pixel",
      "language": "TypeScript",
      "rule": {"pattern": "new Image().src = $URL"},
      "message": "GDPR Art. 7: Tracking pixel without consent. User consent must be obtained before tracking.",
      "severity": "warning"
    }
  ]
}'::jsonb WHERE id = 'GDPR-CNS-001';

-- PCI-LOG-001: Audit logging for access events
-- Detects missing logging in route handlers
UPDATE compliance_rules SET technical_checks = '{
  "checks": ["access_log_enabled", "log_tamper_protection", "log_retention_90_days"],
  "ast_grep_rules": [
    {
      "id": "pci-log-001-py-print-debug",
      "language": "Python",
      "rule": {"pattern": "print($MSG)"},
      "message": "PCI DSS Req. 10: Using print() instead of proper audit logging. Use structured logging for security events.",
      "severity": "warning"
    },
    {
      "id": "pci-log-001-go-no-log",
      "language": "Go",
      "rule": {"pattern": "fmt.Println($MSG)"},
      "message": "PCI DSS Req. 10: Using fmt.Println instead of structured logging. Use log package for audit trails.",
      "severity": "warning"
    }
  ]
}'::jsonb WHERE id = 'PCI-LOG-001';

-- migrate:down

-- Revert to simple check lists
UPDATE compliance_rules SET technical_checks = '{"checks": ["db_column_encryption", "file_encryption", "backup_encryption"]}'::jsonb WHERE id = 'GDPR-ENC-001';
UPDATE compliance_rules SET technical_checks = '{"checks": ["tls_version_check", "certificate_validation", "hsts_header"]}'::jsonb WHERE id = 'GDPR-ENC-002';
UPDATE compliance_rules SET technical_checks = '{"checks": ["consent_banner_present", "consent_record_stored", "consent_withdrawal_mechanism"]}'::jsonb WHERE id = 'GDPR-CNS-001';
UPDATE compliance_rules SET technical_checks = '{"checks": ["access_log_enabled", "log_tamper_protection", "log_retention_90_days"]}'::jsonb WHERE id = 'PCI-LOG-001';
