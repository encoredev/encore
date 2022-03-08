---
title: Custom Domains
subtitle: Bring more than just your own cloud
---

All application [environments](/docs/deploy/environments) are accessible as subdomains of a shared Encore domain
([encr.app](https://encr.app)). However, when creating API's which you intend to make publicly exposed you may want to
provide a branded URL endpoint to your end users. Encore allows you to easily configure your own domains to serve your
Encore powered backend.

## Adding a Domain

To add your own domain, you will need to modify the DNS records for the domain and add a CNAME record to it pointing at:
`custom-domain.encr.app`. We recommend setting a TTL (Time-To-Live) of 30 minutes for the CNAME record.


<Callout type="warning">

Do not create a CNAME record using a wildcard (e.g. `*.example.com`), instead we require you to explicitly set up a
CNAME record for each domain you wish to serve traffic from. 

</Callout>

Once you've added the CNAME record, head over to the Custom Domains settings page by going to
**[Your apps](https://app.encore.dev/) > (Select your app) > App Settings > App > Custom Domains**. Click on `Add Domain`
on the top right of the page.

Enter the domain name you configured the CNAME on and select which [environment](/docs/deploy/environments) you wish to
serve on that domain, then click `Add`.

At this point Encores platform will start the process of setting up your domain & issuing SSL certificates to serve the
traffic through.

<Callout type="important">

Encore allows you to have multiple domains configured against a single environment and will serve traffic through all
configured domains. The `encr.app` subdomain which was created when you originally created an environment will always be
configured to serve traffic to that environment, this allows you to migrate to a custom domain safety without risking
cutting traffic off to older clients which may be hard coded to access your applications via the default subdomain.

</Callout>

## Domain Statuses

On the Custom Domains settings page Encore will list various statuses throughout the lifecycle of a custom domain.

- `Pending`; This custom domain is currently queued to be provisioned by the Encore platform.
- `Waiting for CNAME`; The Encore platform is waiting for the CNAME to become active and for the SSL certificate to be issued for the custom domain.
- `Configuring Edge Routers`; The SSL certificate has been issued and the Encore edge routers need to be configured to route traffic on this domain.
- `Active`; This custom domain serving traffic to your Encore application
- `Not Working`; A non-recoverable problem has occurred on your custom domain. This could be a result of the CNAME record
   being removed or pointed else where. If you see this error, please contact support.
