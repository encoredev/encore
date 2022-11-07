---
seotitle: How to use Firebase Auth for your backend application
seodesc: Learn how to use Firebase Auth for user authentication in your backend application. In this guide we show you how to integrate your Go backend with Firebase Auth.
title: Use Firebase Auth with your app
---

Encore's [authentication support](/docs/concepts/auth) provides a simple yet powerful
way of dealing with various authentication scenarios.

<a href="https://firebase.google.com/docs/auth" target="_blank" rel="nofollow">Firebase Authentication</a>
{" "}is a common solution for quickly setting up a user store and simplifying social logins.

Encore makes it really easy to integrate with Firebase Authentication on the backend.

## Set up auth handler

First, install two modules:

```shell
$ go get firebase.google.com/go/v4 go4.org/syncutil
```

Next it's time to define your [authentication handler](/docs/concepts/auth).
It can live in whatever service you'd like, but it's usually easiest
to create a designated `user` service.

Create the `user/user.go` file and add the following skeleton code:

```go
package user

import (
	"context"
	"strings"

	"encore.dev/beta/auth"
	firebase "firebase.google.com/go/v4"
	fbauth "firebase.google.com/go/v4/auth"
	"go4.org/syncutil"
	"google.golang.org/api/option"
)

// Data represents the user's data stored in Firebase Auth.
type Data struct {
	// Email is the user's email.
	Email string
	// Name is the user's name.
	Name string
	// Picture is the user's picture URL.
	Picture string
}

// ValidateToken validates an auth token against Firebase Auth.
//encore:authhandler
func ValidateToken(ctx context.Context, token string) (auth.UID, *Data, error) {
    panic("Not Yet Implemented")
}
```

## Initialize Firebase SDK

Next, let's set up the Firebase Auth client. We'll use
&nbsp;<a href="https://pkg.go.dev/go4.org/syncutil#Once" target="_blank" rel="nofollow">`syncutil.Once`</a>&nbsp;
to do it lazily the first time we need it.

Add to the bottom of our file:

```go
var (
	fbAuth    *fbauth.Client
	setupOnce syncutil.Once
)

// setupFB ensures Firebase Auth is setup.
func setupFB() error {
    return setupOnce.Do(func() error {
        opt := option.WithCredentialsJSON([]byte(secrets.FirebasePrivateKey))
        app, err := firebase.NewApp(context.Background(), nil, opt)
        if err == nil {
            fbAuth, err = app.Auth(context.Background())
        }
        return err
    })
}

var secrets struct {
	// FirebasePrivateKey is the JSON credentials for calling Firebase.
	FirebasePrivateKey string
}
```

## Validate token against Firebase

Now that we have the code to initialize Firebase Auth, we can use it from our `ValidateToken` auth handler.
Update the function to look like the following:

```go
func ValidateToken(ctx context.Context, token string) (auth.UID, *Data, error) {
    if err := setupFB(); err != nil {
		return "", nil, err
	}
	tok, err := fbAuth.VerifyIDToken(ctx, token)
	if err != nil {
		return "", nil, err
	}

	email, _ := tok.Claims["email"].(string)
	name, _ := tok.Claims["name"].(string)
	picture, _ := tok.Claims["picture"].(string)
	uid := auth.UID(tok.UID)

	usr := &Data{
		Email:   email,
		Name:    name,
		Picture: picture,
	}
	return uid, usr, nil
}
```

Great! We're done with the code. Now we just need to set up the secret.

## Set Firebase secret credentials

If you haven't already, set up a <a href="https://firebase.google.com" target="_blank" rel="nofollow">Firebase</a> project.

Then, go to **Project settings** and navigate to **Service accounts**.
Select `Go` as the language of choice and click `Generate new private key`.
Download the generated key and take note where it is stored.

Next, store the private key as your firebase secret.
From your terminal (inside your Encore app directory), run:

```shell
$ encore secret set --prod FirebasePrivateKey < /path/to/firebase-private-key.json
Successfully updated production secret FirebasePrivateKey
```

Now you should do the same for the development secret. The most secure way is to
set up a different Firebase project and use that for development.

Depending on your security requirements you could also use the same Firebase project,
but we recommend generating a new private key for development in that case.

Once you have a private key for development, set it similarly to before:

```shell
$ encore secret set --dev FirebasePrivateKey < /path/to/firebase-private-key.json
Successfully updated development secret FirebasePrivateKey
```

That's it! You can now call your Encore application and pass in Firebase tokens.
Encore will run your auth handler and validate the token against Firebase Auth.
