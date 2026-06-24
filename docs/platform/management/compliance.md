---
seotitle: Security & Compliance
seodesc: Learn about Encore's security practices, infrastructure protections, and compliance posture — built on industry-standard controls and trusted cloud providers.
title: Security & Compliance
subtitle: How Encore protects your applications, code, and data
lang: platform
---

_Last updated: June 24, 2026_

Your applications, code, and data are among your most important assets. Security is foundational to everything we build at Encore — it is embedded in our architecture, our processes, and our culture. This document provides a comprehensive overview of the security controls and practices we have in place today, structured around the SOC 2 trust service criteria.

### Security at a glance

| Area | What we do |
| --- | --- |
| **Infrastructure** | Hosted on GCP (ISO 27001 / SOC 2 certified). All servers are private, accessible only via VPN. |
| **Encryption** | AES-256 at rest, TLS 1.2+ and WireGuard in transit. Customer secrets additionally encrypted via GCP KMS. |
| **Zero-trust networking** | All server-to-server communication is authenticated and end-to-end encrypted via Tailscale / WireGuard. |
| **Access control** | Principle of least privilege, MFA enforced, regular access reviews, VPN-only infrastructure access. Time-limited privileged access via GCP Privileged Access Manager (PAM) with full audit trail. |
| **Authentication** | End-user authentication managed by Clerk (SOC 2 certified). Passwordless by default — Encore never stores or handles user passwords. Employee access governed by Google Workspace SSO with mandatory MFA. |
| **Monitoring & alerting** | 24/7 monitoring via Grafana, Sentry, Cronitor, and GCP Cloud Monitoring (all SOC 2 certified). |
| **Vendor security** | All critical vendors are SOC 2 and/or ISO 27001 certified (GCP, Tailscale, Clerk, GitHub, Sentry, Grafana). Vendors are risk-classified and reviewed under a formal Vendor Management Policy. |
| **Code quality** | Mandatory code review, CI/CD with automated testing, automated vulnerability scanning. Governed by a formal Change Management Policy. |
| **Documented policies** | Formal written policy suite in place: Information Security, Vulnerability Management, Incident Response, Change Management, Data Classification, and Vendor Management. |
| **Data privacy** | GDPR compliant. Data minimization by design. Four-tier data classification (Public, Internal, Confidential, Restricted). |
| **Responsible disclosure** | Active bug bounty program for security researchers. |

## SOC 2

SOC is short for "System and Organization Controls" — it is the de facto industry standard for software security and privacy. We have implemented controls aligned with the SOC 2 framework and are actively preparing for a formal SOC 2 Type I audit, during which an external auditor will verify that our controls meet the standard.

As part of this preparation, we have formalized our security program into a written policy suite, including an Information Security Policy, Vulnerability Management Policy, Incident Response Policy, Change Management Policy, Data Classification Policy, and Vendor Management Policy. These documents codify the practices described on this page and are available to customers under NDA as part of a vendor review.

Our current SOC 2 timeline is:

- **SOC 2 Type I:** target completion Q4 2026.
- **SOC 2 Type II:** target completion Q2 2027, with the observation period beginning immediately after Type I.
- **Third-party penetration test:** target completion October 30, 2026, so the report is available within the Type I audit window. An executive summary and remediation status are available without NDA; the full report is available under NDA on request.

After the Type I audit, we will proceed to Type II, which involves continuous monitoring of our controls over an extended observation period.

### Trust Service Criteria

The five SOC 2 trust service criteria are: security, availability, confidentiality, processing integrity, and privacy.

**1\. Security**

Protecting systems against unauthorized access.

**2\. Availability**

Ensuring that the system remains functional and usable.

**3\. Confidentiality**

Restricting the access of data to a specified set of persons or organizations. Ensuring that network communication is encrypted and cannot be intercepted by unauthorized personnel.

**4\. Processing integrity**

Ensuring that a system fulfills its purpose and delivers correct data.

**5\. Privacy**

Minimal processing and use of personal data in accordance with the law.

The following sections describe in detail how Encore implements each trust service principle.

## Security

