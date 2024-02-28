---
seotitle: How to use Clerk to authenticate users in your backend application
seodesc: Learn how to use Clerk for user authentication in your backend application. In this guide we show you how to integrate your Go backend with Clerk.
title: Use Clerk with your app
---

In this guide you will learn how to set up an Encore [auth handler](/docs/develop/auth#the-auth-handler) that makes use of
[Clerk](https://clerk.com/) in order to add an integrated signup and login experience to your web app.

For all the code and instructions of how to clone and run this example locally, see the [Clerk Example](https://github.com/encoredev/examples/tree/main/clerk) in our examples repo.

## Set up the auth handler

In your Encore app, install the following module:

```shell
$ go get github.com/clerkinc/clerk-sdk-go/clerk
```

Create a folder and naming it `auth`, this is where our authentication related backend code will live.

It's time to define your [auth handler](/docs/concepts/auth). Create `auth/auth.go` and paste the following:

```go
package auth

import (
	"context"
	"encore.dev/beta/auth"
	"encore.dev/beta/errs"
	"github.com/clerkinc/clerk-sdk-go/clerk"
)

var secrets struct {
	ClientSecretKey string
}

// Service struct definition.
// Learn more: encore.dev/docs/primitives/services-and-apis/service-structs
//
//encore:service
type Service struct {
	client clerk.Client
}

// initService is automatically called by Encore when the service starts up.
func initService() (*Service, error) {
	client, err := clerk.NewClient(secrets.ClientSecretKey)
	if err != nil {
		return nil, err
	}
	return &Service{client: client}, nil
}

type UserData struct {
	ID                    string               `json:"id"`
	Username              *string              `json:"username"`
	FirstName             *string              `json:"first_name"`
	LastName              *string              `json:"last_name"`
	ProfileImageURL       string               `json:"profile_image_url"`
	PrimaryEmailAddressID *string              `json:"primary_email_address_id"`
	EmailAddresses        []clerk.EmailAddress `json:"email_addresses"`
}

// The `encore:authhandler` annotation tells Encore to run this function for all
// incoming API call that requires authentication.
// Learn more: encore.dev/docs/develop/auth#the-auth-handler
//
//encore:authhandler
func (s *Service) AuthHandler(ctx context.Context, token string) (auth.UID, *UserData, error) {
	// verify the session
	sessClaims, err := s.client.VerifyToken(token)
	if err != nil {
		return "", nil, &errs.Error{
			Code:    errs.Unauthenticated,
			Message: "invalid token",
		}
	}

	user, err := s.client.Users().Read(sessClaims.Claims.Subject)
	if err != nil {
		return "", nil, &errs.Error{
			Code:    errs.Internal,
			Message: err.Error(),
		}
	}

	userData := &UserData{
		ID:                    user.ID,
		Username:              user.Username,
		FirstName:             user.FirstName,
		LastName:              user.LastName,
		ProfileImageURL:       user.ProfileImageURL,
		PrimaryEmailAddressID: user.PrimaryEmailAddressID,
		EmailAddresses:        user.EmailAddresses,
	}

	return auth.UID(user.ID), userData, nil
}
```

## Clerk credentials

Create a Clerk account if you haven't already. Then, in the Clerk dashboard, create a new applications.

Next, go to the *API Keys* page for your app. Copy one of the "Secret keys" (the "Publishable Key" will be used by your frontend).

The `Secret key` is sensitive and should not be hardcoded in your code/config. Instead, you should store that as an [Encore secret](/docs/primitives/secrets).

From your terminal (inside your Encore app directory), run:

```shell
$ encore secret set --prod ClientSecretKey
```

Now you should do the same for the development secret. The most secure way is to create another secret key (Clerk allows you to have multiple).
Once you have a client secret for development, set it similarly to before:

```shell
$ encore secret set --dev ClientSecretKey
```

## Frontend

Clerk offers a [React SDK](https://clerk.com/docs/references/react/overview) for the frontend which makes it really simple to integrate 
a login/signup flow inside your web app as well as getting the token required to communicate with your Encore backend. 

You can use the `useAuth` hook from `@clerk/clerk-react` to get the token and send it to your backend.

```tsx
import { useAuth } from '@clerk/clerk-react';
 
export default function ExternalDataPage() {
  const { getToken, isLoaded, isSignedIn } = useAuth();
 
  if (!isLoaded) {
    // Handle loading state however you like
    return <div>Loading...</div>;
  }
 
  if (!isSignedIn) {
    // Handle signed out state however you like
    return <div>Sign in to view this page</div>;
  }
 
  const fetchDataFromExternalResource = async () => {
    const token = await getToken();
    // Use token to send to Encore backend when fetching data
    return data;
  }
 
  return <div>...</div>;
}
```

For a fully working backend + frontend example see the [Clerk Example](https://github.com/encoredev/examples/tree/main/clerk) in our examples repo.
