package contextholder

import (
	"firebase.google.com/go/v4/auth"
)

type firebaseTokenContextKeyType struct{}

// FirebaseTokenContextKey is the internal key used to store the Firebase
// token in context.
var FirebaseTokenContextKey firebaseTokenContextKeyType = struct{}{}

// FirebaseTokenHolder is a container of both a decoded and raw Firebase token.
type FirebaseTokenHolder struct {
	// Token is the decoded Firebase token.
	Token *auth.Token

	// RawToken is the raw JWT token string.
	RawToken string
}
