---
seotitle: Compliance & Security
seodesc: Encore is designed to help you build secure, scalabl, applications. We take the security and reliability of your application very seriously.
title: Compliance & Security
subtitle: Encore SOC 2 Self-assessment
---

_Last updated: 16 May, 2023_

As an organization or engineer who creates applications, your applications, code, and data are among your most important assets. Encore highly prioritizes the security of these assets, allowing you to concentrate on your goal: designing exceptional applications.

We are arranging for an external review of our security measures and thus, we are preparing for a SOC 2 Type 1 audit. This document presents a summary of our self-evaluation of the current implementation of the SOC 2 trust service criteria at Encore.

## SOC 2

SOC is short for "System and Organization Controls" --- it is the de facto industry standard for software security and privacy. During the SOC 2 audit, an external auditor will carry out an extensive review of our processes (e.g. employee onboarding and offboarding, access review, various policies, disaster recovery exercises, software architecture, physical access, etc.) and ensure that they meet the mark.

The Type 1 audit is a point-in-time audit where the auditor verifies that the controls are satisfied at a specific point in time. We are planning to proceed to Type 2 afterward which is based on continuous monitoring during time periods of varying lengths.

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

Encore believes that the best way to achieve a secure system is to follow best practices and industry standards, and not with obscurity (i.e. attempting to "secure" a system only by making it "difficult to understand") or homegrown technology (e.g. custom encryption algorithms).

We have a designated individual (Security Officer) responsible for all aspects of security, such as infrastructure, software, and data.

### Infrastructure security

Encore's core production infrastructure is hosted on GCP (Google Cloud Platform), an ISO27001/SOC 2 compliant vendor. Auxiliary services are provided by Hetzner, an ISO27001 compliant vendor. Tailscale, a SOC 2 compliant vendor, provider VPN (Virtual Private Network)  services used to provide secure communication between all servers.

All core data processing is carried out in the US East region (us-east-1), and backups are kept in multiple separate regions in the US. Each region is composed of at least three "availability zones" (AZs) which are isolated locations, designed to take over in case of a catastrophic failure at one location. AZs are separated by a significant distance such that it is unlikely that they are affected by the same issues such as power outages, earthquakes, etc. Physical access to GCP is restricted by GCP's security controls. Furthermore, GCP monitors and immediately responds to power, temperature, fire, water leaks, etc.

Access to Encore's production infrastructure is restricted to Encore employees. All systems have controlled access and only a limited number of employees have privileged access. Access is only possible through a VPN over Tailscale.

The production environment is separated from testing environments, using separate accounts and VPCs (Virtual Private Cloud) in GCP. This ensures that any defect in a test environment cannot impact the production system. The connection to the internet is controlled by dedicated gateways.

### Organizational security

Since an organization is only as good as its people, Encore takes great care when selecting and training its staff. All employees undergo a thorough selection process that has been designed to identify the best talent in the world for the job. Many of Encore's employees have extensive experience working in regulated environments such as Online Banking and large-scale Online Payments.

Individual performance monitoring is carried out by managers on a bi-weekly cadence. Overall organizational performance is tracked continuously and reviewed by management on a monthly cadence using Key Performance Indicators determined by management.

Employees are required to complete yearly security awareness training. The training is designed to increase sensitivity to physical security (hardware and media handling, office access control, etc.), digital security (e.g. secure passwords, two-factor authentication), social engineering attacks ("phishing"), and other security-related topics.

Encore employment policy mandates that all hard drives must be encrypted.

### Product security

Encore is aware of how important it is to its customers that all data is handled securely. Therefore, several layers of protection ensure that the data is not accessible to unauthorized persons.

An essential part of software security is "defense in depth" which means that there are multiple layers of protection. In case one layer is breached, the next layer helps to contain the breach and mitigate its consequences. This can be achieved by isolating software components from each other, such that the breach of one component does not affect adjacent software.

Encore's service-based architecture provides natural isolation between components, and we have adopted a zero-trust security model with the use of Tailscale. All server-to-server communication is authenticated and end-to-end encrypted with WireGuard. GCP's VPC (Virtual Private Cloud) provides another layer of isolation from the internet on the network level. None of Encore's servers are publicly accessible on the internet.

