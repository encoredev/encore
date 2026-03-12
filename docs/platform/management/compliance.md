---
seotitle: Security & Compliance
seodesc: Learn about Encore's security practices, infrastructure protections, and compliance posture — built on industry-standard controls and trusted cloud providers.
title: Security & Compliance
subtitle: How Encore protects your applications, code, and data
lang: platform
---

_Last updated: March 3, 2026_

Your applications, code, and data are among your most important assets. Security is foundational to everything we build at Encore — it is embedded in our architecture, our processes, and our culture. This document provides a comprehensive overview of the security controls and practices we have in place today, structured around the SOC 2 trust service criteria.

### Security at a glance

| Area | What we do |
| --- | --- |
| **Infrastructure** | Hosted on GCP (ISO 27001 / SOC 2 certified). All servers are private, accessible only via VPN. |
| **Encryption** | AES-256 at rest, TLS 1.2+ and WireGuard in transit. Customer secrets additionally encrypted via GCP KMS. |
| **Zero-trust networking** | All server-to-server communication is authenticated and end-to-end encrypted via Tailscale / WireGuard. |
| **Access control** | Principle of least privilege, MFA enforced, regular access reviews, VPN-only infrastructure access. |
| **Authentication** | Managed by Clerk (SOC 2 certified). Passwordless by default — Encore never stores or handles user passwords. |
| **Monitoring & alerting** | 24/7 monitoring via Grafana, Sentry, Cronitor, and GCP Cloud Monitoring (all SOC 2 certified). |
| **Vendor security** | All critical vendors are SOC 2 and/or ISO 27001 certified (GCP, Tailscale, Clerk, GitHub, Sentry, Grafana). |
| **Code quality** | Mandatory code review, CI/CD with automated testing, automated vulnerability scanning. |
| **Data privacy** | GDPR compliant. Data minimization by design. |
| **Responsible disclosure** | Active bug bounty program for security researchers. |

## SOC 2

SOC is short for "System and Organization Controls" — it is the de facto industry standard for software security and privacy. We have implemented controls aligned with the SOC 2 framework and are currently preparing for a formal SOC 2 Type 1 audit, during which an external auditor will verify that our controls meet the standard.

After the Type 1 audit, we plan to proceed to Type 2, which involves continuous monitoring over an extended period.

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

An organization is only as strong as its people. All employees undergo a rigorous selection process, and many of Encore's team members bring extensive experience from regulated environments such as online banking and large-scale payment systems.

Employees are required to complete annual security awareness training covering physical security, digital hygiene (strong passwords, two-factor authentication), social engineering ("phishing"), and related topics. Individual performance is reviewed on a bi-weekly cadence, and organizational performance is tracked via KPIs reviewed monthly by management.

Encore employment policy mandates full-disk encryption on all employee devices.

### Product security

Multiple layers of protection ensure that customer data is not accessible to unauthorized persons.

Encore's service-based architecture provides natural isolation between components, and we have adopted a zero-trust security model with Tailscale. All server-to-server communication is authenticated and end-to-end encrypted with WireGuard. GCP's VPC (Virtual Private Cloud) provides another layer of isolation from the internet on the network level. None of Encore's servers are publicly accessible on the internet.