We believe security is achieved through proven best practices and industry standards — not through obscurity or homegrown cryptography. Our approach is defense in depth: multiple overlapping layers of protection so that the compromise of any single layer does not result in a breach.

We have a designated Security Officer responsible for all aspects of security across infrastructure, software, and data.

### Infrastructure security

Encore's core production infrastructure is hosted on GCP (Google Cloud Platform), an ISO27001/SOC 2 compliant vendor. Auxiliary services are provided by Hetzner, an ISO27001 compliant vendor. Tailscale, a SOC 2 compliant vendor, provides VPN (Virtual Private Network) services used to secure communication between all servers.

All core data processing is carried out in the US East region (us-east-1), and backups are kept in multiple separate regions in the US. Each region is composed of at least three "availability zones" (AZs) which are isolated locations, designed to take over in case of a catastrophic failure at one location. AZs are separated by a significant distance such that it is unlikely that they are affected by the same issues such as power outages, earthquakes, etc. Physical access to GCP is restricted by GCP's security controls. Furthermore, GCP monitors and immediately responds to power, temperature, fire, water leaks, etc.

Access to Encore's production infrastructure is restricted to Encore employees. All systems have access controls and only a limited number of employees have privileged access. Access is only possible through a VPN over Tailscale.

The production environment is separated from testing environments, using separate accounts and VPCs (Virtual Private Cloud) in GCP. This ensures that any defect in a test environment cannot impact the production system. The connection to the internet is controlled by dedicated gateways.

### Organizational security

Since an organization is only as good as its people, Encore takes great care when selecting and training its staff. All employees undergo a thorough selection process that has been designed to identify the best talent in the world for the job. Many of Encore's employees have extensive experience working in regulated environments such as Online Banking and large-scale Online Payments.

Individual performance monitoring is carried out by managers on a bi-weekly cadence. Overall organizational performance is tracked continuously and reviewed by management on a monthly cadence using Key Performance Indicators determined by management.

Encore is rolling out mandatory annual security awareness training for all employees and contractors, to be in place ahead of the SOC 2 Type I audit in Q4 2026. The training covers security responsibilities, data handling, access protection, phishing and social engineering, device security, incident reporting, and the relevant company security policies. It is designed to increase sensitivity to physical security (hardware and media handling, office access control, etc.), digital security (e.g. secure passwords, two-factor authentication), social engineering attacks ("phishing"), and other security-related topics.

Encore employment policy mandates full-disk encryption on all employee devices.

### Product security

Encore is aware of how important it is for its customers that all data is handled securely. Therefore, several layers of protection ensure that the data is not accessible to unauthorized persons.

An essential part of software security is "defense in depth" which means that there are multiple layers of protection. In case one layer is breached, the next layer helps to contain the breach and mitigate its consequences. This can be achieved by isolating software components from each other, such that the breach of one component does not affect adjacent software.

Encore's service-based architecture provides natural isolation between components, and we have adopted a zero-trust security model with the use of Tailscale. All server-to-server communication is authenticated and end-to-end encrypted with WireGuard. GCP's VPC (Virtual Private Cloud) provides another layer of isolation from the internet on the network level. None of Encore's servers are publicly accessible on the internet.

