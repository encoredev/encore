---
seotitle: Encore Cloud API Reference
seodesc: Learn how to use the Encore Cloud API.
title: Encore Cloud API Reference
lang: platform
---

Encore Cloud provides an API for programmatic access to control certain parts of the platform.

We're working on expanding the set of features available over the API.
Please reach out to us [on Discord](https://encore.dev/discord) if you have use cases where additional API functionality would be useful.

The Base URL for the Encore Cloud API is `https://api.encore.cloud`.

## Authentication

All API calls require valid authentication, which is provided by sending an access token in the `Authorization` header,
in the format `Authorization: Bearer ${ACCESS_TOKEN}`.

You can retrieve an API access token from the OAuth Token endpoint, using an OAuth Client.
An API access token expires after one hour. For continuous access, shortly before an API access token expires, request a new API access token from Encore Cloud's OAuth token endpoint.

OAuth client libraries in popular programming languages can handle the API access token generation and renewal.

See the [OAuth Clients](/docs/platform/integrations/oauth-clients) for more information on creating OAuth Clients.

## OAuth

**Method**: `POST` <br/>
**Path**: `/api/oauth/token`

#### Query Parameters

| Parameter         | Description                                                    |
| ----------------- | -------------------------------------------------------------- |
| **client_id**     | The client id of the OAuth Client to generate a token for.     |
| **client_secret** | The client secret of the OAuth Client to generate a token for. |

#### Response

The API responds with a 2xx status code on successful creation of an API access token.

```typescript
type Token = {
  // The access token itself.
  "access_token": string;

  // The access token expires after 1 hour (3600 seconds).
  "expires_in": 3600;

  // The actor the token belongs to, in this case the OAuth2 client id.
  actor: string;


  // Indicates the access token should be passed as a "Bearer" token in the Authorization header.
  "token_type": "Bearer";
}
```

## Rollouts

Encore Cloud's deployment system consists of several phases:

* A build phase
* An infrastructure provisioning phase
* A deployment phase

These phases are combined into a unified entity called a *Rollout*.
A rollout represents the coordinated process of rolling out a specific version of an Encore application.

We use the term *rollout* to disambiguate from the *deployment phase*, which specifically
refers to the last phase of the rollout process (where the version is being deployed onto the provisioned infrastructure).

### The Rollout Object

The Rollout object represents the state of a rollout.

```typescript
// The representation of a rollout.
type Rollout = {
  // Unique id of the rollout.
  id: string;

  // The current status of the rollout.
  status: "pending" | "queued" | "running" | "completed";

  // What the conclusion was of the rollout (when status is "completed").
  // If the status is not "completed" the conclusion is "pending".
  conclusion: "pending" | "canceled" | "failure" | "success";

  // When the rollout was queued, started, and completed.
  queued_at: Date | null;
  started_at: Date | null;
  completed_at: Date | null;

  // Information about the various rollout phases.
  // See type definitions below.
  build: RolloutPhase<BuildStatus, BuildConclusion>;
  infra: RolloutPhase<InfraStatus, InfraConclusion>;
  deploy: RolloutPhase<DeployStatus, DeployConclusion>;
}

// Common structure of the various rollout phases.
type RolloutPhase<Status, Conclusion> = {
  // Unique id of the phase.
  id: string;

  // The current status of the rollout phase.
  status: Status;

  // What the conclusion was of the phase.
  conclusion: Conclusion;

  // When the phase was queued, started, and completed.
  queued_at: Date | null;
  started_at: Date | null;
  completed_at: Date | null;
}

// The current status and conclusion of a build.
// If the status is not "completed" the conclusion is "unknown".
type BuildStatus = "queued" | "running" | "completed";
type BuildConclusion = "unknown" | "canceled" | "failure" | "success";

// The current status and conclusion of an infra change.
// The "proposed" status means the change is awaiting human approval.
// The "rejected" conclusion means a human rejected the proposed infra change.
type InfraStatus = "pending" | "proposed" | "queued" | "running" | "completed";
type InfraConclusion = "unknown" | "canceled" | "failure" | "rejected" | "success";

// The current status and conclusion of a deploy.
// If the status is not "completed" the conclusion is "unknown".
type DeployStatus = "queued" | "running" | "completed";
type DeployConclusion = "unknown" | "canceled" | "failure" | "success";
```

### Triggering a rollout

**Method**: `POST` <br/>
**Path**: `/api/apps/${APP_ID}/envs/${ENV_NAME}/rollouts`

#### Path Parameters

| Parameter    | Description                                                |
| ------------ | ---------------------------------------------------------- |
| **APP_ID**   | The id of the Encore application to trigger a rollout for. |
| **ENV_NAME** | The name of the environment to trigger a rollout for.      |

#### JSON Request Body
A rollout can be triggered either with a commit sha or the name of a branch,
depending on the JSON field passed in. Exactly one of these must be provided.

```typescript
{
  // The commit hash to trigger a deploy for.
  "sha": string;
} | {
  // Name of the branch to trigger a deploy from.
  "branch": string;
}
```

#### Response

The API responds with a 2xx status code on successful creation of a new rollout.

On success it returns a **Rollout** object as its JSON response payload,
representing the current state of the newly created rollout.

## Member Management

Encore Cloud provides APIs for managing application members, including inviting users, listing members, and updating member roles.

### Member Object

```typescript
type Member = {
  // The user's email address
  email: string;

  // The member's role in the application
  role: "owner" | "reader" | "writer" | "none";

  // When the member was invited to the application
  invited: timestamp;

  // When the member accepted the invitation
  accepted: timestamp;

  // When the membership expires
  expires: timestamp;

  // The member's username
  username: string;

  // The member's full name
  full_name: string;

  // The member's picture URL
  picture_url: string;
}
```

### Available Roles

- **owner**: Full control over the application
- **writer**: Can write application resources
- **reader**: Can read application resources
- **none**: Used to revoke access to an application

### Invite Member

Invite a new member to an Encore application.

**Method**: `POST` <br/>
**Path**: `/api/apps/${APP_ID}/member`

#### Path Parameters

| Parameter  | Description                                    |
| ---------- | ---------------------------------------------- |
| **APP_ID** | The id of the Encore application.             |

#### JSON Request Body

```typescript
{
  // The email address of the user to invite
  "email": string;

  // The role to assign to the invited member
  "role": "owner" | "reader" | "writer" | "none";
}
```

#### Response

The API responds with a 2xx status code on successful invitation.

On success it returns a **Member** object as its JSON response payload,
representing the newly invited member.

### List Members

Retrieve a list of all members for an Encore application.

**Method**: `GET` <br/>
**Path**: `/api/apps/${APP_ID}/members`

#### Path Parameters

| Parameter  | Description                                    |
| ---------- | ---------------------------------------------- |
| **APP_ID** | The id of the Encore application.             |

#### Response

The API responds with a 2xx status code on success.

On success it returns an array of **Member** objects as its JSON response payload,
representing all current members and pending invites for the application.

```typescript
type Response = Member[];
```

### Update Member Role

Update the role of an existing member.

**Method**: `PUT` <br/>
**Path**: `/api/apps/${APP_ID}/members`

#### Path Parameters

| Parameter  | Description                                    |
| ---------- | ---------------------------------------------- |
| **APP_ID** | The id of the Encore application.             |

#### JSON Request Body

```typescript
{
  // The email address of the member to update
  "email": string;

  // The new role to assign to the member
  "role": "owner" | "reader" | "writer" | "none";
}
```

#### Response

The API responds with a 2xx status code on successful update.

#### Error Cases

- **403 Forbidden**: Insufficient permissions to manage members
- **409 Conflict**: Attempting to remove the last owner (error detail: "last_owner")
- **404 Not Found**: Member not found
