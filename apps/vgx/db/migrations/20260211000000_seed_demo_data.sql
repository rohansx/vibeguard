-- migrate:up

-- ============================================================
-- DEMO SEED DATA
-- Fixed IDs for deterministic seeding and clean rollback
-- ============================================================

-- Organization
INSERT INTO organizations (id, clerk_org_id, name, slug, plan, settings) VALUES
    ('org_demo_001', 'org_clerk_demo', 'VibeGuard Demo', 'vibeguard-demo', 'free', '{"max_documents": 10, "max_scans_per_month": 50}');

-- Users
INSERT INTO users (id, clerk_user_id, org_id, email, name, role) VALUES
    ('usr_demo_admin', 'user_clerk_admin', 'org_demo_001', 'admin@vibeguard-demo.io', 'Alice Chen', 'admin'),
    ('usr_demo_dev', 'user_clerk_dev', 'org_demo_001', 'dev@vibeguard-demo.io', 'Bob Martinez', 'developer');

-- Documents
INSERT INTO documents (id, org_id, name, description, file_name, file_path, file_size, page_count, status, rule_count, approved_count, uploaded_by) VALUES
    ('doc_demo_gdpr', 'org_demo_001', 'GDPR Technical Requirements',
     'EU General Data Protection Regulation — technical implementation requirements extracted from Articles 5, 25, 32, and 35.',
     'gdpr-technical-requirements.pdf', '/data/uploads/gdpr-technical-requirements.pdf',
     2458624, 48, 'processed', 6, 3, 'usr_demo_admin'),
    ('doc_demo_pci', 'org_demo_001', 'PCI DSS v4.0 Summary',
     'Payment Card Industry Data Security Standard v4.0 — key requirements for cardholder data protection.',
     'pci-dss-v4-summary.pdf', '/data/uploads/pci-dss-v4-summary.pdf',
     1843200, 32, 'processing', 0, 0, 'usr_demo_admin'),
    ('doc_demo_soc2', 'org_demo_001', 'SOC 2 Type II Controls',
     'SOC 2 Type II — Trust Services Criteria for security, availability, and confidentiality.',
     'soc2-type2-controls.pdf', '/data/uploads/soc2-type2-controls.pdf',
     983040, 18, 'uploaded', 0, 0, 'usr_demo_dev');

-- Compliance rules (active, human-verified — from GDPR doc)
INSERT INTO compliance_rules (id, version, regulation, article, paragraph, title, requirement, source_text, source_page, check_type, severity, confidence, languages, technical_checks, cross_references, remediation, source_document_id, approved_by, approved_at, status) VALUES
    ('GDPR-ENC-001', '1.0', 'GDPR', 'Art. 32', '1(a)',
     'Encryption of personal data at rest',
     'All personal data must be encrypted at rest using AES-256 or equivalent.',
     'the controller and the processor shall implement appropriate technical and organisational measures to ensure a level of security appropriate to the risk, including as appropriate: (a) the pseudonymisation and encryption of personal data',
     12, 'encryption', 'critical', 'deterministic',
     '{python,go,typescript}',
     '{"checks": ["db_column_encryption", "file_encryption", "backup_encryption"]}',
     '{GDPR-ENC-002,GDPR-RET-001}',
     '{"steps": ["Enable TDE on database", "Encrypt file storage volumes", "Verify backup encryption"]}',
     'doc_demo_gdpr', 'usr_demo_admin', now() - interval '3 days', 'active'),

    ('GDPR-ENC-002', '1.0', 'GDPR', 'Art. 32', '1(a)',
     'Encryption of personal data in transit',
     'All data transmission containing personal data must use TLS 1.2+ encryption.',
     'the controller and the processor shall implement appropriate technical and organisational measures to ensure a level of security appropriate to the risk, including as appropriate: (a) the pseudonymisation and encryption of personal data',
     12, 'encryption', 'critical', 'deterministic',
     '{python,go,typescript}',
     '{"checks": ["tls_version_check", "certificate_validation", "hsts_header"]}',
     '{GDPR-ENC-001}',
     '{"steps": ["Enforce TLS 1.2+ on all endpoints", "Add HSTS headers", "Disable older TLS versions"]}',
     'doc_demo_gdpr', 'usr_demo_admin', now() - interval '3 days', 'active'),

    ('GDPR-RET-001', '1.0', 'GDPR', 'Art. 5', '1(e)',
     'Data retention limits',
     'Personal data must not be retained longer than necessary. Automated deletion after retention period.',
     'kept in a form which permits identification of data subjects for no longer than is necessary for the purposes for which the personal data are processed',
     5, 'data-retention', 'high', 'deterministic',
     '{python,go}',
     '{"checks": ["retention_policy_defined", "auto_deletion_enabled", "retention_audit_log"]}',
     '{GDPR-ENC-001}',
     '{"steps": ["Define retention policies per data category", "Implement automated purge jobs", "Log all deletions"]}',
     'doc_demo_gdpr', 'usr_demo_admin', now() - interval '2 days', 'active'),

    ('GDPR-CNS-001', '1.0', 'GDPR', 'Art. 7', '1',
     'Consent management',
     'Valid consent must be obtained before processing. Consent records must be maintained with timestamps.',
     'Where processing is based on consent, the controller shall be able to demonstrate that the data subject has consented to processing of his or her personal data',
     8, 'consent', 'high', 'deterministic',
     '{typescript}',
     '{"checks": ["consent_banner_present", "consent_record_stored", "consent_withdrawal_mechanism"]}',
     '{}',
     '{"steps": ["Implement consent banner", "Store consent records with timestamps", "Provide withdrawal mechanism"]}',
     'doc_demo_gdpr', NULL, NULL, 'active'),

    ('PCI-ENC-001', '1.0', 'PCI DSS', 'Req. 4', '4.1',
     'Encrypt cardholder data in transit',
     'Use strong cryptography to safeguard cardholder data during transmission over open, public networks.',
     'Use strong cryptography and security protocols to safeguard sensitive cardholder data during transmission over open, public networks',
     15, 'encryption', 'critical', 'deterministic',
     '{go,python}',
     '{"checks": ["tls_version_check", "cipher_suite_validation", "certificate_pinning"]}',
     '{PCI-LOG-001}',
     '{"steps": ["Enforce TLS 1.2+", "Configure strong cipher suites", "Implement certificate pinning"]}',
     'doc_demo_gdpr', NULL, NULL, 'active'),

    ('PCI-LOG-001', '1.0', 'PCI DSS', 'Req. 10', '10.2',
     'Audit logging for access events',
     'Implement automated audit trails for all system components to reconstruct security-relevant events.',
     'Implement automated audit trails for all system components to reconstruct the following events: all individual user accesses to cardholder data',
     22, 'logging', 'high', 'deterministic',
     '{go,python}',
     '{"checks": ["access_log_enabled", "log_tamper_protection", "log_retention_90_days"]}',
     '{PCI-ENC-001}',
     '{"steps": ["Enable access logging on all endpoints", "Implement append-only audit log", "Set 90-day retention"]}',
     'doc_demo_gdpr', NULL, NULL, 'active');

