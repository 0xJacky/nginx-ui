# Security Policy

## Supported Versions

Security support status for currently maintained versions:

| Version | Support Status        |
|---------|-----------------------|
| 2.x     | ✅ Actively Maintained |
| 1.x     | ❌ End of Life         |

## Vulnerability Reporting

### Submit Vulnerability
Please submit reports via [GitHub Security Advisory](https://github.com/0xJacky/nginx-ui/security/advisories/new) with:
- Affected version(s)
- Detailed vulnerability description
- Reproducible PoC (Proof of Concept)
- Environment configuration details

### Handling Process
- Valid reports will be tracked through private advisory channels
- Within 21-31 days after remediation:
  - Request CVE identifier from numbering authorities
  - Publish technical details on GitHub Advisory
  - Update Release Notes with impact assessment

### Requirements
- **Testing Restrictions**: All security validation must be conducted in locally built isolated environments. Online demo systems are strictly prohibited for testing purposes
- **Environment Isolation**: Testing environments must be network-segregated from production systems. Test traffic must not leak beyond isolated networks
- Destructive testing is prohibited without explicit authorization
- Adhere to Coordinated Disclosure principles
- Vulnerability details must remain confidential until public disclosure

> Security researchers will be acknowledged in project credits based on contribution significance
