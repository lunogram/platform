# Authentication Migration Documentation

This directory contains the complete proposal and documentation for migrating authentication endpoints from the Platform service (Node.js) to the Nexus service (Go).

## Documents Overview

### 📋 [Full Technical Proposal](./auth-migration.md)
**For: Engineers, Architects**

Comprehensive 672-line technical specification including:
- Detailed architecture design
- Interface definitions and code examples
- All 6 provider implementations
- Complete OpenAPI specifications
- Database integration patterns
- Security considerations
- Testing strategies
- Migration phases (6 weeks)

👉 **Start here if**: You're implementing the migration or need deep technical details.

---

### 📊 [Executive Summary](./auth-migration-summary.md)
**For: Engineering Managers, Product Owners, Stakeholders**

High-level overview including:
- Why this migration matters
- What's being migrated
- Key design principles
- Implementation phases
- Timeline and deliverables
- Risk mitigation
- Success criteria

👉 **Start here if**: You need to understand the scope, timeline, and business impact.

---

### 🔧 [Quick Reference Guide](./auth-migration-quickref.md)
**For: Developers, Implementation Team**

Developer cheat sheet including:
- Interface and type definitions
- Common code patterns
- Configuration examples
- Testing templates
- Debug checklist
- Useful commands

👉 **Start here if**: You're actively coding and need quick reference material.

---

### ✅ [Implementation Checklist](./auth-migration-checklist.md)
**For: Project Managers, Implementation Team**

Detailed task checklist including:
- Phase 1: Foundation (Week 1)
- Phase 2: Simple Providers (Week 2)
- Phase 3: OAuth/OIDC (Week 3)
- Phase 4: SAML (Week 4)
- Phase 5: Cloud Provider (Week 5)
- Phase 6: Cutover (Week 6)
- Success criteria

👉 **Start here if**: You're tracking implementation progress or planning sprints.

---

## Quick Links

### Current Implementation
- **Platform Service**: [`services/platform/src/auth/`](../../services/platform/src/auth/)
- **Nexus Service**: [`services/nexus/internal/http/auth/`](../../services/nexus/internal/http/auth/)

### Key Files to Review
- [`AuthProvider.ts`](../../services/platform/src/auth/AuthProvider.ts) - Abstract base class
- [`Auth.ts`](../../services/platform/src/auth/Auth.ts) - Provider initialization
- [`AuthController.ts`](../../services/platform/src/auth/AuthController.ts) - Route definitions
- [`auth.go`](../../services/nexus/internal/http/auth/auth.go) - Existing middleware
- [`admins.go`](../../services/nexus/internal/store/admins.go) - Admin store

## Getting Started

### For Implementation

1. **Review the proposal**
   ```bash
   cd docs/proposals
   # Read the full proposal
   cat auth-migration.md
   ```

2. **Set up your environment**
   ```bash
   # Install Go dependencies (will be added during implementation)
   cd services/nexus
   go mod download
   
   # Set up test database
   # (See nexus README for database setup)
   ```

3. **Start with Phase 1**
   - Review the [checklist](./auth-migration-checklist.md)
   - Create provider interface
   - Implement registry
   - Add OpenAPI spec

### For Review

1. **Technical Review**
   - Read [Full Proposal](./auth-migration.md)
   - Review interface design
   - Validate security considerations
   - Check against existing patterns

2. **Business Review**
   - Read [Executive Summary](./auth-migration-summary.md)
   - Verify timeline is acceptable
   - Confirm resource allocation
   - Approve rollback plan

3. **Implementation Review**
   - Use [Checklist](./auth-migration-checklist.md) to track progress
   - Review completed phases
   - Validate testing coverage

## Architecture at a Glance

```
┌─────────────────────────────────────────────────────────┐
│                     Frontend (React)                    │
│                                                         │
│  /auth/methods                                          │
│  /auth/login/:driver                                    │
│  /auth/login/:driver/callback                           │
└────────────┬────────────────────────────────────────────┘
             │
             │ HTTP/HTTPS
             │
┌────────────▼────────────────────────────────────────────┐
│                   Nexus Service (Go)                     │
│                                                         │
│  ┌────────────────────────────────────────────────┐    │
│  │              Auth Handlers                      │    │
│  └────────────┬───────────────────────────────────┘    │
│               │                                         │
│  ┌────────────▼───────────────────────────────────┐    │
│  │           Provider Registry                     │    │
│  │  ┌──────────────────────────────────────────┐  │    │
│  │  │ Basic │ Email │ SAML │ OIDC │ Google ... │  │    │
│  │  └──────────────────────────────────────────┘  │    │
│  └────────────┬───────────────────────────────────┘    │
│               │                                         │
│  ┌────────────▼───────────────────────────────────┐    │
│  │           Token Management                      │    │
│  │  • JWT Generation                               │    │
│  │  • Cookie Management                            │    │
│  │  • Token Storage                                │    │
│  └────────────┬───────────────────────────────────┘    │
│               │                                         │
│  ┌────────────▼───────────────────────────────────┐    │
│  │              Database Store                      │    │
│  │  • Admins                                       │    │
│  │  • Access Tokens                                │    │
│  │  • Organizations                                │    │
│  └─────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

## Authentication Flow

```
1. User → Frontend: Click "Login with Google"
   Frontend → Nexus: GET /auth/login/google?r=/dashboard