As a general principle, all of Encore's data is encrypted while being transported across networks and when stored ("in transit and at rest"). In case of unauthorized access to the data, an attacker would only see undecipherable garbage which cannot be decrypted without the corresponding keys. The encryption methods employed by Encore are industry standard and deemed unbreakable by contemporary standards. Data at rest (virtual filesystems, relational databases, and object storage) is encrypted using GCP's industry-standard AES-256, while data in transit is encrypted with TLS ≥ 1.2 (for Encore's REST API) or WireGuard (for internal communication).

All customer secret information is further encrypted using GCP's Key Management Service (KMS). Any access to encrypted data by Encore employees requires elevated access and approval by multiple parties, and all such activity is audited.

User account authentication is provided by Auth0, a SOC 2 compliant vendor.

There are two ways for a user to log in to Encore: Single sign-on (SSO) and username plus password. Single sign-on can be used by organizations to fully manage access to Encore and, for example, ensure that former employees no longer have access after the offboarding period. Encore supports Google and GitHub SSO using OAuth.

If no SSO is used, the default login method is username and password, also handled by Auth0. Encore does not store or in any way handle passwords, neither in plaintext nor cryptographic hash form. This means that Encore does not know the passwords of any users, and no passwords can be reconstructed from our databases.

Encore offers bug bounty incentives to individuals who discover any security discrepancies. The objective of offering bug bounty incentives is to receive security-related bug reports from trusted "white hat hackers" before the vulnerability is actively exploited in a malicious way. This contributes to maintaining Encore's product security.

All security issues undergo a triaging process by Encore's designated Security Officer and are escalated based on their criticality.

Encore uses automated scans to detect software vulnerabilities. All teams are continuously monitoring their services for vulnerabilities and are committed to pro-actively reducing them. The progress is supervised by the Security Officer.

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

Encore uses a number of performance monitoring systems, such as Sentry, Cronitor, Grafana, and Google Cloud Monitoring. Grafana, a SOC 2 compliant vendor, is used to monitor application performance, such as server response times and user interface speed. Grafana also collects server-side metrics like CPU and RAM usage. Additionally, Encore monitors the performance of databases with GCP tooling.

Slack, a SOC 2 compliant vendor, is used as the alerting channel to notify the developers in case the performance of the system has regressed, for example, due to increased response times, or increased error rates. To enable the root cause analysis of bugs, Encore collects system logs from all parts of the system. These logs can only be accessed by authorized users.

Encore is planning to offer a public status page as part of the SOC 2 certification

process.

### Backups and disaster recovery

To reduce the risk of simultaneous failure, Encore backs up data to multiple US regions in GCP, with very limited access. Relational databases are backed up on a daily schedule.

Encore is currently planning a rehearsal of disaster recovery in Q4 of 2023. In this exercise, a clone of the production environment will be recovered from scratch using backups and tested for soundness.

### Incident handling

Whenever an incident occurs, Encore's designated on-call engineer initiates an investigation and escalates to the broader engineering team as necessary based on severity. For issues deemed critical, they follow an iterative response process to identify and contain errors, recover systems and services, and remediate vulnerabilities.

Customers and users can report outages via regular support channels (for example via email, or using the Slack chat group). Encore's internal communication systems have dedicated channels for incident escalation.

## Confidentiality

When you use Encore, other users won't be able to see your content, unless you grant access explicitly by inviting them to your application. Encore engineers may use your data to provide support and when necessary to fix bugs.

### Access controls

All employees and contractors are contractually bound to confidentiality, which persists after the termination of the work contract.

As part of a "clean screen" policy, all computers used by Encore staff must be set to automatically lock the screen after 1 minute of inactivity.

All systems access is subject to the "principle of least privilege", meaning that every employee only has access to the systems necessary to perform their official duties.

### Deletion

User data will be stored by Encore after the termination of a subscription term, according to [Encore's Terms of Service](https://encore.dev/legal/terms). When a user requests the deletion of data, the data is made inaccessible or physically deleted, depending on the data type and storage location.  For technical reasons, data may remain in backups after this point.

## Processing integrity

### Quality assurance

Product quality is very important to Encore. There are many different facets, including:

-   Accuracy and usability of services provided

-   High performance of the user interface and Encore services

-   Almost zero downtime

Several measures are put into place in order to keep product quality high:

-   Code review: Every single code change is reviewed by a peer of the developer before it is accepted into the main code branch (for critical systems) or in a weekly post-hoc review process (for auxiliary systems). For critical systems code can only be merged if the reviewer agrees. For auxiliary systems any requested changes by reviewers are made as part of the review process. It is often necessary to add a test alongside, and the code review process ensures that this has been done as well.

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

One fundamental rule of the GDPR is the principle of "data minimization", which ensures that we are not processing more personal data than necessary. As a result, the Encore platform uses only minimal personal data for user authentication and essential communication (which is a name, contact email, and a password hash).

### Privacy policy

We are aware that confidential handling of your data is essential to establishing trust. Therefore, [Encore's Privacy Policy](https://encore.dev/legal/privacy) ensures that the data of our users is protected according to the high standards of GDPR.

## Questions and clarifications

If you have any questions regarding Encore's security or compliance strategy, please feel free to contact us [via email](mailto:hello@encore.dev) (hello@encore.dev).