-- Proposed rules (pending human review — from PCI-DSS doc)
INSERT INTO proposed_rules (id, document_id, rule_id, version, regulation, article, paragraph, title, requirement, source_text, source_page, check_type, severity, confidence, languages, technical_checks, cross_references, remediation, review_status) VALUES
    ('a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d', 'doc_demo_pci', 'PCI-KEY-001', '1.0', 'PCI DSS', 'Req. 3', '3.5',
     'Cryptographic key management',
     'Protect cryptographic keys used for encryption of cardholder data against disclosure and misuse.',
     'Protect cryptographic keys used for encryption of cardholder data against both disclosure and misuse',
     18, 'key-management', 'critical', 'high',
     '{go,python}',
     '{"checks": ["key_rotation_enabled", "key_storage_hsm", "key_access_restricted"]}',
     '{PCI-ENC-001}',
     '{"steps": ["Implement key rotation schedule", "Use HSM or KMS for key storage", "Restrict key access to need-to-know"]}',
     'pending'),

    ('b2c3d4e5-f6a7-4b8c-9d0e-1f2a3b4c5d6e', 'doc_demo_pci', 'PCI-NET-001', '1.0', 'PCI DSS', 'Req. 1', '1.3',
     'Network segmentation',
     'Prohibit direct public access between the internet and any system component in the cardholder data environment.',
     'Prohibit direct public access between the Internet and any system component in the cardholder data environment',
     6, 'network', 'critical', 'high',
     '{go}',
     '{"checks": ["firewall_rules_configured", "dmz_isolation", "internal_segmentation"]}',
     '{}',
     '{"steps": ["Configure firewall rules", "Implement DMZ isolation", "Segment internal networks"]}',
     'pending'),

    ('c3d4e5f6-a7b8-4c9d-0e1f-2a3b4c5d6e7f', 'doc_demo_pci', 'PCI-ACC-001', '1.0', 'PCI DSS', 'Req. 7', '7.1',
     'Restrict access by business need-to-know',
     'Limit access to system components and cardholder data to only those individuals whose job requires such access.',
     'Limit access to system components and cardholder data to only those individuals whose job requires such access',
     20, 'access-control', 'high', 'medium',
     '{go,typescript}',
     '{"checks": ["rbac_implemented", "least_privilege_enforced", "access_reviews_scheduled"]}',
     '{PCI-LOG-001}',
     '{"steps": ["Implement RBAC", "Enforce least privilege", "Schedule quarterly access reviews"]}',
     'pending'),

    ('d4e5f6a7-b8c9-4d0e-1f2a-3b4c5d6e7f8a', 'doc_demo_pci', 'PCI-VUL-001', '1.0', 'PCI DSS', 'Req. 6', '6.2',
     'Vulnerability management',
     'Ensure all system components are protected from known vulnerabilities by installing applicable security patches.',
     'Ensure that all system components and software are protected from known vulnerabilities by installing applicable vendor-supplied security patches',
     16, 'vulnerability', 'high', 'high',
     '{go,python,typescript}',
     '{"checks": ["dependency_scanning", "patch_sla_compliance", "cve_monitoring"]}',
     '{}',
     '{"steps": ["Enable automated dependency scanning", "Define patch SLA (critical: 24h)", "Monitor CVE feeds"]}',
     'pending');