2. Nexus → Registry: Get Google provider
   Provider.Start() → Generate auth URL + state cookie

3. Nexus → User: Redirect to Google with state

4. Google → User: Login prompt
   User → Google: Credentials

5. Google → Nexus: Redirect to /auth/login/google/callback?code=xyz

6. Nexus → Provider: Validate callback
   Provider.Validate() → Exchange code, verify token, extract profile

7. Provider → Nexus: Return AdminInfo

8. Nexus → Database: Get or create admin

9. Nexus → Token Manager: Generate JWT + OAuth cookie

10. Nexus → User: Redirect to /dashboard with cookie set
```

## Provider Comparison

| Feature | Basic | Email | SAML | OpenID | Google | Clerk |
|---------|-------|-------|------|--------|--------|-------|
| **Complexity** | Low | Low | High | Medium | Medium | Medium |
| **External Deps** | None | SMTP | SAML IdP | OIDC Provider | Google | Clerk |
| **Use Case** | Dev/Test | Simple Auth | Enterprise SSO | Standards-based | Consumer | Managed |
| **Webhooks** | No | No | No | No | No | Yes |
| **Week** | 2 | 2 | 4 | 3 | 3 | 5 |

## Timeline

```
Week 1: Foundation
  ├── Provider interface
  ├── Registry system
  ├── Token utilities
  └── OpenAPI spec

Week 2: Simple Providers
  ├── BasicProvider
  └── EmailProvider

Week 3: OAuth/OIDC
  ├── OpenIDProvider
  └── GoogleProvider

Week 4: SAML
  └── SAMLProvider

Week 5: Cloud
  └── ClerkProvider

Week 6: Cutover
  ├── Feature flag
  ├── Monitoring
  ├── Gradual rollout
  └── Cleanup
```

## Key Decisions

### ✅ Maintain Modularity
Provider-based architecture preserved with Go interfaces.

### ✅ OpenAPI-First
All endpoints defined in OpenAPI before implementation.

### ✅ Zero Breaking Changes
Complete backwards compatibility maintained.

### ✅ Phased Rollout
Incremental migration with rollback capability.

### ✅ Security First
CSRF protection, webhook verification, audit logging.

## Questions & Answers

### Q: Will the frontend need changes?
**A:** No. Endpoint URLs, request/response formats, and authentication flow remain identical.

### Q: What if something goes wrong during migration?
**A:** Feature flag allows instant rollback to platform service within 5 minutes.

### Q: How long will the migration take?
**A:** 6 weeks for full implementation with all 6 providers.

### Q: Can we migrate incrementally?
**A:** Yes. Each provider can be enabled independently via configuration.

### Q: What about existing sessions?
**A:** JWT tokens remain valid. Users won't need to re-authenticate.

### Q: Is this tested?
**A:** Yes. Unit tests, integration tests, and manual testing for each provider.

## Contributing

When implementing this proposal:

1. **Follow the checklist** - Use [auth-migration-checklist.md](./auth-migration-checklist.md)
2. **Reference the proposal** - Link to specific sections in your PRs
3. **Update the checklist** - Check off items as you complete them
4. **Write tests first** - TDD approach for each provider
5. **Document decisions** - Add notes to the checklist

## Status

- **Status**: 📝 Proposal Complete
- **Phase**: Planning
- **Next Step**: Technical review and approval
- **Owner**: TBD
- **Start Date**: TBD
- **Target Completion**: TBD + 6 weeks

## Feedback

To provide feedback on this proposal:

1. Review the appropriate document for your role
2. Create a GitHub issue with label `auth-migration`
3. Tag relevant team members
4. Reference specific sections of the documentation

## License

This documentation is part of the Lunogram platform and follows the project's license.

---

**Last Updated**: 2025-12-16
**Version**: 1.0
**Authors**: GitHub Copilot (analysis and proposal generation)