As a general principle, all of Encore's data is encrypted while being transported across networks and when stored ("in transit and at rest"). In case of unauthorized access to the data, an attacker would only see undecipherable garbage which cannot be decrypted without the corresponding keys. The encryption methods employed by Encore are industry standard and deemed unbreakable by contemporary standards. Data at rest (virtual filesystems, relational databases, and object storage) is encrypted using GCP's industry-standard AES-256, while data in transit is encrypted with TLS ≥ 1.2 (for Encore's REST API) or WireGuard (for internal communication).

All customer secret information is further encrypted using GCP's Key Management Service (KMS). Any access to encrypted data by Encore employees requires elevated access and approval by multiple parties, and all such activity is audited. Elevated access to production systems is granted on a time-limited basis through GCP's Privileged Access Manager (PAM), which provides an audit trail of every request, its purpose, and its duration. Production database access does not expose cryptographic keys or secrets, which remain secured in GCP Cloud KMS.

User account authentication is provided by _Clerk_, a SOC 2 compliant vendor.

There are two ways for a user to log in to Encore: Single sign-on (SSO) and username plus password. Single sign-on can be used by organizations to fully manage access to Encore and, for example, ensure that former employees no longer have access after the offboarding period. Encore supports Google and GitHub SSO using OAuth.

If no SSO is used, the default login method is passwordless login using email and "magic link", also handled by _Clerk_. Encore does not store or in any way handle passwords, neither in plaintext nor cryptographic hash form. This means that Encore does not know the passwords of any users, and no passwords can be reconstructed from our databases.

Encore offers bug bounty incentives to individuals who discover any security discrepancies. The objective of offering bug bounty incentives is to receive security-related bug reports from trusted "white hat hackers" before the vulnerability is actively exploited in a malicious way. This contributes to maintaining Encore's product security.

All security issues undergo a triaging process by Encore's designated Security Officer and are escalated based on their criticality.

Encore uses automated scans to detect software vulnerabilities. All teams are continuously monitoring their services for vulnerabilities and are committed to pro-actively reducing them. The progress is supervised by the Security Officer.

Our Vulnerability Management Policy defines how findings are identified, prioritized based on actual risk (not scanner severity alone), and tracked to a clear outcome. Material findings are reviewed by a human, and each is fixed, mitigated, determined not applicable, deferred with a documented rationale, or accepted as a known risk. We target the following remediation timelines:

| Severity | Remediation target |
| --- | --- |
| Critical | 7 days, or faster if actively exploited |
| High | 30 days |
| Medium | 90 days |
| Low | Best effort / backlog |

These targets are guided by judgment: a critical issue under active exploitation may warrant same-day mitigation, while a high-severity finding in unused code may be downgraded or closed as not applicable.

### Access control

We regularly keep track of and review the list of employees who have access to which systems and remove access where applicable to ensure least access principles apply.

Offboarding processes ensure that former employees cannot access internal systems anymore after the termination of their contract. Thanks to the VPN, Encore can centrally restrict access to internal networks.

#### MFA

Multi-factor authentication (MFA) adds another layer of security on top of classic password authentication. In addition to username and password, the user requires another individual token of access.

Stealing or guessing the password is not enough for an attacker to gain access to a system, because the second factor would also need to be stolen.

Usually, the second factor is a physical device, such as a mobile phone which has been paired with the authentication system. Encore employs MFA to protect access to the infrastructure provider (GCP) and the version control systems (GitHub), among other systems.

Employee access to internal systems is governed by Google Workspace SSO, configured to require mandatory MFA for all employees and to enforce strong password requirements. SMS-based factors are disabled in favour of stronger methods due to SIM-swap and spoofing risks. Server (SSH) access is available only over the Tailscale VPN, and obtaining `sudo` or root access requires re-authenticating with our SSO provider. Employee endpoints are used for development only and do not have standing access to sensitive production systems; elevated access requires Tailscale or time-limited elevation through GCP Privileged Access Manager.

### Vendor and third-party risk management

Encore relies on third-party services to build and operate its platform, and we manage the risk they introduce through a formal Vendor Management Policy. We maintain an inventory of vendors used for production systems, customer support, engineering, security, and other business-critical functions. Each vendor has a designated internal owner responsible for ensuring it is reviewed, used for its intended purpose, and offboarded when no longer needed.

Vendors are reviewed in proportion to the risk they introduce, rather than with a single one-size-fits-all process. A vendor is classified as **Critical** when its failure or compromise could materially affect our ability to serve customers, protect customer data, or operate production systems — for example cloud infrastructure providers, identity providers, and platforms with administrative access to production. **High-risk** vendors have sensitive access or data but are not essential to running the service, and **Medium** and **Low-risk** vendors have progressively more limited access.

Before adopting a Critical or High-risk vendor, we assess what data and systems it can access, what happens if it is breached or unavailable, and whether it provides appropriate security evidence such as a SOC 2 report, ISO 27001 certificate, or penetration test summary. Data protection terms (such as a DPA) are put in place where personal data is processed, access is limited to the minimum necessary, and SSO with MFA is required for administrative access. Critical and High-risk vendors are reviewed at least annually, and vendor access is removed and documented on offboarding. All of Encore's current critical vendors (including GCP, Tailscale, Clerk, GitHub, Sentry, and Grafana) are SOC 2 and/or ISO 27001 certified.

## Availability

Hosted on a cloud infrastructure, Encore implements a service-based architecture where many dedicated software components operate isolated from one another, but in a coordinated way, much like a complex machine where individual parts can be replaced independently from one another.

During the release of a new version of Encore services, Encore's engineers take great care during the preparation of the update so that in case of an unexpected problem, the system can be restored to the previous state in a manner that minimizes user impact.

### Performance monitoring

Encore uses a number of performance monitoring systems, such as Sentry, Cronitor, Grafana, and Google Cloud Monitoring (all being SOC 2 compliant vendors). Grafana is used to monitor application performance, such as server response times and user interface speed. Grafana also collects server-side metrics like CPU and RAM usage. Additionally, Encore monitors the performance of databases with GCP tooling.

Slack, a SOC 2 compliant vendor, is used as the alerting channel to notify the developers in case the performance of the system has regressed, for example, due to increased response times, or increased error rates. To enable root cause analysis of bugs, Encore collects system logs from all parts of the system. These logs can only be accessed by authorized users.

Encore offers a public "Status page" where users and customers can find the current status of Encore systems. It is available at: [https://status.encore.dev/](https://status.encore.dev/).

### Backups and disaster recovery

To reduce the risk of simultaneous failure, Encore backs up data to multiple US regions in GCP, with very limited access. Relational databases are backed up on a daily schedule.

Encore will conduct its first full disaster recovery exercise in Q3 2026, cloning the production environment from backups end-to-end and validating its soundness. The exercise covers restoring the primary relational databases and object storage, end-to-end validation against our documented recovery objectives (an 8-hour Recovery Time Objective and a 24-hour Recovery Point Objective), and a tabletop walk-through of our business continuity plan for personnel and communications. DR testing will then run at least annually, plus on any material architecture change. Outcomes and remediation items are recorded and available for SOC 2 audit and, under NDA, for customer review.

### Incident handling

Whenever an incident occurs, Encore's designated on-call engineer initiates an investigation and escalates to the broader engineering team as necessary based on severity. Our Incident Response Policy defines escalation paths and a severity model (Low, Medium, High, and Critical) that distinguishes unverified suspicions from confirmed, actively exploited risks. For issues deemed critical, the response team follows an iterative process: a war room is convened, a breach timeline and indicators of compromise are maintained, emergency and long-term mitigations are tracked, and the incident is driven to a documented post-mortem. High-severity incidents require a retrospective. Critical incidents escalate directly to the CEO, CTO, and COO, and engage legal and PR where breach notification may be required.

Customers and users can report outages via regular support channels (for example via email, or using the [Discord](https://encore.dev/discord) chat group). Encore's internal communication systems have dedicated channels for incident escalation, with out-of-band communication arrangements in case primary channels are compromised.

## Confidentiality

When you use Encore, other users won't be able to see your content, unless you grant access explicitly by inviting them to your application. Encore engineers may use your data to provide support and when necessary to fix bugs.

### Data classification

Our Data Classification Policy defines four sensitivity levels, each with corresponding handling requirements:

- **Public:** information approved for public release, such as published documentation and marketing content.
- **Internal:** information intended for company use that is not harmful if disclosed in small amounts.
- **Confidential:** information that could harm customers, the company, employees, or partners if disclosed. Customer production data, private source code, and support tickets default to Confidential. Access requires a business need, is protected with SSO, MFA, and role-based access, and is encrypted in transit and at rest.
- **Restricted:** information where disclosure could cause serious harm, such as production secrets, tokens, private keys, and credentials. Access is limited to specific authorized people or systems, stored only in approved secret managers, and rotated if exposure is suspected.

Access to internal support tooling is curated so that it does not expose Confidential customer data, is available only over the Tailscale VPN with SSO authentication, and logs all actions for audit. Access to Confidential and Restricted data is granted on a business-need basis and removed promptly on role change, offboarding, or when the need ends.

### Access controls

All employees and contractors are contractually bound to confidentiality, which persists after the termination of the work contract.

As part of a "clean screen" policy, all computers used by Encore staff must be set to automatically lock the screen after 1 minute of inactivity.

All systems access is subject to the "principle of least privilege", meaning that every employee only has access to the systems necessary to perform their official duties.

### Deletion

User data will be stored by Encore after the termination of a subscription term, according to [Encore's Terms of Service](/legal/terms). When a user requests the deletion of data, the data is made inaccessible or physically deleted, depending on the data type and storage location.  For technical reasons, data may remain in backups after this point.

## Processing integrity

### Quality assurance

Product quality is very important to Encore. There are many different facets, including:

-   Accuracy and usability of services provided

-   High performance of the user interface and Encore services

-   Almost zero downtime

Several measures are put into place in order to keep product quality high:

-   Code review: Code changes are reviewed by a peer of the developer before it is accepted into the main code branch (for critical systems) or in a weekly post-hoc review process (for auxiliary systems). For critical systems code can only be merged if the reviewer agrees. For auxiliary systems any requested changes by reviewers are made as part of the review process. It is often necessary to add a test alongside, and the code review process ensures that this has been done as well.

-   Continuous integration: Before code is accepted, it is built by our continuous integration environment and tests are executed. If the build fails, the developer is notified immediately and a fix is required before the code can be merged.

-   Manual testing: Once the code has been merged, the change is deployed and in most cases tested manually post-release to verify quality in the production environment.

-   Automated integration testing: A large battery of automated tests is executed against the local and production environments and checks many common workflows for regressions of any kind. In case the tests fail, the engineer will address the issues before proceeding with attempting to merge again.

-   Testing of Open-Source libraries: Encore uses Open-Source libraries to provide certain functionality. Overnight tests run daily to discover potential issues, and manual testing is performed when any Open-Source libraries are version updated.

Any code change is released only if all these steps succeed. Furthermore, access to the code base is protected via multi-factor authentication (MFA), which poses another layer of defense against the malicious injection of code.

These practices are governed by our Change Management Policy, which requires that production changes go through GitHub pull requests, are reviewed by someone other than the author (segregation of duties enforced via protected branches), pass required automated checks before merge, and are deployed through approved CI/CD workflows. The pull request, review history, CI results, and deployment record together form the audit trail. Higher-risk changes — such as those affecting authentication, cryptography, secrets, billing, network exposure, or destructive database migrations — require additional approval from an appropriate owner and, where applicable, a documented rollback or recovery plan. Emergency changes may be expedited to contain an incident but must be documented and reviewed afterward.

Since Encore depends on third-party software, we regularly contribute to the quality assurance of our suppliers. Whenever Encore becomes aware of regressions or bugs, they are reported upstream. In this way, Encore is contributing to the quality, stability, and accuracy of other software in the space.

### Process monitoring

Where possible, Encore uses software to enforce processes. For example, code review and having tests passed are enforced by the source control management tool GitHub.

Regular reviews on different levels (individual, team, company) foster alignment between all individuals and the company objectives.

### Privacy

Encore takes data privacy very seriously and complies with the rules of the European Union's GDPR (General Data Protection Regulation). GDPR grants a wide range of rights to Encore's users, such as the right to be informed, the right to access, the right to rectification, the right to erasure, and others.

One fundamental rule of the GDPR is the principle of "data minimization", which ensures that we are not processing more personal data than necessary. As a result, Encore Cloud uses only minimal personal data for user authentication and essential communication (a name and contact email). As described above, Encore does not store or handle passwords in any form.

### Privacy policy

We are aware that confidential handling of your data is essential to establishing trust. Therefore, [Encore's Privacy Policy](/legal/privacy) ensures that the data of our users is protected according to the high standards of GDPR.

## Questions and further information

We are committed to transparency in our security practices. If you have questions about our security or compliance posture, or would like to request additional documentation for your vendor review process, please contact us at [hello@encore.dev](mailto:hello@encore.dev).