-- Scan results
INSERT INTO scan_results (id, org_id, initiated_by, repository_url, commit_hash, branch, scan_type, status, started_at, completed_at, total_rules_checked, total_findings, critical_findings, high_findings, medium_findings, low_findings, compliance_score, findings, scan_meta) VALUES
    ('e5f6a7b8-c9d0-4e1f-2a3b-4c5d6e7f8a9b', 'org_demo_001', 'usr_demo_dev',
     'https://github.com/vibeguard-demo/sample-app', 'a1b2c3d4e5f6', 'main',
     'full', 'completed',
     now() - interval '1 day', now() - interval '1 day' + interval '4 minutes',
     6, 3, 1, 1, 1, 0, 78.50,
     '[{"rule_id": "GDPR-ENC-001", "severity": "critical", "status": "fail", "message": "Database column ''users.email'' is not encrypted at rest", "file": "db/schema.sql", "line": 15},
       {"rule_id": "GDPR-RET-001", "severity": "high", "status": "fail", "message": "No retention policy defined for ''sessions'' table", "file": "db/schema.sql", "line": 42},
       {"rule_id": "GDPR-CNS-001", "severity": "medium", "status": "fail", "message": "Consent banner missing withdrawal mechanism", "file": "src/components/ConsentBanner.tsx", "line": 8}]',
     '{"scanner_version": "0.1.0", "duration_seconds": 243, "files_scanned": 127}'),

    ('f6a7b8c9-d0e1-4f2a-3b4c-5d6e7f8a9b0c', 'org_demo_001', 'usr_demo_admin',
     'https://github.com/vibeguard-demo/sample-app', 'b2c3d4e5f6a7', 'feat/add-encryption',
     'full', 'running',
     now() - interval '2 minutes', NULL,
     0, 0, 0, 0, 0, 0, NULL,
     '[]',
     '{"scanner_version": "0.1.0"}');

-- Audit log entries (append-only, with hash chain)
INSERT INTO audit_log (org_id, actor_id, actor_type, event_type, resource_type, resource_id, details, previous_hash, entry_hash) VALUES
    ('org_demo_001', 'usr_demo_admin', 'user', 'document.uploaded', 'document', 'doc_demo_gdpr',
     '{"file_name": "gdpr-technical-requirements.pdf", "file_size": 2458624}',
     NULL,
     encode(sha256(('document.uploaded:doc_demo_gdpr:' || now()::text)::bytea), 'hex')),

    ('org_demo_001', 'usr_demo_admin', 'user', 'rule.approved', 'compliance_rule', 'GDPR-ENC-001',
     '{"proposed_rule_id": null, "regulation": "GDPR", "title": "Encryption of personal data at rest"}',
     encode(sha256(('document.uploaded:doc_demo_gdpr:' || now()::text)::bytea), 'hex'),
     encode(sha256(('rule.approved:GDPR-ENC-001:' || now()::text)::bytea), 'hex')),

    ('org_demo_001', 'usr_demo_dev', 'user', 'scan.initiated', 'scan_result', 'e5f6a7b8-c9d0-4e1f-2a3b-4c5d6e7f8a9b',
     '{"repository_url": "https://github.com/vibeguard-demo/sample-app", "branch": "main", "scan_type": "full"}',
     encode(sha256(('rule.approved:GDPR-ENC-001:' || now()::text)::bytea), 'hex'),
     encode(sha256(('scan.initiated:e5f6a7b8-c9d0-4e1f-2a3b-4c5d6e7f8a9b:' || now()::text)::bytea), 'hex'));

-- migrate:down

-- Delete in reverse dependency order
DELETE FROM audit_log WHERE org_id = 'org_demo_001';
DELETE FROM scan_results WHERE org_id = 'org_demo_001';
DELETE FROM proposed_rules WHERE document_id IN ('doc_demo_gdpr', 'doc_demo_pci', 'doc_demo_soc2');
DELETE FROM compliance_rules WHERE id IN ('GDPR-ENC-001', 'GDPR-ENC-002', 'GDPR-RET-001', 'GDPR-CNS-001', 'PCI-ENC-001', 'PCI-LOG-001');
DELETE FROM documents WHERE id IN ('doc_demo_gdpr', 'doc_demo_pci', 'doc_demo_soc2');
DELETE FROM users WHERE id IN ('usr_demo_admin', 'usr_demo_dev');
DELETE FROM organizations WHERE id = 'org_demo_001';