As a general principle, all of Encore's data is encrypted while being transported across networks and when stored ("in transit and at rest"). In case of unauthorized access to the data, an attacker would only see undecipherable garbage which cannot be decrypted without the corresponding keys. The encryption methods employed by Encore are industry standard and deemed unbreakable by contemporary standards. Data at rest (virtual filesystems, relational databases, and object storage) is encrypted using GCP's industry-standard AES-256, while data in transit is encrypted with TLS ≥ 1.2 (for Encore's REST API) or WireGuard (for internal communication).

All customer secret information is further encrypted using GCP's Key Management Service (KMS). Any access to encrypted data by Encore employees requires elevated access and approval by multiple parties, and all such activity is audited.

User account authentication is provided by _Clerk_, a SOC 2 compliant vendor.

There are two ways for a user to log in to Encore: Single sign-on (SSO) and username plus password. Single sign-on can be used by organizations to fully manage access to Encore and, for example, ensure that former employees no longer have access after the offboarding period. Encore supports Google and GitHub SSO using OAuth.

If no SSO is used, the default login method is passwordless login using email and "magic link", also handled by _Clerk_. Encore does not store or in any way handle passwords, neither in plaintext nor cryptographic hash form. This means that Encore does not know the passwords of any users, and no passwords can be reconstructed from our databases.

Encore uses automated vulnerability scanning across its codebase and dependencies. All teams continuously monitor their services for vulnerabilities and proactively remediate them, supervised by the Security Officer.

All security issues undergo a triaging process by the Security Officer and are escalated based on criticality.

### Responsible disclosure

We maintain an active bug bounty program to encourage security researchers to report vulnerabilities before they can be exploited. If you discover a security issue, please report it to [security@encore.dev](mailto:security@encore.dev). We are committed to investigating all reports promptly and working with researchers to resolve issues responsibly.

### Access control

We regularly keep track of and review the list of employees who have access to which systems and remove access where applicable to ensure least access principles apply.

Offboarding processes ensure that former employees cannot access internal systems anymore after the termination of their contract. Thanks to the VPN, Encore can centrally restrict access to internal networks.

#### MFA

Multi-factor authentication (MFA) adds another layer of security on top of classic password authentication. In addition to username and password, the user requires another individual token of access.

Stealing or guessing the password is not enough for an attacker to gain access to a system, because the second factor would also need to be stolen.

Usually, the second factor is a physical device, such as a mobile phone which has been paired with the authentication system. Encore employs MFA to protect access to the infrastructure provider (GCP) and the version control systems (GitHub), among other systems.

## Availability

Hosted on a cloud infrastructure, Encore implements a service-based architecture where many dedicated software components operate isolated from one another, but in a coordinated way, much like a complex machine where individual parts can be replaced independently from one another.

During the release of a new version of Encore services, Encore's engineers take great care during the preparation of the update so that in case of an unexpected problem, the system can be restored to the previous state in a manner that minimizes user impact.

### Performance monitoring

Encore uses a number of performance monitoring systems, such as Sentry, Cronitor, Grafana, and Google Cloud Monitoring (all being SOC 2 compliant vendors). Grafana is used to monitor application performance, such as server response times and user interface speed. Grafana also collects server-side metrics like CPU and RAM usage. Additionally, Encore monitors the performance of databases with GCP tooling.

Slack, a SOC 2 compliant vendor, is used as the alerting channel to notify the developers in case the performance of the system has regressed, for example, due to increased response times, or increased error rates. To enable root cause analysis of bugs, Encore collects system logs from all parts of the system. These logs can only be accessed by authorized users.

Encore offers a public "Status page" where users and customers can find the current status of Encore systems. It is available at: [https://status.encore.dev/](https://status.encore.dev/).

### Backups and disaster recovery

To reduce the risk of simultaneous failure, Encore backs up data to multiple US regions in GCP, with very limited access. Relational databases are backed up on a daily schedule.

Encore is currently planning a rehearsal of disaster recovery in Q4 of 2026. In this exercise, a clone of the production environment will be recovered from scratch using backups and tested for soundness.

### Incident handling

Whenever an incident occurs, Encore's designated on-call engineer initiates an investigation and escalates to the broader engineering team as necessary based on severity. For issues deemed critical, they follow an iterative response process to identify and contain errors, recover systems and services, and remediate vulnerabilities.

Customers and users can report outages via regular support channels (for example via email, or using the [Discord](https://encore.dev/discord) chat group). Encore's internal communication systems have dedicated channels for incident escalation.

## Confidentiality

When you use Encore, other users won't be able to see your content, unless you grant access explicitly by inviting them to your application. Encore engineers may use your data to provide support and when necessary to fix bugs.

### Access controls

All employees and contractors are contractually bound to confidentiality, which persists after the termination of the work contract.

As part of a "clean screen" policy, all computers used by Encore staff must be set to automatically lock the screen after 1 minute of inactivity.

All systems access is subject to the "principle of least privilege", meaning that every employee only has access to the systems necessary to perform their official duties.

### Deletion

User data will be stored by Encore after the termination of a subscription term, according to [Encore's Terms of Service](https://encore.cloud/legal/terms). When a user requests the deletion of data, the data is made inaccessible or physically deleted, depending on the data type and storage location.  For technical reasons, data may remain in backups after this point.

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

Since Encore depends on third-party software, we regularly contribute to the quality assurance of our suppliers. Whenever Encore becomes aware of regressions or bugs, they are reported upstream. In this way, Encore is contributing to the quality, stability, and accuracy of other software in the space.

### Process monitoring

Where possible, Encore uses software to enforce processes. For example, code review and having tests passed are enforced by the source control management tool GitHub.

Regular reviews on different levels (individual, team, company) foster alignment between all individuals and the company objectives.

### Privacy

Encore takes data privacy very seriously and complies with the rules of the European Union's GDPR (General Data Protection Regulation). GDPR grants a wide range of rights to Encore's users, such as the right to be informed, the right to access, the right to rectification, the right to erasure, and others.

One fundamental rule of the GDPR is the principle of "data minimization", which ensures that we are not processing more personal data than necessary. As a result, Encore Cloud uses only minimal personal data for user authentication and essential communication (a name and contact email). As described above, Encore does not store or handle passwords in any form.

### Privacy policy

We are aware that confidential handling of your data is essential to establishing trust. Therefore, [Encore's Privacy Policy](https://encore.cloud/legal/privacy) ensures that the data of our users is protected according to the high standards of GDPR.

## Questions and further information

We are committed to transparency in our security practices. If you have questions about our security or compliance posture, or would like to request additional documentation for your vendor review process, please contact us at [hello@encore.dev](mailto:hello@encore.dev).
